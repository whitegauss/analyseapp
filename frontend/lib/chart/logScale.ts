/**
 * Axis bounds for a linear scale: the data range plus 5% breathing room on
 * each side. A single data point (or many identical ones) has no spread to
 * scale, so it falls back to ±1 rather than collapsing to a zero-width axis.
 */
export function linearRange(values: number[]): [number, number] | undefined {
  if (values.length === 0) return undefined;
  const min = Math.min(...values);
  const max = Math.max(...values);
  const pad = (max - min) * 0.05 || 1;
  return [min - pad, max + pad];
}

/**
 * Axis bounds for a log scale, **in log10 units** — Plotly reads a log axis's
 * `range` that way, so [0, 2] means 1 to 100.
 *
 * Non-positive values cannot appear on a log axis at all and are dropped
 * before the range is computed; if nothing positive remains there is no range
 * to show. The no-spread fallback is ±0.5 decades rather than the linear ±1.
 */
export function logRange(values: number[]): [number, number] | undefined {
  const positive = values.filter((v) => v > 0);
  if (positive.length === 0) return undefined;
  const logMin = Math.log10(Math.min(...positive));
  const logMax = Math.log10(Math.max(...positive));
  const pad = (logMax - logMin) * 0.05 || 0.5;
  return [logMin - pad, logMax + pad];
}

/**
 * Tick positions and labels for the standard 1-2-5 log series (..., 2, 5, 10,
 * 20, 50, 100, ...) covering [min, max] — the usual pattern in scientific
 * plots.
 *
 * Plotly's automatic tick selection lands on the same values, but its default
 * label formatting truncates the non-decade ones (500 renders as "5", 20 as
 * "2"). Computing tickvals/ticktext explicitly guarantees each label matches
 * the value it sits on.
 */
export function logTicks(
  min: number,
  max: number,
): { tickvals: number[]; ticktext: string[] } {
  if (!(min > 0) || !(max > 0) || min >= max) {
    return { tickvals: [], ticktext: [] };
  }
  const startExp = Math.floor(Math.log10(min));
  const endExp = Math.floor(Math.log10(max));
  const tickvals: number[] = [];
  for (let exp = startExp; exp <= endExp; exp++) {
    for (const m of [1, 2, 5]) {
      const v = m * Math.pow(10, exp);
      if (v >= min && v <= max) tickvals.push(v);
    }
  }
  const ticktext = tickvals.map((v) => Number(v.toPrecision(6)).toString());
  return { tickvals, ticktext };
}

/** Converts log10-unit axis bounds back to real values, as logTicks expects. */
export function ticksForLogRange(
  logBounds: [number, number] | undefined,
): { tickvals: number[]; ticktext: string[] } | undefined {
  if (!logBounds) return undefined;
  return logTicks(Math.pow(10, logBounds[0]), Math.pow(10, logBounds[1]));
}
