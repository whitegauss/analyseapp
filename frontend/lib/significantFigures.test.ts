import { describe, expect, it } from "vitest";
import {
  formatToPrecision,
  formatUncertainty,
  roundToUncertainty,
} from "./significantFigures";

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

describe("formatToPrecision", () => {
  it("keeps the digits of a round integer instead of stripping its zeros", () => {
    // "1000".toPrecision(4) has no decimal point, so trimming trailing zeros
    // would report a partial derivative of 1000 as 1.
    expect(formatToPrecision(1000)).toBe("1000");
    expect(formatToPrecision(1200)).toBe("1200");
    expect(formatToPrecision(2000)).toBe("2000");
  });

  it("trims trailing zeros that follow a decimal point", () => {
    expect(formatToPrecision(1.5)).toBe("1.5");
    expect(formatToPrecision(0.25)).toBe("0.25");
  });

  it("switches to exponential notation outside the readable range", () => {
    expect(formatToPrecision(1234567)).toBe("1.23e+6");
    expect(formatToPrecision(0.0000123)).toBe("1.23e-5");
  });

  it("reports zero and non-finite values without NaN leaking into the UI", () => {
    expect(formatToPrecision(0)).toBe("0");
    expect(formatToPrecision(Infinity)).toBe("—");
    expect(formatToPrecision(NaN)).toBe("—");
  });
});
