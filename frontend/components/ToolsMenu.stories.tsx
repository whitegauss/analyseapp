import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, spyOn } from "storybook/test";
import ToolsMenu from "./ToolsMenu";

const meta = { component: ToolsMenu } satisfies Meta<typeof ToolsMenu>;
export default meta;

type Story = StoryObj<typeof meta>;

const menuButton = { name: "メニュー" };
const toolsLink = { name: "計算ツール" };

export const OpenAndClose: Story = {
  play: async ({ canvas, userEvent }) => {
    const button = canvas.getByRole("button", menuButton);
    await expect(button).toHaveAttribute("aria-expanded", "false");
    await expect(canvas.queryByRole("link", toolsLink)).toBeNull();

    await userEvent.click(button);
    await expect(button).toHaveAttribute("aria-expanded", "true");
    await expect(canvas.getByRole("link", toolsLink)).toHaveAttribute(
      "href",
      "/tools",
    );

    // The button toggles.
    await userEvent.click(button);
    await expect(button).toHaveAttribute("aria-expanded", "false");
    await expect(canvas.queryByRole("link", toolsLink)).toBeNull();
  },
};

export const ClickOutsideCloses: Story = {
  play: async ({ canvas, canvasElement, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", menuButton));
    const menu = canvas.getByRole("link", toolsLink).parentElement!;

    // Inside the menu, but not on the link: stays open.
    await userEvent.click(menu);
    await expect(canvas.getByRole("link", toolsLink)).toBeVisible();

    // Anywhere else on the page: closes.
    await userEvent.click(canvasElement.ownerDocument.body);
    await expect(canvas.queryByRole("link", toolsLink)).toBeNull();
  },
};

export const ChoosingTheLinkCloses: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", menuButton));
    await userEvent.click(canvas.getByRole("link", toolsLink));
    await expect(canvas.queryByRole("link", toolsLink)).toBeNull();
  },
};

// The page-wide mousedown listener exists only while the menu is open:
// closed, it would be a listener on every click of the page for nothing,
// and one left behind after closing would be a leak.
//
// Counted rather than compared by identity: the recorded arguments are not
// the functions themselves (Storybook's instrumentation wraps them), so
// `toBe` fails even for the listener that really was removed.
export const ListensOnlyWhileOpen: Story = {
  play: async ({ canvas, canvasElement, userEvent }) => {
    const doc = canvasElement.ownerDocument;
    const add = spyOn(doc, "addEventListener");
    const remove = spyOn(doc, "removeEventListener");
    const count = (spy: typeof add) =>
      spy.mock.calls.filter(([type]) => type === "mousedown").length;
    const button = canvas.getByRole("button", menuButton);

    try {
      await expect(count(add)).toBe(0);

      // Twice round, so a listener that is added but never removed shows
      // up as a growing difference.
      for (const round of [1, 2]) {
        await userEvent.click(button);
        await expect(count(add)).toBe(round);
        await expect(count(remove)).toBe(round - 1);

        await userEvent.click(button);
        await expect(count(remove)).toBe(round);
      }
    } finally {
      add.mockRestore();
      remove.mockRestore();
    }
  },
};
