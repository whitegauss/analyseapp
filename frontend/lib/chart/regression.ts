import { parseSegments } from "@/lib/mathText";
import {
  formatUncertainty,
  roundToUncertainty,
} from "@/lib/significantFigures";
import type { LinearRegressionResult } from "@/lib/experiment";

/**
 * Renders `$...$` spans as Plotly's HTML italics.
 *
 * Plotly's legend and title text accept a small HTML subset (`<i>`, `<b>`,
 * ...) but not KaTeX, so math is approximated with native italics rather than
 * typeset. Reusing the axis label's exact parse is what keeps `$v$` meaning
 * the same thing in the legend as on the axis.
 */
export function mathItalicHtml(text: string): string {
  return parseSegments(text)
    .map((seg) => (seg.math ? `<i>${seg.content}</i>` : seg.content))
    .join("");
}

/** `+ 1.23` / `- 1.23` — the sign is separated so it reads as an operator. */
export function formatSigned(value: number, decimals: number): string {
  const sign = value < 0 ? "-" : "+";
  return `${sign} ${Math.abs(value).toFixed(decimals)}`;
}

/**
 * A non-positive or non-finite stderr is treated as exactly zero uncertainty
 * rather than as a reason to skip the band. The fit still has a best estimate
 * worth drawing; what is missing is only the spread around it.
 */
export function safeStderr(stderr: number): number {
  return Number.isFinite(stderr) && stderr > 0 ? stderr : 0;
}

/** `+ (1.23 ± 0.05)` — the parenthesised form used for the intercept term. */
export function formatSignedWithUncertainty(
  value: number,
  decimals: number,
  uncertainty: number,
): string {
  const sign = value < 0 ? "-" : "+";
  return `${sign} (${Math.abs(value).toFixed(decimals)} ± ${formatUncertainty(uncertainty)})`;
}

/**
 * Evaluates the fitted model at a real (never-transformed) x, returning a real
 * y — it undoes whatever log10 transform the fit was computed under, so the
 * result can be plotted directly whatever the axis scale is.
 *
 * xLog/yLog must match what the regression was actually fit with
 * (`LinearRegressionResult.x_log` / `y_log`).
 */
export function evaluateModel(
  regression: LinearRegressionResult,
  xValue: number,
  xLog: boolean,
  yLog: boolean,
): number {
  const xFit = xLog ? Math.log10(xValue) : xValue;
  const yFit = regression.slope * xFit + regression.intercept;
  return yLog ? Math.pow(10, yFit) : yFit;
}

/**
 * The ±1σ envelope at one x value.
 *
 * This is deliberately not a rigorous confidence band: it ignores the
 * slope/intercept covariance and just takes the widest spread across all four
 * ±slope_stderr / ±intercept_stderr combinations — the "steepest vs shallowest
 * plausible line" envelope drawn by hand in an intro physics lab.
 *
 * The spread is computed in whichever space the fit was done in, then
 * converted back. 10^t is monotonic, so taking min/max before converting is
 * equivalent to after. A zero stderr collapses the band onto the line.
 */
export function uncertaintyBoundsAt(
  regression: LinearRegressionResult,
  xValue: number,
  xLog: boolean,
  yLog: boolean,
): { lower: number; upper: number } {
  const slopeErr = safeStderr(regression.slope_stderr);
  const interceptErr = safeStderr(regression.intercept_stderr);
  const xFit = xLog ? Math.log10(xValue) : xValue;

  const candidates = [
    (regression.slope + slopeErr) * xFit +
      (regression.intercept + interceptErr),
    (regression.slope + slopeErr) * xFit +
      (regression.intercept - interceptErr),
    (regression.slope - slopeErr) * xFit +
      (regression.intercept + interceptErr),
    (regression.slope - slopeErr) * xFit +
      (regression.intercept - interceptErr),
  ];
  const lowerFit = Math.min(...candidates);
  const upperFit = Math.max(...candidates);

  return yLog
    ? { lower: Math.pow(10, lowerFit), upper: Math.pow(10, upperFit) }
    : { lower: lowerFit, upper: upperFit };
}

/**
 * Legend text for the fit line, in whichever of the four
 * linear / semi-log / log-log forms the regression was fit under — e.g.
 * `y = ax + b`, or `log₁₀(y) = a·log₁₀(x) + b` for a power law.
 */
export function formatModelEquation(
  regression: LinearRegressionResult,
): string {
  const { x_log: xLog, y_log: yLog } = regression;
  const slope = roundToUncertainty(regression.slope, regression.slope_stderr);
  const intercept = roundToUncertainty(
    regression.intercept,
    regression.intercept_stderr,
  );
  const slopeTerm = `(${slope.rounded.toFixed(slope.decimals)} ± ${formatUncertainty(regression.slope_stderr)})`;
  const interceptTerm = formatSignedWithUncertainty(
    intercept.rounded,
    intercept.decimals,
    regression.intercept_stderr,
  );
  const lhs = yLog ? "log₁₀($y$)" : "$y$";
  const xTerm = xLog ? "log₁₀($x$)" : "$x$";
  return mathItalicHtml(`${lhs} = ${slopeTerm}${xTerm} ${interceptTerm}`);
}

/**
 * Cache key for a lazily-fetched log-scale fit. Each axis-scale combination is
 * a separate fit from the server, so the two booleans fully identify one.
 */
export function regressionCacheKey(xLog: boolean, yLog: boolean): string {
  return `${xLog}:${yLog}`;
}
