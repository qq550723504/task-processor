import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { TaskRevisionHistoryPanel } from "@/components/listingkit/tasks/task-revision-history-panel";

const revisionHistoryMock = vi.fn();
const revisionHistoryDetailMock = vi.fn();

vi.mock("@/lib/query/use-revision-history", () => ({
  useTaskRevisionHistory: (...args: unknown[]) => revisionHistoryMock(...args),
  useTaskRevisionHistoryDetail: (...args: unknown[]) => revisionHistoryDetailMock(...args),
}));

describe("TaskRevisionHistoryPanel", () => {
  beforeEach(() => {
    revisionHistoryMock.mockReset();
    revisionHistoryDetailMock.mockReset();
    revisionHistoryMock.mockReturnValue({
      data: {
        items: [
          {
            revision_id: "rev-refresh",
            updated_at: "2026-05-28T06:00:00Z",
            action_type: "edit",
            reason: "Refresh SHEIN category",
            timeline: {
              headline: "刷新 SHEIN 类目模板",
              relation_text: "将重算类目 / 普通属性 / 销售属性",
            },
          },
          {
            revision_id: "rev-edit",
            updated_at: "2026-05-28T05:00:00Z",
            action_type: "edit",
            reason: "manual adjustment",
            timeline: {
              headline: "更新 SHEIN 资料",
            },
          },
        ],
      },
      isLoading: false,
    });
    revisionHistoryDetailMock.mockImplementation((_, revisionId: string) => ({
      data:
        revisionId === "rev-refresh"
          ? {
              record: {
                revision_id: "rev-refresh",
                updated_at: "2026-05-28T06:00:00Z",
                action_type: "edit",
                reason: "Refresh SHEIN category",
                timeline: {
                  headline: "刷新 SHEIN 类目模板",
                  relation_text: "将重算类目 / 普通属性 / 销售属性",
                },
              },
            }
          : {
              record: {
                revision_id: "rev-edit",
                updated_at: "2026-05-28T05:00:00Z",
                action_type: "edit",
                reason: "manual adjustment",
                timeline: {
                  headline: "更新 SHEIN 资料",
                },
              },
            },
      isLoading: false,
    }));
  });

  it("marks refresh revisions and shows their impact scope", () => {
    render(<TaskRevisionHistoryPanel taskId="task-1" />);

    expect(screen.getAllByText("刷新 SHEIN 类目模板").length).toBeGreaterThan(0);
    expect(screen.getAllByText("刷新").length).toBeGreaterThan(0);
    expect(screen.getByText("影响范围：将重算类目 / 普通属性 / 销售属性")).toBeInTheDocument();
  });

  it("filters revision history down to refresh-only records", () => {
    render(<TaskRevisionHistoryPanel taskId="task-1" />);

    fireEvent.click(screen.getByRole("button", { name: "刷新型（1）" }));

    expect(screen.getAllByText("刷新 SHEIN 类目模板").length).toBeGreaterThan(0);
    expect(screen.queryByText("更新 SHEIN 资料")).not.toBeInTheDocument();
    expect(screen.getByText("刷新型修订")).toBeInTheDocument();
  });

  it("keeps the history action and detail layout mobile-friendly", () => {
    const { container } = render(<TaskRevisionHistoryPanel taskId="task-1" />);

    expect(screen.getByRole("button", { name: "收起历史" })).toHaveClass("w-full");
    expect(
      Array.from(container.querySelectorAll("div")).some((element) =>
        element.className.includes("xl:grid-cols-[280px_minmax(0,1fr)]"),
      ),
    ).toBe(true);
  });
});
