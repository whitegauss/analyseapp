"""Ordinary (or inverse-variance weighted, if a y_error column is present)
least-squares linear regression: y = slope * x + intercept.
"""

from typing import Any

import numpy as np

from app.analysis import MissingColumnError, register
from app.schemas import DataSeries


def _require_column(data: DataSeries, name: str) -> list[float]:
    values = data.columns.get(name)
    if values is None:
        raise MissingColumnError(name)
    return values


@register("linear_regression")
def linear_regression(data: DataSeries, params: dict[str, Any]) -> dict[str, Any]:
    x = np.array(_require_column(data, "x"), dtype=float)
    y = np.array(_require_column(data, "y"), dtype=float)

    y_error = data.columns.get("y_error")
    weights = None
    if y_error is not None:
        # Inverse-variance weighting; polyfit's `w` is applied to the
        # residuals, so 1/stderr (not 1/variance) gives the standard
        # weighted-least-squares result.
        weights = 1.0 / np.array(y_error, dtype=float)

    (slope, intercept), cov = np.polyfit(x, y, deg=1, w=weights, cov=True)
    slope_stderr, intercept_stderr = np.sqrt(np.diag(cov))

    predicted_y = slope * x + intercept
    residuals = y - predicted_y
    ss_res = float(np.sum(residuals**2))
    ss_tot = float(np.sum((y - np.mean(y)) ** 2))
    r_squared = 1.0 - ss_res / ss_tot if ss_tot > 0 else 1.0

    return {
        "slope": float(slope),
        "intercept": float(intercept),
        "slope_stderr": float(slope_stderr),
        "intercept_stderr": float(intercept_stderr),
        "r_squared": r_squared,
        "predicted_y": predicted_y.tolist(),
        "residuals": residuals.tolist(),
        "weighted": weights is not None,
    }
