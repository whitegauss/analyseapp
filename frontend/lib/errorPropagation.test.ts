import { describe, expect, it } from "vitest";
import { propagateError } from "./errorPropagation";

describe("propagateError", () => {
  it("adds values and combines uncertainties in quadrature", () => {
    const { value, uncertainty } = propagateError("add", 5, 0.2, 3, 0.1);
    expect(value).toBeCloseTo(8, 10);
    expect(uncertainty).toBeCloseTo(Math.sqrt(0.2 ** 2 + 0.1 ** 2), 10);
  });

  it("subtracts values and combines uncertainties in quadrature (same as add)", () => {
    const { value, uncertainty } = propagateError("subtract", 5, 0.2, 3, 0.1);
    expect(value).toBeCloseTo(2, 10);
    expect(uncertainty).toBeCloseTo(Math.sqrt(0.2 ** 2 + 0.1 ** 2), 10);
  });

  it("multiplies values using the product rule for uncertainty", () => {
    const { value, uncertainty } = propagateError("multiply", 5, 0.2, 3, 0.1);
    expect(value).toBeCloseTo(15, 10);
    expect(uncertainty).toBeCloseTo(
      Math.sqrt((3 * 0.2) ** 2 + (5 * 0.1) ** 2),
      10,
    );
  });

  it("divides values using the quotient rule for uncertainty", () => {
    const { value, uncertainty } = propagateError("divide", 6, 0.3, 2, 0.1);
    expect(value).toBeCloseTo(3, 10);
    expect(uncertainty).toBeCloseTo(
      Math.sqrt((0.3 / 2) ** 2 + ((6 * 0.1) / 4) ** 2),
      10,
    );
  });

  it("raises to an exact power, propagating only the base's uncertainty", () => {
    const { value, uncertainty } = propagateError("power", 2, 0.1, 3, 0);
    expect(value).toBeCloseTo(8, 10);
    expect(uncertainty).toBeCloseTo(Math.abs(3 * 2 ** 2) * 0.1, 10);
  });

  it("ignores sy entirely for power (exponent has no uncertainty)", () => {
    const withZeroSy = propagateError("power", 2, 0.1, 3, 0);
    const withNonZeroSy = propagateError("power", 2, 0.1, 3, 999);
    expect(withNonZeroSy).toEqual(withZeroSy);
  });
});
