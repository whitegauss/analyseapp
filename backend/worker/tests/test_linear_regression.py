import math

import numpy as np
import pytest

from app.analysis import (
    AnalysisError,
    InsufficientDataError,
    MissingColumnError,
    run,
)
from app.schemas import AnalysisRequest, DataSeries
from tests.conftest import fit


def test_exact_line_no_noise():
    # y = 2x + 1 exactly
    x = [0.0, 1.0, 2.0, 3.0, 4.0]
    y = [1.0, 3.0, 5.0, 7.0, 9.0]

    result = fit(x, y)

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

    result = fit(x, y)

    assert result["slope"] == pytest.approx(2.0, abs=0.2)
    assert 0.0 < result["r_squared"] < 1.0


def test_weighted_regression_uses_y_error():
    x = [0.0, 1.0, 2.0, 3.0]
    y = [1.0, 3.0, 5.0, 100.0]  # last point is an outlier
    y_error = [0.01, 0.01, 0.01, 1000.0]  # ...but we barely trust it

    result = fit(x, y, y_error=y_error)

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

    result = fit(x, y, y_log=True)

    assert result["x_log"] is False
    assert result["y_log"] is True
    assert result["slope"] == pytest.approx(0.5)
    assert result["intercept"] == pytest.approx(np.log10(3.0))
    assert result["r_squared"] == pytest.approx(1.0)


def test_x_log_recovers_logarithmic_relationship():
    # y = 2 * log10(x) + 1
    x = [1.0, 10.0, 100.0, 1000.0]
    y = [2.0 * np.log10(xi) + 1.0 for xi in x]

    result = fit(x, y, x_log=True)

    assert result["x_log"] is True
    assert result["y_log"] is False
    assert result["slope"] == pytest.approx(2.0)
    assert result["intercept"] == pytest.approx(1.0)


def test_log_log_recovers_power_law():
    # y = 5 * x^3  =>  log10(y) = 3*log10(x) + log10(5)
    x = [1.0, 2.0, 4.0, 8.0, 16.0]
    y = [5.0 * xi**3 for xi in x]

    result = fit(x, y, x_log=True, y_log=True)

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

    result = fit(x, y, x_log=True)

    # log10([1, 10, 100, 1000]) = [0, 1, 2, 3], and y = [1, 2, 3, 4]: y = x_fit + 1.
    assert result["slope"] == pytest.approx(1.0)
    assert result["intercept"] == pytest.approx(1.0)


def test_log_fit_raises_insufficient_data_when_too_few_points_remain():
    x = [-1.0, -2.0, 10.0]  # only one positive point after filtering
    y = [1.0, 2.0, 3.0]

    with pytest.raises(InsufficientDataError):
        fit(x, y, x_log=True)


def test_y_log_weighted_regression_propagates_error_through_log():
    # y = 10^x exactly, except one heavily-distrusted outlier.
    x = [0.0, 1.0, 2.0, 3.0]
    y = [1.0, 10.0, 100.0, 500.0]  # last point is off (should be 1000)
    y_error = [0.001, 0.001, 0.001, 5000.0]  # ...but we barely trust it

    result = fit(x, y, y_error=y_error, y_log=True)

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


# --------------------------------------------------------------------------
# 境界値テスト (KAN-45)
#
# ここから下は「あるべき挙動」ではなく、実際に動かして観測した **現状の挙動**
# を固定している。numpy が投げる素の LinAlgError / ValueError は
# AnalysisError ではないため envelope のハンドラに捕まらず HTTP 500 になる。
# 挙動の修正は本 PR には含めず、別課題で扱う。
# --------------------------------------------------------------------------

INSUFFICIENT_DATA_MESSAGE = (
    "at least 2 data points with positive values are required for a log-scale fit"
)
TOO_FEW_POINTS_FOR_COV = (
    "the number of data points must exceed order to scale the covariance matrix"
)


@pytest.mark.parametrize(
    "x, y",
    [([1.0], [1.0]), ([], [])],
    ids=["one-point", "no-points"],
)
def test_fewer_than_two_points_without_log_raises_insufficient_data(x, y):
    # `len(x) < 2` のガードは log 分岐の外（関数本体レベル）にあるので、log を
    # 使わない呼び出しでも必ずここで 400 になる。未処理例外にはならない。
    # log フィルタ後に点が減るケースは
    # test_log_fit_raises_insufficient_data_when_too_few_points_remain が担当。
    with pytest.raises(InsufficientDataError) as excinfo:
        fit(x, y)

    # BUG(現状固定): log を要求していない呼び出しにも "log-scale fit" と名乗る
    # メッセージが返り、log 専用の制約だと誤解させる。文言修正は別課題。
    assert str(excinfo.value) == INSUFFICIENT_DATA_MESSAGE


def test_exactly_two_points_raises_bare_value_error():
    # polyfit(cov=True) は共分散をスケールするのに order より多い点数を要求する
    # ため、2 点ちょうどの fit は numpy の中で落ちる。
    with pytest.raises(ValueError) as excinfo:
        fit([1.0, 2.0], [1.0, 2.0])

    assert type(excinfo.value) is ValueError
    assert TOO_FEW_POINTS_FOR_COV in str(excinfo.value)
    # BUG(現状固定): AnalysisError ではないので envelope を通らず 500 になる。
    assert not isinstance(excinfo.value, AnalysisError)


@pytest.mark.parametrize(
    "x, y, y_error, expected_message",
    [
        ([1.0, 1.0, 1.0], [1.0, 2.0, 3.0], None, "Singular matrix"),
        ([1.0, 1.0], [1.0, 2.0], None, "Singular matrix"),
        ([0.0, 1.0, 2.0], [1.0, 3.0, 5.0], [0.1, 0.0, 0.1], "SVD did not converge"),
        ([0.0, 1.0, 2.0], [1.0, 3.0, 5.0], [0.0, 0.0, 0.0], "SVD did not converge"),
        ([1.0, float("nan"), 3.0], [1.0, 2.0, 3.0], None, "SVD did not converge"),
        ([1.0, float("inf"), 3.0], [1.0, 2.0, 3.0], None, "SVD did not converge"),
        ([1.0, 2.0, 3.0], [1.0, 2.0, 3.0], [0.1, float("nan"), 0.1], "SVD did not converge"),
    ],
    ids=[
        "x-zero-variance",
        "x-zero-variance-two-points",
        "y-error-contains-zero",
        "y-error-all-zero",
        "x-contains-nan",
        "x-contains-inf",
        "y-error-contains-nan",
    ],
)
def test_degenerate_input_raises_bare_linalg_error(x, y, y_error, expected_message):
    # 分散ゼロは特異行列、y_error のゼロは 1/0 = inf の重み、NaN/inf はそのまま
    # 行列に入り、いずれも numpy の LinAlgError になる。
    with pytest.raises(np.linalg.LinAlgError) as excinfo:
        fit(x, y, y_error=y_error)

    assert expected_message in str(excinfo.value)
    # BUG(現状固定): AnalysisError ではないので envelope を通らず 500 になる。
    assert not isinstance(excinfo.value, AnalysisError)


@pytest.mark.parametrize("bad_y", [float("nan"), float("inf")], ids=["nan", "inf"])
def test_non_finite_y_returns_a_nan_result_instead_of_raising(bad_y):
    # BUG(現状固定): y 側の NaN/inf は例外にならず NaN 入りの結果が返る。JSON に
    # NaN は書けないので、HTTP 層では応答の直列化で 500 になる
    # (test_main.py の test_analyze_nan_result_fails_during_serialization 参照)。
    result = fit([1.0, 2.0, 3.0], [1.0, bad_y, 3.0])

    assert math.isnan(result["slope"])
    assert math.isnan(result["intercept"])
    assert math.isnan(result["slope_stderr"])
    assert all(math.isnan(v) for v in result["predicted_y"])
    assert all(math.isnan(v) for v in result["residuals"])
    # ss_tot も NaN になり `ss_tot > 0` が False -> :80 の else 分岐の 1.0。
    assert result["r_squared"] == 1.0


def test_nan_x_with_x_log_is_filtered_out_leaving_too_few_points():
    # `nan > 0` は False なので log フィルタが NaN を巻き添えで落とし、残り 2 点
    # では cov=True が成立しない。LinAlgError ではなく素の ValueError になる。
    with pytest.raises(ValueError) as excinfo:
        fit([1.0, float("nan"), 3.0], [1.0, 2.0, 3.0], x_log=True)

    assert TOO_FEW_POINTS_FOR_COV in str(excinfo.value)


def test_y_log_error_propagation_never_divides_by_zero():
    # 誤差伝播 (:65) は y で割るため、y=0 が残っていればここでも inf の重みが
    # でき、y_error にゼロを混ぜたときと同じ LinAlgError になりうる。実際には
    # log フィルタ (:46-52) が先に y=0 を落とすのでゼロ除算には至らない。
    result = fit(
        [0.0, 1.0, 2.0, 3.0],
        [0.0, 10.0, 100.0, 1000.0],
        y_error=[1.0, 1.0, 1.0, 1.0],
        y_log=True,
    )

    assert result["weighted"] is True
    assert result["slope"] == pytest.approx(1.0)
    assert all(math.isfinite(v) for v in result["predicted_y"])


def test_negative_y_error_is_accepted_and_fits_normally():
    # 物理的には無意味な入力だが、重みの符号は最小二乗解に効かないので
    # バリデーションされずそのまま通る（現状固定）。
    result = fit([0.0, 1.0, 2.0], [1.0, 3.0, 5.0], y_error=[0.1, -0.1, 0.1])

    assert result["weighted"] is True
    assert result["slope"] == pytest.approx(2.0)
    assert result["intercept"] == pytest.approx(1.0)


def test_constant_y_falls_back_to_r_squared_one():
    # y が全て同値 -> ss_tot == 0 -> :80 の `else 1.0` フォールバック。
    result = fit([1.0, 2.0, 3.0], [5.0, 5.0, 5.0])

    assert result["slope"] == pytest.approx(0.0, abs=1e-12)
    assert result["intercept"] == pytest.approx(5.0)
    assert result["r_squared"] == 1.0
