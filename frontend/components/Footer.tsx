export default function Footer() {
  return (
    <footer className="border-t border-zinc-200 bg-white px-6 py-4 dark:border-zinc-800 dark:bg-zinc-900">
      <div className="flex items-center justify-center gap-4 text-xs text-zinc-500 dark:text-zinc-400">
        <a
          href="https://github.com/whitegauss/analyseapp"
          target="_blank"
          rel="noopener noreferrer"
          className="underline hover:text-zinc-700 dark:hover:text-zinc-200"
        >
          GitHub
        </a>
      </div>
    </footer>
  );
}
