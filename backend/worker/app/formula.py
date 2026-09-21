"""A small expression language for formulas typed by hand, e.g.
``A*exp(-x/tau)*cos(w*x + phi)``.

This is the Python half of the grammar the frontend's error-propagation tool
already speaks (``frontend/lib/formula.ts``). The two are kept in step by
``tests/fixtures/formula_grammar.json``, which both test suites read: a
formula that parses one way in the browser has to parse the same way here,
or the same text typed into two parts of the same app would mean two
different things.

**Python's own ``ast`` module cannot be used for this.** ``^`` is the power
operator here, as it is on paper and in the error-propagation tool, while
``ast.parse`` reads it as bitwise XOR -- so ``x^2`` would parse happily and
silently mean something else. Hence a parser written out by hand, which also
means no ``eval`` anywhere near user input.

Unlike the TypeScript version there is no automatic differentiation here:
the only caller is the curve fitter, and SciPy estimates its own Jacobian
numerically. What this does instead is evaluate over numpy arrays, so a
formula is applied to a whole column at once.
"""

import math
import re
from dataclasses import dataclass
from typing import Any

import numpy as np


class FormulaError(Exception):
    """A formula that cannot be parsed or evaluated.

    ``position`` is the character offset the problem was found at, so a
    caller can point at it; -1 when the problem is not tied to one spot.
    """

    def __init__(self, message: str, position: int = -1):
        super().__init__(message)
        self.position = position


# Resolved by the parser itself, so a name in here is never a variable and
# so can never become a fit parameter.
CONSTANTS: dict[str, float] = {
    "pi": math.pi,
    "e": math.e,
}

# numpy equivalents of the frontend's Math.* calls, chosen so they apply
# element-wise to an array. Arities match FUNCTIONS in formula.ts; the
# fixture test is what keeps them matching.
FUNCTIONS: dict[str, tuple[int, Any]] = {
    "sqrt": (1, np.sqrt),
    "exp": (1, np.exp),
    "ln": (1, np.log),
    "log10": (1, np.log10),
    "sin": (1, np.sin),
    "cos": (1, np.cos),
    "tan": (1, np.tan),
    "asin": (1, np.arcsin),
    "acos": (1, np.arccos),
    "atan": (1, np.arctan),
    "atan2": (2, np.arctan2),
    "sinh": (1, np.sinh),
    "cosh": (1, np.cosh),
    "tanh": (1, np.tanh),
    "abs": (1, np.abs),
}

OPERATORS = frozenset("+-*/^")

_NUMBER_START = re.compile(r"[0-9.]")
_DIGIT = re.compile(r"[0-9]")
# Unicode letters, so the Greek names physicists actually write (θ, λ, Δx)
# are variables like any other.
_NAME_START = re.compile(r"[^\W\d]|_", re.UNICODE)
_NAME_PART = re.compile(r"\w", re.UNICODE)


@dataclass(frozen=True)
class Token:
    kind: str  # number | name | operator | paren | comma
    text: str
    position: int
    value: float | None = None


def tokenize(source: str) -> list[Token]:
    """Split a formula into tokens, mirroring formula.ts's tokenizer."""
    tokens: list[Token] = []
    i = 0
    n = len(source)

    while i < n:
        char = source[i]

        if char.isspace():
            i += 1
            continue

        if _NUMBER_START.match(char):
            start = i
            while i < n and _NUMBER_START.match(source[i]):
                i += 1
            # Exponent notation, but only when digits actually follow:
            # `1.6e-19` is one number, while the `e` in `2*e` is Euler's
            # constant and the one in `2e` is a typo worth complaining
            # about rather than swallowing.
            following = source[i + 1] if i + 1 < n else ""
            exponent_sign = 1 if following in "+-" else 0
            after_sign = source[i + 1 + exponent_sign] if i + 1 + exponent_sign < n else ""
            if i < n and source[i] in "eE" and _DIGIT.match(after_sign or ""):
                i += 1 + exponent_sign
                while i < n and _DIGIT.match(source[i]):
                    i += 1
            text = source[start:i]
            try:
                value = float(text)
            except ValueError as exc:
                raise FormulaError(f"数値として読めません: {text}", start) from exc
            if not math.isfinite(value):
                raise FormulaError(f"数値として読めません: {text}", start)
            tokens.append(Token("number", text, start, value))
            continue

        if _NAME_START.match(char):
            start = i
            while i < n and _NAME_PART.match(source[i]):
                i += 1
            tokens.append(Token("name", source[start:i], start))
            continue

        if char in OPERATORS:
            tokens.append(Token("operator", char, i))
            i += 1
            continue

        if char in "()":
            tokens.append(Token("paren", char, i))
            i += 1
            continue

        if char == ",":
            tokens.append(Token("comma", char, i))
            i += 1
            continue

        raise FormulaError(f"使えない文字です: {char}", i)

    return tokens


# Node is a tagged tuple tree rather than a class hierarchy: it is only ever
# built here and walked by _evaluate, and the shape mirrors formula.ts's
# discriminated union one for one.
#
#   ("number", value)
#   ("constant", name, value)
#   ("variable", name)
#   ("negate", operand)
#   ("binary", op, left, right)
#   ("call", name, args)
Node = tuple


class _Parser:
    """Recursive descent over the usual arithmetic precedence:

        expression := term (('+' | '-') term)*
        term       := unary (('*' | '/') unary)*
        unary      := ('-' | '+') unary | power
        power      := atom ('^' unary)?
        atom       := number | name | name '(' args ')' | '(' expression ')'

    ``^`` is right-associative (a^b^c = a^(b^c)) and binds tighter than
    unary minus (-x^2 = -(x^2)), matching how the same formula reads on
    paper. Its right side is a unary so ``x^-2`` works.
    """

    def __init__(self, tokens: list[Token], source: str):
        self.tokens = tokens
        self.source = source
        self.index = 0
        # First-appearance order, so the inputs a caller builds from this
        # follow the formula as it reads.
        self.variables: list[str] = []

    def parse(self) -> Node:
        if not self.tokens:
            raise FormulaError("式が空です", 0)
        node = self.parse_expression()
        token = self.peek()
        if token is not None:
            raise FormulaError(f"余分な入力です: {token.text}", token.position)
        return node

    def peek(self) -> Token | None:
        return self.tokens[self.index] if self.index < len(self.tokens) else None

    def next(self) -> Token:
        token = self.peek()
        if token is None:
            raise FormulaError("式が途中で終わっています", len(self.source))
        self.index += 1
        return token

    def parse_expression(self) -> Node:
        node = self.parse_term()
        while True:
            token = self.peek()
            if token is not None and token.kind == "operator" and token.text in "+-":
                self.index += 1
                node = ("binary", token.text, node, self.parse_term())
            else:
                return node

    def parse_term(self) -> Node:
        node = self.parse_unary()
        while True:
            token = self.peek()
            if token is not None and token.kind == "operator" and token.text in "*/":
                self.index += 1
                node = ("binary", token.text, node, self.parse_unary())
            else:
                return node

    def parse_unary(self) -> Node:
        token = self.peek()
        if token is not None and token.kind == "operator" and token.text in "+-":
            self.index += 1
            operand = self.parse_unary()
            return ("negate", operand) if token.text == "-" else operand
        return self.parse_power()

    def parse_power(self) -> Node:
        base = self.parse_atom()
        token = self.peek()
        if token is not None and token.kind == "operator" and token.text == "^":
            self.index += 1
            return ("binary", "^", base, self.parse_unary())
        return base

    def parse_atom(self) -> Node:
        token = self.next()

        if token.kind == "number":
            assert token.value is not None
            return ("number", token.value)

        if token.kind == "name":
            following = self.peek()
            if following is not None and following.kind == "paren" and following.text == "(":
                return self.parse_call(token)
            if token.text in CONSTANTS:
                return ("constant", token.text, CONSTANTS[token.text])
            if token.text in FUNCTIONS:
                raise FormulaError(
                    f"{token.text} は関数です。{token.text}(...) と書いてください",
                    token.position,
                )
            if token.text not in self.variables:
                self.variables.append(token.text)
            return ("variable", token.text)

        if token.kind == "paren" and token.text == "(":
            node = self.parse_expression()
            closing = self.peek()
            if closing is None or closing.kind != "paren" or closing.text != ")":
                # Pointed at where the ')' should have been, not at the '('
                # that opened it -- which is where formula.ts points too.
                raise FormulaError(
                    "閉じ括弧がありません",
                    closing.position if closing else len(self.source),
                )
            self.index += 1
            return node

        raise FormulaError(f"ここには書けません: {token.text}", token.position)

    def parse_call(self, name_token: Token) -> Node:
        spec = FUNCTIONS.get(name_token.text)
        if spec is None:
            raise FormulaError(f"知らない関数です: {name_token.text}", name_token.position)
        arity, _ = spec

        self.index += 1  # consume '('
        args: list[Node] = []
        closing = self.peek()
        if closing is not None and closing.kind == "paren" and closing.text == ")":
            self.index += 1
        else:
            while True:
                args.append(self.parse_expression())
                separator = self.peek()
                if separator is not None and separator.kind == "comma":
                    self.index += 1
                    continue
                if separator is not None and separator.kind == "paren" and separator.text == ")":
                    self.index += 1
                    break
                raise FormulaError(
                    "引数の区切りか閉じ括弧が必要です",
                    separator.position if separator else len(self.source),
                )

        if len(args) != arity:
            raise FormulaError(
                f"{name_token.text} は引数{arity}個です（{len(args)}個でした）",
                name_token.position,
            )
        return ("call", name_token.text, args)


@dataclass(frozen=True)
class ParsedFormula:
    ast: Node
    # Variable names in first-appearance order.
    variables: tuple[str, ...]


def parse_formula(source: str) -> ParsedFormula:
    """Parse a formula, returning its tree and the variables it mentions.

    Names resolved as constants (``pi``, ``e``) or functions are not
    variables, so they can never be mistaken for something to fit.
    """
    parser = _Parser(tokenize(source), source)
    ast = parser.parse()
    return ParsedFormula(ast=ast, variables=tuple(parser.variables))


def evaluate(ast: Node, values: dict[str, Any]) -> Any:
    """Evaluate a parsed formula.

    Each value may be a scalar or a numpy array; arrays are applied
    element-wise, which is what lets a fitter evaluate the model over a
    whole x column at once. Mixing the two follows numpy's broadcasting.

    Non-finite results are not rejected here: a fitter walks through
    parameter values that may well be out of a function's domain on the way
    to a good answer, and numpy's nan is the right thing to hand back for
    that. Callers that need a finite answer check the one they end up with.
    """
    kind = ast[0]

    if kind in ("number", "constant"):
        return ast[-1]

    if kind == "variable":
        name = ast[1]
        if name not in values:
            raise FormulaError(f"{name} の値がありません")
        return values[name]

    if kind == "negate":
        return -evaluate(ast[1], values)

    if kind == "binary":
        _, op, left_node, right_node = ast
        left = evaluate(left_node, values)
        right = evaluate(right_node, values)
        # numpy's warnings for divide-by-zero, overflow and invalid powers
        # are suppressed rather than raised: they are ordinary events while
        # a fitter searches, and the nan/inf they produce is the answer.
        with np.errstate(all="ignore"):
            if op == "+":
                return left + right
            if op == "-":
                return left - right
            if op == "*":
                return left * right
            if op == "/":
                return np.divide(left, right)
            if op == "^":
                # float exponents of negative bases are nan, not an error,
                # matching JavaScript's ** and keeping the fitter moving.
                return np.power(np.asarray(left, dtype=float), right)
        raise FormulaError(f"知らない演算子です: {op}")

    if kind == "call":
        _, name, arg_nodes = ast
        _, fn = FUNCTIONS[name]
        args = [evaluate(node, values) for node in arg_nodes]
        with np.errstate(all="ignore"):
            return fn(*args)

    raise FormulaError(f"知らないノードです: {kind}")
