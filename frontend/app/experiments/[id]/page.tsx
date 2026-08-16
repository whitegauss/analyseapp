import { notFound, redirect } from "next/navigation";
import { callGoApi, GoApiError } from "@/lib/api";
import ExperimentChart, {
  type LinearRegressionResult,
} from "@/components/ExperimentChart";
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

function readAxisLabel(config: Record<string, unknown>, key: string): string {
  const label = config[key];
  return typeof label === "string" ? label : "";
}

// The regression line is best-effort: too little data, non-numeric
// columns, etc. just mean no overlay rather than a broken page.
async function fetchLinearRegression(
  id: string,
): Promise<LinearRegressionResult | null> {
  try {
    const res = await callGoApi<{
      type: string;
      result: LinearRegressionResult;
    }>(`/api/v1/experiments/${id}/analyze`, {
      method: "POST",
      body: JSON.stringify({ type: "linear_regression" }),
    });
    return res?.result ?? null;
  } catch {
    return null;
  }
}

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

  const regression = await fetchLinearRegression(id);

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
        regression={regression}
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
      </div>
    </CenteredCard>
  );
}
