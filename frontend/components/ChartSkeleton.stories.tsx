import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect } from "storybook/test";
import ChartSkeleton from "./ChartSkeleton";

const meta = { component: ChartSkeleton } satisfies Meta<typeof ChartSkeleton>;
export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvas }) => {
    const message = canvas.getByText("グラフを準備中...");
    // Same height as the chart it stands in for, so nothing jumps when
    // Plotly arrives.
    await expect(message.parentElement).toHaveStyle({ height: "480px" });
  },
};
