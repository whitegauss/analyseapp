import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect } from "storybook/test";
import AxisLabel from "./AxisLabel";

// Only the rendering: how a label splits into text and math is
// lib/mathText.ts's, and tested there.
const meta = { component: AxisLabel } satisfies Meta<typeof AxisLabel>;
export default meta;

type Story = StoryObj<typeof meta>;

export const PlainText: Story = {
  args: { label: "時間 (s)" },
  play: async ({ canvas, canvasElement }) => {
    await expect(canvas.getByText("時間 (s)")).toBeInTheDocument();
    await expect(canvasElement.querySelector(".katex")).toBeNull();
  },
};

export const WithMath: Story = {
  args: { label: "$x$" },
  play: async ({ canvasElement }) => {
    // KaTeX's markup (HTML only -- no MathML, see katexHtml), with the
    // variable set in math italic: the reason for the $...$ notation.
    await expect(canvasElement.querySelector(".katex")).not.toBeNull();
    await expect(
      canvasElement.querySelector(".katex .mathnormal"),
    ).toHaveTextContent("x");
  },
};

export const MixedTextAndMath: Story = {
  args: { label: "速度 $v$ (m/s)" },
  play: async ({ canvas, canvasElement }) => {
    await expect(canvasElement.querySelectorAll(".katex")).toHaveLength(1);
    await expect(
      canvas.getByText("速度", { exact: false }),
    ).toBeInTheDocument();
    await expect(
      canvas.getByText("(m/s)", { exact: false }),
    ).toBeInTheDocument();
  },
};

// A blank label renders nothing at all, so the chart falls back to its
// default axis title instead of an empty box.
export const Empty: Story = {
  args: { label: "  " },
  play: async ({ canvasElement }) => {
    await expect(canvasElement.textContent).toBe("");
  },
};

// An unmatched $ is text, not the start of an equation that swallows the
// rest of the label.
export const UnclosedDollar: Story = {
  args: { label: "価格 $5" },
  play: async ({ canvas, canvasElement }) => {
    await expect(canvas.getByText("価格 $5")).toBeInTheDocument();
    await expect(canvasElement.querySelector(".katex")).toBeNull();
  },
};

// The y axis label, written top-to-bottom beside the chart.
export const Vertical: Story = {
  args: { label: "距離 $x$ (m)", vertical: true },
  play: async ({ canvasElement }) => {
    await expect(canvasElement.firstElementChild).toHaveClass("rotate-180");
  },
};
