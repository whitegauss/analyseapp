import {
  propagate,
  type MeasuredValue,
  type PropagationResult,
} from "@/lib/errorPropagation";
import { FormulaError, parseFormula, type ParsedFormula } from "@/lib/formula";
import {
  formatUncertainty,
  roundToUncertainty,
} from "@/lib/significantFigures";

// The error-propagation calculator's form, read without React: what was
// typed for each variable, and the result that follows from it. The
// propagation itself is errorPropagation.ts's and tested there.

// Entered text is kept per variable name rather than per row, so editing the
// formula (adding a term, fixing a typo) keeps the numbers already typed for
// every name that survives the edit.
export type Entry = { value: string; uncertainty: string };
export type Entries = Record<string, Entry>;

/**
 * Variable names come from the formula the user typed, so a plain property
 * read would resolve `constructor` or `toString` to something inherited --
 * and `entry.value.trim()` on a function is a crash, not a wrong number.
 */
export function entryOf(entries: Entries, name: string): Entry | undefined {
  return Object.hasOwn(entries, name) ? entries[name] : undefined;
}

/** `entries` with `name`'s fields replaced by `patch`, as a new object. */
export function withEntry(
  entries: Entries,
  name: string,
  patch: Partial<Entry>,
): Entries {
  const next: Entries = Object.create(null);
  Object.assign(next, entries);
  const existing = entryOf(entries, name);
  next[name] = {
    value: existing?.value ?? "",
    uncertainty: existing?.uncertainty ?? "",
    ...patch,
  };
  return next;
}

/** Whether every variable has a value typed (σ may be blank). */
export function allValuesFilled(
  variables: string[],
  entries: Entries,
): boolean {
  return (
    variables.length > 0 &&
    variables.every(
      (name) => (entryOf(entries, name)?.value ?? "").trim() !== "",
    )
  );
}

/**
 * The propagated result, or null while any value is missing or not a
 * number, or when the formula cannot be evaluated there.
 */
export function computePropagation(
  formula: ParsedFormula | null,
  variables: string[],
  entries: Entries,
): PropagationResult | null {
  if (!formula || variables.length === 0) return null;

  const measured: Record<string, MeasuredValue> = Object.create(null);
  for (const name of variables) {
    const entry = entryOf(entries, name);
    if (!entry || entry.value.trim() === "") return null;
    const value = Number(entry.value);
    // A blank uncertainty means "exact", which is how constants written
    // into the formula as a name (g, c, ...) are meant to behave.
    const uncertainty =
      entry.uncertainty.trim() === "" ? 0 : Number(entry.uncertainty);
    if (!Number.isFinite(value) || !Number.isFinite(uncertainty)) return null;
    measured[name] = { value, uncertainty: Math.abs(uncertainty) };
  }

  try {
    const propagated = propagate(formula, measured);
    // The uncertainty is checked as well as the value: sqrt(x) at x = 0 has
    // a finite value and an infinite derivative, and formatUncertainty
    // renders a non-finite uncertainty as "0.0" -- an exact-looking answer
    // to a formula that is anything but.
    if (
      !Number.isFinite(propagated.value) ||
      !Number.isFinite(propagated.uncertainty)
    ) {
      return null;
    }
    return propagated;
  } catch {
    return null;
  }
}

export type FormulaInput = {
  formula: ParsedFormula | null;
  // The parser's message for a formula that does not parse; null for one
  // that does, and for an empty field (nothing typed is not a mistake).
  error: string | null;
};

/** The formula field, parsed. Only a FormulaError is caught -- anything
 * else is a bug and is left to surface. */
export function readFormula(source: string): FormulaInput {
  if (source.trim() === "") return { formula: null, error: null };
  try {
    return { formula: parseFormula(source), error: null };
  } catch (e) {
    if (e instanceof FormulaError) return { formula: null, error: e.message };
    throw e;
  }
}

export type ResultDisplay = {
  // "z = <value> ± <uncertainty>", the value rounded to the uncertainty.
  line: string;
  // "1.23%", or null when it would divide by zero or when there is no
  // uncertainty to be relative about.
  relative: string | null;
};

/** The result lines under the table. */
export function formatPropagation(result: PropagationResult): ResultDisplay {
  const { rounded, decimals } = roundToUncertainty(
    result.value,
    result.uncertainty,
  );
  return {
    line: `z = ${rounded.toFixed(decimals)} ± ${formatUncertainty(result.uncertainty)}`,
    relative:
      result.value !== 0 && result.uncertainty > 0
        ? `${((result.uncertainty / Math.abs(result.value)) * 100).toFixed(2)}%`
        : null,
  };
}
