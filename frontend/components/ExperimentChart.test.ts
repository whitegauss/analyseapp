import { describe, expect, it } from "vitest";
import type { LinearRegressionResult } from "@/lib/experiment";
import { evaluateModel, formatModelEquation } from "./ExperimentChart";

function makeRegression(
  overrides: Partial<LinearRegressionResult>,
): LinearRegressionResult {
  return {
    slope: 2,
    intercept: 1,
    slope_stderr: 0.1,
    intercept_stderr: 0.1,
    r_squared: 0.99,
    weighted: false,
    x_log: false,
    y_log: false,
    ...overrides,
  };
}

describe("evaluateModel", () => {
  it("evaluates a plain linear fit (y = slope*x + intercept)", () => {
    const regression = makeRegression({ slope: 2, intercept: 1 });
    expect(evaluateModel(regression, 5, false, false)).toBeCloseTo(11, 10);
  });

  it("evaluates a semi-log (x_log) fit: y = slope*log10(x) + intercept", () => {
    const regression = makeRegression({ slope: 2, intercept: 1, x_log: true });
    // log10(100) = 2 -> y = 2*2 + 1 = 5
    expect(evaluateModel(regression, 100, true, false)).toBeCloseTo(5, 10);
  });

  it("evaluates a semi-log (y_log) fit: y = 10^(slope*x + intercept)", () => {
    const regression = makeRegression({
      slope: 0.5,
      intercept: 1,
      y_log: true,
    });
    // log10(y) = 0.5*4 + 1 = 3 -> y = 1000
    expect(evaluateModel(regression, 4, false, true)).toBeCloseTo(1000, 6);
  });

  it("evaluates a log-log (power law) fit: y = 10^intercept * x^slope", () => {
    const regression = makeRegression({
      slope: 3,
      intercept: Math.log10(5),
      x_log: true,
      y_log: true,
    });
    // y = 5 * x^3, at x=2 -> 5*8 = 40
    expect(evaluateModel(regression, 2, true, true)).toBeCloseTo(40, 6);
  });
});

describe("formatModelEquation", () => {
  it("uses plain y/x for a linear fit", () => {
    const html = formatModelEquation(makeRegression({}));
    expect(html).toContain("<i>y</i>");
    expect(html).toContain("<i>x</i>");
    expect(html).not.toContain("log");
  });

  it("labels the x term as log10(x) for an x_log fit", () => {
    const html = formatModelEquation(makeRegression({ x_log: true }));
    expect(html).toContain("log₁₀(<i>x</i>)");
    expect(html.startsWith("<i>y</i>")).toBe(true);
  });

  it("labels the left-hand side as log10(y) for a y_log fit", () => {
    const html = formatModelEquation(makeRegression({ y_log: true }));
    expect(html).toContain("log₁₀(<i>y</i>)");
    expect(html).toContain("<i>x</i>");
    expect(html).not.toContain("log₁₀(<i>x</i>)");
  });

  it("labels both sides as log10 for a log-log fit", () => {
    const html = formatModelEquation(
      makeRegression({ x_log: true, y_log: true }),
    );
    expect(html).toContain("log₁₀(<i>y</i>)");
    expect(html).toContain("log₁₀(<i>x</i>)");
  });
});
