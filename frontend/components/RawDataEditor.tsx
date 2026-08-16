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
import PasteDataFields from "./PasteDataFields";
import InlineEditCard from "./InlineEditCard";

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

  return (
    <InlineEditCard
      label="データを編集"
      editing={editing}
      onStartEditing={() => setEditing(true)}
      onCancel={() => setEditing(false)}
      formAction={formAction}
      pending={pending}
      submitDisabled={!rebuiltColumns}
      error={state.error}
    >
      <h2 className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
        データを編集
      </h2>
      <input type="hidden" name="id" value={id} />

      <PasteDataFields
        pastedText={pastedText}
        onPastedTextChange={setPastedText}
        parsed={parsed}
        extraRoles={extraRoles}
        customNames={customNames}
        onRoleChange={(col, value) =>
          setExtraRoles((prev) => ({ ...prev, [col]: value }))
        }
        onCustomNameChange={(col, value) =>
          setCustomNames((prev) => ({ ...prev, [col]: value }))
        }
      />

      <input
        type="hidden"
        name="columns"
        value={rebuiltColumns ? JSON.stringify(rebuiltColumns) : ""}
      />
    </InlineEditCard>
  );
}
