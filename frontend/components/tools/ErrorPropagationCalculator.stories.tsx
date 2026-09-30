import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, within } from "storybook/test";
import ErrorPropagationCalculator from "./ErrorPropagationCalculator";

// Only the wiring; the propagation is lib/errorPropagation.ts's and the
// grammar lib/formula.ts's.
const meta = {
  component: ErrorPropagationCalculator,
} satisfies Meta<typeof ErrorPropagationCalculator>;
export default meta;

type Story = StoryObj<typeof meta>;

// The value and σ inputs in the row for `name`.
function row(canvasElement: HTMLElement, name: string) {
  const tr = within(canvasElement)
    .getAllByRole("row")
    .find((r) => r.querySelector("td")?.textContent === name);
  if (!tr) throw new Error(`no row for ${name}`);
  const [value, sigma] = within(tr).getAllByRole("spinbutton");
  return { value, sigma };
}

const formula = { name: "式" };

// The default formula's variables are listed, and filling them in gives
// the result. A blank σ means an exact constant (g here).
export const Propagates: Story = {
  play: async ({ canvas, canvasElement, userEvent }) => {
    await expect(canvas.getByRole("textbox", formula)).toHaveValue(
      "2*pi*sqrt(L/g)",
    );
    await expect(canvas.queryByText(/^z =/)).toBeNull();

    const L = row(canvasElement, "L");
    await userEvent.type(L.value, "1");
    await userEvent.type(L.sigma, "0.01");
    await userEvent.type(row(canvasElement, "g").value, "9.8");

    await expect(canvas.getByText("z = 2.01 ± 0.01")).toBeVisible();
  },
};

// An example replaces the formula, and a value typed for a name the new
// formula also has survives the switch.
export const ExampleKeepsSharedValues: Story = {
  play: async ({ canvas, canvasElement, userEvent }) => {
    await userEvent.type(row(canvasElement, "L").value, "1");

    await userEvent.click(
      canvas.getByRole("button", { name: "重力加速度（振り子から）" }),
    );
    await expect(canvas.getByRole("textbox", formula)).toHaveValue(
      "4*pi^2*L/T^2",
    );
    await expect(row(canvasElement, "L").value).toHaveValue(1);
    await expect(row(canvasElement, "T").value).toHaveValue(null);
  },
};

export const SyntaxError: Story = {
  play: async ({ canvas, userEvent }) => {
    const input = canvas.getByRole("textbox", formula);
    await userEvent.clear(input);
    await userEvent.type(input, "a*(b");
    await expect(canvas.getByText(/閉じ括弧/)).toBeVisible();
    await expect(canvas.queryByRole("table")).toBeNull();
  },
};

// Out of the formula's domain: said, rather than a NaN or an exact-looking
// "± 0.0".
export const OutOfDomain: Story = {
  play: async ({ canvas, canvasElement, userEvent }) => {
    const input = canvas.getByRole("textbox", formula);
    await userEvent.clear(input);
    await userEvent.type(input, "sqrt(x)");
    await userEvent.type(row(canvasElement, "x").value, "-1");

    await expect(canvas.getByText(/この入力では計算できません/)).toBeVisible();
    await expect(canvas.queryByText(/^z =/)).toBeNull();
  },
};
