import { describe, expect, it } from "vitest";
import { fitParameters, mergeConfig, parseFitForm, readFitConfig } from "./fit";

describe("fitParameters", () => {
  it("lists every variable but x, in first-appearance order", () => {
    expect(fitParameters("C + A*exp(-x/tau)")).toEqual({
      ok: true,
      names: ["C", "A", "tau"],
    });
  });

  it("does not count constants or functions as parameters", () => {
    expect(fitParameters("A*sin(2*pi*x) + e")).toEqual({
      ok: true,
      names: ["A"],
    });
  });

  it("points at a syntax error, 1-based", () => {
    const result = fitParameters("a*x+");
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.error).toContain("5文字目");
  });

  it.each([
    ["", "式を入力してください"],
    ["   ", "式を入力してください"],
    ["y*a", "y は測定値"],
    ["2*x + pi", "パラメータがありません"],
  ])("rejects %j", (formula, message) => {
    const result = fitParameters(formula);
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.error).toContain(message);
  });
});

describe("readFitConfig", () => {
  it("reads a saved fit", () => {
    expect(
      readFitConfig({ fit: { formula: "a*x^2 + b", initial: { a: 2 } } }),
    ).toEqual({ formula: "a*x^2 + b", initial: { a: 2 } });
  });

  it("treats a missing initial as none", () => {
    expect(readFitConfig({ fit: { formula: "a*x" } })).toEqual({
      formula: "a*x",
      initial: {},
    });
  });

  it.each([
    ["no fit key", {}],
    ["fit is null", { fit: null }],
    ["fit is an array", { fit: ["a*x"] }],
    ["formula is not a string", { fit: { formula: 3 } }],
    ["formula no longer parses", { fit: { formula: "a*(x" } }],
    ["formula has nothing to fit", { fit: { formula: "2*x" } }],
  ])("falls back to the straight line when %s", (_, config) => {
    expect(readFitConfig(config)).toBeNull();
  });

  it("drops starting values the formula does not use, or that are not numbers", () => {
    expect(
      readFitConfig({
        fit: { formula: "a*x + b", initial: { a: 1, b: "2", c: 3 } },
      }),
    ).toEqual({ formula: "a*x + b", initial: { a: 1 } });
  });
});

describe("parseFitForm", () => {
  it("maps the straight line to no fit, ignoring the other fields", () => {
    expect(parseFitForm("linear", "garbage(", "not json")).toEqual({
      ok: true,
      fit: null,
    });
  });

  it("accepts a formula with starting values", () => {
    expect(
      parseFitForm("curve", "  A*sin(w*x) ", JSON.stringify({ w: 3.1 })),
    ).toEqual({
      ok: true,
      fit: { formula: "A*sin(w*x)", initial: { w: 3.1 } },
    });
  });

  it("treats a missing initial field as none", () => {
    expect(parseFitForm("curve", "a*x", null)).toEqual({
      ok: true,
      fit: { formula: "a*x", initial: {} },
    });
  });

  it("rejects an unknown mode", () => {
    expect(parseFitForm("spline", "a*x", "{}").ok).toBe(false);
    expect(parseFitForm(null, "a*x", "{}").ok).toBe(false);
  });

  it("rejects a formula the worker would reject", () => {
    const result = parseFitForm("curve", "y*a", "{}");
    expect(result.ok).toBe(false);
    if (!result.ok) expect(result.error).toContain("y は測定値");
  });

  it.each([
    ["not JSON", "{"],
    ["an array", "[1]"],
    ["null", "null"],
    ["a number", "3"],
  ])("rejects initial that is %s", (_, initial) => {
    expect(parseFitForm("curve", "a*x", initial).ok).toBe(false);
  });

  it("rejects a starting value for a parameter the formula lacks", () => {
    const result = parseFitForm("curve", "a*x", JSON.stringify({ b: 1 }));
    expect(result).toEqual({
      ok: false,
      error: "b は式に含まれないパラメータです",
    });
  });

  it.each([["1"], [null], [true]])("rejects a starting value of %j", (v) => {
    const result = parseFitForm("curve", "a*x", JSON.stringify({ a: v }));
    expect(result.ok).toBe(false);
  });

  it("rejects an infinite starting value (1e999 is valid JSON)", () => {
    expect(parseFitForm("curve", "a*x", '{"a": 1e999}').ok).toBe(false);
  });
});

describe("mergeConfig", () => {
  it("keeps keys the patch does not mention", () => {
    const current = {
      x_axis_label: "t",
      fit: { formula: "a*x", initial: {} },
    };
    expect(mergeConfig(current, { x_axis_label: "time" })).toEqual({
      x_axis_label: "time",
      fit: { formula: "a*x", initial: {} },
    });
  });

  it("removes a key set to undefined", () => {
    expect(
      mergeConfig({ x_axis_label: "t", fit: {} }, { fit: undefined }),
    ).toEqual({ x_axis_label: "t" });
  });

  it("does not modify the config it was given", () => {
    const current = { x_axis_label: "t" };
    mergeConfig(current, { x_axis_label: "time" });
    expect(current).toEqual({ x_axis_label: "t" });
  });
});
