"use client";

import katex from "katex";

export type AxisLabelRun = { text: string; italic: boolean };

// KaTeX ships no CJK font, so runs containing non-ASCII text (Japanese,
// unicode Greek, etc.) are rendered as plain DOM text instead — matching
// PDR.md §6's "math parts via KaTeX, text parts via DOM" split.
const NON_ASCII = /[^\x00-\x7F]/;
const LATEX_SPECIAL = /([\\{}_^#&%$~])/g;

function katexHtml(run: AxisLabelRun): string {
  const escaped = run.text.replace(LATEX_SPECIAL, "\\$1");
  const latex = run.italic ? escaped : `\\mathrm{${escaped}}`;
  try {
    return katex.renderToString(latex, { throwOnError: false, output: "html" });
  } catch {
    return escaped;
  }
}

type Props = {
  runs: AxisLabelRun[];
  vertical?: boolean;
  className?: string;
};

export default function AxisLabelRuns({ runs, vertical = false, className = "" }: Props) {
  if (runs.length === 0) return null;

  return (
    <span
      className={`inline-flex items-baseline whitespace-nowrap text-sm text-zinc-700 dark:text-zinc-300 ${
        vertical ? "[writing-mode:vertical-rl] rotate-180" : ""
      } ${className}`}
    >
      {runs.map((run, i) =>
        NON_ASCII.test(run.text) ? (
          <span key={i} className={run.italic ? "italic" : ""}>
            {run.text}
          </span>
        ) : (
          <span key={i} dangerouslySetInnerHTML={{ __html: katexHtml(run) }} />
        ),
      )}
    </span>
  );
}
