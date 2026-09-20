"use client";

import { useState } from "react";
import { deleteProject } from "@/app/projects/actions";

type Props = {
  id: string;
  title: string;
  experimentCount: number;
};

// Same inline two-step as DeleteExperimentButton (no browser confirm()), but
// the confirmation has to spell out the cascade: deleting a project deletes
// every experiment in it, through the ON DELETE CASCADE on
// experiments.project_id (PDR.md section 5). Showing the count is the point
// -- "3件も一緒に削除されます" is the difference between a safe click and a
// lost afternoon.
export default function DeleteProjectButton({
  id,
  title,
  experimentCount,
}: Props) {
  const [confirming, setConfirming] = useState(false);

  if (!confirming) {
    return (
      <button
        type="button"
        onClick={() => setConfirming(true)}
        className="shrink-0 text-xs text-red-600 underline hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
      >
        削除
      </button>
    );
  }

  return (
    <div className="flex shrink-0 flex-wrap items-center gap-2 text-xs">
      <span className="text-zinc-600 dark:text-zinc-400">
        「{title}」を削除しますか？
        {experimentCount > 0 && (
          <span className="font-medium text-red-600 dark:text-red-400">
            {" "}
            配下の実験{experimentCount}件も一緒に削除されます。
          </span>
        )}
      </span>
      <form action={deleteProject}>
        <input type="hidden" name="id" value={id} />
        <button
          type="submit"
          className="font-medium text-red-600 underline hover:text-red-700 dark:text-red-400 dark:hover:text-red-300"
        >
          削除する
        </button>
      </form>
      <button
        type="button"
        onClick={() => setConfirming(false)}
        className="text-zinc-500 underline hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
      >
        キャンセル
      </button>
    </div>
  );
}
