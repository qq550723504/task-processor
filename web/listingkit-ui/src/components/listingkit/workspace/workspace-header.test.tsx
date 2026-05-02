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

    fireEvent.click(screen.getByRole("button", { name: "Review fallback" }));

    expect(onSelectRecovery).toHaveBeenCalledWith(
      expect.objectContaining({
        recovery_hint: "review_fallback",
      }),
    );
  });

  it("shows navigation links for tasks and shein studio", () => {
    render(
      <WorkspaceHeader
        title="Queue task-123"
        showSheinStudioLink
      />,
    );

    expect(
      screen.getByRole("link", { name: "返回任务列表" }),
    ).toHaveAttribute("href", "/listing-kits/tasks");
    expect(
      screen.getByRole("link", { name: "返回 SHEIN 工作室" }),
    ).toHaveAttribute("href", "/listing-kits/shein");
  });
});
