"""The analysis registry (app/analysis/__init__.py): register, run, and the
error codes that end up as the envelope's error.code."""

import pytest

from app import analysis
from app.analysis import (
    REGISTRY,
    AnalysisError,
    DegenerateInputError,
    FitFailedError,
    InsufficientDataError,
    InvalidFormulaError,
    InvalidParamsError,
    MissingColumnError,
    UnknownAnalysisTypeError,
    register,
    run,
)
from app.schemas import DataSeries

DATA = DataSeries(columns={"x": [1.0, 2.0], "y": [2.0, 4.0]})


@pytest.fixture
def registry():
    """REGISTRY, restored afterwards: it is module-global, so a test that
    registers a fake would otherwise leave it there for every later test."""
    saved = dict(REGISTRY)
    yield REGISTRY
    REGISTRY.clear()
    REGISTRY.update(saved)


def test_the_shipped_analysis_types_are_registered():
    # The Go API and the frontend ask for these by name.
    assert {"linear_regression", "curve_fit"} <= set(REGISTRY)


def test_register_adds_the_function_under_its_name_and_returns_it_unwrapped(registry):
    def fake(data, params):
        return {"called": True}

    decorated = register("fake")(fake)

    assert registry["fake"] is fake
    # Not wrapped, so a registered analysis can still be called directly.
    assert decorated is fake


def test_run_dispatches_to_the_registered_function_with_data_and_params(registry):
    calls = []

    @register("recorder")
    def recorder(data, params):
        calls.append((data, params))
        return {"n": len(data.columns["x"])}

    params = {"x_log": True}
    result = run("recorder", DATA, params)

    assert result == {"n": 2}
    assert calls == [(DATA, params)]


def test_run_returns_the_analysis_result_unchanged():
    result = run("linear_regression", DATA, {})

    assert result["slope"] == pytest.approx(2.0)


def test_run_of_an_unknown_type_names_it():
    with pytest.raises(UnknownAnalysisTypeError, match="nope") as exc:
        run("nope", DATA, {})

    assert exc.value.code == "unknown_analysis_type"


def test_registering_a_name_twice_replaces_the_first(registry):
    # Pinned as it is today: the later registration wins, silently. Whether
    # it should warn instead is a separate decision (KAN-46).
    register("dup")(lambda data, params: "first")
    register("dup")(lambda data, params: "second")

    assert run("dup", DATA, {}) == "second"


@pytest.mark.parametrize(
    "exc_type, code",
    [
        (AnalysisError, "analysis_failed"),
        (UnknownAnalysisTypeError, "unknown_analysis_type"),
        (InsufficientDataError, "insufficient_data"),
        (DegenerateInputError, "degenerate_input"),
        (InvalidParamsError, "invalid_params"),
        (InvalidFormulaError, "invalid_formula"),
        (FitFailedError, "fit_failed"),
    ],
)
def test_each_error_carries_its_envelope_code(exc_type, code):
    # These strings are the API's error.code values; the frontend and the Go
    # API may branch on them, so renaming one is a breaking change.
    assert exc_type.code == code
    assert issubclass(exc_type, AnalysisError)


def test_missing_column_error_names_the_column():
    exc = MissingColumnError("y_error")

    assert exc.code == "missing_column"
    assert exc.column == "y_error"
    assert "y_error" in str(exc)


def test_every_analysis_error_subclass_is_covered_above():
    # A new error type added without a pinned code would fall back to the
    # base class's "analysis_failed" -- legal, but almost never intended.
    subclasses = {
        cls
        for cls in vars(analysis).values()
        if isinstance(cls, type) and issubclass(cls, AnalysisError)
    }
    pinned = {
        AnalysisError,
        UnknownAnalysisTypeError,
        InsufficientDataError,
        DegenerateInputError,
        InvalidParamsError,
        InvalidFormulaError,
        FitFailedError,
        MissingColumnError,
    }
    assert subclasses == pinned
