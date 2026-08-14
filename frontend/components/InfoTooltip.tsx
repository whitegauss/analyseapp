export default function InfoTooltip({ text }: { text: string }) {
  return (
    <span className="group relative inline-flex cursor-help items-center">
      <span className="flex h-4 w-4 items-center justify-center rounded-full border border-zinc-400 text-[10px] leading-none text-zinc-500 dark:border-zinc-600 dark:text-zinc-400">
        i
      </span>
      <span className="pointer-events-none absolute bottom-full left-1/2 z-10 mb-1 w-64 -translate-x-1/2 rounded-md border border-zinc-200 bg-white p-2 text-xs font-normal text-zinc-700 opacity-0 shadow-lg transition-opacity group-hover:opacity-100 dark:border-zinc-700 dark:bg-zinc-800 dark:text-zinc-200">
        {text}
      </span>
    </span>
  );
}
