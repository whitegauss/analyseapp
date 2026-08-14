import { notFound, redirect } from "next/navigation";
import { callGoApi, GoApiError } from "@/lib/api";
import ExperimentChart from "@/components/ExperimentChart";

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

  return (
    <div className="flex min-h-screen items-center justify-center bg-zinc-50 p-8 dark:bg-black">
      <main className="flex w-full max-w-3xl flex-col gap-6 rounded-lg border border-zinc-200 bg-white p-8 dark:border-zinc-800 dark:bg-zinc-900">
        <h1 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
          {experiment.title ?? "(無題)"}
        </h1>
        <ExperimentChart
          columns={experiment.raw_data.columns}
          title={experiment.title ?? "(無題)"}
          xAxisLabel={readAxisLabel(experiment.config, "x_axis_label")}
          yAxisLabel={readAxisLabel(experiment.config, "y_axis_label")}
        />
      </main>
    </div>
  );
}
