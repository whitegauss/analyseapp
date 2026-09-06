import { buildCsvRow } from "@/lib/csv";
import { orderedColumnKeys } from "@/lib/columns";
import {
  readAxisLabel,
  type ExperimentConfig,
  type LinearRegressionResult,
} from "@/lib/experiment";

/** The parts of an experiment the CSV export actually reads. */
export type CsvExperiment = {
  id: string;
  title: string | null;
  raw_data: { columns: Record<string, number[]> };
  config: ExperimentConfig;
  created_at: string;
};

/**
 * Renders an experiment as CSV, preceded by `#` comment lines carrying the
 * metadata that has nowhere to live in a flat table (title, timestamp, axis
 * labels, and the fitted model). Spreadsheets ignore those lines on import
 * while a human reading the file still sees where the numbers came from.
 *
 * Rows are CRLF-terminated per RFC 4180, including a trailing terminator.
 */
export function buildCsv(
  experiment: CsvExperiment,
  regression: LinearRegressionResult | null,
): string {
  const columns = experiment.raw_data.columns;
  const columnKeys = orderedColumnKeys(columns);
  const xLabel = readAxisLabel(experiment.config, "x_axis_label");
  const yLabel = readAxisLabel(experiment.config, "y_axis_label");

  const lines: string[] = [];
  lines.push(`# title: ${experiment.title ?? "(無題)"}`);
  lines.push(`# created_at: ${experiment.created_at}`);
  if (xLabel) lines.push(`# x_axis_label: ${xLabel}`);
  if (yLabel) lines.push(`# y_axis_label: ${yLabel}`);
  if (regression) {
    // A two-point fit reports no standard errors; writing "± null" into the
    // header would be worse than dropping the term, and "± 0" would claim a
    // perfect measurement.
    const withError = (value: number, stderr: number | null) =>
      stderr === null ? `${value}` : `${value} ± ${stderr}`;
    lines.push(
      `# regression: slope=${withError(regression.slope, regression.slope_stderr)}, ` +
        `intercept=${withError(regression.intercept, regression.intercept_stderr)}, ` +
        `r_squared=${regression.r_squared}, x_log=${regression.x_log}, y_log=${regression.y_log}`,
    );
  }

  // The axis label doubles as the column heading when the user set one, so the
  // exported file reads the same way the chart does.
  const headerNames = columnKeys.map((key) => {
    if (key === "x" && xLabel) return xLabel;
    if (key === "y" && yLabel) return yLabel;
    return key;
  });
  lines.push(buildCsvRow(headerNames));

  // Columns are not guaranteed to be the same length, so the longest one sets
  // the row count and shorter columns leave empty cells rather than truncating.
  const rowCount = Math.max(
    0,
    ...columnKeys.map((key) => columns[key]?.length ?? 0),
  );
  for (let i = 0; i < rowCount; i++) {
    lines.push(buildCsvRow(columnKeys.map((key) => columns[key]?.[i] ?? "")));
  }

  return lines.join("\r\n") + "\r\n";
}

// Filesystem/header-safe filename built from the experiment title. Strips
// only characters that would break a quoted Content-Disposition value or a
// path (quotes, slashes, control characters); everything else, including
// Japanese text, is kept and carried via the RFC 5987 filename* parameter.
export function csvFilename(title: string | null, id: string): string {
  const sanitized = (title ?? "").trim().replace(/["/\\\r\n]/g, "_");
  return `${sanitized || id}.csv`;
}

/**
 * Percent-encodes a filename for the RFC 8187 `filename*` parameter.
 *
 * encodeURIComponent leaves `!'()*~` alone, but only `!` and `~` are in that
 * spec's attr-char set. The apostrophe is the dangerous one: ext-value is
 * `charset'language'value`, so a literal `'` in the name gives a strict parser
 * a third delimiter to trip over.
 */
function encodeExtValue(filename: string): string {
  return encodeURIComponent(filename).replace(
    /['()*]/g,
    (c) => `%${c.charCodeAt(0).toString(16).toUpperCase()}`,
  );
}

/**
 * Builds the Content-Disposition value, giving the same name twice: a
 * plain-ASCII `filename` that every client can parse, and the RFC 8187
 * `filename*` that carries the real, possibly non-ASCII, name.
 */
export function contentDispositionValue(filename: string): string {
  const asciiFallback = filename.replace(/[^\x20-\x7E]/g, "_");
  return (
    `attachment; filename="${asciiFallback}"; ` +
    `filename*=UTF-8''${encodeExtValue(filename)}`
  );
}
