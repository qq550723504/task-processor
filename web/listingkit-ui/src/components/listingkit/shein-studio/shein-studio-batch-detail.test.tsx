import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinStudioBatchDetail } from "@/components/listingkit/shein-studio/shein-studio-batch-detail";
import { ApiError } from "@/lib/api/client";

const getSheinStudioBatch = vi.fn();
const saveSheinStudioBatch = vi.fn();
const deleteSheinStudioBatch = vi.fn();
const createSheinReviewTasks = vi.fn();

vi.mock("next/link", () => ({
  default: ({
    children,
    href,
  }: {
    children: React.ReactNode;
    href: string;
  }) => <a href={href}>{children}</a>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-batch-publish-gate", () => ({
  SheinBatchPublishGate: () => <div>publish gate</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-batch-task-tracker", () => ({
  SheinBatchTaskTracker: () => <div>task tracker</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-design-preview-grid", () => ({
  SheinDesignPreviewGrid: () => <div>design preview grid</div>,
}));

vi.mock("@/lib/shein-studio/create-review-tasks", () => ({
  createSheinReviewTasks: (...args: unknown[]) => createSheinReviewTasks(...args),
}));

vi.mock("@/lib/utils/shein-studio-batches", () => ({
  getSheinStudioBatch: (...args: unknown[]) => getSheinStudioBatch(...args),
  saveSheinStudioBatch: (...args: unknown[]) => saveSheinStudioBatch(...args),
  deleteSheinStudioBatch: (...args: unknown[]) => deleteSheinStudioBatch(...args),
}));

describe("SheinStudioBatchDetail", () => {
  it("shows not found only for a real 404 batch lookup", async () => {
    getSheinStudioBatch.mockRejectedValueOnce(
      new ApiError("ListingKit API request failed: 404", 404, {
        message: "not found",
      }),
    );

    render(<SheinStudioBatchDetail batchId="missing-batch" />);

    await waitFor(() =>
      expect(screen.getByText("未找到批次")).toBeInTheDocument(),
    );
    expect(screen.queryByText("批次加载失败")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "重试加载" })).not.toBeInTheDocument();
  });

  it("surfaces server errors instead of pretending the batch is missing", async () => {
    getSheinStudioBatch
      .mockRejectedValueOnce(
        new ApiError("ListingKit API request failed: 500", 500, {
          message: "upstream unavailable",
        }),
      )
      .mockResolvedValueOnce({
        id: "batch-1",
        name: "retro cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 2,
          prototypeGroupId: 3,
          layerId: "layer-1",
          productName: "Curtain",
          variantLabel: "Blue",
        },
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-18T18:30:00.000Z",
      });

    render(<SheinStudioBatchDetail batchId="batch-1" />);

    await waitFor(() =>
      expect(screen.getByText("批次加载失败")).toBeInTheDocument(),
    );
    expect(screen.getByText("ListingKit API request failed: 500")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "重试加载" }));

    await waitFor(() =>
      expect(
        screen.getByRole("heading", { level: 1, name: "retro cherries" }),
      ).toBeInTheDocument(),
    );
    expect(screen.queryByText("批次加载失败")).not.toBeInTheDocument();
    expect(getSheinStudioBatch.mock.calls.length).toBeGreaterThanOrEqual(2);
  });
});
