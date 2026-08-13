export default function ChartSkeleton() {
  return (
    <div
      className="flex w-full animate-pulse items-center justify-center rounded-md border border-zinc-200 bg-zinc-100 dark:border-zinc-800 dark:bg-zinc-800"
      style={{ height: "480px" }}
    >
      <span className="text-sm text-zinc-400 dark:text-zinc-500">
        グラフを準備中...
      </span>
    </div>
  );
}
