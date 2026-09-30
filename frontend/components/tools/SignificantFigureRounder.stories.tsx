import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect } from "storybook/test";
import SignificantFigureRounder from "./SignificantFigureRounder";

// Only the wiring: that what is typed reaches the result. The rounding
// itself is lib/significantFigures.ts's, and tested there.
const meta = {
  component: SignificantFigureRounder,
} satisfies Meta<typeof SignificantFigureRounder>;
export default meta;

type Story = StoryObj<typeof meta>;

export const Rounds: Story = {
  play: async ({ canvas, userEvent }) => {
    await expect(canvas.queryByText(/±/)).toBeNull();

    await userEvent.type(canvas.getByLabelText("値"), "9.8123");
    await userEvent.type(canvas.getByLabelText("不確かさ（1σ）"), "0.034");
    await expect(canvas.getByText("9.81 ± 0.03")).toBeVisible();
  },
};

// Half an input is not a result.
export const NeedsBoth: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(canvas.getByLabelText("値"), "9.8123");
    await expect(canvas.queryByText(/±/)).toBeNull();
  },
};
