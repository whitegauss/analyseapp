import Link from "next/link";
import { redirect } from "next/navigation";
import { callGoApi } from "@/lib/api";
import {
  attachProjectTitles,
  type ExperimentSummary,
  type ProjectSummary,
} from "@/lib/dashboard";
import ExperimentListWithCompare from "@/components/ExperimentListWithCompare";
import CenteredCard from "@/components/CenteredCard";

export const dynamic = "force-dynamic";

export default async function ExperimentsListPage() {
  // The cross-project list (PDR.md section 8: kept for picking a copy
  // source and for comparing across projects). Since the project is what
  // distinguishes two similarly named experiments here, the projects are
  // read alongside and each row is labelled with its own.
  const [experiments, projects] = await Promise.all([
    callGoApi<ExperimentSummary[]>("/api/v1/experiments"),
    callGoApi<ProjectSummary[]>("/api/v1/projects"),
  ]);

  if (!experiments || !projects) {
    redirect("/login");
  }

  return (
    <CenteredCard maxWidth="max-w-3xl">
      <div className="flex items-center justify-between gap-4">
        <h1 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
          すべての実験
        </h1>
        <Link
          href="/"
          className="shrink-0 text-xs text-zinc-500 underline dark:text-zinc-400"
        >
          プロジェクト一覧へ
        </Link>
      </div>

      {experiments.length === 0 ? (
        <p className="text-sm text-zinc-500 dark:text-zinc-400">
          まだ実験がありません。
          <Link href="/" className="underline">
            プロジェクト
          </Link>
          を開いて「実験を追加」から保存すると、ここに表示されます。
        </p>
      ) : (
        <ExperimentListWithCompare
          experiments={attachProjectTitles(projects, experiments)}
        />
      )}
    </CenteredCard>
  );
}
