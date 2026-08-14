"use client";

import katex from "katex";

// Text wrapped in $...$ is rendered as KaTeX math (LaTeX's own italics/upright
// rules apply, e.g. $v$ is italic, $\mathrm{m/s}$ is upright). Everything
// outside $...$ is plain DOM text — this is also how Japanese ends up
// upright, since KaTeX has no CJK font.
const MATH_SEGMENT = /\$([^$]+)\$/g;

function parseSegments(label: string): { math: boolean; content: string }[] {
  const segments: { math: boolean; content: string }[] = [];
  let lastIndex = 0;
  for (const match of label.matchAll(MATH_SEGMENT)) {
    if (match.index > lastIndex) {
      segments.push({ math: false, content: label.slice(lastIndex, match.index) });
    }
    segments.push({ math: true, content: match[1] });
    lastIndex = match.index + match[0].length;
  }
  if (lastIndex < label.length) {
    segments.push({ math: false, content: label.slice(lastIndex) });
  }
  return segments;
}

function katexHtml(tex: string): string {
  try {
    return katex.renderToString(tex, { throwOnError: false, output: "html" });
  } catch {
    return tex;
  }
}

type Props = {
  label: string;
  vertical?: boolean;
  className?: string;
};

export default function AxisLabel({ label, vertical = false, className = "" }: Props) {
  if (!label.trim()) return null;

  return (
    <span
      className={`inline-flex items-baseline whitespace-nowrap text-sm text-zinc-700 dark:text-zinc-300 ${
        vertical ? "[writing-mode:vertical-rl] rotate-180" : ""
      } ${className}`}
    >
      {parseSegments(label).map((seg, i) =>
        seg.math ? (
          <span key={i} dangerouslySetInnerHTML={{ __html: katexHtml(seg.content) }} />
        ) : (
          <span key={i}>{seg.content}</span>
        ),
      )}
    </span>
  );
}
