import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ImageAgentTaskRunLauncher } from "@/components/listingkit/image-agent/image-agent-task-run-launcher";

const mocks = vi.hoisted(() => ({
  launchImageAgentTaskRun: vi.fn(),
}));

vi.mock("@/lib/api/image-agent", () => ({
  launchImageAgentTaskRun: mocks.launchImageAgentTaskRun,
}));

describe("ImageAgentTaskRunLauncher", () => {
  const onLaunched = vi.fn();

  beforeEach(() => {
    mocks.launchImageAgentTaskRun.mockReset();
    onLaunched.mockReset();
  });

  it("launches a task-scoped run with the selected scene and hands back the run id", async () => {
    mocks.launchImageAgentTaskRun.mockResolvedValue({ run_id: "image-agent-run-1", status: "accepted" });
    const user = userEvent.setup();
    render(
      <ImageAgentTaskRunLauncher
        taskId="task-1"
        targetPlatform="shein"
        country="US"
        onLaunched={onLaunched}
      />,
    );

    await user.click(screen.getByRole("button", { name: "创建图片方案" }));
    await user.click(screen.getByLabelText("鞋履"));
    await user.click(screen.getByRole("button", { name: "开始生成" }));

    await vi.waitFor(() => expect(onLaunched).toHaveBeenCalledWith("image-agent-run-1"));
    expect(mocks.launchImageAgentTaskRun).toHaveBeenCalledWith({
      business_task_id: "task-1",
      target_platform: "shein",
      image_policy_context: { country: "us", family: "default", scene_category: "shoes" },
    });
  });

  it("shows the failure and keeps the form usable when the launch is rejected", async () => {
    mocks.launchImageAgentTaskRun.mockRejectedValue(new Error("image agent command is not valid"));
    const user = userEvent.setup();
    render(
      <ImageAgentTaskRunLauncher
        taskId="task-1"
        targetPlatform="shein"
        onLaunched={onLaunched}
      />,
    );

    await user.click(screen.getByRole("button", { name: "创建图片方案" }));
    await user.click(screen.getByLabelText("箱包"));
    await user.click(screen.getByRole("button", { name: "开始生成" }));

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("image agent command is not valid");
    expect(onLaunched).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "开始生成" })).toBeEnabled();
  });

  it("disables launching until a platform and scene are available", async () => {
    const user = userEvent.setup();
    render(<ImageAgentTaskRunLauncher taskId="task-1" onLaunched={onLaunched} />);

    await user.click(screen.getByRole("button", { name: "创建图片方案" }));

    expect(screen.getByRole("alert").textContent).toContain("当前任务未确定目标平台");
    const launchButton = screen.getByRole("button", { name: "开始生成" });
    expect(launchButton).toBeDisabled();
    expect(mocks.launchImageAgentTaskRun).not.toHaveBeenCalled();
  });
});
