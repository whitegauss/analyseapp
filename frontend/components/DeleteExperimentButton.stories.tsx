import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { expect, mocked } from "storybook/test";
import { deleteExperiment } from "@/app/experiments/actions";
import DeleteExperimentButton from "./DeleteExperimentButton";

// The two-step confirmation is the only thing standing between a stray
// click and a deleted experiment, so every story checks what the first
// click does not do.
const meta = {
  component: DeleteExperimentButton,
  args: { id: "0b6f3c1e-8a2d-4f5b-9c7e-1d2e3f4a5b6c", title: "落下運動" },
  beforeEach: () => {
    mocked(deleteExperiment).mockReset();
  },
} satisfies Meta<typeof DeleteExperimentButton>;
export default meta;

type Story = StoryObj<typeof meta>;

export const Idle: Story = {
  play: async ({ canvas, userEvent }) => {
    await expect(canvas.getByRole("button", { name: "削除" })).toBeVisible();
    await expect(canvas.queryByText(/削除しますか/)).toBeNull();

    // First click: asks, deletes nothing.
    await userEvent.click(canvas.getByRole("button", { name: "削除" }));
    await expect(
      canvas.getByText("「落下運動」を削除しますか？"),
    ).toBeVisible();
    await expect(deleteExperiment).not.toHaveBeenCalled();
  },
};

export const CancelGoesBack: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: "削除" }));
    await userEvent.click(canvas.getByRole("button", { name: "キャンセル" }));

    await expect(canvas.getByRole("button", { name: "削除" })).toBeVisible();
    await expect(canvas.queryByText(/削除しますか/)).toBeNull();
    await expect(deleteExperiment).not.toHaveBeenCalled();
  },
};

// The second click submits the experiment's id -- and no project, so the
// action lands on the cross-project list.
export const ConfirmDeletes: Story = {
  play: async ({ canvas, args, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: "削除" }));
    await userEvent.click(canvas.getByRole("button", { name: "削除する" }));

    await expect(deleteExperiment).toHaveBeenCalledOnce();
    const form = mocked(deleteExperiment).mock.calls[0][0];
    await expect(form.get("id")).toBe(args.id);
    await expect(form.has("projectId")).toBe(false);
  },
};

// From a project's page the project goes along too, so the action can land
// back there (KAN-88).
export const ConfirmDeletesFromAProject: Story = {
  args: { projectId: "9a8b7c6d-5e4f-4a3b-8c2d-1e0f9a8b7c6d" },
  play: async ({ canvas, args, userEvent }) => {
    await userEvent.click(canvas.getByRole("button", { name: "削除" }));
    await userEvent.click(canvas.getByRole("button", { name: "削除する" }));

    const form = mocked(deleteExperiment).mock.calls[0][0];
    await expect(form.get("projectId")).toBe(args.projectId);
  },
};
