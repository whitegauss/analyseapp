"use client";

import Link from "next/link";
import { useActionState } from "react";
import {
  signInWithGoogle,
  signInWithPassword,
  type AuthActionState,
} from "./actions";

const initialState: AuthActionState = {};

export default function LoginPage() {
  const [state, formAction, pending] = useActionState(
    signInWithPassword,
    initialState,
  );

  return (
    <div className="flex min-h-screen items-center justify-center bg-zinc-50 dark:bg-black">
      <main className="flex w-full max-w-sm flex-col gap-6 rounded-lg border border-zinc-200 bg-white p-8 dark:border-zinc-800 dark:bg-zinc-900">
        <h1 className="text-2xl font-semibold text-zinc-900 dark:text-zinc-50">
          ログイン
        </h1>

        <form action={formAction} className="flex flex-col gap-4">
          <label className="flex flex-col gap-1 text-sm text-zinc-700 dark:text-zinc-300">
            メールアドレス
            <input
              type="email"
              name="email"
              required
              className="rounded-md border border-zinc-300 bg-transparent px-3 py-2 text-sm dark:border-zinc-700"
            />
          </label>
          <label className="flex flex-col gap-1 text-sm text-zinc-700 dark:text-zinc-300">
            パスワード
            <input
              type="password"
              name="password"
              required
              className="rounded-md border border-zinc-300 bg-transparent px-3 py-2 text-sm dark:border-zinc-700"
            />
          </label>

          {state.error && (
            <p className="text-sm text-red-600 dark:text-red-400">{state.error}</p>
          )}

          <button
            type="submit"
            disabled={pending}
            className="rounded-md bg-zinc-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-50 dark:bg-zinc-50 dark:text-zinc-900"
          >
            {pending ? "ログイン中..." : "ログイン"}
          </button>
        </form>

        <form action={signInWithGoogle}>
          <button
            type="submit"
            className="w-full rounded-md border border-zinc-300 px-4 py-2 text-sm font-medium text-zinc-700 dark:border-zinc-700 dark:text-zinc-300"
          >
            Googleでログイン
          </button>
        </form>

        <p className="text-center text-sm text-zinc-500 dark:text-zinc-400">
          アカウントをお持ちでない方は{" "}
          <Link href="/signup" className="underline">
            新規登録
          </Link>
        </p>
      </main>
    </div>
  );
}
