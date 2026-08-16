"use client";

import type { ReactNode } from "react";

type Props = {
  label: string;
  editing: boolean;
  onStartEditing: () => void;
  onCancel: () => void;
  formAction: (formData: FormData) => void;
  pending: boolean;
  submitDisabled?: boolean;
  error?: string;
  children: ReactNode;
};

// The "collapsed toggle button, expanded into a bordered form with
// save/cancel" pattern shared by AxisLabelEditor and RawDataEditor for
// editing a saved experiment's fields in place (no browser dialogs, no
// separate edit page).
export default function InlineEditCard({
  label,
  editing,
  onStartEditing,
  onCancel,
  formAction,
  pending,
  submitDisabled,
  error,
  children,
}: Props) {
  if (!editing) {
    return (
      <button
        type="button"
        onClick={onStartEditing}
        className="self-start text-xs text-zinc-500 underline hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
      >
        {label}
      </button>
    );
  }

  return (
    <form
      action={formAction}
      className="flex w-full flex-col gap-3 rounded-md border border-zinc-200 p-3 dark:border-zinc-800"
    >
      {children}

      {error && (
        <p className="text-sm text-red-600 dark:text-red-400">{error}</p>
      )}

      <div className="flex items-center gap-2">
        <button
          type="submit"
          disabled={pending || submitDisabled}
          className="rounded-md bg-zinc-900 px-3 py-1.5 text-xs font-medium text-white disabled:opacity-50 dark:bg-zinc-50 dark:text-zinc-900"
        >
          {pending ? "保存中..." : "保存"}
        </button>
        <button
          type="button"
          onClick={onCancel}
          className="text-xs text-zinc-500 underline hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
        >
          キャンセル
        </button>
      </div>
    </form>
  );
}
