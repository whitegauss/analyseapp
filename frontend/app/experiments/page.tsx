import Link from "next/link";
import { redirect } from "next/navigation";
import { callGoApi } from "@/lib/api";

export const dynamic = "force-dynamic";

type ExperimentSummary = {
  id: string;
  title: string | null;
  created_at: string;
};

export default async function ExperimentsListPage() {
  const experiments = await callGoApi<ExperimentSummary[]>(
    "/api/v1/experiments",
  );

  if (!experiments) {
    redirect("/login");
  }

  return (
    <div className="flex min-h-screen items-center justify-center bg-zinc-50 p-8 dark:bg-black">
      <main className="flex w-full max-w-3xl flex-col gap-6 rounded-lg border border-zinc-200 bg-white p-8 dark:border-zinc-800 dark:bg-zinc-900">
        <h1 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
          実験一覧
        </h1>

        {experiments.length === 0 ? (
          <p className="text-sm text-zinc-500 dark:text-zinc-400">
            まだ実験がありません。
            <Link href="/" className="underline">
              トップページ
            </Link>
            からデータを保存すると、ここに表示されます。
          </p>
        ) : (
          <ul className="flex flex-col divide-y divide-zinc-200 dark:divide-zinc-800">
            {experiments.map((e) => (
              <li key={e.id}>
                <Link
                  href={`/experiments/${e.id}`}
                  className="flex items-center justify-between gap-4 py-3 hover:bg-zinc-50 dark:hover:bg-zinc-800"
                >
                  <span className="text-sm text-zinc-900 dark:text-zinc-50">
                    {e.title ?? "(無題)"}
                  </span>
                  <span className="shrink-0 text-xs text-zinc-500 dark:text-zinc-400">
                    {e.created_at.slice(0, 10)}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        )}
      </main>
    </div>
  );
}
