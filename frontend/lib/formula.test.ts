import { describe, expect, it } from "vitest";
import { evaluateWithGradient, FormulaError, parseFormula } from "./formula";

function evaluate(source: string, values: Record<string, number> = {}) {
  return evaluateWithGradient(parseFormula(source).ast, values);
}

describe("parseFormula", () => {
  it("collects variables in order of first appearance, without duplicates", () => {
    const { variables } = parseFormula("m*g*h + m*v^2/2");
    expect(variables).toEqual(["m", "g", "h", "v"]);
  });

  it("does not treat pi and e as variables", () => {
    expect(parseFormula("2*pi*r + e").variables).toEqual(["r"]);
  });

  it("accepts Greek and subscript-ish names physicists write", () => {
    expect(parseFormula("θ + λ_0").variables).toEqual(["θ", "λ_0"]);
  });

  it("reads exponent notation as a single number", () => {
    expect(evaluate("1.6e-19").value).toBeCloseTo(1.6e-19, 30);
    expect(evaluate("2e3").value).toBe(2000);
  });

  it.each([
    ["式が空です", "   "],
    ["閉じ括弧が足りません", "2*(3+4"],
    ["式が途中で終わっています", "2*"],
    ["使えない文字です", "2 & 3"],
    ["知らない関数です", "foo(2)"],
    ["引数が 1 個です", "sqrt(2, 3)"],
    ["括弧が要ります", "sqrt"],
    ["log は底が曖昧です", "log(100)"],
  ])("rejects with %s", (message, source) => {
    expect(() => parseFormula(source)).toThrow(FormulaError);
    expect(() => parseFormula(source)).toThrow(new RegExp(message));
  });
});

describe("evaluateWithGradient", () => {
  it("applies the usual precedence and associativity", () => {
    expect(evaluate("2+3*4").value).toBe(14);
    expect(evaluate("(2+3)*4").value).toBe(20);
    // ^ binds tighter than unary minus and associates to the right.
    expect(evaluate("-2^2").value).toBe(-4);
    expect(evaluate("2^3^2").value).toBe(512);
    expect(evaluate("2^-2").value).toBe(0.25);
  });

  it("differentiates a product by the product rule", () => {
    const { value, gradient } = evaluate("m*g*h", { m: 2, g: 9.8, h: 3 });
    expect(value).toBeCloseTo(58.8, 10);
    expect(gradient.m).toBeCloseTo(9.8 * 3, 10);
    expect(gradient.g).toBeCloseTo(2 * 3, 10);
    expect(gradient.h).toBeCloseTo(2 * 9.8, 10);
  });

  it("differentiates a quotient by the quotient rule", () => {
    const { gradient } = evaluate("x/y", { x: 6, y: 2 });
    expect(gradient.x).toBeCloseTo(1 / 2, 10);
    expect(gradient.y).toBeCloseTo(-6 / 4, 10);
  });

  it("adds up the partials when a variable appears more than once", () => {
    // d/dx (x*x) = 2x, which only comes out right if both occurrences
    // contribute instead of the second overwriting the first.
    expect(evaluate("x*x", { x: 5 }).gradient.x).toBeCloseTo(10, 10);
    expect(evaluate("x + x^2", { x: 3 }).gradient.x).toBeCloseTo(1 + 6, 10);
  });

  it("differentiates a power with a measured exponent on both sides", () => {
    const { gradient } = evaluate("x^n", { x: 2, n: 3 });
    expect(gradient.x).toBeCloseTo(3 * 2 ** 2, 10);
    expect(gradient.n).toBeCloseTo(2 ** 3 * Math.log(2), 10);
  });

  it("keeps a negative base usable when the exponent is a plain constant", () => {
    // ln(-2) is NaN, so the exponent's own term has to stay out of the
    // gradient entirely rather than be multiplied by a zero partial.
    const { value, gradient } = evaluate("x^2", { x: -2 });
    expect(value).toBe(4);
    expect(gradient.x).toBeCloseTo(-4, 10);
  });

  it("differentiates the supported functions by the chain rule", () => {
    expect(evaluate("sqrt(x)", { x: 9 }).gradient.x).toBeCloseTo(1 / 6, 10);
    expect(evaluate("ln(x)", { x: 4 }).gradient.x).toBeCloseTo(0.25, 10);
    expect(evaluate("log10(x)", { x: 100 }).gradient.x).toBeCloseTo(
      1 / (100 * Math.LN10),
      10,
    );
    expect(evaluate("exp(2*x)", { x: 1 }).gradient.x).toBeCloseTo(
      2 * Math.exp(2),
      10,
    );
    expect(evaluate("sin(x)", { x: 0 }).gradient.x).toBeCloseTo(1, 10);
    expect(evaluate("atan2(y, x)", { y: 1, x: 1 }).gradient.y).toBeCloseTo(
      0.5,
      10,
    );
  });

  it("treats trigonometric arguments as radians", () => {
    expect(evaluate("sin(pi/2)").value).toBeCloseTo(1, 10);
    expect(evaluate("sin(theta*pi/180)", { theta: 30 }).value).toBeCloseTo(
      0.5,
      10,
    );
  });

  it("agrees with a central finite difference on a mixed formula", () => {
    // An independent check on the chain-rule bookkeeping: the derivative the
    // AD produces has to match the slope actually measured on the function.
    const source = "2*pi*sqrt(L/g) + sin(L)*ln(g)";
    const at = { L: 1.234, g: 9.8 };
    const { gradient } = evaluate(source, at);

    for (const name of ["L", "g"] as const) {
      const step = 1e-6 * Math.abs(at[name]);
      const forward = evaluate(source, {
        ...at,
        [name]: at[name] + step,
      }).value;
      const backward = evaluate(source, {
        ...at,
        [name]: at[name] - step,
      }).value;
      expect(gradient[name]).toBeCloseTo((forward - backward) / (2 * step), 5);
    }
  });

  it("reports a missing value instead of computing with undefined", () => {
    expect(() => evaluate("x+y", { x: 1 })).toThrow(FormulaError);
  });
});
