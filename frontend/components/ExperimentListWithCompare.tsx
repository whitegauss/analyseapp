"use client";

import Link from "next/link";
import { useState } from "react";
import DeleteExperimentButton from "./DeleteExperimentButton";

type ExperimentSummary = {
  id: string;
  title: string | null;
  created_at: string;
};

type Props = {
  experiments: ExperimentSummary[];
};

export default function ExperimentListWithCompare({ experiments }: Props) {
  const [selectedIds, setSelectedIds] = useState<Set<string>>(new Set());

  function toggle(id: string) {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  const compareHref =
    selectedIds.size >= 2
      ? `/experiments/compare?ids=${Array.from(selectedIds).join(",")}`
      : null;

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center justify-between gap-3">
        <p className="text-xs text-zinc-500 dark:text-zinc-400">
          チェックボックスで2件以上選ぶと比較できます
        </p>
        {compareHref ? (
          <Link
            href={compareHref}
            className="shrink-0 rounded-md bg-zinc-900 px-3 py-1.5 text-xs font-medium text-white dark:bg-zinc-50 dark:text-zinc-900"
          >
            選択した{selectedIds.size}件を比較
          </Link>
        ) : (
          <span className="shrink-0 rounded-md bg-zinc-200 px-3 py-1.5 text-xs font-medium text-zinc-400 dark:bg-zinc-800 dark:text-zinc-600">
            比較（2件以上選択）
          </span>
        )}
      </div>

      <ul className="flex flex-col divide-y divide-zinc-200 dark:divide-zinc-800">
        {experiments.map((e) => (
          <li
            key={e.id}
            className="flex items-center gap-4 py-3 hover:bg-zinc-50 dark:hover:bg-zinc-800"
          >
            <input
              type="checkbox"
              checked={selectedIds.has(e.id)}
              onChange={() => toggle(e.id)}
              aria-label={`「${e.title ?? "(無題)"}」を比較対象に選択`}
              className="shrink-0"
            />
            <Link
              href={`/experiments/${e.id}`}
              className="flex min-w-0 flex-1 items-center justify-between gap-4"
            >
              <span className="truncate text-sm text-zinc-900 dark:text-zinc-50">
                {e.title ?? "(無題)"}
              </span>
              <span className="shrink-0 text-xs text-zinc-500 dark:text-zinc-400">
                {e.created_at.slice(0, 10)}
              </span>
            </Link>
            <DeleteExperimentButton id={e.id} title={e.title ?? "(無題)"} />
          </li>
        ))}
      </ul>
    </div>
  );
}
