"""Request validation (app/schemas.py): what reaches an analysis at all."""

import pytest
from pydantic import ValidationError

from app.schemas import AnalysisRequest, DataSeries


@pytest.mark.parametrize(
    "columns",
    [
        {"x": [1, 2], "y": [1, 2]},
        {"x": [1, 2], "y": [1, 2], "y_error": [0.1, 0.1]},
        {"x": [1, 2, 3]},
        {"x": [], "y": []},
        # Nothing to compare, so nothing to reject. Pinned as it is: an
        # analysis that needs x and y reports the missing column itself.
        {},
    ],
    ids=["two", "three", "single", "all-empty", "no-columns"],
)
def test_equal_length_columns_are_accepted(columns):
    assert DataSeries(columns=columns).columns == columns


@pytest.mark.parametrize(
    "columns",
    [
        {"x": [1, 2], "y": [1, 2, 3]},
        # Only one of three differs: the check compares every column, not
        # just the first pair.
        {"x": [1, 2], "y": [1, 2], "y_error": [0.1]},
    ],
    ids=["pair", "one-of-three"],
)
def test_unequal_length_columns_are_rejected_with_their_lengths(columns):
    with pytest.raises(ValidationError) as exc:
        DataSeries(columns=columns)

    message = str(exc.value)
    assert "same length" in message
    for name, values in columns.items():
        assert f"'{name}': {len(values)}" in message


def test_non_numeric_values_are_rejected():
    with pytest.raises(ValidationError):
        DataSeries(columns={"x": [1, "two"]})


def test_numeric_strings_are_coerced_to_floats():
    # Pydantic's default (lax) mode. Pinned so a switch to strict mode is a
    # visible decision: the Go API forwards raw_data as the frontend saved
    # it, and older rows may hold numbers as strings.
    assert DataSeries(columns={"x": ["1.5", 2]}).columns == {"x": [1.5, 2.0]}


def test_params_default_to_an_empty_dict():
    req = AnalysisRequest(type="linear_regression", data={"columns": {"x": [1], "y": [1]}})

    assert req.params == {}


def test_params_default_is_not_shared_between_requests():
    first = AnalysisRequest(type="t", data={"columns": {}})
    first.params["x_log"] = True

    assert AnalysisRequest(type="t", data={"columns": {}}).params == {}


@pytest.mark.parametrize("missing", ["type", "data"])
def test_type_and_data_are_required(missing):
    fields = {"type": "linear_regression", "data": {"columns": {}}}
    del fields[missing]

    with pytest.raises(ValidationError, match=missing):
        AnalysisRequest(**fields)
