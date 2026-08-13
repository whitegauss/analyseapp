import pytest

from app.analysis import MissingColumnError, run
from app.schemas import AnalysisRequest, DataSeries


def test_exact_line_no_noise():
    # y = 2x + 1 exactly
    x = [0.0, 1.0, 2.0, 3.0, 4.0]
    y = [1.0, 3.0, 5.0, 7.0, 9.0]
    data = DataSeries(columns={"x": x, "y": y})

    result = run("linear_regression", data, {})

    assert result["slope"] == pytest.approx(2.0)
    assert result["intercept"] == pytest.approx(1.0)
    assert result["r_squared"] == pytest.approx(1.0)
    assert result["weighted"] is False
    assert result["predicted_y"] == pytest.approx(y)
    assert result["residuals"] == pytest.approx([0.0] * len(x))


def test_noisy_line_r_squared_between_zero_and_one():
    x = [0.0, 1.0, 2.0, 3.0, 4.0]
    y = [1.1, 2.9, 5.2, 6.8, 9.3]
    data = DataSeries(columns={"x": x, "y": y})

    result = run("linear_regression", data, {})

    assert result["slope"] == pytest.approx(2.0, abs=0.2)
    assert 0.0 < result["r_squared"] < 1.0


def test_weighted_regression_uses_y_error():
    x = [0.0, 1.0, 2.0, 3.0]
    y = [1.0, 3.0, 5.0, 100.0]  # last point is an outlier
    y_error = [0.01, 0.01, 0.01, 1000.0]  # ...but we barely trust it
    data = DataSeries(columns={"x": x, "y": y, "y_error": y_error})

    result = run("linear_regression", data, {})

    assert result["weighted"] is True
    # Heavily downweighting the outlier should recover close to y = 2x + 1.
    assert result["slope"] == pytest.approx(2.0, abs=0.1)
    assert result["intercept"] == pytest.approx(1.0, abs=0.1)


def test_missing_required_column_raises():
    data = DataSeries(columns={"x": [1.0, 2.0]})

    with pytest.raises(MissingColumnError):
        run("linear_regression", data, {})


def test_mismatched_column_lengths_rejected_by_schema():
    with pytest.raises(ValueError):
        DataSeries(columns={"x": [1.0, 2.0], "y": [1.0]})


def test_unknown_analysis_type_via_request_schema():
    req = AnalysisRequest(
        type="does_not_exist",
        data=DataSeries(columns={"x": [1.0], "y": [1.0]}),
    )
    from app.analysis import UnknownAnalysisTypeError

    with pytest.raises(UnknownAnalysisTypeError):
        run(req.type, req.data, req.params)
