import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ImageAgentTaskRunLauncher } from "@/components/listingkit/image-agent/image-agent-task-run-launcher";

const mocks = vi.hoisted(() => ({
  getImageAgentTaskAssets: vi.fn(),
  launchImageAgentTaskRun: vi.fn(),
}));

vi.mock("@/lib/api/image-agent", () => ({
  getImageAgentTaskAssets: mocks.getImageAgentTaskAssets,
  launchImageAgentTaskRun: mocks.launchImageAgentTaskRun,
}));

const taskAssets = {
  business_task_id: "task-1",
  target_platform: "shein",
  sources: [
    { id: "source-1", type: "source" as const, label: "Source 1" },
    { id: "source-2", type: "source" as const, label: "Source 2" },
  ],
  styles: [{ id: "style-1", type: "style" as const, label: "Style 1" }],
};

describe("ImageAgentTaskRunLauncher", () => {
  const onLaunched = vi.fn();

  beforeEach(() => {
    mocks.getImageAgentTaskAssets.mockReset();
    mocks.launchImageAgentTaskRun.mockReset();
    onLaunched.mockReset();
    mocks.getImageAgentTaskAssets.mockResolvedValue(taskAssets);
  });

  it("requires an explicit source asset and launches with it", async () => {
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

    // The launch button stays disabled until a source asset is selected.
    expect(screen.getByRole("button", { name: "开始生成" })).toBeDisabled();

    await user.click(screen.getByLabelText("Source 2"));
    await user.click(screen.getByRole("button", { name: "开始生成" }));

    await vi.waitFor(() => expect(onLaunched).toHaveBeenCalledWith("image-agent-run-1"));
    expect(mocks.getImageAgentTaskAssets).toHaveBeenCalledWith("task-1", "shein", expect.anything());
    expect(mocks.launchImageAgentTaskRun).toHaveBeenCalledWith({
      business_task_id: "task-1",
      target_platform: "shein",
      image_policy_context: { country: "us", family: "default", scene_category: "shoes" },
      source_asset_id: "source-2",
      style_asset_ids: undefined,
    });
  });

  it("passes the optional style selection through to the launch", async () => {
    mocks.launchImageAgentTaskRun.mockResolvedValue({ run_id: "image-agent-run-2", status: "accepted" });
    const user = userEvent.setup();
    render(
      <ImageAgentTaskRunLauncher
        taskId="task-1"
        targetPlatform="shein"
        onLaunched={onLaunched}
      />,
    );

    await user.click(screen.getByRole("button", { name: "创建图片方案" }));
    await user.click(screen.getByLabelText("Source 1"));
    await user.click(screen.getByLabelText("Style 1"));
    await user.click(screen.getByLabelText("箱包"));
    await user.click(screen.getByRole("button", { name: "开始生成" }));

    await vi.waitFor(() => expect(onLaunched).toHaveBeenCalledWith("image-agent-run-2"));
    expect(mocks.launchImageAgentTaskRun).toHaveBeenCalledWith(
      expect.objectContaining({ source_asset_id: "source-1", style_asset_ids: ["style-1"] }),
    );
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
    await user.click(await screen.findByLabelText("Source 1"));
    await user.click(screen.getByLabelText("箱包"));
    await user.click(screen.getByRole("button", { name: "开始生成" }));

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("image agent command is not valid");
    expect(onLaunched).not.toHaveBeenCalled();
    expect(screen.getByRole("button", { name: "开始生成" })).toBeEnabled();
  });

  it("surfaces preflight failures without offering a launch", async () => {
    mocks.getImageAgentTaskAssets.mockRejectedValue(new Error("image agent command is not valid"));
    const user = userEvent.setup();
    render(
      <ImageAgentTaskRunLauncher
        taskId="task-1"
        targetPlatform="shein"
        onLaunched={onLaunched}
      />,
    );

    await user.click(screen.getByRole("button", { name: "创建图片方案" }));

    const alert = await screen.findByRole("alert");
    expect(alert.textContent).toContain("image agent command is not valid");
    expect(screen.getByRole("button", { name: "开始生成" })).toBeDisabled();
    expect(mocks.launchImageAgentTaskRun).not.toHaveBeenCalled();
  });

  it("disables launching until a platform and scene are available", async () => {
    const user = userEvent.setup();
    render(<ImageAgentTaskRunLauncher taskId="task-1" onLaunched={onLaunched} />);

    await user.click(screen.getByRole("button", { name: "创建图片方案" }));

    expect(screen.getByRole("alert").textContent).toContain("当前任务未确定目标平台");
    expect(screen.getByRole("button", { name: "开始生成" })).toBeDisabled();
    expect(mocks.getImageAgentTaskAssets).not.toHaveBeenCalled();
    expect(mocks.launchImageAgentTaskRun).not.toHaveBeenCalled();
  });
});
