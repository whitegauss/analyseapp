import type { ParsedTable } from "@/lib/pasteDataParsing";
import ColumnRoleSelector from "./ColumnRoleSelector";

type Props = {
  pastedText: string;
  onPastedTextChange: (value: string) => void;
  parsed: ParsedTable;
  extraRoles: Record<number, string>;
  customNames: Record<number, string>;
  onRoleChange: (col: number, value: string) => void;
  onCustomNameChange: (col: number, value: string) => void;
  placeholder?: string;
};

// The paste-and-parse data input shared by the create flow (ExperimentEditor)
// and the post-save edit flow (RawDataEditor): a tab/comma/space-delimited
// textarea, a parse-error message, and the 3rd-column-onward role selector.
export default function PasteDataFields({
  pastedText,
  onPastedTextChange,
  parsed,
  extraRoles,
  customNames,
  onRoleChange,
  onCustomNameChange,
  placeholder,
}: Props) {
  const extraColumnIndexes = Array.from(
    { length: Math.max(parsed.columnCount - 2, 0) },
    (_, i) => i + 2,
  );

  return (
    <>
      <label className="flex flex-col gap-1 text-sm text-zinc-700 dark:text-zinc-300">
        データ（スプレッドシートからコピー＆ペースト。1列目=x, 2列目=y）
        <textarea
          rows={8}
          value={pastedText}
          onChange={(e) => onPastedTextChange(e.target.value)}
          placeholder={placeholder}
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
          onRoleChange={onRoleChange}
          onCustomNameChange={onCustomNameChange}
        />
      )}
    </>
  );
}
