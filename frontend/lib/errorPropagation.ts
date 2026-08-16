export type PropagationOperation =
  "add" | "subtract" | "multiply" | "divide" | "power";

export type PropagationResult = { value: number; uncertainty: number };

// Standard first-order (linear) error propagation for two independent,
// uncorrelated measurements x±sx and y±sy: σz = sqrt(sum of (∂z/∂xi * σxi)^2).
// For "power" (z = x^n), y doubles as the exponent n and is treated as an
// exact constant with no uncertainty of its own (sy is ignored).
export function propagateError(
  operation: PropagationOperation,
  x: number,
  sx: number,
  y: number,
  sy: number,
): PropagationResult {
  switch (operation) {
    case "add":
      return { value: x + y, uncertainty: Math.sqrt(sx ** 2 + sy ** 2) };
    case "subtract":
      return { value: x - y, uncertainty: Math.sqrt(sx ** 2 + sy ** 2) };
    case "multiply":
      return {
        value: x * y,
        uncertainty: Math.sqrt((y * sx) ** 2 + (x * sy) ** 2),
      };
    case "divide":
      return {
        value: x / y,
        uncertainty: Math.sqrt((sx / y) ** 2 + ((x * sy) / y ** 2) ** 2),
      };
    case "power":
      return {
        value: Math.pow(x, y),
        uncertainty: Math.abs(y * Math.pow(x, y - 1)) * sx,
      };
  }
}
