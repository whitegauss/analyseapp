import { describe, expect, it } from "vitest";
import {
  evaluateModel,
  formatModelEquation,
  formatUncertainty,
  logTicks,
  roundToUncertainty,
  type LinearRegressionResult,
} from "./ExperimentChart";

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

describe("roundToUncertainty", () => {
  it("rounds to the uncertainty's leading (1 significant figure) decimal place", () => {
    const { rounded, decimals } = roundToUncertainty(2.03847, 0.0523);
    expect(decimals).toBe(2);
    expect(rounded).toBeCloseTo(2.04, 10);
  });

  it("handles a one-digit (ones-place) uncertainty", () => {
    const { rounded, decimals } = roundToUncertainty(17.6, 5.23);
    expect(decimals).toBe(0);
    expect(rounded).toBe(18);
  });

  it("handles an uncertainty greater than 10 by rounding to the nearest ten", () => {
    const { rounded, decimals } = roundToUncertainty(517, 52.3);
    expect(decimals).toBe(0);
    expect(rounded).toBe(520);
  });

  it("handles a small uncertainty (three leading zeros)", () => {
    const { rounded, decimals } = roundToUncertainty(1.23456, 0.000891);
    expect(decimals).toBe(4);
    expect(rounded).toBeCloseTo(1.2346, 10);
  });

  it("caps displayed decimals at 4 even when the uncertainty's leading digit is finer", () => {
    const { rounded, decimals } = roundToUncertainty(1.234567, 0.0000523);
    expect(decimals).toBe(4);
    expect(rounded.toFixed(decimals)).toBe("1.2346");
  });

  it("truncates the value at the uncertainty's leading digit when the uncertainty spans multiple digits (hundreds place)", () => {
    const { rounded, decimals } = roundToUncertainty(45678, 837);
    expect(decimals).toBe(0);
    expect(rounded).toBe(45700);
  });

  it("truncates the value at the uncertainty's leading digit for a thousands-place uncertainty", () => {
    const { rounded, decimals } = roundToUncertainty(45678, 8370);
    expect(decimals).toBe(0);
    expect(rounded).toBe(46000);
  });

  it("falls back to a fixed precision when the uncertainty is zero, negative, or non-finite", () => {
    expect(roundToUncertainty(2.03847, 0)).toEqual({
      rounded: 2.03847,
      decimals: 4,
    });
    expect(roundToUncertainty(2.03847, -1)).toEqual({
      rounded: 2.03847,
      decimals: 4,
    });
    expect(roundToUncertainty(2.03847, NaN)).toEqual({
      rounded: 2.03847,
      decimals: 4,
    });
  });
});

describe("formatUncertainty", () => {
  it("rounds an uncertainty to its own leading significant figure", () => {
    expect(formatUncertainty(0.0523)).toBe("0.05");
    expect(formatUncertainty(5.23)).toBe("5");
    expect(formatUncertainty(52.3)).toBe("50");
  });

  it("truncates a large (multi-digit) uncertainty down to its own leading digit", () => {
    expect(formatUncertainty(837)).toBe("800");
    expect(formatUncertainty(8370)).toBe("8000");
  });

  it("displays 0.0 for a zero, negative, or non-finite uncertainty (e.g. cancellation on a perfect fit)", () => {
    expect(formatUncertainty(0)).toBe("0.0");
    expect(formatUncertainty(-1)).toBe("0.0");
    expect(formatUncertainty(NaN)).toBe("0.0");
    expect(formatUncertainty(Infinity)).toBe("0.0");
  });
});

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

describe("logTicks", () => {
  it("places ticks at the 1-2-5 series within a single decade", () => {
    const { tickvals } = logTicks(1, 9);
    expect(tickvals).toEqual([1, 2, 5]);
  });

  it("labels every tick with its full, untruncated value (no dropped zeros)", () => {
    const { tickvals, ticktext } = logTicks(2, 1000);
    const asPairs = Object.fromEntries(
      tickvals.map((v, i) => [v, ticktext[i]]),
    );
    expect(asPairs[2]).toBe("2");
    expect(asPairs[5]).toBe("5");
    expect(asPairs[20]).toBe("20");
    expect(asPairs[50]).toBe("50");
    expect(asPairs[100]).toBe("100");
    expect(asPairs[200]).toBe("200");
    expect(asPairs[500]).toBe("500");
    expect(asPairs[1000]).toBe("1000");
  });

  it("spans multiple decades with the 1-2-5 series repeated in each", () => {
    const { tickvals } = logTicks(1, 1000);
    expect(tickvals).toEqual([1, 2, 5, 10, 20, 50, 100, 200, 500, 1000]);
  });

  it("excludes ticks outside the given bounds", () => {
    const { tickvals } = logTicks(3, 40);
    expect(tickvals).toEqual([5, 10, 20]);
  });

  it("returns empty arrays for non-positive or inverted bounds", () => {
    expect(logTicks(0, 100)).toEqual({ tickvals: [], ticktext: [] });
    expect(logTicks(-5, 100)).toEqual({ tickvals: [], ticktext: [] });
    expect(logTicks(100, 1)).toEqual({ tickvals: [], ticktext: [] });
  });
});
