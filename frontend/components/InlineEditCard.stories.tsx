import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn } from "storybook/test";
import InlineEditCard from "./InlineEditCard";

// The pattern every play function here follows: arrange with args, act
// with userEvent, assert on the canvas or on the fn() spies passed as
// callbacks.
const meta = {
  component: InlineEditCard,
  args: {
    label: "軸ラベルを編集",
    editing: false,
    pending: false,
    onStartEditing: fn(),
    onCancel: fn(),
    formAction: fn(),
    children: <input aria-label="X軸" defaultValue="時間 $t$ (s)" />,
  },
} satisfies Meta<typeof InlineEditCard>;
export default meta;

type Story = StoryObj<typeof meta>;

export const Collapsed: Story = {
  play: async ({ canvas, args, userEvent }) => {
    // Nothing but the toggle: no form until editing starts.
    await expect(
      canvas.queryByRole("button", { name: "保存" }),
    ).not.toBeInTheDocument();
    await expect(canvas.queryByLabelText("X軸")).not.toBeInTheDocument();

    await userEvent.click(canvas.getByRole("button", { name: args.label }));
    await expect(args.onStartEditing).toHaveBeenCalledOnce();
  },
};

export const Editing: Story = {
  args: { editing: true },
  play: async ({ canvas, args, userEvent }) => {
    await expect(canvas.getByLabelText("X軸")).toBeInTheDocument();
    await expect(canvas.getByRole("button", { name: "保存" })).toBeEnabled();

    await userEvent.click(canvas.getByRole("button", { name: "キャンセル" }));
    await expect(args.onCancel).toHaveBeenCalledOnce();
  },
};

// While the action runs the submit button must not take a second click.
export const Pending: Story = {
  args: { editing: true, pending: true },
  play: async ({ canvas }) => {
    await expect(
      canvas.getByRole("button", { name: "保存中..." }),
    ).toBeDisabled();
  },
};

export const WithError: Story = {
  args: { editing: true, error: "x列とy列のデータが必要です" },
  play: async ({ canvas, args }) => {
    await expect(canvas.getByText(args.error!)).toBeInTheDocument();
  },
};

// The editor decides when its input is not worth submitting (FitEditor with
// a formula that does not parse).
export const SubmitDisabled: Story = {
  args: { editing: true, submitDisabled: true },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("button", { name: "保存" })).toBeDisabled();
    await expect(
      canvas.getByRole("button", { name: "キャンセル" }),
    ).toBeEnabled();
  },
};
