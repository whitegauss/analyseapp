"""Analysis type registry (PDR.md section 9's "type -> handler" pattern,
applied within the Python worker). Adding a new analysis type means adding a
new file here and registering it with @register("name") -- nothing else
needs to change.
"""

from typing import Any, Callable

from app.schemas import DataSeries

AnalysisFunc = Callable[[DataSeries, dict[str, Any]], dict[str, Any]]

REGISTRY: dict[str, AnalysisFunc] = {}


def register(name: str):
    def decorator(fn: AnalysisFunc) -> AnalysisFunc:
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


def run(type_: str, data: DataSeries, params: dict[str, Any]) -> dict[str, Any]:
    fn = REGISTRY.get(type_)
    if fn is None:
        raise UnknownAnalysisTypeError(f"unsupported analysis type: {type_}")
    return fn(data, params)


# Import submodules so their @register decorators run.
from app.analysis import linear_regression  # noqa: E402,F401
