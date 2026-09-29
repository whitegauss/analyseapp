import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, within } from "storybook/test";
import ExperimentListWithCompare from "./ExperimentListWithCompare";

const experiments = [
  {
    id: "a1",
    project_id: "p1",
    title: "落下運動",
    created_at: "2026-09-01T00:00:00Z",
    projectTitle: "力学",
  },
  {
    id: "b2",
    project_id: "p1",
    title: "振り子",
    created_at: "2026-09-02T00:00:00Z",
    projectTitle: "力学",
  },
  {
    id: "c3",
    project_id: "p2",
    title: null,
    created_at: "2026-09-03T00:00:00Z",
    projectTitle: null,
  },
];

const meta = {
  component: ExperimentListWithCompare,
  args: { experiments },
} satisfies Meta<typeof ExperimentListWithCompare>;
export default meta;

type Story = StoryObj<typeof meta>;

const checkbox = (title: string) => ({
  name: `「${title}」を比較対象に選択`,
});

export const Rows: Story = {
  play: async ({ canvas }) => {
    const items = canvas.getAllByRole("listitem");
    await expect(items).toHaveLength(3);

    const first = within(items[0]);
    await expect(first.getByRole("link", { name: /落下運動/ })).toHaveAttribute(
      "href",
      "/experiments/a1",
    );
    await expect(first.getByText("力学")).toBeVisible();
    await expect(first.getByText("2026-09-01")).toBeVisible();

    // An untitled experiment reads as one; a missing project, as nothing.
    const third = within(items[2]);
    await expect(third.getByText("(無題)")).toBeVisible();
    await expect(third.queryByText("力学")).toBeNull();
  },
};

// Comparing needs two: no link until then, however it got there.
export const CompareNeedsTwo: Story = {
  play: async ({ canvas, userEvent }) => {
    await expect(canvas.queryByRole("link", { name: /件を比較/ })).toBeNull();
    await expect(canvas.getByText("比較（2件以上選択）")).toBeVisible();

    await userEvent.click(canvas.getByRole("checkbox", checkbox("落下運動")));
    await expect(canvas.queryByRole("link", { name: /件を比較/ })).toBeNull();

    await userEvent.click(canvas.getByRole("checkbox", checkbox("振り子")));
    await expect(
      canvas.getByRole("link", { name: "選択した2件を比較" }),
    ).toHaveAttribute("href", "/experiments/compare?ids=a1,b2");

    // Back to one: the link goes again.
    await userEvent.click(canvas.getByRole("checkbox", checkbox("落下運動")));
    await expect(canvas.queryByRole("link", { name: /件を比較/ })).toBeNull();
  },
};

// The ids follow the order they were ticked in, which is the order the
// comparison page lists them.
export const CompareInSelectionOrder: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("checkbox", checkbox("(無題)")));
    await userEvent.click(canvas.getByRole("checkbox", checkbox("落下運動")));
    await userEvent.click(canvas.getByRole("checkbox", checkbox("振り子")));

    await expect(
      canvas.getByRole("link", { name: "選択した3件を比較" }),
    ).toHaveAttribute("href", "/experiments/compare?ids=c3,a1,b2");
  },
};
