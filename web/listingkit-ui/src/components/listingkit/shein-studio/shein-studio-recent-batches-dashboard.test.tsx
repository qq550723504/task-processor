import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinStudioRecentBatchesDashboard } from "@/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard";

describe("SheinStudioRecentBatchesDashboard", () => {
  it("renders recent batch cards and forwards selection", () => {
    const onSelectSummary = vi.fn();

    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={onSelectSummary}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Retro Cherries",
            primaryProductName: "tee",
            productCount: 2,
            promptPreview: "retro cherries",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
        ]}
      />,
    );

    expect(screen.getByText("最近批次")).toBeInTheDocument();
    expect(screen.getByText("Retro Cherries")).toBeInTheDocument();
    expect(screen.getByText("2 款商品")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Retro Cherries/ }));
    expect(onSelectSummary).toHaveBeenCalledWith(
      expect.objectContaining({
        id: "batch-1",
      }),
    );
  });

  it("shows the empty state when no recent batches exist", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        summaries={[]}
      />,
    );

    expect(
      screen.getByText("还没有可继续的批次。先在选品区选择 SDS 商品，创建第一批内容。"),
    ).toBeInTheDocument();
  });
});
