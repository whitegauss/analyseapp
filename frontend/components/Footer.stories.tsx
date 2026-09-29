import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect } from "storybook/test";
import Footer from "./Footer";

const meta = { component: Footer } satisfies Meta<typeof Footer>;
export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  play: async ({ canvas }) => {
    const link = canvas.getByRole("link", { name: "GitHub" });
    await expect(link).toHaveAttribute(
      "href",
      "https://github.com/whitegauss/analyseapp",
    );
    // A new tab must not be able to reach back into this one.
    await expect(link).toHaveAttribute("target", "_blank");
    await expect(link).toHaveAttribute("rel", "noopener noreferrer");
  },
};
