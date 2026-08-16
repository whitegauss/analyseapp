import numpy as np
import pytest

from app.analysis import InsufficientDataError, MissingColumnError, run
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
    assert result["x_log"] is False
    assert result["y_log"] is False


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


def test_y_log_recovers_exponential_relationship():
    # y = 3 * 10^(0.5x)  =>  log10(y) = 0.5x + log10(3)
    x = [0.0, 1.0, 2.0, 3.0, 4.0]
    y = [3.0 * 10 ** (0.5 * xi) for xi in x]
    data = DataSeries(columns={"x": x, "y": y})

    result = run("linear_regression", data, {"y_log": True})

    assert result["x_log"] is False
    assert result["y_log"] is True
    assert result["slope"] == pytest.approx(0.5)
    assert result["intercept"] == pytest.approx(np.log10(3.0))
    assert result["r_squared"] == pytest.approx(1.0)


def test_x_log_recovers_logarithmic_relationship():
    # y = 2 * log10(x) + 1
    x = [1.0, 10.0, 100.0, 1000.0]
    y = [2.0 * np.log10(xi) + 1.0 for xi in x]
    data = DataSeries(columns={"x": x, "y": y})

    result = run("linear_regression", data, {"x_log": True})

    assert result["x_log"] is True
    assert result["y_log"] is False
    assert result["slope"] == pytest.approx(2.0)
    assert result["intercept"] == pytest.approx(1.0)


def test_log_log_recovers_power_law():
    # y = 5 * x^3  =>  log10(y) = 3*log10(x) + log10(5)
    x = [1.0, 2.0, 4.0, 8.0, 16.0]
    y = [5.0 * xi**3 for xi in x]
    data = DataSeries(columns={"x": x, "y": y})

    result = run("linear_regression", data, {"x_log": True, "y_log": True})

    assert result["x_log"] is True
    assert result["y_log"] is True
    assert result["slope"] == pytest.approx(3.0)
    assert result["intercept"] == pytest.approx(np.log10(5.0))
    assert result["r_squared"] == pytest.approx(1.0)


def test_log_fit_drops_non_positive_points_rather_than_erroring():
    # A negative x can't be log10'd -- it should just be excluded from the
    # fit, not blow up the whole request.
    x = [-1.0, 1.0, 10.0, 100.0, 1000.0]
    y = [999.0, 1.0, 2.0, 3.0, 4.0]  # y for x=-1 is nonsense, must be dropped too
    data = DataSeries(columns={"x": x, "y": y})

    result = run("linear_regression", data, {"x_log": True})

    # log10([1, 10, 100, 1000]) = [0, 1, 2, 3], and y = [1, 2, 3, 4]: y = x_fit + 1.
    assert result["slope"] == pytest.approx(1.0)
    assert result["intercept"] == pytest.approx(1.0)


def test_log_fit_raises_insufficient_data_when_too_few_points_remain():
    x = [-1.0, -2.0, 10.0]  # only one positive point after filtering
    y = [1.0, 2.0, 3.0]
    data = DataSeries(columns={"x": x, "y": y})

    with pytest.raises(InsufficientDataError):
        run("linear_regression", data, {"x_log": True})


def test_y_log_weighted_regression_propagates_error_through_log():
    # y = 10^x exactly, except one heavily-distrusted outlier.
    x = [0.0, 1.0, 2.0, 3.0]
    y = [1.0, 10.0, 100.0, 500.0]  # last point is off (should be 1000)
    y_error = [0.001, 0.001, 0.001, 5000.0]  # ...but we barely trust it
    data = DataSeries(columns={"x": x, "y": y, "y_error": y_error})

    result = run("linear_regression", data, {"y_log": True})

    assert result["weighted"] is True
    assert result["slope"] == pytest.approx(1.0, abs=0.05)
    assert result["intercept"] == pytest.approx(0.0, abs=0.05)


def test_unknown_analysis_type_via_request_schema():
    req = AnalysisRequest(
        type="does_not_exist",
        data=DataSeries(columns={"x": [1.0], "y": [1.0]}),
    )
    from app.analysis import UnknownAnalysisTypeError

    with pytest.raises(UnknownAnalysisTypeError):
        run(req.type, req.data, req.params)
