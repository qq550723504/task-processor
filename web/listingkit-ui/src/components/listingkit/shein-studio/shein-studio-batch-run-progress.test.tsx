import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinStudioBatchRunProgress } from "@/components/listingkit/shein-studio/shein-studio-batch-run-progress";
import {
  recoverSheinStudioBatchRun,
  getSheinStudioBatchRun,
  listSheinStudioBatchRunItems,
} from "@/lib/api/shein-studio-batch-runs";

vi.mock("@/lib/api/shein-studio-batch-runs", () => ({
  cancelSheinStudioBatchRun: vi.fn(),
  getSheinStudioBatchRun: vi.fn(),
  listSheinStudioBatchRunItems: vi.fn(),
  recoverSheinStudioBatchRun: vi.fn(),
}));

const mockedRecoverSheinStudioBatchRun = vi.mocked(recoverSheinStudioBatchRun);
const mockedGetSheinStudioBatchRun = vi.mocked(getSheinStudioBatchRun);
const mockedListSheinStudioBatchRunItems = vi.mocked(listSheinStudioBatchRunItems);

describe("SheinStudioBatchRunProgress", () => {
  beforeEach(() => {
    mockedRecoverSheinStudioBatchRun.mockReset();
    mockedGetSheinStudioBatchRun.mockResolvedValue({
      id: "run-1",
      mode: "generate",
      failurePolicy: "continue_on_error",
      status: "running",
      currentBatchId: "batch-1",
      currentIndex: 1,
      totalBatches: 1,
      completedBatches: 0,
      succeededBatches: 0,
      failedBatches: 0,
      cancelRequested: true,
      createdAt: "2026-05-31T12:00:00Z",
      updatedAt: "2026-05-31T12:00:01Z",
    });
    mockedListSheinStudioBatchRunItems.mockResolvedValue([
      {
        id: "run-1:1",
        runId: "run-1",
        batchId: "batch-1",
        position: 1,
        status: "running",
        createdAt: "2026-05-31T12:00:00Z",
        updatedAt: "2026-05-31T12:00:01Z",
      },
    ]);
  });

  it("shows cancelling state when cancel was requested but the run is still draining", async () => {
    const { container } = render(
      <SheinStudioBatchRunProgress onBack={vi.fn()} runId="run-1" />,
    );

    await waitFor(() => {
      expect(mockedGetSheinStudioBatchRun).toHaveBeenCalledWith("run-1");
    });

    expect(await screen.findByText("正在取消批量生成")).toBeInTheDocument();
    expect(screen.getAllByText("取消中")).toHaveLength(2);
    expect(screen.queryByRole("button", { name: "取消本轮生成" })).not.toBeInTheDocument();

    const title = screen.getByRole("heading", { name: "正在取消批量生成" });
    const headerRow = title.closest("div")?.parentElement;
    expect(headerRow).not.toBeNull();
    expect(headerRow?.className).toContain("flex-col");

    const metricGrid = container.querySelector(
      ".grid.gap-3",
    ) as HTMLDivElement | null;
    expect(metricGrid).not.toBeNull();
    expect(metricGrid?.className).not.toContain("md:grid-cols-4");
  });

  it("allows recovering a failed run and shows async diagnostics", async () => {
    mockedGetSheinStudioBatchRun.mockResolvedValueOnce({
      id: "run-2",
      mode: "generate",
      failurePolicy: "continue_on_error",
      status: "failed",
      currentBatchId: "batch-2",
      currentIndex: 2,
      totalBatches: 2,
      completedBatches: 2,
      succeededBatches: 1,
      failedBatches: 1,
      lastError: "runner stopped on upstream timeout",
      cancelRequested: false,
      createdAt: "2026-05-31T12:00:00Z",
      updatedAt: "2026-05-31T12:00:01Z",
    });
    mockedGetSheinStudioBatchRun.mockResolvedValueOnce({
      id: "run-2",
      mode: "generate",
      failurePolicy: "continue_on_error",
      status: "pending",
      currentIndex: 0,
      totalBatches: 2,
      completedBatches: 0,
      succeededBatches: 0,
      failedBatches: 0,
      cancelRequested: false,
      createdAt: "2026-05-31T12:00:00Z",
      updatedAt: "2026-05-31T12:01:00Z",
    });
    mockedListSheinStudioBatchRunItems.mockResolvedValueOnce([
      {
        id: "run-2:1",
        runId: "run-2",
        batchId: "batch-1",
        position: 1,
        status: "succeeded",
        batchStatus: "succeeded",
        asyncJobId: "job-1",
        createdAt: "2026-05-31T12:00:00Z",
        updatedAt: "2026-05-31T12:00:30Z",
      },
      {
        id: "run-2:2",
        runId: "run-2",
        batchId: "batch-2",
        position: 2,
        status: "failed",
        batchStatus: "processing",
        asyncJobId: "job-2",
        batchLastError: "provider timeout",
        createdAt: "2026-05-31T12:00:00Z",
        updatedAt: "2026-05-31T12:00:31Z",
      },
    ]);
    mockedListSheinStudioBatchRunItems.mockResolvedValueOnce([
      {
        id: "run-2:1",
        runId: "run-2",
        batchId: "batch-1",
        position: 1,
        status: "pending",
        asyncJobId: "job-1",
        createdAt: "2026-05-31T12:00:00Z",
        updatedAt: "2026-05-31T12:01:00Z",
      },
    ]);

    const user = userEvent.setup();
    render(<SheinStudioBatchRunProgress onBack={vi.fn()} runId="run-2" />);

    expect(await screen.findByRole("button", { name: "恢复本轮生成" })).toBeInTheDocument();
    expect(screen.getByText("子任务状态：处理中 · Async Job：job-2")).toBeInTheDocument();
    expect(screen.getByText("provider timeout")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "恢复本轮生成" }));

    await waitFor(() => {
      expect(mockedRecoverSheinStudioBatchRun).toHaveBeenCalledWith("run-2");
    });
    await waitFor(() => {
      expect(mockedGetSheinStudioBatchRun.mock.calls.length).toBeGreaterThanOrEqual(2);
    });
  });

  it("uses task-creation wording for create-task runs", async () => {
    mockedGetSheinStudioBatchRun.mockResolvedValueOnce({
      id: "run-3",
      mode: "create_tasks",
      failurePolicy: "continue_on_error",
      status: "failed",
      currentIndex: 0,
      totalBatches: 1,
      completedBatches: 1,
      succeededBatches: 0,
      failedBatches: 1,
      cancelRequested: false,
      createdAt: "2026-05-31T12:00:00Z",
      updatedAt: "2026-05-31T12:00:01Z",
    });
    mockedListSheinStudioBatchRunItems.mockResolvedValueOnce([
      {
        id: "run-3:1",
        runId: "run-3",
        batchId: "batch-3",
        position: 1,
        status: "failed",
        createdAt: "2026-05-31T12:00:00Z",
        updatedAt: "2026-05-31T12:00:01Z",
      },
    ]);

    render(<SheinStudioBatchRunProgress onBack={vi.fn()} runId="run-3" />);

    expect(await screen.findByText("批量创建任务已结束")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "恢复本轮任务创建" })).toBeInTheDocument();
  });
});
