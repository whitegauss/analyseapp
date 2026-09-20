"use client";

import { useActionState, useState } from "react";
import type { ProjectSummary } from "@/lib/dashboard";
import InlineEditCard from "./InlineEditCard";

type State = { error?: string };

type Props = {
  // The experiment being copied or moved. Sent as the form's `id`.
  experimentId: string;
  currentProjectId: string;
  projects: ProjectSummary[];
  action: (state: State, formData: FormData) => Promise<State>;
  label: string;
  description: string;
  submitLabel: string;
  pendingLabel: string;
  // Whether the project the experiment is already in stays selectable.
  // Copy keeps it (duplicating in place is a real thing to want); moving
  // there would do nothing, so move drops it.
  includeCurrentProject: boolean;
};

const initialState: State = {};

// The shell shared by "copy to another project" and "move to another
// project": pick a destination from the user's projects, submit. Both
// actions redirect on success, so neither needs to close this itself.
export default function ProjectPickerCard({
  experimentId,
  currentProjectId,
  projects,
  action,
  label,
  description,
  submitLabel,
  pendingLabel,
  includeCurrentProject,
}: Props) {
  const [editing, setEditing] = useState(false);
  const [state, formAction, pending] = useActionState(action, initialState);

  const options = includeCurrentProject
    ? projects
    : projects.filter((project) => project.id !== currentProjectId);

  // Start somewhere other than where the experiment already is: that is
  // what both buttons are for.
  const [projectId, setProjectId] = useState(
    options.find((project) => project.id !== currentProjectId)?.id ??
      options[0]?.id ??
      "",
  );

  // Nowhere to go. For move this is the common case of a user with a
  // single project; for copy it means every project vanished in another
  // tab, since this experiment's own must exist.
  if (options.length === 0) return null;

  return (
    <InlineEditCard
      label={label}
      editing={editing}
      onStartEditing={() => setEditing(true)}
      onCancel={() => setEditing(false)}
      formAction={formAction}
      pending={pending}
      submitLabel={submitLabel}
      pendingLabel={pendingLabel}
      error={state.error}
    >
      <div className="flex flex-col gap-1">
        <div className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
          {label}
        </div>
        <p className="text-xs text-zinc-500 dark:text-zinc-400">
          {description}
        </p>
      </div>

      <input type="hidden" name="id" value={experimentId} />
      <label className="flex flex-col gap-1 text-sm text-zinc-700 dark:text-zinc-300">
        {includeCurrentProject ? "コピー先" : "移動先"}
        <select
          name="projectId"
          value={projectId}
          onChange={(e) => setProjectId(e.target.value)}
          className="rounded-md border border-zinc-300 bg-transparent px-3 py-2 text-sm dark:border-zinc-700"
        >
          {options.map((project) => (
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
