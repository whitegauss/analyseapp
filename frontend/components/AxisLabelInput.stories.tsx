import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn } from "storybook/test";
import AxisLabelInput from "./AxisLabelInput";

const meta = {
  component: AxisLabelInput,
  args: { label: "X軸", value: "", onChange: fn() },
} satisfies Meta<typeof AxisLabelInput>;
export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  play: async ({ canvas }) => {
    const input = canvas.getByLabelText("X軸");
    await expect(input).toHaveValue("");
    // The hint shows the $...$ notation, since nothing else does.
    await expect(input).toHaveAttribute("placeholder", "例: 速度 $v$ (m/s)");
  },
};

export const WithValue: Story = {
  args: { value: "時間 (s)" },
  play: async ({ canvas }) => {
    await expect(canvas.getByLabelText("X軸")).toHaveValue("時間 (s)");
  },
};

// Controlled: each keystroke is reported, and the value shown is whatever
// the parent passes back -- here it never does, so it stays empty.
export const WithMathNotation: Story = {
  play: async ({ canvas, args, userEvent }) => {
    await userEvent.type(canvas.getByLabelText("X軸"), "$v$");
    await expect(args.onChange).toHaveBeenCalledTimes(3);
    await expect(args.onChange).toHaveBeenLastCalledWith("$");
  },
};
