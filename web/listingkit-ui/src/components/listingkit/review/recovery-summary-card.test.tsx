import { fireEvent, render, screen } from "@testing-library/react";
import { vi } from "vitest";

import { RecoverySummaryCard } from "@/components/listingkit/review/recovery-summary-card";

describe("RecoverySummaryCard", () => {
  it("renders readable urgency metadata", () => {
    render(
      <RecoverySummaryCard
        summary={{
          title: "Use fallback review",
          summary: "A fallback result is available and should be reviewed first.",
          severity: "medium",
          urgency: "now",
          cta_kind: "review",
        }}
      />,
    );

    expect(screen.getByText("使用兜底结果继续检查")).toBeInTheDocument();
    expect(screen.getByText("中优先级 / 立即处理")).toBeInTheDocument();
    expect(screen.getByText("检查恢复项")).toBeInTheDocument();
  });

  it("triggers the primary recovery action", () => {
    const onSelect = vi.fn();

    render(
      <RecoverySummaryCard
        summary={{
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
        onSelect={onSelect}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "检查恢复项" }));

    expect(onSelect).toHaveBeenCalledWith(
      expect.objectContaining({
        recovery_hint: "review_fallback",
      }),
    );
  });
});
