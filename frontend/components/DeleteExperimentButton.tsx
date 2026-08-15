"use client";

import { useState } from "react";
import { deleteExperiment } from "@/app/experiments/actions";

type Props = {
  id: string;
  title: string;
};

export default function DeleteExperimentButton({ id, title }: Props) {
  const [confirming, setConfirming] = useState(false);

  if (confirming) {
    return (
      <div className="flex shrink-0 items-center gap-2 text-xs">
        <span className="text-zinc-600 dark:text-zinc-400">
          「{title}」を削除しますか？
        </span>
        <form action={deleteExperiment}>
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
