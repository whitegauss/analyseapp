const ROLE_OPTIONS = [
  { value: "y_error", label: "y誤差 (y_error)" },
  { value: "x_error", label: "x誤差 (x_error)" },
  { value: "__ignore__", label: "使わない" },
  { value: "__custom__", label: "その他（名前を指定）" },
] as const;

type Props = {
  extraColumnIndexes: number[];
  extraRoles: Record<number, string>;
  customNames: Record<number, string>;
  onRoleChange: (col: number, value: string) => void;
  onCustomNameChange: (col: number, value: string) => void;
};

export default function ColumnRoleSelector({
  extraColumnIndexes,
  extraRoles,
  customNames,
  onRoleChange,
  onCustomNameChange,
}: Props) {
  return (
    <div className="flex flex-col gap-2">
      <h2 className="text-sm font-medium text-zinc-500 dark:text-zinc-400">
        3列目以降の役割
      </h2>
      {extraColumnIndexes.map((col) => (
        <div key={col} className="flex items-center gap-2">
          <span className="text-sm text-zinc-600 dark:text-zinc-400">
            {col + 1}列目:
          </span>
          <select
            value={extraRoles[col] ?? "__ignore__"}
            onChange={(e) => onRoleChange(col, e.target.value)}
            className="rounded-md border border-zinc-300 bg-transparent px-2 py-1 text-sm dark:border-zinc-700"
          >
            {ROLE_OPTIONS.map((opt) => (
              <option key={opt.value} value={opt.value}>
                {opt.label}
              </option>
            ))}
          </select>
          {extraRoles[col] === "__custom__" && (
            <input
              type="text"
              placeholder="カラム名"
              value={customNames[col] ?? ""}
              onChange={(e) => onCustomNameChange(col, e.target.value)}
              className="rounded-md border border-zinc-300 bg-transparent px-2 py-1 text-sm dark:border-zinc-700"
            />
          )}
        </div>
      ))}
    </div>
  );
}
