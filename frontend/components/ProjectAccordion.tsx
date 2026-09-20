"use client";

import Link from "next/link";
import { useState } from "react";
import type { ProjectWithExperiments } from "@/lib/dashboard";
import DeleteProjectButton from "./DeleteProjectButton";
import RenameProjectForm from "./RenameProjectForm";

type Props = {
  projects: ProjectWithExperiments[];
};

// The dashboard's project list: each row expands in place to show the
// experiments in that project. Expanding costs nothing -- the page already
// fetched every experiment to count them (see lib/dashboard) -- so this is
// state, not a fetch.
export default function ProjectAccordion({ projects }: Props) {
  const [expandedIds, setExpandedIds] = useState<Set<string>>(new Set());

  function toggle(id: string) {
    setExpandedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) {
        next.delete(id);
      } else {
        next.add(id);
      }
      return next;
    });
  }

  return (
    <ul className="flex flex-col divide-y divide-zinc-200 dark:divide-zinc-800">
      {projects.map((project) => {
        const expanded = expandedIds.has(project.id);
        return (
          <li key={project.id} className="flex flex-col gap-2 py-3">
            <div className="flex items-center gap-3">
              <button
                type="button"
                onClick={() => toggle(project.id)}
                aria-expanded={expanded}
                className="flex min-w-0 flex-1 items-center gap-2 text-left"
              >
                <span
                  aria-hidden
                  className="shrink-0 text-xs text-zinc-400 dark:text-zinc-500"
                >
                  {expanded ? "▾" : "▸"}
                </span>
                <span className="truncate text-sm font-medium text-zinc-900 dark:text-zinc-50">
                  {project.title}
                </span>
                <span className="shrink-0 text-xs text-zinc-500 dark:text-zinc-400">
                  {project.experiments.length}件
                </span>
              </button>
              <DeleteProjectButton
                id={project.id}
                title={project.title}
                experimentCount={project.experiments.length}
              />
            </div>

            {expanded && (
              <div className="flex flex-col gap-2 pl-5">
                {project.description && (
                  <p className="text-xs text-zinc-500 dark:text-zinc-400">
                    {project.description}
                  </p>
                )}

                {project.experiments.length === 0 ? (
                  <p className="text-xs text-zinc-500 dark:text-zinc-400">
                    まだ実験がありません。
                  </p>
                ) : (
                  <ul className="flex flex-col">
                    {project.experiments.map((experiment) => (
                      <li key={experiment.id}>
                        <Link
                          href={`/experiments/${experiment.id}`}
                          className="flex items-center justify-between gap-4 py-1.5 text-sm hover:bg-zinc-50 dark:hover:bg-zinc-800"
                        >
                          <span className="truncate text-zinc-900 dark:text-zinc-50">
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

                <div className="flex flex-wrap items-center gap-4">
                  <Link
                    href={`/projects/${project.id}`}
                    className="text-xs text-zinc-900 underline dark:text-zinc-50"
                  >
                    + 実験を追加
                  </Link>
                  <RenameProjectForm
                    id={project.id}
                    title={project.title}
                    description={project.description}
                  />
                </div>
              </div>
            )}
          </li>
        );
      })}
    </ul>
  );
}
