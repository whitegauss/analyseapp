// The contract between this parser and the worker's.
//
// The same expression language is implemented twice: here for the
// error-propagation tool, and in `backend/worker/app/formula.py` for the
// curve fitter. A formula typed into one has to mean the same thing in the
// other, so both test suites read `shared/formula-grammar.json` and this
// file is what fails when only one side changes.
//
// Python's own `ast` module could not be reused over there, because it
// reads `^` as bitwise XOR while it is exponentiation here -- so `x^2`
// would have parsed happily and meant something else. Hence two parsers,
// hence this fixture.

import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";
import {
  constantNames,
  evaluateWithGradient,
  FormulaError,
  functionNames,
  parseFormula,
} from "./formula";

type Fixture = {
  evaluate: {
    formula: string;
    variables: string[];
    values: Record<string, number>;
    // JSON has no NaN/Infinity, so those arrive as strings.
    result: number | "NaN" | "Infinity" | "-Infinity";
  }[];
  parse_errors: { formula: string; position: number }[];
  functions: string[];
  constants: string[];
};

const fixture: Fixture = JSON.parse(
  readFileSync(
    new URL("../../shared/formula-grammar.json", import.meta.url),
    "utf-8",
  ),
);

function expected(result: Fixture["evaluate"][number]["result"]): number {
  if (result === "NaN") return NaN;
  if (result === "Infinity") return Infinity;
  if (result === "-Infinity") return -Infinity;
  return result;
}

describe("the grammar shared with the worker", () => {
  it.each(fixture.evaluate)("parses and evaluates $formula", (testCase) => {
    const parsed = parseFormula(testCase.formula);

    expect(parsed.variables).toEqual(testCase.variables);

    const { value } = evaluateWithGradient(parsed.ast, testCase.values);
    const want = expected(testCase.result);
    if (Number.isNaN(want)) {
      expect(value).toBeNaN();
    } else if (Number.isFinite(want)) {
      expect(value).toBeCloseTo(want, 10);
    } else {
      expect(value).toBe(want);
    }
  });

  it.each(fixture.parse_errors)(
    "rejects $formula at the same position",
    (testCase) => {
      let thrown: unknown;
      try {
        parseFormula(testCase.formula);
      } catch (e) {
        thrown = e;
      }

      expect(thrown).toBeInstanceOf(FormulaError);
      // The caret position is part of the contract: both implementations
      // point the user at the same character.
      expect((thrown as FormulaError).position).toBe(testCase.position);
    },
  );

  it("offers exactly the functions and constants the fixture lists", () => {
    // A function added on one side only would otherwise go unnoticed until
    // someone typed it and got an error from the other half of the app.
    expect([...functionNames].sort()).toEqual([...fixture.functions].sort());
    expect([...constantNames].sort()).toEqual([...fixture.constants].sort());
  });
});
