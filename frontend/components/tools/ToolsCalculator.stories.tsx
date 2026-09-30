import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect } from "storybook/test";
import ToolsCalculator from "./ToolsCalculator";

const meta = {
  component: ToolsCalculator,
} satisfies Meta<typeof ToolsCalculator>;
export default meta;

type Story = StoryObj<typeof meta>;

// One tool at a time, starting on error propagation.
export const Tabs: Story = {
  play: async ({ canvas, userEvent }) => {
    await expect(canvas.getByRole("textbox", { name: "式" })).toBeVisible();

    await userEvent.click(canvas.getByRole("button", { name: "単位変換" }));
    await expect(canvas.getByLabelText("種類")).toBeVisible();
    await expect(canvas.queryByRole("textbox", { name: "式" })).toBeNull();

    await userEvent.click(
      canvas.getByRole("button", { name: "有効数字の丸め" }),
    );
    await expect(canvas.getByLabelText("不確かさ（1σ）")).toBeVisible();

    await userEvent.click(canvas.getByRole("button", { name: "統計量" }));
    await expect(canvas.getByRole("textbox")).toHaveAttribute("rows", "6");
  },
};
