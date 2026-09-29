import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect } from "storybook/test";
import CenteredCard from "./CenteredCard";

const meta = {
  component: CenteredCard,
  args: {
    maxWidth: "max-w-md",
    children: <p>カードの中身</p>,
  },
} satisfies Meta<typeof CenteredCard>;
export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvas }) => {
    const main = canvas.getByRole("main");
    await expect(main).toContainElement(canvas.getByText("カードの中身"));
    await expect(main).toHaveClass("max-w-md");
  },
};

// The experiment pages hang from the top instead of centring vertically.
export const TopAligned: Story = {
  args: { maxWidth: "max-w-3xl", verticallyCentered: false },
  play: async ({ canvas }) => {
    await expect(canvas.getByRole("main").parentElement).not.toHaveClass(
      "items-center",
    );
  },
};
