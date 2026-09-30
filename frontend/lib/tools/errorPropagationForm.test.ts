import { describe, expect, it } from "vitest";
import { parseFormula } from "@/lib/formula";
import {
  allValuesFilled,
  computePropagation,
  entryOf,
  formatPropagation,
  readFormula,
  withEntry,
  type Entries,
} from "./errorPropagationForm";

function entries(
  values: Readonly<Record<string, readonly [string, string]>>,
): Entries {
  let e: Entries = Object.create(null);
  for (const [name, [value, uncertainty]] of Object.entries(values)) {
    e = withEntry(e, name, { value, uncertainty });
  }
  return e;
}

describe("readFormula", () => {
  it("parses a formula", () => {
    const { formula, error } = readFormula("2*pi*sqrt(L/g)");
    expect(error).toBeNull();
    expect(formula?.variables).toEqual(["L", "g"]);
  });

  it("is neither a formula nor an error when blank", () => {
    expect(readFormula("  ")).toEqual({ formula: null, error: null });
  });

  it("reports the parser's message for a formula that does not parse", () => {
    const { formula, error } = readFormula("a*(b");
    expect(formula).toBeNull();
    expect(error).toMatch(/閉じ括弧/);
  });
});

describe("entryOf / withEntry", () => {
  it("does not read inherited names as entries", () => {
    expect(entryOf(Object.create(null), "constructor")).toBeUndefined();
    expect(entryOf({} as Entries, "toString")).toBeUndefined();
  });

  it("fills the other field with an empty string", () => {
    expect(entryOf(withEntry({}, "L", { value: "1" }), "L")).toEqual({
      value: "1",
      uncertainty: "",
    });
  });

  it("keeps the other field and does not modify the original", () => {
    const before = entries({ L: ["1", "0.01"] });
    const after = withEntry(before, "L", { value: "2" });

    expect(entryOf(after, "L")).toEqual({ value: "2", uncertainty: "0.01" });
    expect(entryOf(before, "L")).toEqual({ value: "1", uncertainty: "0.01" });
  });
});

describe("allValuesFilled", () => {
  it.each([
    [
      "every value typed, σ blank",
      ["L", "g"],
      { L: ["1", ""], g: ["9.8", ""] },
      true,
    ],
    ["one value blank", ["L", "g"], { L: ["1", "0.01"], g: [" ", ""] }, false],
    ["one variable never touched", ["L", "g"], { L: ["1", ""] }, false],
    ["no variables", [], {}, false],
  ] as const)("%s -> %s", (_, variables, values, want) => {
    expect(allValuesFilled([...variables], entries(values))).toBe(want);
  });
});

describe("computePropagation", () => {
  const pendulum = parseFormula("2*pi*sqrt(L/g)");
  const vars = pendulum.variables;

  it("propagates, treating a blank σ as exact", () => {
    const result = computePropagation(
      pendulum,
      vars,
      entries({ L: ["1", "0.01"], g: ["9.8", ""] }),
    );
    expect(result?.value).toBeCloseTo(2.00709, 4);
    // Only L contributes: σT = T/2 · σL/L.
    expect(result?.uncertainty).toBeCloseTo(0.0100354, 6);
    expect(result?.terms.map((t) => t.share)).toEqual([1, 0]);
  });

  it("takes |σ|, so a negative uncertainty is not a smaller one", () => {
    const negative = computePropagation(
      pendulum,
      vars,
      entries({ L: ["1", "-0.01"], g: ["9.8", ""] }),
    );
    expect(negative?.uncertainty).toBeCloseTo(0.0100354, 6);
  });

  it.each([
    ["no formula", null, { L: ["1", ""], g: ["9.8", ""] }],
    ["a value missing", pendulum, { L: ["1", ""] }],
    ["a value blank", pendulum, { L: ["1", ""], g: ["", ""] }],
    ["a value not a number", pendulum, { L: ["abc", ""], g: ["9.8", ""] }],
    ["a σ not a number", pendulum, { L: ["1", "abc"], g: ["9.8", ""] }],
    // Out of the domain: sqrt of a negative.
    ["a non-finite value", pendulum, { L: ["-1", ""], g: ["9.8", ""] }],
  ] as const)("is null with %s", (_, formula, values) => {
    expect(computePropagation(formula, vars, entries(values))).toBeNull();
  });

  // sqrt(x) at 0 has a value but an infinite derivative: null, not an
  // exact-looking "± 0.0".
  it("is null for a non-finite uncertainty", () => {
    const root = parseFormula("sqrt(x)");
    expect(
      computePropagation(root, root.variables, entries({ x: ["0", "0.1"] })),
    ).toBeNull();
  });
});

describe("formatPropagation", () => {
  it("rounds to the uncertainty and gives the relative uncertainty", () => {
    expect(
      formatPropagation({ value: 2.00709, uncertainty: 0.0100354, terms: [] }),
    ).toEqual({ line: "z = 2.01 ± 0.01", relative: "0.50%" });
  });

  it.each([
    ["a zero value", { value: 0, uncertainty: 0.1, terms: [] }],
    ["an exact result", { value: 2, uncertainty: 0, terms: [] }],
  ])("has no relative uncertainty for %s", (_, result) => {
    expect(formatPropagation(result).relative).toBeNull();
  });
});
