import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SdsHomepageEntry } from "@/components/listingkit/sds/sds-homepage-entry";
import {
  getSheinStudioBatchRun,
  listSheinStudioBatchRunItems,
  startSheinStudioBatchRun,
} from "@/lib/api/shein-studio-batch-runs";
import { buildRecentBatchSummaries } from "@/lib/shein-studio/recent-batch-summaries";
import { listSheinStudioBatches } from "@/lib/utils/shein-studio-batches";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-workbench-hooks", () => ({
  clearLocalSheinStudioDraftSnapshot: vi.fn(),
  loadLocalSheinStudioDraftSnapshotDetail: vi.fn(() => null),
}));

vi.mock("@/lib/utils/shein-studio-batches", () => ({
  deleteSheinStudioBatch: vi.fn(),
  getSheinStudioBatch: vi.fn(),
  listSheinStudioBatches: vi.fn(),
  saveSheinStudioBatch: vi.fn(),
}));

vi.mock("@/lib/shein-studio/recent-batch-summaries", () => ({
  buildRecentBatchSummaries: vi.fn(),
}));

vi.mock("@/lib/api/shein-studio-batch-runs", () => ({
  cancelSheinStudioBatchRun: vi.fn(),
  getSheinStudioBatchRun: vi.fn(),
  listSheinStudioBatchRunItems: vi.fn(),
  startSheinStudioBatchRun: vi.fn(),
}));

const mockedListSheinStudioBatches = vi.mocked(listSheinStudioBatches);
const mockedBuildRecentBatchSummaries = vi.mocked(buildRecentBatchSummaries);
const mockedStartSheinStudioBatchRun = vi.mocked(startSheinStudioBatchRun);
const mockedGetSheinStudioBatchRun = vi.mocked(getSheinStudioBatchRun);
const mockedListSheinStudioBatchRunItems = vi.mocked(listSheinStudioBatchRunItems);

describe("SdsHomepageEntry", () => {
  beforeEach(() => {
    push.mockReset();
    mockedListSheinStudioBatches.mockResolvedValue([]);
    mockedBuildRecentBatchSummaries.mockReturnValue([
      {
        id: "batch-1",
        source: "batch",
        isRecoverableDraft: false,
        title: "Retro Cherries",
        primaryProductName: "tee",
        productCount: 1,
        promptPreview: "retro cherries",
        storeSummary: "869",
        designCount: 0,
        createdTaskCount: 0,
        updatedAt: "2026-05-31T12:00:00Z",
        alerts: [],
      },
    ]);
    mockedStartSheinStudioBatchRun.mockResolvedValue({
      run: {
        id: "run-1",
        mode: "generate",
        failurePolicy: "continue_on_error",
        status: "pending",
        currentIndex: 0,
        totalBatches: 1,
        completedBatches: 0,
        succeededBatches: 0,
        failedBatches: 0,
        cancelRequested: false,
        createdAt: "2026-05-31T12:00:00Z",
        updatedAt: "2026-05-31T12:00:00Z",
      },
      items: [],
    });
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
      cancelRequested: false,
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

  it("starts a backend batch run when the user launches bulk continue generate", async () => {
    render(<SdsHomepageEntry />);

    await screen.findByText("最近批次摘要");
    fireEvent.click(screen.getByRole("button", { name: "查看全部批次" }));
    fireEvent.click(await screen.findByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 1 个" }));

    await waitFor(() => {
      expect(mockedStartSheinStudioBatchRun).toHaveBeenCalledWith(["batch-1"]);
    });
    expect(await screen.findByText("运行中批量生成")).toBeInTheDocument();
    expect(screen.getByText("当前运行 ID：run-1")).toBeInTheDocument();
  });
});
