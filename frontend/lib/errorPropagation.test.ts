import { describe, expect, it } from "vitest";
import { propagateFormula } from "./errorPropagation";
import { FormulaError } from "./formula";

describe("propagateFormula", () => {
  it("combines uncertainties in quadrature for a sum", () => {
    const { value, uncertainty } = propagateFormula("x+y", {
      x: { value: 5, uncertainty: 0.2 },
      y: { value: 3, uncertainty: 0.1 },
    });
    expect(value).toBeCloseTo(8, 10);
    expect(uncertainty).toBeCloseTo(Math.sqrt(0.2 ** 2 + 0.1 ** 2), 10);
  });

  it("gives a difference the same uncertainty as the sum", () => {
    const { value, uncertainty } = propagateFormula("x-y", {
      x: { value: 5, uncertainty: 0.2 },
      y: { value: 3, uncertainty: 0.1 },
    });
    expect(value).toBeCloseTo(2, 10);
    expect(uncertainty).toBeCloseTo(Math.sqrt(0.2 ** 2 + 0.1 ** 2), 10);
  });

  it("uses the product rule for a product", () => {
    const { value, uncertainty } = propagateFormula("x*y", {
      x: { value: 5, uncertainty: 0.2 },
      y: { value: 3, uncertainty: 0.1 },
    });
    expect(value).toBeCloseTo(15, 10);
    expect(uncertainty).toBeCloseTo(
      Math.sqrt((3 * 0.2) ** 2 + (5 * 0.1) ** 2),
      10,
    );
  });

  it("uses the quotient rule for a ratio", () => {
    const { value, uncertainty } = propagateFormula("x/y", {
      x: { value: 6, uncertainty: 0.3 },
      y: { value: 2, uncertainty: 0.1 },
    });
    expect(value).toBeCloseTo(3, 10);
    expect(uncertainty).toBeCloseTo(
      Math.sqrt((0.3 / 2) ** 2 + ((6 * 0.1) / 4) ** 2),
      10,
    );
  });

  it("propagates only the base's uncertainty when the exponent is a literal", () => {
    const { value, uncertainty } = propagateFormula("x^3", {
      x: { value: 2, uncertainty: 0.1 },
    });
    expect(value).toBeCloseTo(8, 10);
    expect(uncertainty).toBeCloseTo(Math.abs(3 * 2 ** 2) * 0.1, 10);
  });

  it("matches the textbook relative-uncertainty form of a pendulum period", () => {
    // T = 2π√(L/g); σT/T = ½·√((σL/L)² + (σg/g)²).
    const L = { value: 1.234, uncertainty: 0.002 };
    const g = { value: 9.8, uncertainty: 0.05 };
    const { value, uncertainty } = propagateFormula("2*pi*sqrt(L/g)", { L, g });

    const expectedValue = 2 * Math.PI * Math.sqrt(L.value / g.value);
    const expectedRelative =
      0.5 *
      Math.sqrt(
        (L.uncertainty / L.value) ** 2 + (g.uncertainty / g.value) ** 2,
      );
    expect(value).toBeCloseTo(expectedValue, 10);
    expect(uncertainty).toBeCloseTo(expectedValue * expectedRelative, 10);
  });

  it("reports each variable's share of the variance, summing to 1", () => {
    const { terms } = propagateFormula("x+y", {
      x: { value: 1, uncertainty: 3 },
      y: { value: 1, uncertainty: 4 },
    });
    // σ = 5, so x accounts for 9/25 of the variance and y for 16/25.
    expect(terms.map((t) => t.name)).toEqual(["x", "y"]);
    expect(terms[0].share).toBeCloseTo(0.36, 10);
    expect(terms[1].share).toBeCloseTo(0.64, 10);
    expect(terms[0].share + terms[1].share).toBeCloseTo(1, 10);
  });

  it("reports the partial derivative and the contribution separately", () => {
    // A variable can matter a lot per unit (large ∂z/∂x) and still barely
    // move the result if it was measured precisely -- the tool has to show
    // both to be worth anything.
    const { terms } = propagateFormula("100*x + y", {
      x: { value: 1, uncertainty: 0.001 },
      y: { value: 1, uncertainty: 1 },
    });
    const [x, y] = terms;
    expect(x.partial).toBeCloseTo(100, 10);
    expect(x.contribution).toBeCloseTo(0.1, 10);
    expect(y.partial).toBeCloseTo(1, 10);
    expect(y.contribution).toBeCloseTo(1, 10);
    expect(y.share).toBeGreaterThan(x.share);
  });

  it("drops exact quantities (σ = 0) from the total but keeps them in the value", () => {
    const withExactG = propagateFormula("m*g*h", {
      m: { value: 2, uncertainty: 0.01 },
      g: { value: 9.80665, uncertainty: 0 },
      h: { value: 3, uncertainty: 0.02 },
    });
    expect(withExactG.value).toBeCloseTo(2 * 9.80665 * 3, 10);
    expect(withExactG.terms[1].contribution).toBe(0);
    expect(withExactG.terms[1].share).toBe(0);
    expect(withExactG.uncertainty).toBeCloseTo(
      Math.sqrt((9.80665 * 3 * 0.01) ** 2 + (2 * 9.80665 * 0.02) ** 2),
      10,
    );
  });

  it("reports zero shares rather than NaN when every input is exact", () => {
    const { uncertainty, terms } = propagateFormula("x*y", {
      x: { value: 2, uncertainty: 0 },
      y: { value: 3, uncertainty: 0 },
    });
    expect(uncertainty).toBe(0);
    expect(terms.every((term) => term.share === 0)).toBe(true);
  });

  it("propagates variables named after Object.prototype members", () => {
    const { value, uncertainty, terms } = propagateFormula(
      "__proto__ + constructor",
      {
        ["__proto__"]: { value: 2, uncertainty: 0.3 },
        constructor: { value: 3, uncertainty: 0.4 },
      },
    );
    expect(value).toBe(5);
    expect(uncertainty).toBeCloseTo(0.5, 10);
    expect(terms.map((t) => t.name)).toEqual(["__proto__", "constructor"]);
  });

  it("reports a non-finite uncertainty rather than hiding it as zero", () => {
    // sqrt(x) at x = 0: the value is finite but ∂z/∂x is not, so callers have
    // to be able to see that the linear approximation broke down here.
    const { value, uncertainty } = propagateFormula("sqrt(x)", {
      x: { value: 0, uncertainty: 0.1 },
    });
    expect(value).toBe(0);
    expect(Number.isFinite(uncertainty)).toBe(false);
  });

  it("refuses a formula whose variables were not all given values", () => {
    expect(() =>
      propagateFormula("x+y", { x: { value: 1, uncertainty: 0 } }),
    ).toThrow(FormulaError);
  });
});
