import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn } from "storybook/test";
import ColumnRoleSelector from "./ColumnRoleSelector";

const meta = {
  component: ColumnRoleSelector,
  args: {
    extraColumnIndexes: [2],
    extraRoles: {},
    customNames: {},
    onRoleChange: fn(),
    onCustomNameChange: fn(),
  },
} satisfies Meta<typeof ColumnRoleSelector>;
export default meta;

type Story = StoryObj<typeof meta>;

// A column nobody has assigned yet is left out of the data, not guessed at.
export const Unassigned: Story = {
  play: async ({ canvas, args, userEvent }) => {
    const select = canvas.getByRole("combobox");
    await expect(canvas.getByText("3列目:")).toBeInTheDocument();
    await expect(select).toHaveValue("__ignore__");

    await userEvent.selectOptions(select, "y_error");
    await expect(args.onRoleChange).toHaveBeenCalledWith(2, "y_error");
  },
};

export const ErrorColumns: Story = {
  args: {
    extraColumnIndexes: [2, 3],
    extraRoles: { 2: "y_error", 3: "x_error" },
  },
  play: async ({ canvas }) => {
    const [third, fourth] = canvas.getAllByRole("combobox");
    await expect(third).toHaveValue("y_error");
    await expect(fourth).toHaveValue("x_error");
    await expect(canvas.queryByPlaceholderText("カラム名")).toBeNull();
  },
};

// Only a custom role asks for a name.
export const CustomName: Story = {
  args: { extraRoles: { 2: "__custom__" }, customNames: { 2: "temperature" } },
  play: async ({ canvas, args, userEvent }) => {
    const name = canvas.getByPlaceholderText("カラム名");
    await expect(name).toHaveValue("temperature");

    await userEvent.type(name, "!");
    await expect(args.onCustomNameChange).toHaveBeenCalledWith(
      2,
      "temperature!",
    );
  },
};
