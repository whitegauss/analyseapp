import Link from "next/link";
import { createClient } from "@/lib/supabase/server";
import { callGoApi } from "@/lib/api";
import { apiBaseUrl } from "@/lib/env";
import {
  groupExperimentsByProject,
  type ExperimentSummary,
  type ProjectSummary,
} from "@/lib/dashboard";
import ExperimentEditor from "@/components/ExperimentEditor";
import ProjectAccordion from "@/components/ProjectAccordion";
import CreateProjectForm from "@/components/CreateProjectForm";
import CenteredCard from "@/components/CenteredCard";

export const dynamic = "force-dynamic";

type HealthResult =
  { ok: true; status: number; body: string } | { ok: false; error: string };

async function checkApiHealth(): Promise<HealthResult> {
  try {
    const res = await fetch(`${apiBaseUrl()}/healthz`, { cache: "no-store" });
    return res.ok
      ? { ok: true, status: res.status, body: await res.text() }
      : { ok: false, error: `HTTP ${res.status}: ${await res.text()}` };
  } catch (e) {
    return { ok: false, error: e instanceof Error ? e.message : String(e) };
  }
}

export default async function Home() {
  const health = await checkApiHealth();
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();

  // Two reads, joined in lib/dashboard, rather than one per project: the
  // list shows a count for every project, which needs all the experiments
  // anyway. Skipped entirely when logged out -- the scratchpad below is
  // client-side and needs no session.
  const [projects, experiments] = user
    ? await Promise.all([
        callGoApi<ProjectSummary[]>("/api/v1/projects"),
        callGoApi<ExperimentSummary[]>("/api/v1/experiments"),
      ])
    : [null, null];

  const grouped = groupExperimentsByProject(projects ?? [], experiments ?? []);

  return (
    <CenteredCard maxWidth="max-w-5xl" verticallyCentered={false}>
      <h1 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
        AnalyseApp
      </h1>

      {user ? (
        <div className="flex flex-col gap-3">
          <div className="flex items-center justify-between gap-4">
            <h2 className="text-xl font-semibold text-zinc-900 dark:text-zinc-50">
              プロジェクト
            </h2>
            <Link
              href="/experiments"
              className="shrink-0 text-xs text-zinc-500 underline dark:text-zinc-400"
            >
              すべての実験を見る
            </Link>
          </div>

          {grouped.length === 0 ? (
            <p className="text-sm text-zinc-500 dark:text-zinc-400">
              まだプロジェクトがありません。作成すると、そこに実験を追加できます。
            </p>
          ) : (
            <ProjectAccordion projects={grouped} />
          )}

          <CreateProjectForm />
        </div>
      ) : (
        <p className="text-sm text-zinc-500 dark:text-zinc-400">
          <Link href="/login" className="underline">
            ログイン
          </Link>
          するとプロジェクトを作って実験を保存できます。下のグラフはログインしなくても使えます。
        </p>
      )}

      <div className="flex flex-col gap-3 border-t border-zinc-200 pt-6 dark:border-zinc-800">
        <div className="flex flex-col gap-1">
          <h2 className="text-xl font-semibold text-zinc-900 dark:text-zinc-50">
            グラフをすぐ見る
          </h2>
          <p className="text-xs text-zinc-500 dark:text-zinc-400">
            貼り付けたデータをその場で描くだけの下書きです。
            <strong className="font-medium">保存はされません</strong>
            （リロードすると消えます）。残したいときはプロジェクトを開いて「実験を追加」から保存してください。
          </p>
        </div>
        <ExperimentEditor />
      </div>

      <div className="flex flex-col gap-2 border-t border-zinc-200 pt-6 dark:border-zinc-800">
        <h2 className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
          API接続確認（Go API Gateway /healthz）
        </h2>
        {health.ok ? (
          <div className="rounded-md border border-green-300 bg-green-50 p-4 dark:border-green-800 dark:bg-green-950">
            <p className="text-sm font-medium text-green-800 dark:text-green-300">
              接続成功（HTTP {health.status}）
            </p>
            <pre className="mt-2 overflow-x-auto text-xs text-green-700 dark:text-green-400">
              {health.body}
            </pre>
          </div>
        ) : (
          <div className="rounded-md border border-red-300 bg-red-50 p-4 dark:border-red-800 dark:bg-red-950">
            <p className="text-sm font-medium text-red-800 dark:text-red-300">
              接続失敗
            </p>
            <p className="mt-2 text-xs text-red-700 dark:text-red-400">
              {health.error}
            </p>
          </div>
        )}
      </div>
    </CenteredCard>
  );
}
