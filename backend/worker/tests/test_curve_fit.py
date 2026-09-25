import math

import numpy as np
import pytest

from app.analysis import (
    DegenerateInputError,
    FitFailedError,
    InsufficientDataError,
    InvalidFormulaError,
    InvalidParamsError,
    MissingColumnError,
    run,
)
from tests.conftest import series

X = np.linspace(0.0, 5.0, 20)
# Deterministic, zero-mean-ish scatter: enough for the covariance to be
# finite and non-zero, small enough that the fitted values stay near truth.
WIGGLE = 0.02 * np.cos(7.0 * X)


def curve_fit(x, y, formula, y_error=None, **params):
    """Run curve_fit over the given columns, returning its result dict."""
    return run(
        "curve_fit",
        series(list(map(float, x)), list(map(float, y)), y_error),
        {"formula": formula, **params},
    )


def by_name(result):
    """The result's parameters as {name: (value, stderr)}."""
    return {p["name"]: (p["value"], p["stderr"]) for p in result["parameters"]}


def test_exponential_decay_recovers_parameters():
    y = 3.0 * np.exp(-X / 1.5) + 0.5 + WIGGLE

    result = curve_fit(X, y, "A*exp(-x/tau) + C")

    params = by_name(result)
    assert params["A"][0] == pytest.approx(3.0, abs=0.05)
    assert params["tau"][0] == pytest.approx(1.5, abs=0.05)
    assert params["C"][0] == pytest.approx(0.5, abs=0.05)
    assert all(0 < stderr < 0.1 for _, stderr in params.values())
    assert 0.99 < result["r_squared"] < 1.0
    assert result["weighted"] is False
    assert result["formula"] == "A*exp(-x/tau) + C"


def test_parameters_are_reported_in_first_appearance_order():
    result = curve_fit(X, 2 * X + 1, "b + a*x")

    assert [p["name"] for p in result["parameters"]] == ["b", "a"]


def test_exact_fit_has_zero_residuals():
    y = 2.0 * X**2 + 1.0

    result = curve_fit(X, y, "a*x^2 + b")

    params = by_name(result)
    assert params["a"][0] == pytest.approx(2.0)
    assert params["b"][0] == pytest.approx(1.0)
    assert result["predicted_y"] == pytest.approx(list(y))
    assert result["residuals"] == pytest.approx([0.0] * len(X), abs=1e-9)
    assert result["r_squared"] == pytest.approx(1.0)


def test_constants_and_functions_are_not_parameters():
    result = curve_fit(X, 2.0 * np.sin(math.pi * X), "A*sin(pi*x)")

    assert [p["name"] for p in result["parameters"]] == ["A"]
    assert by_name(result)["A"][0] == pytest.approx(2.0)


def test_formula_without_x_fits_a_constant():
    result = curve_fit(X, 4.0 + WIGGLE, "c")

    assert by_name(result)["c"][0] == pytest.approx(4.0, abs=0.01)
    assert len(result["predicted_y"]) == len(X)


def test_initial_values_steer_the_fit_to_the_right_minimum():
    y = 2.0 * np.sin(3.0 * X)

    # From the default 1.0 the frequency settles in a local minimum.
    default = curve_fit(X, y, "A*sin(w*x)")
    assert by_name(default)["w"][0] != pytest.approx(3.0, abs=0.1)

    guided = curve_fit(X, y, "A*sin(w*x)", initial={"A": 2, "w": 3.1})
    assert by_name(guided)["w"][0] == pytest.approx(3.0)
    assert by_name(guided)["A"][0] == pytest.approx(2.0)


def test_weighted_fit_uses_y_error():
    y = list(2.0 * X + 1.0)
    y[-1] = 100.0  # an outlier...
    y_error = [0.01] * (len(X) - 1) + [1000.0]  # ...that we barely trust

    result = curve_fit(X, y, "a*x + b", y_error=y_error)

    assert result["weighted"] is True
    assert by_name(result)["a"][0] == pytest.approx(2.0, abs=0.01)
    assert by_name(result)["b"][0] == pytest.approx(1.0, abs=0.01)


def test_matches_linear_regression_on_a_line():
    y = 2.0 * X + 1.0 + WIGGLE

    nonlinear = by_name(curve_fit(X, y, "a*x + b"))
    linear = run("linear_regression", series(list(X), list(y)), {})

    assert nonlinear["a"][0] == pytest.approx(linear["slope"])
    assert nonlinear["b"][0] == pytest.approx(linear["intercept"])
    assert nonlinear["a"][1] == pytest.approx(linear["slope_stderr"], rel=1e-4)
    assert nonlinear["b"][1] == pytest.approx(linear["intercept_stderr"], rel=1e-4)


# --- standard errors that cannot be known ---


def test_as_many_points_as_parameters_gives_null_stderr():
    result = curve_fit([1.0, 2.0], [3.0, 5.0], "a*x + b")

    assert by_name(result) == {"a": (pytest.approx(2.0), None), "b": (pytest.approx(1.0), None)}


@pytest.mark.parametrize("formula", ["a*b*x", "a*x + b*x"])
def test_parameters_the_data_cannot_separate_give_null_stderr(formula):
    # Only a*b (or a+b) is determined. SciPy's pseudo-inverse still returns
    # a finite covariance for these, which is why the Jacobian rank is
    # checked separately.
    result = curve_fit(X, 2.0 * X, formula)

    assert all(stderr is None for _, stderr in by_name(result).values())
    assert result["predicted_y"] == pytest.approx(list(2.0 * X))


def test_parameters_on_very_different_scales_keep_their_stderr():
    y = 1e3 * np.exp(-1e-3 * X) + 0.01 * np.cos(9.0 * X)

    result = curve_fit(X, y, "A*exp(-k*x)", initial={"A": 1000, "k": 0.001})

    assert all(stderr is not None and stderr > 0 for _, stderr in by_name(result).values())


# --- fit failures ---


def test_model_undefined_at_the_starting_point_says_to_set_initial():
    # 1/(x - a) with a = 1.0 divides by zero at x = 1.
    with pytest.raises(FitFailedError, match="初期値で式を計算できない"):
        curve_fit([1.0, 2.0, 3.0], [1.0, 2.0, 3.0], "1/(x - a)")


def test_starting_point_inside_the_domain_fits():
    y = 1.0 / (X + 1.0)

    result = curve_fit(X, y, "1/(x - a)", initial={"a": -0.5})

    assert by_name(result)["a"][0] == pytest.approx(-1.0)


def test_non_convergence_is_fit_failed_not_a_crash():
    with pytest.raises(FitFailedError):
        curve_fit(X, np.exp(50.0 * X), "exp(a*x)")


# --- invalid formula / params ---


@pytest.mark.parametrize("params", [{}, {"formula": ""}, {"formula": "  "}, {"formula": 3}])
def test_formula_is_required(params):
    with pytest.raises(InvalidParamsError, match="formula"):
        run("curve_fit", series([0.0, 1.0], [0.0, 1.0]), params)


def test_syntax_error_reports_its_position():
    with pytest.raises(InvalidFormulaError, match="5文字目"):
        curve_fit(X, X, "a*x+")


def test_formula_naming_y_is_rejected():
    with pytest.raises(InvalidFormulaError, match="y は測定値"):
        curve_fit(X, X, "y*a")


def test_formula_with_nothing_to_fit_is_rejected():
    with pytest.raises(InvalidFormulaError, match="パラメータがありません"):
        curve_fit(X, X, "2*x + pi")


def test_initial_for_an_unknown_parameter_is_rejected():
    with pytest.raises(InvalidParamsError, match="b"):
        curve_fit(X, X, "a*x", initial={"b": 1.0})


@pytest.mark.parametrize("bad", [True, "1", None, float("nan"), float("inf")])
def test_initial_value_must_be_a_finite_number(bad):
    with pytest.raises(InvalidParamsError, match="initial.a"):
        curve_fit(X, X, "a*x", initial={"a": bad})


def test_initial_must_be_an_object():
    with pytest.raises(InvalidParamsError, match="initial"):
        curve_fit(X, X, "a*x", initial=[1.0])


# --- data checks shared with linear_regression ---


def test_fewer_points_than_parameters_is_insufficient_data():
    with pytest.raises(InsufficientDataError, match="3 parameters"):
        curve_fit([1.0, 2.0], [1.0, 2.0], "a*x^2 + b*x + c")


def test_missing_y_column_raises():
    from app.schemas import DataSeries

    with pytest.raises(MissingColumnError):
        run("curve_fit", DataSeries(columns={"x": [1.0, 2.0]}), {"formula": "a*x"})


def test_non_finite_y_is_degenerate_input():
    with pytest.raises(DegenerateInputError):
        curve_fit([0.0, 1.0, 2.0], [0.0, float("nan"), 2.0], "a*x")


def test_non_positive_y_error_is_degenerate_input():
    with pytest.raises(DegenerateInputError, match="y_error"):
        curve_fit([0.0, 1.0, 2.0], [0.0, 1.0, 2.0], "a*x", y_error=[0.1, 0.0, 0.1])
