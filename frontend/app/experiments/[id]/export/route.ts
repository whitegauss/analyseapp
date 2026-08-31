import { NextResponse } from "next/server";
import { callGoApi, GoApiError } from "@/lib/api";
import { fetchRegression } from "@/app/experiments/actions";
import {
  buildCsv,
  contentDispositionValue,
  csvFilename,
  type CsvExperiment,
} from "@/lib/experimentCsv";

export async function GET(
  request: Request,
  { params }: { params: Promise<{ id: string }> },
) {
  const { id } = await params;

  let experiment: CsvExperiment | null;
  try {
    experiment = await callGoApi<CsvExperiment>(`/api/v1/experiments/${id}`);
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

  return new NextResponse(buildCsv(experiment, regression), {
    headers: {
      "Content-Type": "text/csv; charset=utf-8",
      "Content-Disposition": contentDispositionValue(
        csvFilename(experiment.title, experiment.id),
      ),
    },
  });
}
