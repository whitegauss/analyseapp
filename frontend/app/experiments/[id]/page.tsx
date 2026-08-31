import { notFound, redirect } from "next/navigation";
import { callGoApi, GoApiError } from "@/lib/api";
import { readAxisLabel } from "@/lib/experiment";
import { fetchRegression } from "@/app/experiments/actions";
import ExperimentChart from "@/components/ExperimentChart";
import AxisLabelEditor from "@/components/AxisLabelEditor";
import RawDataEditor from "@/components/RawDataEditor";
import CenteredCard from "@/components/CenteredCard";

export const dynamic = "force-dynamic";

type Experiment = {
  id: string;
  title: string | null;
  raw_data: { columns: Record<string, number[]> };
  config: Record<string, unknown>;
  created_at: string;
};

export default async function ExperimentPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  let experiment: Experiment | null;
  try {
    experiment = await callGoApi<Experiment>(`/api/v1/experiments/${id}`);
  } catch (e) {
    if (e instanceof GoApiError && e.status === 404) {
      notFound();
    }
    throw e;
  }

  if (!experiment) {
    redirect("/login");
  }

  // Only the default linear-scale fit is computed up front; the log-scale
  // variants (x/y/log-log) are fetched on demand from the client when the
  // user actually switches the chart to a log axis (see ExperimentChart).
  const regression = await fetchRegression(id, {});

  return (
    <CenteredCard maxWidth="max-w-3xl">
      <h1 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
        {experiment.title ?? "(無題)"}
      </h1>
      <ExperimentChart
        columns={experiment.raw_data.columns}
        title={experiment.title ?? "(無題)"}
        xAxisLabel={readAxisLabel(experiment.config, "x_axis_label")}
        yAxisLabel={readAxisLabel(experiment.config, "y_axis_label")}
        experimentId={experiment.id}
        initialRegression={regression}
      />
      <div className="flex flex-wrap items-start gap-3">
        <AxisLabelEditor
          id={experiment.id}
          xAxisLabel={readAxisLabel(experiment.config, "x_axis_label")}
          yAxisLabel={readAxisLabel(experiment.config, "y_axis_label")}
        />
        <RawDataEditor
          id={experiment.id}
          columns={experiment.raw_data.columns}
        />
        <a
          href={`/experiments/${experiment.id}/export`}
          className="self-start text-xs text-zinc-500 underline hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
        >
          CSVダウンロード
        </a>
      </div>
    </CenteredCard>
  );
}
