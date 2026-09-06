import math

import numpy as np
import pytest

from app.analysis import (
    AnalysisError,
    DegenerateInputError,
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
# 境界値テスト (KAN-45 で追加、KAN-57 / KAN-59 / KAN-60 で期待値を更新)
#
# 縮退した入力はすべて入力段で DegenerateInputError になる。かつては numpy の
# 素の LinAlgError / ValueError がそのまま出ていて、AnalysisError ではないため
# envelope のハンドラに捕まらず HTTP 500 になっていた。
# --------------------------------------------------------------------------

INSUFFICIENT_DATA_MESSAGE = (
    "at least 2 data points with positive values are required for a log-scale fit"
)
NO_LOG_INSUFFICIENT_DATA_MESSAGE = "at least 2 data points are required"


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

    # log を要求していないので log に言及してはいけない。"with positive
    # values" も線形フィットの制約ではない (KAN-59)。
    assert str(excinfo.value) == NO_LOG_INSUFFICIENT_DATA_MESSAGE
    assert "log" not in str(excinfo.value)


def test_exactly_two_points_fits_without_standard_errors():
    # polyfit(cov=True) は共分散をスケールするのに order より多い点数を要求する
    # ため、2 点ちょうどでは標準誤差を出せない。ただし直線自体は 2 点で一意に
    # 定まるので、フィットは返して誤差だけ「不明」とする (KAN-57)。
    result = fit([1.0, 2.0], [1.0, 3.0])

    assert result["slope"] == pytest.approx(2.0)
    assert result["intercept"] == pytest.approx(-1.0)
    assert result["slope_stderr"] is None
    assert result["intercept_stderr"] is None
    # 2 点は必ず直線上に乗るので、残差はゼロ。
    assert result["r_squared"] == pytest.approx(1.0)
    assert result["predicted_y"] == pytest.approx([1.0, 3.0])


def test_three_points_still_report_standard_errors():
    # 2 点の分岐が 3 点以上を巻き込んでいないこと。
    result = fit([0.0, 1.0, 2.0], [1.0, 3.0, 5.2])

    assert result["slope_stderr"] is not None
    assert result["intercept_stderr"] is not None
    assert result["slope_stderr"] > 0


@pytest.mark.parametrize(
    "x, y, y_error, expected_message",
    [
        ([1.0, 1.0, 1.0], [1.0, 2.0, 3.0], None, "column 'x' must contain at least two"),
        ([1.0, 1.0], [1.0, 2.0], None, "column 'x' must contain at least two"),
        ([0.0, 1.0, 2.0], [1.0, 3.0, 5.0], [0.1, 0.0, 0.1], "must be strictly positive"),
        ([0.0, 1.0, 2.0], [1.0, 3.0, 5.0], [0.0, 0.0, 0.0], "must be strictly positive"),
        ([0.0, 1.0, 2.0], [1.0, 3.0, 5.0], [0.1, -0.1, 0.1], "must be strictly positive"),
        ([1.0, float("nan"), 3.0], [1.0, 2.0, 3.0], None, "column 'x' must contain only finite"),
        ([1.0, float("inf"), 3.0], [1.0, 2.0, 3.0], None, "column 'x' must contain only finite"),
        (
            [1.0, 2.0, 3.0],
            [1.0, 2.0, 3.0],
            [0.1, float("nan"), 0.1],
            "column 'y_error' must contain only finite",
        ),
        (
            [1.0, 2.0, 3.0],
            [1.0, float("nan"), 3.0],
            None,
            "column 'y' must contain only finite",
        ),
    ],
    ids=[
        "x-zero-variance",
        "x-zero-variance-two-points",
        "y-error-contains-zero",
        "y-error-all-zero",
        "y-error-contains-negative",
        "x-contains-nan",
        "x-contains-inf",
        "y-error-contains-nan",
        "y-contains-nan",
    ],
)
def test_degenerate_input_raises_degenerate_input_error(x, y, y_error, expected_message):
    # 分散ゼロは特異行列、y_error のゼロは 1/0 = inf の重み、NaN/inf はそのまま
    # 行列に入り、いずれも numpy の素の LinAlgError になっていた。AnalysisError
    # ではないため envelope を通らず 500 で返っていたものを、入力段で 400 として
    # 弾く (KAN-57 / KAN-60)。
    with pytest.raises(DegenerateInputError) as excinfo:
        fit(x, y, y_error=y_error)

    # どの列のどの制約に反したのかが分からないと、ユーザーは直しようがない。
    assert expected_message in str(excinfo.value)
    # envelope のハンドラに捕まる型であることがこの課題の要点。
    assert isinstance(excinfo.value, AnalysisError)
    assert excinfo.value.code == "degenerate_input"


@pytest.mark.parametrize("bad_y", [float("nan"), float("inf")], ids=["nan", "inf"])
def test_non_finite_y_is_rejected_at_the_input(bad_y):
    # かつては例外にならず NaN 入りの結果が返り、応答の直列化で初めて 500 に
    # なっていた。入力段で弾くようになったので、算出まで到達しない (KAN-57)。
    with pytest.raises(DegenerateInputError) as excinfo:
        fit([1.0, 2.0, 3.0], [1.0, bad_y, 3.0])

    assert "column 'y' must contain only finite" in str(excinfo.value)


def test_overflowing_y_does_not_report_a_perfect_fit():
    # 入力は全て有限だが、二乗和が inf に振り切れる大きさ。ss_tot は inf
    # なので `> 0` を満たし NaN 分岐には入らないが、ss_res も inf になるため
    # inf/inf で NaN になる。直線に乗っているデータではあるものの、この規模
    # では 1.0 とは言い切れないので NaN が返るのが正しい。
    result = fit([0.0, 1.0, 2.0], [0.0, 1e300, 2e300])

    assert math.isnan(result["r_squared"])


def test_nan_x_with_x_log_is_reported_not_silently_filtered():
    # `nan > 0` は False なので、検証が log フィルタより後だと NaN が「非正の
    # 点」として黙って落とされ、点数不足という別のエラーに化けていた。検証を
    # フィルタの前に置いてあるので、NaN は NaN として報告される (KAN-57)。
    with pytest.raises(DegenerateInputError) as excinfo:
        fit([1.0, float("nan"), 3.0], [1.0, 2.0, 3.0], x_log=True)

    assert "column 'x' must contain only finite" in str(excinfo.value)


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


def test_negative_y_error_is_rejected():
    # 重みの符号は最小二乗解に効かない（二乗されるため）ので、負の y_error は
    # 絶対値を使ったのと同じ「それらしい」答えを返してしまっていた。測定の標準
    # 偏差が負というのはあり得ないので、符号の取り違えや列の選択ミスを黙って
    # 飲み込まずに拒否する (KAN-60)。
    with pytest.raises(DegenerateInputError) as excinfo:
        fit([0.0, 1.0, 2.0], [1.0, 3.0, 5.0], y_error=[0.1, -0.1, 0.1])

    assert "must be strictly positive" in str(excinfo.value)


def test_positive_y_error_still_fits_as_before():
    # 重み付き回帰の数値が変わっていないこと (KAN-60 の完了条件)。
    result = fit([0.0, 1.0, 2.0], [1.0, 3.0, 5.0], y_error=[0.1, 0.1, 0.1])

    assert result["weighted"] is True
    assert result["slope"] == pytest.approx(2.0)
    assert result["intercept"] == pytest.approx(1.0)


def test_constant_y_falls_back_to_r_squared_one():
    # y が全て同値 -> ss_tot == 0 -> `else 1.0` フォールバック。ss_tot が
    # 有限のゼロであるこの枝は KAN-58 の修正後も従来どおり 1.0 を返す。
    result = fit([1.0, 2.0, 3.0], [5.0, 5.0, 5.0])

    assert result["slope"] == pytest.approx(0.0, abs=1e-12)
    assert result["intercept"] == pytest.approx(5.0)
    assert result["r_squared"] == 1.0
