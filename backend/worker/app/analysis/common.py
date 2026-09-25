"""Input checks and statistics shared by the fitting analyses.

Every fit reads the same x / y / y_error columns and has to reject the same
bad input before numpy or SciPy sees it, so the checks live here once rather
than drifting apart per analysis type.
"""

import numpy as np

from app.analysis import DegenerateInputError, MissingColumnError
from app.schemas import DataSeries


def require_column(data: DataSeries, name: str) -> list[float]:
    """Extract a required column from the data series, raising an error if missing."""
    values = data.columns.get(name)
    if values is None:
        raise MissingColumnError(name)
    return values


def require_finite(name: str, values: np.ndarray) -> None:
    """Reject NaN/inf in a column.

    numpy does not raise on these: they flow into the least-squares matrices
    and come back either as a bare LinAlgError ("SVD did not converge"),
    which is not an AnalysisError and so escapes the envelope as a 500, or --
    for y -- as a result full of NaN that only fails later, when the response
    cannot be serialised to JSON (KAN-57).
    """
    if not np.isfinite(values).all():
        raise DegenerateInputError(f"column '{name}' must contain only finite numbers")


def read_xy(data: DataSeries) -> tuple[np.ndarray, np.ndarray, np.ndarray | None]:
    """Read and validate the x, y and optional y_error columns."""
    x = np.array(require_column(data, "x"), dtype=float)
    y = np.array(require_column(data, "y"), dtype=float)

    y_error_col = data.columns.get("y_error")
    y_error = np.array(y_error_col, dtype=float) if y_error_col is not None else None

    require_finite("x", x)
    require_finite("y", y)
    if y_error is not None:
        require_finite("y_error", y_error)
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
    return x, y, y_error


def r_squared(y: np.ndarray, predicted: np.ndarray) -> float:
    """Coefficient of determination of `predicted` against `y`."""
    residuals = y - predicted
    ss_res = float(np.sum(residuals**2))
    ss_tot = float(np.sum((y - np.mean(y)) ** 2))
    # Three cases, spelled out rather than written as a single
    # `if ss_tot > 0 else 1.0`. That form tests a negation: a NaN ss_tot
    # (a NaN or inf reached y, and numpy propagates it here rather than
    # raising) fails `> 0` just like a zero one does, so it took the
    # else branch and reported a perfect fit for garbage (KAN-58).
    if np.isnan(ss_tot):
        # The fit itself is non-finite; the residuals are already NaN, and
        # R-squared is no more knowable than they are.
        return float("nan")
    if ss_tot > 0:
        # An ss_tot of inf lands here: the y values are so spread out that
        # their squares overflow. Then ss_res is either inf too (giving NaN,
        # which is honest) or small enough to be finite, which for a spread
        # that large means the points really are on the curve -- 1.0.
        return 1.0 - ss_res / ss_tot
    # ss_tot == 0: every y is the same value. A linear fit is then the
    # horizontal line through all of them with zero residuals; reported as a
    # perfect fit.
    return 1.0
