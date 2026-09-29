import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect } from "storybook/test";
import InfoTooltip from "./InfoTooltip";

const meta = { component: InfoTooltip } satisfies Meta<typeof InfoTooltip>;
export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    text: "$...$ で囲んだ部分だけ TeX の数式として斜体表示されます",
  },
  play: async ({ canvas, args }) => {
    // Shown on hover by CSS alone, so it is always in the DOM -- which is
    // also what keeps it readable to screen readers.
    await expect(canvas.getByText(args.text)).toBeInTheDocument();
  },
};

// Longer than the tooltip is wide: it wraps inside its fixed width rather
// than running off the side.
export const LongText: Story = {
  args: {
    text: "任意入力です。$...$で囲んだ部分だけTeXの数式として斜体表示、それ以外は日本語も含めそのまま立体表示されます（例: 速度 $v$ (m/s)）",
  },
  play: async ({ canvas, args }) => {
    await expect(canvas.getByText(args.text)).toHaveClass("w-64");
  },
};
