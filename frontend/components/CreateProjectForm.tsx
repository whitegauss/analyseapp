"use client";

import { useActionState, useState } from "react";
import { createProject, type ProjectFormState } from "@/app/projects/actions";
import InlineEditCard from "./InlineEditCard";

const initialState: ProjectFormState = {};

export default function CreateProjectForm() {
  const [editing, setEditing] = useState(false);
  const [state, formAction, pending] = useActionState(
    createProject,
    initialState,
  );
  const [title, setTitle] = useState("");
  const [description, setDescription] = useState("");

  // Close and empty the fields once the project exists, so the next one
  // starts from a blank form rather than the previous project's name.
  // Adjusted during render rather than in an effect, same as
  // RenameProjectForm -- see the note there.
  const [lastState, setLastState] = useState(state);
  if (state !== lastState) {
    setLastState(state);
    if (state.ok) {
      setEditing(false);
      setTitle("");
      setDescription("");
    }
  }

  return (
    <InlineEditCard
      label="+ 新しく作る"
      editing={editing}
      onStartEditing={() => setEditing(true)}
      onCancel={() => setEditing(false)}
      formAction={formAction}
      pending={pending}
      submitDisabled={title.trim() === ""}
      error={state.error}
    >
      <div className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
        新しいプロジェクト
      </div>
      <label className="flex flex-col gap-1 text-sm text-zinc-700 dark:text-zinc-300">
        プロジェクト名
        <input
          type="text"
          name="title"
          value={title}
          onChange={(e) => setTitle(e.target.value)}
          placeholder="落下運動の実験"
          className="rounded-md border border-zinc-300 bg-transparent px-3 py-2 text-sm dark:border-zinc-700"
        />
      </label>
      <label className="flex flex-col gap-1 text-sm text-zinc-700 dark:text-zinc-300">
        説明（任意）
        <input
          type="text"
          name="description"
          value={description}
          onChange={(e) => setDescription(e.target.value)}
          placeholder="2026年度、力学実験レポート用"
          className="rounded-md border border-zinc-300 bg-transparent px-3 py-2 text-sm dark:border-zinc-700"
        />
      </label>
    </InlineEditCard>
  );
}
