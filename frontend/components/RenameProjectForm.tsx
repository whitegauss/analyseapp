"use client";

import { useActionState, useState } from "react";
import { updateProject, type ProjectFormState } from "@/app/projects/actions";
import InlineEditCard from "./InlineEditCard";

const initialState: ProjectFormState = {};

type Props = {
  id: string;
  title: string;
  description: string;
};

export default function RenameProjectForm({ id, title, description }: Props) {
  const [editing, setEditing] = useState(false);
  const [state, formAction, pending] = useActionState(
    updateProject,
    initialState,
  );
  const [nextTitle, setNextTitle] = useState(title);
  const [nextDescription, setNextDescription] = useState(description);

  // The experiment editors close by navigating away; this one stays on the
  // dashboard, so it has to close itself when the action reports success.
  // Adjusted during render rather than in an effect (React's own advice for
  // "state that changes when something else changes", and what the
  // react-hooks lint rules require): useActionState hands back a new state
  // object per run, so this fires once per submission and reopening the
  // form does not re-trigger it.
  const [lastState, setLastState] = useState(state);
  if (state !== lastState) {
    setLastState(state);
    if (state.ok) setEditing(false);
  }

  return (
    <InlineEditCard
      label="名前を変更"
      editing={editing}
      onStartEditing={() => setEditing(true)}
      onCancel={() => setEditing(false)}
      formAction={formAction}
      pending={pending}
      submitDisabled={nextTitle.trim() === ""}
      error={state.error}
    >
      <input type="hidden" name="id" value={id} />
      <label className="flex flex-col gap-1 text-sm text-zinc-700 dark:text-zinc-300">
        プロジェクト名
        <input
          type="text"
          name="title"
          value={nextTitle}
          onChange={(e) => setNextTitle(e.target.value)}
          className="rounded-md border border-zinc-300 bg-transparent px-3 py-2 text-sm dark:border-zinc-700"
        />
      </label>
      <label className="flex flex-col gap-1 text-sm text-zinc-700 dark:text-zinc-300">
        説明（任意）
        <input
          type="text"
          name="description"
          value={nextDescription}
          onChange={(e) => setNextDescription(e.target.value)}
          className="rounded-md border border-zinc-300 bg-transparent px-3 py-2 text-sm dark:border-zinc-700"
        />
      </label>
    </InlineEditCard>
  );
}
