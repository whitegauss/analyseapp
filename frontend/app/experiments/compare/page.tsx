import { redirect } from "next/navigation";
import { callGoApi, GoApiError } from "@/lib/api";
import { fetchRegression } from "@/app/experiments/actions";
import ComparisonChart, {
  type ComparedExperiment,
} from "@/components/ComparisonChart";
import { readAxisLabel, type LinearRegressionResult } from "@/lib/experiment";
import { parseCompareIds } from "@/lib/compareParams";
import CenteredCard from "@/components/CenteredCard";

export const dynamic = "force-dynamic";

type Experiment = {
  id: string;
  title: string | null;
  raw_data: { columns: Record<string, number[]> };
  config: Record<string, unknown>;
};

export default async function ComparePage({
  searchParams,
}: {
  searchParams: Promise<{ ids?: string }>;
}) {
  const { ids: idsParam } = await searchParams;
  const ids = parseCompareIds(idsParam);

  if (ids.length < 2) {
    redirect("/experiments");
  }

  // Each id is fetched independently: a 404 (deleted since the list was
  // loaded, or someone else's id in a hand-edited URL) just drops that one
  // experiment from the comparison rather than failing the whole page. A
  // missing session (null, uniform across every id since it depends only on
  // the request's cookies) redirects to /login below.
  const results = await Promise.all(
    ids.map(async (id) => {
      try {
        const experiment = await callGoApi<Experiment>(
          `/api/v1/experiments/${id}`,
        );
        if (!experiment) return "no_session" as const;
        const regression = await fetchRegression(id, {});
        return { experiment, regression };
      } catch (e) {
        if (e instanceof GoApiError && e.status === 404) {
          return "not_found" as const;
        }
        throw e;
      }
    }),
  );

  if (results.some((r) => r === "no_session")) {
    redirect("/login");
  }

  const items = results.filter(
    (
      r,
    ): r is {
      experiment: Experiment;
      regression: LinearRegressionResult | null;
    } => typeof r === "object",
  );

  if (items.length < 2) {
    redirect("/experiments");
  }

  const experiments: ComparedExperiment[] = items.map(
    ({ experiment, regression }) => ({
      id: experiment.id,
      title: experiment.title ?? "(無題)",
      columns: experiment.raw_data.columns,
      xAxisLabel: readAxisLabel(experiment.config, "x_axis_label"),
      yAxisLabel: readAxisLabel(experiment.config, "y_axis_label"),
      regression,
    }),
  );

  return (
    <CenteredCard maxWidth="max-w-4xl">
      <h1 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
        実験の比較
      </h1>
      <p className="text-sm text-zinc-500 dark:text-zinc-400">
        {experiments.map((e) => e.title).join(" / ")}
      </p>
      <ComparisonChart experiments={experiments} />
    </CenteredCard>
  );
}
