import type { ParsedTable } from "@/lib/pasteDataParsing";
import { computeColumnStats } from "@/lib/statistics";
import {
  formatUncertainty,
  roundToUncertainty,
} from "@/lib/significantFigures";

/**
 * The pasted table regrouped column by column, as numbers. Nothing for a
 * table that failed to parse or has no rows -- the calculator shows the
 * parse error instead of statistics over whatever did parse.
 */
export function splitColumns(parsed: ParsedTable): number[][] {
  if (parsed.error || parsed.rows.length === 0) return [];
  const cols: number[][] = Array.from({ length: parsed.columnCount }, () => []);
  for (const row of parsed.rows) {
    row.forEach((cell, i) => cols[i].push(Number(cell)));
  }
  return cols;
}

/**
 * Rounds the mean to the sem's leading significant digit, same convention as
 * the regression slope/intercept display. A null/zero sem (n < 2, or every
 * value identical) falls back to roundToUncertainty's own fixed-precision
 * default rather than a meaningless rounding place.
 */
export function formatMean(mean: number, sem: number | null): string {
  if (!Number.isFinite(mean)) return "-";
  const { rounded, decimals } = roundToUncertainty(mean, sem ?? -1);
  return rounded.toFixed(decimals);
}

export type ColumnSummary = {
  n: number;
  // "mean ± sem", or the mean alone when there is no spread to report.
  mean: string;
  // The sample standard deviation, or "-" below two values.
  stdev: string;
};

/** One row of the statistics table, as displayed. */
export function summarizeColumn(values: number[]): ColumnSummary {
  const { n, mean, stdev, sem } = computeColumnStats(values);
  return {
    n,
    mean:
      sem !== null && sem > 0
        ? `${formatMean(mean, sem)} ± ${formatUncertainty(sem)}`
        : formatMean(mean, sem),
    stdev: stdev !== null ? formatUncertainty(stdev) : "-",
  };
}
