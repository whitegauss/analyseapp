"""Analysis type registry (PDR.md section 9's "type -> handler" pattern,
applied within the Python worker). Adding a new analysis type means adding a
new file here and registering it with @register("name") -- nothing else
needs to change.
"""

from collections.abc import Callable
from typing import Any

from app.schemas import DataSeries

AnalysisFunc = Callable[[DataSeries, dict[str, Any]], dict[str, Any]]

REGISTRY: dict[str, AnalysisFunc] = {}


def register(name: str):
    """Decorator to register an analysis function in the global registry.

    Usage: @register("analysis_type_name") above an analysis function that
    takes (DataSeries, dict) and returns a result dict.
    """

    def decorator(fn: AnalysisFunc) -> AnalysisFunc:
        """Register the decorated function in the analysis registry."""
        REGISTRY[name] = fn
        return fn

    return decorator


class AnalysisError(Exception):
    """Base error for analysis failures; carries a machine-readable code for
    the response envelope's error.code field."""

    code = "analysis_failed"


class UnknownAnalysisTypeError(AnalysisError):
    code = "unknown_analysis_type"


class MissingColumnError(AnalysisError):
    code = "missing_column"

    def __init__(self, column: str):
        self.column = column
        super().__init__(f"required column '{column}' is missing")


class InsufficientDataError(AnalysisError):
    code = "insufficient_data"


class DegenerateInputError(AnalysisError):
    """The data is well-formed but cannot be fitted: a non-finite value, a
    non-positive uncertainty, or an x column with no spread. These come from
    what the client sent, so they are a 400 -- reaching numpy instead produced
    a bare LinAlgError/ValueError, which is not an AnalysisError and so left
    the envelope entirely and surfaced as a 500 (KAN-57)."""

    code = "degenerate_input"


def run(type_: str, data: DataSeries, params: dict[str, Any]) -> dict[str, Any]:
    """Look up and execute the analysis function for the given type.

    Dispatches to the registered handler for the requested analysis type,
    passing the data and parameters, and returns the analysis result.
    """
    fn = REGISTRY.get(type_)
    if fn is None:
        raise UnknownAnalysisTypeError(f"unsupported analysis type: {type_}")
    return fn(data, params)


# Import submodules so their @register decorators run.
from app.analysis import linear_regression  # noqa: E402,F401
