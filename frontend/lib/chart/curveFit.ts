import { evaluateWithGradient, parseFormula } from "@/lib/formula";
import {
  formatToPrecision,
  formatUncertainty,
  roundToUncertainty,
} from "@/lib/significantFigures";
import type { CurveFitParameter } from "@/lib/experiment";
import { mathItalicHtml, UNKNOWN_UNCERTAINTY_TEXT } from "./regression";

// Enough for a smooth curve across the plot width without making Plotly
// work for it; a formula is evaluated once per sample.
export const CURVE_SAMPLES = 200;

/**
 * The fitted curve sampled across [xMin, xMax], ready to hand to Plotly.
 *
 * Unlike the straight line, a curve cannot be drawn from its two endpoints,
 * so it is evaluated here in the browser with the same formula the worker
 * fitted. On a log x axis the samples are spaced evenly in log10(x), or the
 * left of the plot would get almost none of them. Only the display changes
 * with the axis: the fit itself was done on the raw values.
 *
 * A point where the model is not finite (1/(x - a) at x = a, sqrt of a
 * negative) becomes null, which Plotly draws as a gap rather than a spike.
 */
export function sampleCurve(
  formula: string,
  parameters: CurveFitParameter[],
  bounds: [number, number],
  xLog: boolean,
  samples = CURVE_SAMPLES,
): { x: number[]; y: (number | null)[] } {
  const { ast } = parseFormula(formula);
  const values: Record<string, number> = {};
  for (const p of parameters) values[p.name] = p.value;

  const [lo, hi] = xLog ? bounds.map(Math.log10) : bounds;
  const xs: number[] = [];
  const ys: (number | null)[] = [];
  for (let i = 0; i < samples; i++) {
    const t = lo + ((hi - lo) * i) / (samples - 1);
    const xv = xLog ? Math.pow(10, t) : t;
    const yv = evaluateWithGradient(ast, { ...values, x: xv }).value;
    xs.push(xv);
    ys.push(Number.isFinite(yv) ? yv : null);
  }
  return { x: xs, y: ys };
}

/**
 * `A = 3.10 ± 0.06` -- the value rounded to its own uncertainty, as the
 * straight-line fit does. With no uncertainty to round to (the worker
 * reports null when the data cannot pin a parameter down), the value is
 * shown to a few significant figures and the error is said to be unknown
 * rather than written as "± 0".
 */
export function formatFitParameter(p: CurveFitParameter): string {
  if (p.stderr === null) {
    return `${p.name} = ${formatToPrecision(p.value)}（誤差${UNKNOWN_UNCERTAINTY_TEXT}）`;
  }
  const { rounded, decimals } = roundToUncertainty(p.value, p.stderr);
  return `${p.name} = ${rounded.toFixed(decimals)} ± ${formatUncertainty(p.stderr)}`;
}

/** Legend text for the fitted curve: the model as the user wrote it. */
export function formatCurveLegend(formula: string): string {
  return mathItalicHtml(`$y$ = ${formula}`);
}
