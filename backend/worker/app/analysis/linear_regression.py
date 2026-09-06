"""Ordinary (or inverse-variance weighted, if a y_error column is present)
least-squares linear regression: y = slope * x + intercept.

`params.x_log` / `params.y_log` (bool, default False) fit the line to
log10(x) and/or log10(y) instead -- a semi-log fit (one of the two) turns an
exponential relationship into a line, a log-log fit (both) turns a power law
into one. `slope`/`intercept` (and everything derived from them) then
describe the fit in that transformed space, not the original x/y space --
the caller is expected to already know which mode it asked for and interpret
(or back-transform, e.g. for plotting) accordingly.
"""

from typing import Any

import numpy as np

from app.analysis import (
    DegenerateInputError,
    InsufficientDataError,
    MissingColumnError,
    register,
)
from app.schemas import DataSeries


def _require_column(data: DataSeries, name: str) -> list[float]:
    """Extract a required column from the data series, raising an error if missing."""
    values = data.columns.get(name)
    if values is None:
        raise MissingColumnError(name)
    return values


def _require_finite(name: str, values: np.ndarray) -> None:
    """Reject NaN/inf in a column.

    numpy does not raise on these: they flow into the least-squares matrices
    and come back either as a bare LinAlgError ("SVD did not converge"),
    which is not an AnalysisError and so escapes the envelope as a 500, or --
    for y -- as a result full of NaN that only fails later, when the response
    cannot be serialised to JSON (KAN-57).
    """
    if not np.isfinite(values).all():
        raise DegenerateInputError(f"column '{name}' must contain only finite numbers")


@register("linear_regression")
def linear_regression(data: DataSeries, params: dict[str, Any]) -> dict[str, Any]:
    """Perform ordinary or weighted least-squares linear regression.

    Fits y = slope * x + intercept, optionally in log-transformed space if
    params.x_log or params.y_log are True. If a y_error column is present,
    performs inverse-variance weighted regression. Returns the fitted parameters,
    their standard errors, R-squared, predicted values, and residuals.
    """
    x = np.array(_require_column(data, "x"), dtype=float)
    y = np.array(_require_column(data, "y"), dtype=float)

    y_error_col = data.columns.get("y_error")
    y_error = np.array(y_error_col, dtype=float) if y_error_col is not None else None

    # Validated before the log filter below, which compares with `> 0` and so
    # would silently drop a NaN as if it were a non-positive point rather than
    # report it.
    _require_finite("x", x)
    _require_finite("y", y)
    if y_error is not None:
        _require_finite("y_error", y_error)
        # A measurement's standard deviation cannot be zero or negative. Zero
        # becomes an infinite weight and takes the fit down with it; a
        # negative one passed silently, because the sign cancels in the
        # least-squares solution -- so a flipped sign or a mis-picked column
        # produced a plausible-looking answer from meaningless input
        # (KAN-60).
        if not (y_error > 0).all():
            raise DegenerateInputError(
                "column 'y_error' must be strictly positive; "
                "an uncertainty of zero or less is not a measurement error"
            )

    x_log = bool(params.get("x_log", False))
    y_log = bool(params.get("y_log", False))

    # log10 is undefined for zero/negative values, and such points wouldn't
    # be visible on a log-scaled axis either -- drop them from the fit
    # rather than erroring, the same way they'd silently drop out of a log
    # plot.
    keep = np.ones(len(x), dtype=bool)
    if x_log:
        keep &= x > 0
    if y_log:
        keep &= y > 0
    if not keep.all():
        x = x[keep]
        y = y[keep]
        if y_error is not None:
            y_error = y_error[keep]

    if len(x) < 2:
        # This guard is at function-body level, so it catches a short input
        # whether or not a log scale was asked for. Naming log unconditionally
        # sent callers looking for a log setting they never touched, and
        # "with positive values" is not a constraint a linear fit has at all
        # (KAN-59).
        if x_log or y_log:
            raise InsufficientDataError(
                "at least 2 data points with positive values are required for a log-scale fit"
            )
        raise InsufficientDataError("at least 2 data points are required")

    weights = None
    if y_error is not None:
        effective_error = y_error
        if y_log:
            # Error propagation through y -> log10(y): d(log10 y)/dy is
            # 1/(y * ln 10), so sigma_log10(y) ~= sigma_y / (y * ln 10).
            effective_error = y_error / (np.abs(y) * np.log(10))
        # polyfit's `w` is applied to the residuals, so 1/stderr (not
        # 1/variance) gives the standard weighted-least-squares result.
        weights = 1.0 / effective_error

    x_fit = np.log10(x) if x_log else x
    y_fit = np.log10(y) if y_log else y

    # Every x the same means there is no line to fit -- the least-squares
    # matrix is singular, and numpy says so with a bare LinAlgError
    # ("Singular matrix") that escapes the envelope as a 500 (KAN-57).
    if len(np.unique(x_fit)) < 2:
        raise DegenerateInputError(
            "column 'x' must contain at least two distinct values; "
            "a line cannot be fitted through a single x"
        )

    # np.polyfit needs more points than the polynomial order to scale the
    # covariance matrix, so exactly two of them cannot produce standard
    # errors -- with cov=True it raises a bare ValueError instead (KAN-57).
    # The line itself is perfectly well defined by two points though, and
    # refusing to draw it would be less useful than saying the uncertainty is
    # unknown, so the fit is returned with null standard errors.
    can_estimate_error = len(x_fit) > 2
    if can_estimate_error:
        (slope, intercept), cov = np.polyfit(x_fit, y_fit, deg=1, w=weights, cov=True)
        slope_stderr, intercept_stderr = (float(v) for v in np.sqrt(np.diag(cov)))
    else:
        slope, intercept = np.polyfit(x_fit, y_fit, deg=1, w=weights)
        slope_stderr = intercept_stderr = None

    predicted = slope * x_fit + intercept
    residuals = y_fit - predicted
    ss_res = float(np.sum(residuals**2))
    ss_tot = float(np.sum((y_fit - np.mean(y_fit)) ** 2))
    # Three cases, spelled out rather than written as a single
    # `if ss_tot > 0 else 1.0`. That form tests a negation: a NaN ss_tot
    # (a NaN or inf reached y, and numpy propagates it here rather than
    # raising) fails `> 0` just like a zero one does, so it took the
    # else branch and reported a perfect fit for garbage (KAN-58).
    if np.isnan(ss_tot):
        # The fit itself is non-finite; slope and the residuals are already
        # NaN, and R-squared is no more knowable than they are.
        r_squared = float("nan")
    elif ss_tot > 0:
        # An ss_tot of inf lands here: the y values are so spread out that
        # their squares overflow. Then ss_res is either inf too (giving NaN,
        # which is honest) or small enough to be finite, which for a spread
        # that large means the points really are on the line -- 1.0.
        r_squared = 1.0 - ss_res / ss_tot
    else:
        # ss_tot == 0: every y is the same value, so the fit is the
        # horizontal line through all of them and the residuals are zero.
        r_squared = 1.0

    return {
        "slope": float(slope),
        "intercept": float(intercept),
        "slope_stderr": slope_stderr,
        "intercept_stderr": intercept_stderr,
        "r_squared": r_squared,
        "predicted_y": predicted.tolist(),
        "residuals": residuals.tolist(),
        "weighted": weights is not None,
        "x_log": x_log,
        "y_log": y_log,
    }
