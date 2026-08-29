import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ImageAgentLaunchPanel } from "@/components/listingkit/image-agent/image-agent-launch-panel";
import {
  createImageAgentWorkspaceRun,
  getImageAgentWorkspaceAssets,
} from "@/lib/api/image-agent";

vi.mock("@/lib/api/image-agent", () => ({
  createImageAgentWorkspaceRun: vi.fn(),
  getImageAgentWorkspaceAssets: vi.fn(),
}));

const getAssets = vi.mocked(getImageAgentWorkspaceAssets);
const createRun = vi.mocked(createImageAgentWorkspaceRun);

describe("ImageAgentLaunchPanel", () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it("requires one source and submits only target/source/selected styles", async () => {
    getAssets.mockResolvedValue({
      target_platform: "shein",
      source_assets: [{ id: "source-1", label: "主图", display_url: "https://cdn.example.test/source.png" }],
      style_candidates: [{ id: "style-1", label: "生活方式", display_url: "https://cdn.example.test/style.png" }],
    });
    createRun.mockResolvedValue({ run_id: "run-1", status: "accepted" });
    const created = vi.fn();
    const user = userEvent.setup();
    render(<ImageAgentLaunchPanel onCreated={created} targetPlatform="shein" taskId="task-1" />);

    await user.click(screen.getByRole("button", { name: "创建图片方案" }));
    await screen.findByLabelText("主图");
    expect(getAssets).toHaveBeenCalledWith("task-1", "shein", expect.any(AbortSignal));
    expect(screen.getByRole("button", { name: "开始生成" })).toBeDisabled();

    await user.click(screen.getByLabelText("主图"));
    await user.click(screen.getByLabelText("生活方式"));
    await user.click(screen.getByRole("button", { name: "开始生成" }));

    await waitFor(() => expect(createRun).toHaveBeenCalledWith("task-1", {
      target_platform: "shein", source_asset_id: "source-1", style_asset_ids: ["style-1"],
    }));
    expect(created).toHaveBeenCalledWith("run-1");
  });
});
