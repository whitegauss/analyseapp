"use client";

import { useActionState, useMemo, useState } from "react";
import {
  updateRawData,
  type UpdateRawDataState,
} from "@/app/experiments/actions";
import {
  buildColumns,
  columnsToInitialRoles,
  columnsToPastedText,
  parsePastedText,
} from "@/lib/pasteDataParsing";
import ColumnRoleSelector from "./ColumnRoleSelector";

const initialState: UpdateRawDataState = {};

type Props = {
  id: string;
  columns: Record<string, number[]>;
};

export default function RawDataEditor({ id, columns }: Props) {
  const [editing, setEditing] = useState(false);
  const [state, formAction, pending] = useActionState(
    updateRawData,
    initialState,
  );

  const initialRoles = useMemo(() => columnsToInitialRoles(columns), [columns]);
  const [pastedText, setPastedText] = useState(() =>
    columnsToPastedText(columns),
  );
  const [extraRoles, setExtraRoles] = useState(initialRoles.extraRoles);
  const [customNames, setCustomNames] = useState(initialRoles.customNames);

  const parsed = useMemo(() => parsePastedText(pastedText), [pastedText]);
  const rebuiltColumns = useMemo(
    () => buildColumns(parsed, extraRoles, customNames),
    [parsed, extraRoles, customNames],
  );

  const extraColumnIndexes = Array.from(
    { length: Math.max(parsed.columnCount - 2, 0) },
    (_, i) => i + 2,
  );

  if (!editing) {
    return (
      <button
        type="button"
        onClick={() => setEditing(true)}
        className="self-start text-xs text-zinc-500 underline hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
      >
        データを編集
      </button>
    );
  }

  return (
    <form
      action={formAction}
      className="flex w-full flex-col gap-3 rounded-md border border-zinc-200 p-3 dark:border-zinc-800"
    >
      <h2 className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
        データを編集
      </h2>
      <input type="hidden" name="id" value={id} />

      <label className="flex flex-col gap-1 text-sm text-zinc-700 dark:text-zinc-300">
        データ（スプレッドシートからコピー＆ペースト。1列目=x, 2列目=y）
        <textarea
          rows={8}
          value={pastedText}
          onChange={(e) => setPastedText(e.target.value)}
          className="rounded-md border border-zinc-300 bg-transparent px-3 py-2 font-mono text-sm dark:border-zinc-700"
        />
      </label>

      {parsed.error && (
        <p className="text-sm text-red-600 dark:text-red-400">{parsed.error}</p>
      )}

      {extraColumnIndexes.length > 0 && !parsed.error && (
        <ColumnRoleSelector
          extraColumnIndexes={extraColumnIndexes}
          extraRoles={extraRoles}
          customNames={customNames}
          onRoleChange={(col, value) =>
            setExtraRoles((prev) => ({ ...prev, [col]: value }))
          }
          onCustomNameChange={(col, value) =>
            setCustomNames((prev) => ({ ...prev, [col]: value }))
          }
        />
      )}

      <input
        type="hidden"
        name="columns"
        value={rebuiltColumns ? JSON.stringify(rebuiltColumns) : ""}
      />

      {state.error && (
        <p className="text-sm text-red-600 dark:text-red-400">{state.error}</p>
      )}

      <div className="flex items-center gap-2">
        <button
          type="submit"
          disabled={pending || !rebuiltColumns}
          className="rounded-md bg-zinc-900 px-3 py-1.5 text-xs font-medium text-white disabled:opacity-50 dark:bg-zinc-50 dark:text-zinc-900"
        >
          {pending ? "保存中..." : "保存"}
        </button>
        <button
          type="button"
          onClick={() => setEditing(false)}
          className="text-xs text-zinc-500 underline hover:text-zinc-700 dark:text-zinc-400 dark:hover:text-zinc-200"
        >
          キャンセル
        </button>
      </div>
    </form>
  );
}
