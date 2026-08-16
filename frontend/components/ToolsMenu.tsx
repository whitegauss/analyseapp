"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";

// Small nav menu for standalone tool pages that aren't tied to a specific
// experiment or to being logged in (unlike the rest of the header, which is
// all about the current experiment/session). A single entry today, but the
// dropdown shape means more tool pages can be added without a redesign.
export default function ToolsMenu() {
  const [open, setOpen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    function handleClickOutside(event: MouseEvent) {
      if (
        containerRef.current &&
        !containerRef.current.contains(event.target as Node)
      ) {
        setOpen(false);
      }
    }
    document.addEventListener("mousedown", handleClickOutside);
    return () => document.removeEventListener("mousedown", handleClickOutside);
  }, [open]);

  return (
    <div ref={containerRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        aria-label="メニュー"
        aria-expanded={open}
        className="flex h-9 w-9 items-center justify-center rounded-md border border-zinc-300 text-zinc-700 dark:border-zinc-700 dark:text-zinc-300"
      >
        <svg
          xmlns="http://www.w3.org/2000/svg"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth={2}
          strokeLinecap="round"
          className="h-5 w-5"
        >
          <line x1="3" y1="6" x2="21" y2="6" />
          <line x1="3" y1="12" x2="21" y2="12" />
          <line x1="3" y1="18" x2="21" y2="18" />
        </svg>
      </button>

      {open && (
        <div className="absolute left-0 top-full z-10 mt-2 w-48 rounded-md border border-zinc-200 bg-white py-1 shadow-lg dark:border-zinc-800 dark:bg-zinc-900">
          <Link
            href="/tools"
            onClick={() => setOpen(false)}
            className="block px-4 py-2 text-sm text-zinc-700 hover:bg-zinc-100 dark:text-zinc-300 dark:hover:bg-zinc-800"
          >
            計算ツール
          </Link>
        </div>
      )}
    </div>
  );
}
