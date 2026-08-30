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

from app.analysis import InsufficientDataError, MissingColumnError, register
from app.schemas import DataSeries


def _require_column(data: DataSeries, name: str) -> list[float]:
    """Extract a required column from the data series, raising an error if missing."""
    values = data.columns.get(name)
    if values is None:
        raise MissingColumnError(name)
    return values


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
        raise InsufficientDataError(
            "at least 2 data points with positive values are required for a log-scale fit"
        )

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

    (slope, intercept), cov = np.polyfit(x_fit, y_fit, deg=1, w=weights, cov=True)
    slope_stderr, intercept_stderr = np.sqrt(np.diag(cov))

    predicted = slope * x_fit + intercept
    residuals = y_fit - predicted
    ss_res = float(np.sum(residuals**2))
    ss_tot = float(np.sum((y_fit - np.mean(y_fit)) ** 2))
    r_squared = 1.0 - ss_res / ss_tot if ss_tot > 0 else 1.0

    return {
        "slope": float(slope),
        "intercept": float(intercept),
        "slope_stderr": float(slope_stderr),
        "intercept_stderr": float(intercept_stderr),
        "r_squared": r_squared,
        "predicted_y": predicted.tolist(),
        "residuals": residuals.tolist(),
        "weighted": weights is not None,
        "x_log": x_log,
        "y_log": y_log,
    }
