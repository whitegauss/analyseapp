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
