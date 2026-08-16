import Link from "next/link";
import { createClient } from "@/lib/supabase/server";
import { signOut } from "@/app/actions";

export default async function Header() {
  const supabase = await createClient();
  const {
    data: { user },
  } = await supabase.auth.getUser();

  return (
    <header className="flex items-center justify-between border-b border-zinc-200 bg-white px-6 py-3 dark:border-zinc-800 dark:bg-zinc-900">
      <Link
        href="/"
        className="text-lg font-semibold text-zinc-900 dark:text-zinc-50"
      >
        AnalyseApp
      </Link>

      <div className="flex items-center gap-4 text-sm">
        {user ? (
          <>
            <Link
              href="/experiments"
              className="text-zinc-600 underline dark:text-zinc-400"
            >
              保存済みの実験一覧
            </Link>
            <span className="hidden text-zinc-500 sm:inline dark:text-zinc-400">
              {user.email}
            </span>
            <form action={signOut}>
              <button
                type="submit"
                className="rounded-md border border-zinc-300 px-3 py-1.5 text-zinc-700 dark:border-zinc-700 dark:text-zinc-300"
              >
                ログアウト
              </button>
            </form>
          </>
        ) : (
          <>
            <span className="text-zinc-500 dark:text-zinc-400">
              ログインしていません
            </span>
            <Link
              href="/login"
              className="rounded-md bg-zinc-900 px-3 py-1.5 font-medium text-white dark:bg-zinc-50 dark:text-zinc-900"
            >
              ログイン
            </Link>
          </>
        )}
      </div>
    </header>
  );
}
