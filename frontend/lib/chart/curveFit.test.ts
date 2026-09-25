import { describe, expect, it } from "vitest";
import {
  CURVE_SAMPLES,
  formatCurveLegend,
  formatFitParameter,
  sampleCurve,
} from "./curveFit";

describe("sampleCurve", () => {
  it("evaluates the formula with the fitted values across the bounds", () => {
    const { x, y } = sampleCurve(
      "a*x^2 + b",
      [
        { name: "a", value: 2, stderr: 0.1 },
        { name: "b", value: 1, stderr: 0.1 },
      ],
      [0, 4],
      false,
      5,
    );
    expect(x).toEqual([0, 1, 2, 3, 4]);
    expect(y).toEqual([1, 3, 9, 19, 33]);
  });

  it("uses CURVE_SAMPLES points by default, ending exactly on the bounds", () => {
    const { x } = sampleCurve(
      "a*x",
      [{ name: "a", value: 1, stderr: null }],
      [-1, 3],
      false,
    );
    expect(x).toHaveLength(CURVE_SAMPLES);
    expect(x[0]).toBe(-1);
    expect(x[x.length - 1]).toBe(3);
  });

  it("spaces samples evenly in log10(x) on a log axis", () => {
    const { x } = sampleCurve(
      "a*x",
      [{ name: "a", value: 1, stderr: null }],
      [1, 1000],
      true,
      4,
    );
    expect(x.map((v) => Math.round(v))).toEqual([1, 10, 100, 1000]);
  });

  it("leaves a gap (null) where the model is not finite", () => {
    const { y } = sampleCurve(
      "1/(x - a)",
      [{ name: "a", value: 1, stderr: 0.1 }],
      [0, 2],
      false,
      3,
    );
    expect(y).toEqual([-1, null, 1]);
  });

  it("gives every sample a value for a formula without x", () => {
    const { y } = sampleCurve(
      "c",
      [{ name: "c", value: 4, stderr: 0.1 }],
      [0, 1],
      false,
      3,
    );
    expect(y).toEqual([4, 4, 4]);
  });
});

describe("formatFitParameter", () => {
  it("rounds the value to its uncertainty", () => {
    expect(
      formatFitParameter({ name: "A", value: 3.0959, stderr: 0.0627 }),
    ).toBe("A = 3.10 ± 0.06");
  });

  it("says the error is unknown rather than writing ± 0", () => {
    expect(
      formatFitParameter({ name: "k", value: 0.0012346, stderr: null }),
    ).toBe("k = 0.001235（誤差算出不可）");
  });
});

describe("formatCurveLegend", () => {
  it("prefixes the formula with an italic y", () => {
    expect(formatCurveLegend("A*exp(-x/tau)")).toBe("<i>y</i> = A*exp(-x/tau)");
  });
});
