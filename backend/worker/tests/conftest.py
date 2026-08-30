"""Shared helpers for the worker test-suite.

`series()` / `fit()` collapse the DataSeries-building boilerplate that every
linear_regression case repeats, so a test body is mostly its assertions.
"""

from typing import Any

from app.analysis import run
from app.schemas import DataSeries


def series(x: list[float], y: list[float], y_error: list[float] | None = None) -> DataSeries:
    columns: dict[str, list[float]] = {"x": list(x), "y": list(y)}
    if y_error is not None:
        columns["y_error"] = list(y_error)
    return DataSeries(columns=columns)


def fit(
    x: list[float],
    y: list[float],
    y_error: list[float] | None = None,
    **params: Any,
) -> dict[str, Any]:
    """Run linear_regression over the given columns, returning its result dict."""
    return run("linear_regression", series(x, y, y_error), params)
