"use client";

import { useActionState, useState } from "react";
import {
  copyExperiment,
  type CopyExperimentState,
} from "@/app/experiments/actions";
import type { ProjectSummary } from "@/lib/dashboard";
import InlineEditCard from "./InlineEditCard";

const initialState: CopyExperimentState = {};

type Props = {
  id: string;
  currentProjectId: string;
  projects: ProjectSummary[];
};

export default function CopyExperimentButton({
  id,
  currentProjectId,
  projects,
}: Props) {
  const [editing, setEditing] = useState(false);
  const [state, formAction, pending] = useActionState(
    copyExperiment,
    initialState,
  );
  // Preselect somewhere other than where the experiment already is, since
  // that is what the button is for. Copying into the same project is still
  // allowed -- duplicating an experiment to try a variant is a real thing
  // to want -- so that option stays in the list, just labelled.
  const [projectId, setProjectId] = useState(
    projects.find((project) => project.id !== currentProjectId)?.id ??
      currentProjectId,
  );

  // Nothing to copy into. Only reachable if every project was deleted in
  // another tab, since this experiment's own project must exist.
  if (projects.length === 0) return null;

  return (
    <InlineEditCard
      label="他のプロジェクトへコピー"
      editing={editing}
      onStartEditing={() => setEditing(true)}
      onCancel={() => setEditing(false)}
      formAction={formAction}
      pending={pending}
      error={state.error}
    >
      <div className="flex flex-col gap-1">
        <div className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
          他のプロジェクトへコピー
        </div>
        <p className="text-xs text-zinc-500 dark:text-zinc-400">
          複製です。コピー元はこのまま残り、コピーは別の実験として独立します（片方を編集しても、もう片方は変わりません）。
        </p>
      </div>

      <input type="hidden" name="id" value={id} />
      <label className="flex flex-col gap-1 text-sm text-zinc-700 dark:text-zinc-300">
        コピー先
        <select
          name="projectId"
          value={projectId}
          onChange={(e) => setProjectId(e.target.value)}
          className="rounded-md border border-zinc-300 bg-transparent px-3 py-2 text-sm dark:border-zinc-700"
        >
          {projects.map((project) => (
            <option key={project.id} value={project.id}>
              {project.title}
              {project.id === currentProjectId && "（現在のプロジェクト）"}
            </option>
          ))}
        </select>
      </label>
    </InlineEditCard>
  );
}
