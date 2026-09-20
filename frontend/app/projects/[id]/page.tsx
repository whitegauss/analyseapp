import Link from "next/link";
import { notFound, redirect } from "next/navigation";
import { callGoApi, GoApiError } from "@/lib/api";
import type { ExperimentSummary, ProjectSummary } from "@/lib/dashboard";
import ExperimentEditor from "@/components/ExperimentEditor";
import CenteredCard from "@/components/CenteredCard";

export const dynamic = "force-dynamic";

export default async function ProjectPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;

  let project: ProjectSummary | null;
  let experiments: ExperimentSummary[] | null;
  try {
    // The nested list already 404s for a project that isn't the caller's,
    // but the page needs the project's title either way, so both are read.
    [project, experiments] = await Promise.all([
      callGoApi<ProjectSummary>(`/api/v1/projects/${id}`),
      callGoApi<ExperimentSummary[]>(`/api/v1/projects/${id}/experiments`),
    ]);
  } catch (e) {
    if (e instanceof GoApiError && e.status === 404) {
      notFound();
    }
    throw e;
  }

  if (!project || !experiments) {
    redirect("/login");
  }

  return (
    <CenteredCard maxWidth="max-w-5xl" verticallyCentered={false}>
      <div className="flex flex-col gap-1">
        <Link
          href="/"
          className="text-xs text-zinc-500 underline dark:text-zinc-400"
        >
          ← プロジェクト一覧
        </Link>
        <h1 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
          {project.title}
        </h1>
        {project.description && (
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
            {project.description}
          </p>
        )}
      </div>

      <div className="flex flex-col gap-2">
        <h2 className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
          このプロジェクトの実験（{experiments.length}件）
        </h2>
        {experiments.length === 0 ? (
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
            まだ実験がありません。下のフォームから追加してください。
          </p>
        ) : (
          <ul className="flex flex-col divide-y divide-zinc-200 dark:divide-zinc-800">
            {experiments.map((experiment) => (
              <li key={experiment.id}>
                <Link
                  href={`/experiments/${experiment.id}`}
                  className="flex items-center justify-between gap-4 py-3 hover:bg-zinc-50 dark:hover:bg-zinc-800"
                >
                  <span className="truncate text-sm text-zinc-900 dark:text-zinc-50">
                    {experiment.title ?? "(無題)"}
                  </span>
                  <span className="shrink-0 text-xs text-zinc-500 dark:text-zinc-400">
                    {experiment.created_at.slice(0, 10)}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </div>

      <div className="flex flex-col gap-3 border-t border-zinc-200 pt-6 dark:border-zinc-800">
        <h2 className="text-xl font-semibold text-zinc-900 dark:text-zinc-50">
          実験を追加
        </h2>
        {/* Passing projectId is what turns the editor from the dashboard's
            scratchpad into a form that saves -- into this project, through
            POST /api/v1/projects/{id}/experiments. */}
        <ExperimentEditor projectId={project.id} />
      </div>
    </CenteredCard>
  );
}
