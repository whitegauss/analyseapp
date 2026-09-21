"""Tests for the hand-written formula parser.

The first test is the important one: it reads the same fixture the
frontend's formula.test.ts reads, so the two implementations of this
grammar cannot drift apart without something going red. The rest cover
what only exists on this side -- evaluation over numpy arrays, and the
non-finite results a fitter walks through on its way to an answer.
"""

import json
import math
from pathlib import Path

import numpy as np
import pytest

from app.formula import FormulaError, evaluate, parse_formula

FIXTURE = json.loads(
    (Path(__file__).resolve().parents[3] / "shared" / "formula-grammar.json").read_text(
        encoding="utf-8"
    )
)


def _expected(result):
    """Turn the fixture's JSON stand-ins for non-finite numbers back into floats.

    JSON has no NaN or Infinity, so the fixture carries them as strings --
    which is also why they are worth pinning: they are exactly the values a
    fitter produces mid-search.
    """
    if isinstance(result, str):
        return {"NaN": math.nan, "Infinity": math.inf, "-Infinity": -math.inf}[result]
    return result


@pytest.mark.parametrize("case", FIXTURE["evaluate"], ids=lambda c: c["formula"])
def test_matches_the_shared_grammar(case):
    parsed = parse_formula(case["formula"])

    assert list(parsed.variables) == case["variables"]

    got = float(evaluate(parsed.ast, case["values"]))
    want = _expected(case["result"])
    if math.isnan(want):
        assert math.isnan(got)
    else:
        assert got == pytest.approx(want, rel=1e-12, abs=1e-12)


@pytest.mark.parametrize("case", FIXTURE["parse_errors"], ids=lambda c: c["formula"] or "(empty)")
def test_rejects_what_the_shared_grammar_rejects(case):
    with pytest.raises(FormulaError) as excinfo:
        parse_formula(case["formula"])
    # The caret position is part of the contract: the two implementations
    # point the user at the same character.
    assert excinfo.value.position == case["position"]


def test_fixture_lists_every_function_and_constant():
    # A function added on one side only would otherwise go unnoticed until
    # someone typed it and got "知らない関数です" from the worker alone.
    from app.formula import CONSTANTS, FUNCTIONS

    assert sorted(FUNCTIONS) == sorted(FIXTURE["functions"])
    assert sorted(CONSTANTS) == sorted(FIXTURE["constants"])


def test_evaluates_over_an_array():
    # The reason this parser exists rather than reusing the browser's: the
    # fitter applies the model to a whole x column per iteration.
    parsed = parse_formula("A*exp(-x/tau)")

    got = evaluate(parsed.ast, {"A": 2.0, "x": np.array([0.0, 1.0, 2.0]), "tau": 1.0})

    assert got == pytest.approx([2.0, 2.0 * math.exp(-1), 2.0 * math.exp(-2)])


def test_broadcasts_scalars_against_arrays():
    parsed = parse_formula("a*x + b")

    got = evaluate(parsed.ast, {"a": 3.0, "x": np.array([1.0, 2.0]), "b": 1.0})

    assert got == pytest.approx([4.0, 7.0])


def test_constants_and_functions_are_never_variables():
    # Otherwise `pi` in a model would be offered to the user as something to
    # fit, and `2*pi*sqrt(L/g)` would have three parameters instead of two.
    parsed = parse_formula("2*pi*sqrt(L/g) + e")

    assert list(parsed.variables) == ["L", "g"]


def test_a_variable_repeated_is_listed_once():
    parsed = parse_formula("a*x^2 + a*x + a")

    assert list(parsed.variables) == ["a", "x"]


def test_missing_value_is_reported_by_name():
    parsed = parse_formula("a*x")

    with pytest.raises(FormulaError, match="x"):
        evaluate(parsed.ast, {"a": 1.0})


@pytest.mark.parametrize(
    ("formula", "values"),
    [
        ("1/x", {"x": 0.0}),
        ("ln(x)", {"x": -1.0}),
        ("sqrt(x)", {"x": -1.0}),
        ("x^y", {"x": -8.0, "y": 1 / 3}),
        ("exp(x)", {"x": 10000.0}),
    ],
)
def test_out_of_domain_yields_a_non_finite_number_rather_than_raising(formula, values):
    # A fitter walks through parameter values that are out of a function's
    # domain on the way to a good answer. Raising there would abort a search
    # that was about to succeed; nan/inf lets the fitter score the point and
    # move on. Whether the *final* answer is finite is the fitter's problem,
    # not the parser's.
    got = float(evaluate(parse_formula(formula).ast, values))

    assert not math.isfinite(got)


def test_out_of_domain_does_not_warn():
    # numpy prints a RuntimeWarning for these by default, which in a worker
    # means log noise on every iteration of every fit.
    with np.errstate(all="raise"):
        # errstate(raise) would turn a leaked warning into an exception; the
        # evaluator's own errstate(ignore) has to win over it.
        got = float(evaluate(parse_formula("1/x").ast, {"x": 0.0}))

    assert math.isinf(got)
