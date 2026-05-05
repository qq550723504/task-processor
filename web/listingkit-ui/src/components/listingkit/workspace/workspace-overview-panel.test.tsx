import { render, screen } from "@testing-library/react";

import { WorkspaceOverviewPanel } from "@/components/listingkit/workspace/workspace-overview-panel";

describe("WorkspaceOverviewPanel", () => {
  it("renders overview and review counts", () => {
    render(
      <WorkspaceOverviewPanel
        overview={{
          previewable_items: 7,
          retryable_count: 3,
          approved_sections: 4,
          review_pending_sections: 2,
          resolved_action_summary: {
            title: "Review detail previews",
            summary: "3 preview-ready detail sections need review.",
            cta_kind: "review",
          },
          recovery_summary: {
            title: "Use fallback review",
            summary: "A fallback result is available and should be reviewed first.",
            severity: "medium",
            urgency: "now",
          },
        }}
        reviewSummary={{
          approved_sections: 4,
          deferred_sections: 1,
          pending_sections: 2,
        }}
      />,
    );

    expect(screen.getByText("可预览")).toBeInTheDocument();
    expect(screen.getByText("7")).toBeInTheDocument();
    expect(screen.getByText("可重试")).toBeInTheDocument();
    expect(screen.getByText("3")).toBeInTheDocument();
    expect(screen.getByText("已延后")).toBeInTheDocument();
    expect(screen.getByText("1")).toBeInTheDocument();
    expect(screen.getByText("Review detail previews")).toBeInTheDocument();
    expect(screen.getByText("使用兜底结果继续检查")).toBeInTheDocument();
    expect(screen.getByText("中优先级 / 立即处理")).toBeInTheDocument();
  });
});
