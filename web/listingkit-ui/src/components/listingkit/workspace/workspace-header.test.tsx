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
});
