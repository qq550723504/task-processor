import { render, screen } from "@testing-library/react";
import { vi } from "vitest";

import { QueueScreenBody } from "@/components/listingkit/queue/queue-screen-sections";

vi.mock("@/components/listingkit/queue/queue-filters-bar", () => ({
  QueueFiltersBar: () => <div>队列筛选</div>,
}));

vi.mock("@/components/listingkit/queue/queue-summary-strip", () => ({
  QueueSummaryStrip: () => <div>队列摘要</div>,
}));

vi.mock("@/components/listingkit/queue/queue-table", () => ({
  QueueTable: () => <div>队列表格</div>,
}));

vi.mock("@/components/listingkit/review/review-reasons-card", () => ({
  ReviewReasonsCard: () => <div>审核原因</div>,
}));

vi.mock("@/components/listingkit/tasks/task-progress-notice", () => ({
  TaskProgressNotice: () => <div>任务进度</div>,
}));

vi.mock("@/components/listingkit/tasks/task-status-panel", () => ({
  TaskStatusPanel: () => <div>任务状态</div>,
}));

describe("QueueScreenBody", () => {
  it("keeps POD navigation out of the task queue workflow", () => {
    render(
      <QueueScreenBody
        filters={{} as never}
        onAction={vi.fn()}
        onApplyFilters={vi.fn()}
        onChangePage={vi.fn()}
        onExecuteAction={vi.fn()}
        onSelectNavigation={vi.fn()}
        onSelectRecovery={vi.fn()}
        queueData={{
          items: [],
          page: 1,
          page_size: 20,
          total: 0,
        } as never}
        taskId="task-queue-1"
        taskResult={{ status: "processing" } as never}
      />,
    );

    expect(
      screen.queryByRole("link", { name: "返回 POD 工作室" }),
    ).not.toBeInTheDocument();
  });
});
