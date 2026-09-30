import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect } from "storybook/test";
import UnitConverter from "./UnitConverter";

// Only the wiring; the factors are lib/unitConversion.ts's.
const meta = { component: UnitConverter } satisfies Meta<typeof UnitConverter>;
export default meta;

type Story = StoryObj<typeof meta>;

export const Converts: Story = {
  play: async ({ canvas, userEvent }) => {
    await expect(canvas.getByLabelText("変換元")).toHaveValue("m");
    await expect(canvas.getByLabelText("変換先")).toHaveValue("cm");

    await userEvent.type(canvas.getByLabelText("値"), "1.5");
    await expect(canvas.getByText("1.5 m = 150 cm")).toBeVisible();

    await userEvent.selectOptions(canvas.getByLabelText("変換先"), "km");
    await expect(canvas.getByText("1.5 m = 0.0015 km")).toBeVisible();
  },
};

// A new kind of quantity brings its own units: the old ones would convert
// metres to kelvin.
export const SwitchingKindResetsUnits: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.type(canvas.getByLabelText("値"), "300");
    await userEvent.selectOptions(canvas.getByLabelText("種類"), "temperature");

    await expect(canvas.getByLabelText("変換元")).toHaveValue("K");
    await expect(canvas.getByLabelText("変換先")).toHaveValue("C");
    await expect(canvas.getByText("300 K = 26.85 °C")).toBeVisible();
  },
};
