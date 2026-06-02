import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinStudioBatchRunProgress } from "@/components/listingkit/shein-studio/shein-studio-batch-run-progress";
import {
  getSheinStudioBatchRun,
  listSheinStudioBatchRunItems,
} from "@/lib/api/shein-studio-batch-runs";

vi.mock("@/lib/api/shein-studio-batch-runs", () => ({
  cancelSheinStudioBatchRun: vi.fn(),
  getSheinStudioBatchRun: vi.fn(),
  listSheinStudioBatchRunItems: vi.fn(),
}));

const mockedGetSheinStudioBatchRun = vi.mocked(getSheinStudioBatchRun);
const mockedListSheinStudioBatchRunItems = vi.mocked(listSheinStudioBatchRunItems);

describe("SheinStudioBatchRunProgress", () => {
  beforeEach(() => {
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
});
