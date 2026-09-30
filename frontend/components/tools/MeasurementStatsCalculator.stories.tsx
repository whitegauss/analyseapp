import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, within } from "storybook/test";
import MeasurementStatsCalculator from "./MeasurementStatsCalculator";

// Only the wiring; the statistics are lib/statistics.ts's.
const meta = {
  component: MeasurementStatsCalculator,
} satisfies Meta<typeof MeasurementStatsCalculator>;
export default meta;

type Story = StoryObj<typeof meta>;

export const OneColumn: Story = {
  play: async ({ canvas, userEvent }) => {
    await expect(canvas.queryByRole("table")).toBeNull();

    await userEvent.type(
      canvas.getByRole("textbox"),
      "1.23{enter}1.25{enter}1.19{enter}1.31",
    );
    const cells = within(canvas.getAllByRole("row")[1]).getAllByRole("cell");
    await expect(cells.map((c) => c.textContent)).toEqual([
      "列1",
      "4",
      expect.stringContaining("±"),
      "0.05",
    ]);
  },
};

// Every column gets its own row, computed from its own values -- a row
// for column 2 with nothing in it would still render.
export const SeveralColumns: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(canvas.getByRole("textbox"), "1\t10{enter}2\t20");
    const [, first, second] = canvas.getAllByRole("row");
    const cellsOf = (row: HTMLElement) =>
      within(row)
        .getAllByRole("cell")
        .map((c) => c.textContent);
    await expect(cellsOf(first).slice(0, 2)).toEqual(["列1", "2"]);
    await expect(cellsOf(second).slice(0, 2)).toEqual(["列2", "2"]);
    // The spread differs, so the two rows really are different columns.
    await expect(cellsOf(first)[3]).not.toBe(cellsOf(second)[3]);
  },
};

// What does not parse is said, not silently dropped from the statistics.
export const ParseError: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(canvas.getByRole("textbox"), "1.2{enter}abc");
    await expect(
      canvas.getByText('数値として読めない値があります: "abc"'),
    ).toBeVisible();
    await expect(canvas.queryByRole("table")).toBeNull();
  },
};
