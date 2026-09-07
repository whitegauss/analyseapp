import {
  evaluateWithGradient,
  FormulaError,
  parseFormula,
  type ParsedFormula,
} from "./formula";

export type MeasuredValue = {
  value: number;
  // 1σ uncertainty. Zero means the quantity is treated as exact (a defined
  // constant such as g = 9.80665, or a counted number), which drops it out of
  // the propagated total without removing it from the formula.
  uncertainty: number;
};

export type PropagationTerm = {
  name: string;
  // ∂z/∂x at the given values -- the sensitivity of the result to this
  // variable, independent of how well it was measured.
  partial: number;
  // |∂z/∂x|·σx: how much of the final uncertainty this variable is
  // responsible for, in the units of the result.
  contribution: number;
  // contribution² / σz², i.e. this variable's share of the variance. The
  // shares sum to 1, which is what makes "measure this one better" a
  // decision the numbers can support.
  share: number;
};

export type PropagationResult = {
  value: number;
  uncertainty: number;
  // Ordered as the variables appear in the formula, not by size, so the rows
  // line up with the input fields.
  terms: PropagationTerm[];
};

// First-order (linear) propagation for a formula of independent, uncorrelated
// measurements: σz² = Σ (∂z/∂xi · σxi)².
//
// "First-order" is the assumption to keep in mind: the formula is treated as
// linear over the range of each σ, which is the standard lab convention and
// is accurate as long as the uncertainties are small compared to the values.
// Correlated inputs would need the covariance terms this deliberately omits.
export function propagate(
  formula: ParsedFormula,
  variables: Record<string, MeasuredValue>,
): PropagationResult {
  // Object.create(null) / hasOwn throughout: variable names come from the
  // formula the user typed, and `constructor` or `__proto__` must behave like
  // any other name rather than resolving to something inherited.
  const values: Record<string, number> = Object.create(null);
  for (const name of formula.variables) {
    const measured = Object.hasOwn(variables, name)
      ? variables[name]
      : undefined;
    if (!measured) {
      throw new FormulaError(`${name} の値がありません`, 0);
    }
    values[name] = measured.value;
  }

  const { value, gradient } = evaluateWithGradient(formula.ast, values);

  const contributions = formula.variables.map((name) => {
    const partial = Object.hasOwn(gradient, name) ? gradient[name] : 0;
    return {
      name,
      partial,
      contribution: Math.abs(partial * variables[name].uncertainty),
    };
  });

  const variance = contributions.reduce(
    (total, term) => total + term.contribution ** 2,
    0,
  );
  const uncertainty = Math.sqrt(variance);

  return {
    value,
    uncertainty,
    terms: contributions.map((term) => ({
      ...term,
      // A zero total means every input was exact; the shares are then 0
      // rather than NaN, which is what an exact result should report.
      share: variance > 0 ? term.contribution ** 2 / variance : 0,
    })),
  };
}

// Convenience wrapper for callers holding a formula as text. Throws
// FormulaError on a syntax problem, the same as parseFormula.
export function propagateFormula(
  source: string,
  variables: Record<string, MeasuredValue>,
): PropagationResult {
  return propagate(parseFormula(source), variables);
}
