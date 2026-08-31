"use client";

import katex from "katex";
import { parseSegments } from "@/lib/mathText";

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

export default function AxisLabel({
  label,
  vertical = false,
  className = "",
}: Props) {
  if (!label.trim()) return null;

  return (
    <span
      className={`inline-flex items-baseline whitespace-nowrap text-sm text-zinc-700 dark:text-zinc-300 ${
        vertical ? "[writing-mode:vertical-rl] rotate-180" : ""
      } ${className}`}
    >
      {parseSegments(label).map((seg, i) =>
        seg.math ? (
          <span
            key={i}
            dangerouslySetInnerHTML={{ __html: katexHtml(seg.content) }}
          />
        ) : (
          <span key={i}>{seg.content}</span>
        ),
      )}
    </span>
  );
}
