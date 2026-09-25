"""Nonlinear least-squares fit of a hand-typed model, e.g.
``A*exp(-x/tau) + C`` (KAN-29).

`params.formula` (required) is the right-hand side of ``y = ...`` in the
grammar of ``app.formula`` -- the same one the frontend's error-propagation
tool speaks. ``x`` is the independent variable; every other name in it is a
parameter to fit, reported in the order it first appears. Constants
(``pi``, ``e``) and functions are not parameters.

`params.initial` (optional) maps parameter names to starting values; any it
leaves out start at 1.0. A nonlinear fit only finds the minimum near where
it starts, so for models like ``A*sin(w*x)`` a sensible starting ``w`` is the
difference between a good fit and a meaningless one.

If a y_error column is present, it is passed to SciPy as `sigma`, weighting
each residual by 1/y_error. As with linear_regression (np.polyfit's default),
the covariance is rescaled by the reduced chi-squared (``absolute_sigma``
off), so the errors act as relative weights and the standard errors reflect
the scatter actually seen.
"""

import warnings
from typing import Any

import numpy as np
from scipy import optimize

from app.analysis import (
    FitFailedError,
    InsufficientDataError,
    InvalidFormulaError,
    InvalidParamsError,
    register,
)
from app.analysis.common import r_squared, read_xy
from app.formula import FormulaError, evaluate, parse_formula
from app.schemas import DataSeries

DEFAULT_INITIAL = 1.0
# Singular-value cut-off for the column-normalised Jacobian; see _jacobian_rank.
_RANK_TOL = 1e-8


def _read_formula(params: dict[str, Any]):
    """Parse params.formula, returning the tree and the parameter names."""
    formula = params.get("formula")
    if not isinstance(formula, str) or not formula.strip():
        raise InvalidParamsError("params.formula is required: the model to fit, e.g. 'a*x^2 + b'")

    try:
        parsed = parse_formula(formula)
    except FormulaError as exc:
        where = f"（{exc.position + 1}文字目）" if exc.position >= 0 else ""
        raise InvalidFormulaError(f"{exc}{where}") from exc

    if "y" in parsed.variables:
        # `y` is the measured column, not something to fit. Treating it as a
        # parameter would fit a constant named y -- a plausible-looking
        # answer to a question nobody asked.
        raise InvalidFormulaError(
            "y は測定値なので式の中では使えません。右辺だけを書いてください（例: a*x + b）"
        )
    names = [v for v in parsed.variables if v != "x"]
    if not names:
        raise InvalidFormulaError(
            "フィットするパラメータがありません（x 以外の変数が式にありません）"
        )
    return formula, parsed.ast, names


def _read_initial(params: dict[str, Any], names: list[str]) -> list[float]:
    """Starting values for each parameter, in `names` order."""
    initial = params.get("initial", {})
    if initial is None:
        initial = {}
    if not isinstance(initial, dict):
        raise InvalidParamsError("params.initial must be an object of parameter name -> number")

    # An unknown name is almost always a typo, and silently starting the
    # intended parameter at the default instead is exactly the failure the
    # user was trying to avoid by giving one.
    unknown = sorted(set(initial) - set(names))
    if unknown:
        raise InvalidParamsError(
            f"params.initial names {', '.join(unknown)}, which the formula does not have; "
            f"its parameters are {', '.join(names)}"
        )

    p0 = []
    for name in names:
        value = initial.get(name, DEFAULT_INITIAL)
        # bool is an int in Python; true as a starting value is a mistake.
        if isinstance(value, bool) or not isinstance(value, int | float) or not np.isfinite(value):
            raise InvalidParamsError(f"params.initial.{name} must be a finite number")
        p0.append(float(value))
    return p0


def _jacobian_rank(model, x: np.ndarray, p: np.ndarray) -> int:
    """Numerical rank of d(model)/d(p) at p, by central differences."""
    columns = []
    for j in range(len(p)):
        step = 1e-6 * max(1.0, abs(p[j]))
        up, down = p.copy(), p.copy()
        up[j] += step
        down[j] -= step
        columns.append((model(x, *up) - model(x, *down)) / (2 * step))
    jacobian = np.column_stack(columns)
    if not np.isfinite(jacobian).all():
        return 0
    # Each column scaled to unit length, so a parameter is not judged
    # redundant just for being on a different scale from the others (an
    # amplitude of 1e3 next to a rate of 1e-3). A column of zeros -- a
    # parameter the model does not respond to at all -- stays zero.
    norms = np.linalg.norm(jacobian, axis=0)
    jacobian = jacobian / np.where(norms > 0, norms, 1.0)
    # numpy's default tolerance sits near machine precision, below the
    # ~1e-10 noise that central differences leave behind, so an exactly
    # redundant pair (a*b*x) would still count as full rank.
    return int(np.linalg.matrix_rank(jacobian, tol=_RANK_TOL))


@register("curve_fit")
def curve_fit(data: DataSeries, params: dict[str, Any]) -> dict[str, Any]:
    """Fit y = f(x; parameters) for a user-supplied formula f.

    Returns each parameter's value and standard error, R-squared, predicted
    values and residuals. A fit that does not converge, or converges
    somewhere the model is not finite, is reported as fit_failed rather than
    returned.
    """
    formula, ast, names = _read_formula(params)
    p0 = _read_initial(params, names)
    x, y, y_error = read_xy(data)

    if len(x) < len(names):
        raise InsufficientDataError(
            f"at least {len(names)} data points are required to fit {len(names)} parameters"
        )

    def model(xv: np.ndarray, *p: float) -> np.ndarray:
        """Evaluate the formula over the whole x column for parameters p."""
        values = dict(zip(names, p, strict=True))
        values["x"] = xv
        # A formula without x (a constant model) evaluates to a scalar;
        # SciPy needs one prediction per point.
        return np.broadcast_to(np.asarray(evaluate(ast, values), dtype=float), xv.shape)

    # Checked up front so the message can say what to do about it. Left to
    # SciPy, a model that is nan at the starting point (1/(x-a) with every
    # parameter at 1.0 and an x of 1, say) fails with nothing that points at
    # the starting values.
    if not np.isfinite(model(x, *p0)).all():
        raise FitFailedError(
            "初期値で式を計算できない点があります（0 での割り算・負の数の平方根など）。"
            "params.initial で初期値を指定してください"
        )

    try:
        with warnings.catch_warnings():
            # "Covariance of the parameters could not be estimated" -- handled
            # below by reporting the standard error as unknown.
            warnings.simplefilter("ignore", optimize.OptimizeWarning)
            popt, pcov = optimize.curve_fit(model, x, y, p0=p0, sigma=y_error, absolute_sigma=False)
    except RuntimeError as exc:
        # SciPy's way of saying the iteration limit was reached.
        raise FitFailedError(
            f"フィットが収束しませんでした。初期値（params.initial）を見直してください: {exc}"
        ) from exc

    predicted = model(x, *popt)
    if not (np.isfinite(popt).all() and np.isfinite(predicted).all()):
        raise FitFailedError(
            "フィットの結果、式を計算できない値になりました。初期値（params.initial）を見直してください"
        )

    # Standard errors are unknown, rather than a number, in two cases.
    #
    # With as many parameters as points the curve passes through every one
    # of them and there is no scatter left to estimate an error from (the
    # same reason linear_regression returns null errors for 2 points).
    #
    # A parameter the data cannot pin down -- a and b in a*b*x, where only
    # the product matters -- makes the Jacobian rank-deficient. SciPy inverts
    # it with a pseudo-inverse and so still returns a finite covariance, and
    # the standard errors that come out of it are numbers with no meaning.
    if len(x) > len(names) and _jacobian_rank(model, x, popt) == len(names):
        variances = np.diag(pcov)
    else:
        variances = np.full(len(names), np.nan)
    stderrs = [float(np.sqrt(v)) if np.isfinite(v) and v >= 0 else None for v in variances]

    return {
        "formula": formula,
        "parameters": [
            {"name": name, "value": float(value), "stderr": stderr}
            for name, value, stderr in zip(names, popt, stderrs, strict=True)
        ],
        "r_squared": r_squared(y, predicted),
        "predicted_y": predicted.tolist(),
        "residuals": (y - predicted).tolist(),
        "weighted": y_error is not None,
    }
