import Link from "next/link";
import { createClient } from "@/lib/supabase/server";
import { signOut } from "./actions";
import ExperimentEditor from "@/components/ExperimentEditor";
import CenteredCard from "@/components/CenteredCard";

export const dynamic = "force-dynamic";

type HealthResult =
  { ok: true; status: number; body: string } | { ok: false; error: string };

async function checkApiHealth(): Promise<HealthResult> {
  const apiBaseUrl = process.env.API_INTERNAL_URL ?? "http://localhost:8080";
  try {
    const res = await fetch(`${apiBaseUrl}/healthz`, { cache: "no-store" });
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

  return (
    <CenteredCard maxWidth="max-w-5xl" verticallyCentered={false}>
      <h1 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
        AnalyseApp
      </h1>

      <div className="flex items-center justify-between gap-2 rounded-md border border-zinc-200 p-4 dark:border-zinc-800">
        {user ? (
          <>
            <span className="text-sm text-zinc-700 dark:text-zinc-300">
              ようこそ、{user.email} さん
            </span>
            <form action={signOut}>
              <button
                type="submit"
                className="rounded-md border border-zinc-300 px-3 py-1.5 text-sm text-zinc-700 dark:border-zinc-700 dark:text-zinc-300"
              >
                ログアウト
              </button>
            </form>
          </>
        ) : (
          <>
            <span className="text-sm text-zinc-500 dark:text-zinc-400">
              ログインしていません
            </span>
            <Link
              href="/login"
              className="rounded-md bg-zinc-900 px-3 py-1.5 text-sm font-medium text-white dark:bg-zinc-50 dark:text-zinc-900"
            >
              ログイン
            </Link>
          </>
        )}
      </div>

      {user && (
        <div className="flex flex-col gap-4">
          <div className="flex items-center justify-between">
            <h2 className="text-xl font-semibold text-zinc-900 dark:text-zinc-50">
              実験データを追加
            </h2>
            <Link
              href="/experiments"
              className="text-sm text-zinc-600 underline dark:text-zinc-400"
            >
              保存済みの実験一覧を見る
            </Link>
          </div>
          <ExperimentEditor />
        </div>
      )}

      <div className="flex flex-col gap-2">
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
