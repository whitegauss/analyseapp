import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, fn } from "storybook/test";
import { parsePastedText } from "@/lib/pasteDataParsing";
import PasteDataFields from "./PasteDataFields";

// `parsed` comes from the real parser, as it does in the editors, so a
// story cannot show a state the parser would never produce.
const meta = {
  component: PasteDataFields,
  args: {
    pastedText: "",
    parsed: parsePastedText(""),
    onPastedTextChange: fn(),
    extraRoles: {},
    customNames: {},
    onRoleChange: fn(),
    onCustomNameChange: fn(),
    placeholder: "0\t1.2\n1\t2.9",
  },
} satisfies Meta<typeof PasteDataFields>;
export default meta;

type Story = StoryObj<typeof meta>;

const DATA_LABEL = /データ（スプレッドシートから/;

export const Empty: Story = {
  play: async ({ canvas, args, userEvent }) => {
    const textarea = canvas.getByLabelText(DATA_LABEL);
    await expect(textarea).toHaveValue("");
    await expect(canvas.queryByRole("combobox")).toBeNull();

    await userEvent.type(textarea, "1");
    await expect(args.onPastedTextChange).toHaveBeenCalledWith("1");
  },
};

// Two columns are x and y: nothing left to assign.
export const TwoColumns: Story = {
  args: {
    pastedText: "0\t1.1\n1\t2.9\n2\t5.2",
    parsed: parsePastedText("0\t1.1\n1\t2.9\n2\t5.2"),
  },
  play: async ({ canvas }) => {
    await expect(canvas.queryByText("3列目以降の役割")).toBeNull();
  },
};

// A third column asks what it is.
export const WithErrorColumn: Story = {
  args: {
    pastedText: "0\t1.1\t0.1\n1\t2.9\t0.1",
    parsed: parsePastedText("0\t1.1\t0.1\n1\t2.9\t0.1"),
    extraRoles: { 2: "y_error" },
  },
  play: async ({ canvas }) => {
    await expect(canvas.getByText("3列目:")).toBeInTheDocument();
    await expect(canvas.getByRole("combobox")).toHaveValue("y_error");
  },
};

// A parse error replaces the role selector: assigning roles to columns
// that did not parse would be meaningless.
export const WithParseError: Story = {
  args: {
    pastedText: "0\t1.1\t0.1\n1\tabc\t0.1",
    parsed: parsePastedText("0\t1.1\t0.1\n1\tabc\t0.1"),
  },
  play: async ({ canvas }) => {
    await expect(
      canvas.getByText('数値として読めない値があります: "abc"'),
    ).toBeInTheDocument();
    await expect(canvas.queryByRole("combobox")).toBeNull();
  },
};
