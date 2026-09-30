import {
  formatUncertainty,
  roundToUncertainty,
} from "@/lib/significantFigures";

/**
 * The significant-figure rounder's result line, `"<value> ± <uncertainty>"`,
 * from the two fields as typed -- or null until both hold a number.
 *
 * The rounding follows the regression display's convention (to the
 * uncertainty's leading significant digit); that is significantFigures.ts's
 * and tested there. This is only the reading of the form and the joining.
 */
export function formatRoundedValue(
  value: string,
  uncertainty: string,
): string | null {
  const parsedValue = Number(value);
  const parsedUncertainty = Number(uncertainty);
  const valid =
    value !== "" &&
    uncertainty !== "" &&
    Number.isFinite(parsedValue) &&
    Number.isFinite(parsedUncertainty);
  if (!valid) return null;

  const { rounded, decimals } = roundToUncertainty(
    parsedValue,
    parsedUncertainty,
  );
  return `${rounded.toFixed(decimals)} ± ${formatUncertainty(parsedUncertainty)}`;
}
