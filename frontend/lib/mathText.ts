// Text wrapped in $...$ is math; everything outside it is plain text.
//
// This lives in lib/ rather than beside AxisLabel because two very different
// renderers consume the same parse: the axis label renders math with KaTeX in
// the DOM, while the chart legend can only use Plotly's small HTML subset and
// approximates math with <i>. Sharing the parse is what keeps `速度 $v$ (m/s)`
// authored once and meaning the same thing in both places.
//
// LaTeX's own rules apply inside the delimiters ($v$ is italic,
// $\mathrm{m/s}$ is upright). Japanese text stays outside them and therefore
// upright, which matters because KaTeX ships no CJK font.
const MATH_SEGMENT = /\$([^$]+)\$/g;

export type TextSegment = { math: boolean; content: string };

export function parseSegments(label: string): TextSegment[] {
  const segments: TextSegment[] = [];
  let lastIndex = 0;
  for (const match of label.matchAll(MATH_SEGMENT)) {
    if (match.index > lastIndex) {
      segments.push({
        math: false,
        content: label.slice(lastIndex, match.index),
      });
    }
    segments.push({ math: true, content: match[1] });
    lastIndex = match.index + match[0].length;
  }
  if (lastIndex < label.length) {
    segments.push({ math: false, content: label.slice(lastIndex) });
  }
  return segments;
}
