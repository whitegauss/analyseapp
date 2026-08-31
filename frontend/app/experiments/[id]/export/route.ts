import { NextResponse } from "next/server";
import { callGoApi, GoApiError } from "@/lib/api";
import { fetchRegression } from "@/app/experiments/actions";
import { buildCsvRow } from "@/lib/csv";
import { orderedColumnKeys } from "@/lib/columns";
import { readAxisLabel, type LinearRegressionResult } from "@/lib/experiment";

type Experiment = {
  id: string;
  title: string | null;
  raw_data: { columns: Record<string, number[]> };
  config: Record<string, unknown>;
  created_at: string;
};

function buildCsv(
  experiment: Experiment,
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
    lines.push(
      `# regression: slope=${regression.slope} ± ${regression.slope_stderr}, ` +
        `intercept=${regression.intercept} ± ${regression.intercept_stderr}, ` +
        `r_squared=${regression.r_squared}, x_log=${regression.x_log}, y_log=${regression.y_log}`,
    );
  }

  const headerNames = columnKeys.map((key) => {
    if (key === "x" && xLabel) return xLabel;
    if (key === "y" && yLabel) return yLabel;
    return key;
  });
  lines.push(buildCsvRow(headerNames));

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
function csvFilename(title: string | null, id: string): string {
  const sanitized = (title ?? "").trim().replace(/["/\\\r\n]/g, "_");
  return `${sanitized || id}.csv`;
}

export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;

  let experiment: Experiment | null;
  try {
    experiment = await callGoApi<Experiment>(`/api/v1/experiments/${id}`);
  } catch (e) {
    if (e instanceof GoApiError && e.status === 404) {
      return new NextResponse("experiment not found", { status: 404 });
    }
    throw e;
  }

  if (!experiment) {
    return NextResponse.redirect(new URL("/login", request.url));
  }

  const regression = await fetchRegression(id, {});
  const csv = buildCsv(experiment, regression);
  const filename = csvFilename(experiment.title, experiment.id);
  const asciiFallback = filename.replace(/[^\x20-\x7E]/g, "_");

  return new NextResponse(csv, {
    headers: {
      "Content-Type": "text/csv; charset=utf-8",
      "Content-Disposition":
        `attachment; filename="${asciiFallback}"; ` +
        `filename*=UTF-8''${encodeURIComponent(filename)}`,
    },
  });
}
