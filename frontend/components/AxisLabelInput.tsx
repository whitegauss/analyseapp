type Props = {
  label: string;
  value: string;
  onChange: (value: string) => void;
};

export default function AxisLabelInput({ label, value, onChange }: Props) {
  return (
    <label className="flex flex-col gap-1.5 text-sm text-zinc-700 dark:text-zinc-300">
      {label}
      <input
        type="text"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder="例: 速度 $v$ (m/s)"
        className="rounded-md border border-zinc-300 bg-transparent px-2 py-1.5 text-sm dark:border-zinc-700"
      />
    </label>
  );
}
