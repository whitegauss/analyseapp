// Rounds value to the same place as uncertainty's leading (1-significant-
// figure) digit -- standard convention for reporting a measurement
// alongside its 1σ uncertainty. Works symmetrically in both directions:
// a small uncertainty (e.g. 0.05) rounds the value to that decimal place
// (2.03847 -> 2.04), and a large uncertainty (e.g. 52.3) rounds the value
// to that same tens/hundreds/... place instead (517 -> 520). Displayed
// decimal places are additionally capped at 4 (the value is still rounded
// to the true uncertainty digit even when that digit is finer than 4
// decimals) so a tiny uncertainty doesn't blow up the displayed number
// into a long string of digits.
const MAX_DISPLAY_DECIMALS = 4;

export function roundToUncertainty(
  value: number,
  uncertainty: number | null,
): { rounded: number; decimals: number } {
  if (
    uncertainty === null ||
    !Number.isFinite(uncertainty) ||
    uncertainty <= 0
  ) {
    return { rounded: value, decimals: MAX_DISPLAY_DECIMALS };
  }
  const exponent = Math.floor(Math.log10(uncertainty));
  const scale = Math.pow(10, exponent);
  const decimals = Math.min(MAX_DISPLAY_DECIMALS, Math.max(0, -exponent));
  return { rounded: Math.round(value / scale) * scale, decimals };
}

// Formats an uncertainty (stderr/stdev) value on its own -- as opposed to
// roundToUncertainty, which rounds some other value *to* an uncertainty.
// A non-positive or non-finite uncertainty (e.g. from floating-point
// cancellation, or an undefined stdev) just displays as "0.0" rather than
// propagating NaN/Infinity or a nonsensical value.
export function formatUncertainty(uncertainty: number | null): string {
  if (uncertainty === null || !Number.isFinite(uncertainty) || uncertainty <= 0)
    return "0.0";
  const { rounded, decimals } = roundToUncertainty(uncertainty, uncertainty);
  return rounded.toFixed(decimals);
}

// Formats a number whose scale is not known in advance -- a partial derivative
// can be 1e-7 for one formula and 1e6 for the next, so a fixed number of
// decimals would show "0.00" in one case and a wall of digits in the other.
// Trailing zeros are only trimmed after a decimal point: "1000".toPrecision(4)
// has no point, and stripping its zeros would turn 1000 into 1.
export function formatToPrecision(value: number, digits = 4): string {
  if (!Number.isFinite(value)) return "—";
  if (value === 0) return "0";

  const magnitude = Math.abs(value);
  if (magnitude >= 1e5 || magnitude < 1e-3) return value.toExponential(2);

  const text = value.toPrecision(digits);
  return text.includes(".") ? text.replace(/\.?0+$/, "") : text;
}
