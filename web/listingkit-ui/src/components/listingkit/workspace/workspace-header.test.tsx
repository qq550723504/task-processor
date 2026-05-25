import { fireEvent, render, screen } from "@testing-library/react";
import { vi } from "vitest";

import { WorkspaceHeader } from "@/components/listingkit/workspace/workspace-header";

describe("WorkspaceHeader", () => {
  it("routes recovery clicks to the supplied callback", () => {
    const onSelectRecovery = vi.fn();

    render(
      <WorkspaceHeader
        title="Queue task-123"
        recoverySummary={{
          title: "Use fallback review",
          severity: "medium",
          urgency: "now",
          cta_kind: "review",
          primary_descriptor: {
            recovery_hint: "review_fallback",
            recovery_target: {
              dispatch_kind: "session",
            },
          },
        }}
        onSelectRecovery={onSelectRecovery}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "检查恢复项" }));

    expect(onSelectRecovery).toHaveBeenCalledWith(
      expect.objectContaining({
        recovery_hint: "review_fallback",
      }),
    );
  });

  it("shows navigation links for tasks and pod studio", () => {
    render(
      <WorkspaceHeader
        title="测试任务"
        statusLabel="待确认"
        updatedAtLabel="2026-05-04 10:30"
        subtitle="SHEIN · 最终确认"
        showSheinStudioLink
      />,
    );

    expect(
      screen.getByRole("link", { name: "返回任务列表" }),
    ).toHaveAttribute("href", "/listing-kits");
    expect(
      screen.getByRole("link", { name: "返回 POD 工作室" }),
    ).toHaveAttribute("href", "/listing-kits/sds");
    expect(screen.getByText("任务状态")).toBeInTheDocument();
    expect(screen.getByText("待确认")).toBeInTheDocument();
    expect(screen.getByText("最近更新")).toBeInTheDocument();
    expect(screen.getByText("2026-05-04 10:30")).toBeInTheDocument();
    expect(screen.getByText("SHEIN · 最终确认")).toBeInTheDocument();
  });

  it("renders manual layer action buttons when enabled", () => {
    const onRunStandardLayer = vi.fn();
    const onRunPlatformLayer = vi.fn();

    render(
      <WorkspaceHeader
        title="测试任务"
        showLayerActions
        onRunStandardLayer={onRunStandardLayer}
        onRunPlatformLayer={onRunPlatformLayer}
      />,
    );

    fireEvent.click(screen.getByText("高级操作"));
    fireEvent.click(screen.getByRole("button", { name: "运行标准商品层" }));
    expect(onRunStandardLayer).toHaveBeenCalled();

    fireEvent.click(screen.getByRole("button", { name: "运行平台适配层" }));
    expect(onRunPlatformLayer).toHaveBeenCalled();
  });
});
