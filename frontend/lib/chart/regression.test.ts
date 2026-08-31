import { describe, expect, it } from "vitest";
import {
  evaluateModel,
  formatModelEquation,
  formatSigned,
  formatSignedWithUncertainty,
  mathItalicHtml,
  regressionCacheKey,
  safeStderr,
  uncertaintyBoundsAt,
} from "./regression";
import type { LinearRegressionResult } from "@/lib/experiment";

function fit(overrides: Partial<LinearRegressionResult> = {}) {
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
  it("evaluates a plain linear fit", () => {
    expect(evaluateModel(fit(), 5, false, false)).toBeCloseTo(11, 10);
  });

  it("undoes an x-log transform: y = slope*log10(x) + intercept", () => {
    expect(evaluateModel(fit({ x_log: true }), 100, true, false)).toBeCloseTo(
      5,
      10,
    );
  });

  it("undoes a y-log transform: y = 10^(slope*x + intercept)", () => {
    const r = fit({ slope: 0.5, intercept: 1, y_log: true });
    expect(evaluateModel(r, 4, false, true)).toBeCloseTo(1000, 6);
  });

  it("undoes both, giving a power law y = 10^b * x^a", () => {
    const r = fit({
      slope: 3,
      intercept: Math.log10(5),
      x_log: true,
      y_log: true,
    });
    expect(evaluateModel(r, 2, true, true)).toBeCloseTo(40, 6);
  });
});

describe("safeStderr", () => {
  it("passes a usable stderr through", () => {
    expect(safeStderr(0.25)).toBe(0.25);
  });

  it.each([
    ["zero", 0],
    ["negative", -1],
    ["NaN", NaN],
    ["Infinity", Infinity],
  ])("collapses %s to zero uncertainty", (_label, value) => {
    expect(safeStderr(value)).toBe(0);
  });
});

describe("formatSigned", () => {
  it.each([
    [1.234, 2, "+ 1.23"],
    [-1.234, 2, "- 1.23"],
    [0, 1, "+ 0.0"],
    [-0.004, 2, "- 0.00"],
  ])("formats %d to %d decimals as %s", (value, decimals, want) => {
    expect(formatSigned(value, decimals)).toBe(want);
  });
});

describe("formatSignedWithUncertainty", () => {
  it("parenthesises the value and its uncertainty", () => {
    expect(formatSignedWithUncertainty(1.23, 2, 0.05)).toBe("+ (1.23 ± 0.05)");
  });

  it("moves the sign outside the parentheses", () => {
    expect(formatSignedWithUncertainty(-1.23, 2, 0.05)).toBe("- (1.23 ± 0.05)");
  });
});

describe("uncertaintyBoundsAt", () => {
  it("brackets the fitted value", () => {
    const r = fit();
    const { lower, upper } = uncertaintyBoundsAt(r, 5, false, false);
    const centre = evaluateModel(r, 5, false, false);
    expect(lower).toBeLessThan(centre);
    expect(upper).toBeGreaterThan(centre);
  });

  it("takes the widest of the four +/- combinations", () => {
    // slope 2±0.1, intercept 1±0.1, x = 5
    // widest: (2.1)(5) + 1.1 = 11.6 and (1.9)(5) + 0.9 = 10.4
    const { lower, upper } = uncertaintyBoundsAt(fit(), 5, false, false);
    expect(lower).toBeCloseTo(10.4, 10);
    expect(upper).toBeCloseTo(11.6, 10);
  });

  it("collapses onto the line when both stderrs are zero", () => {
    const r = fit({ slope_stderr: 0, intercept_stderr: 0 });
    const { lower, upper } = uncertaintyBoundsAt(r, 5, false, false);
    expect(lower).toBeCloseTo(11, 10);
    expect(upper).toBeCloseTo(11, 10);
  });

  it("collapses onto the line when the stderrs are unusable", () => {
    const r = fit({ slope_stderr: NaN, intercept_stderr: -1 });
    const { lower, upper } = uncertaintyBoundsAt(r, 5, false, false);
    expect(lower).toBeCloseTo(11, 10);
    expect(upper).toBeCloseTo(11, 10);
  });

  it("widens as x moves away from the intercept", () => {
    const near = uncertaintyBoundsAt(fit(), 1, false, false);
    const far = uncertaintyBoundsAt(fit(), 100, false, false);
    expect(far.upper - far.lower).toBeGreaterThan(near.upper - near.lower);
  });

  it("returns real (non-log) bounds for a y-log fit", () => {
    const r = fit({ slope: 0.5, intercept: 1, y_log: true });
    const { lower, upper } = uncertaintyBoundsAt(r, 4, false, true);
    // centre is 10^3 = 1000; the band must straddle it in real space
    expect(lower).toBeLessThan(1000);
    expect(upper).toBeGreaterThan(1000);
    expect(lower).toBeGreaterThan(0);
  });

  it("takes log10 of x for an x-log fit", () => {
    const r = fit({ slope_stderr: 0, intercept_stderr: 0, x_log: true });
    const { lower } = uncertaintyBoundsAt(r, 100, true, false);
    // 2*log10(100) + 1 = 5
    expect(lower).toBeCloseTo(5, 10);
  });
});

describe("mathItalicHtml", () => {
  it("wraps math spans in <i> and leaves the rest alone", () => {
    expect(mathItalicHtml("速度 $v$ (m/s)")).toBe("速度 <i>v</i> (m/s)");
  });

  it("returns plain text unchanged", () => {
    expect(mathItalicHtml("no math")).toBe("no math");
  });
});

describe("formatModelEquation", () => {
  it("uses plain y and x for a linear fit", () => {
    const html = formatModelEquation(fit());
    expect(html).toContain("<i>y</i>");
    expect(html).toContain("<i>x</i>");
    expect(html).not.toContain("log");
  });

  it("labels the x term as log10(x) for an x-log fit", () => {
    const html = formatModelEquation(fit({ x_log: true }));
    expect(html).toContain("log₁₀(<i>x</i>)");
    expect(html.startsWith("<i>y</i>")).toBe(true);
  });

  it("labels the left-hand side as log10(y) for a y-log fit", () => {
    const html = formatModelEquation(fit({ y_log: true }));
    expect(html).toContain("log₁₀(<i>y</i>)");
    expect(html).not.toContain("log₁₀(<i>x</i>)");
  });

  it("labels both sides for a log-log fit", () => {
    const html = formatModelEquation(fit({ x_log: true, y_log: true }));
    expect(html).toContain("log₁₀(<i>y</i>)");
    expect(html).toContain("log₁₀(<i>x</i>)");
  });

  it("carries the uncertainty on both terms", () => {
    expect(formatModelEquation(fit())).toContain("±");
  });
});

describe("regressionCacheKey", () => {
  it("gives each axis-scale combination its own key", () => {
    const keys = [
      regressionCacheKey(false, false),
      regressionCacheKey(true, false),
      regressionCacheKey(false, true),
      regressionCacheKey(true, true),
    ];
    expect(new Set(keys).size).toBe(4);
  });

  it("is stable for the same combination", () => {
    expect(regressionCacheKey(true, false)).toBe(
      regressionCacheKey(true, false),
    );
  });
});
