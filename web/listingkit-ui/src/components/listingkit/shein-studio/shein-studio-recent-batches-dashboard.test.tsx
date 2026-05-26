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
    expect(screen.getByText("已有 1 张设计")).toBeInTheDocument();
    expect(screen.getByText("待创建任务")).toBeInTheDocument();
    expect(screen.getByText("最近提示词")).toBeInTheDocument();
    expect(screen.getByText(/更新于/)).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: /Retro Cherries/ }));
    expect(onSelectSummary).toHaveBeenCalledWith(
      expect.objectContaining({
        id: "batch-1",
      }),
    );
  });

  it("shows state-driven primary actions and emits the selected action", () => {
    const onSelectSummaryAction = vi.fn();

    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        onSelectSummaryAction={onSelectSummaryAction}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Need Generate",
            primaryProductName: "tee",
            productCount: 1,
            promptPreview: "prompt one",
            storeSummary: "869",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Need Tasks",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "prompt two",
            storeSummary: "869",
            designCount: 2,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T09:00:00.000Z",
          },
          {
            id: "batch-3",
            source: "batch",
            isRecoverableDraft: false,
            title: "Has Tasks",
            primaryProductName: "mug",
            productCount: 1,
            promptPreview: "prompt three",
            storeSummary: "869",
            designCount: 2,
            createdTaskCount: 1,
            updatedAt: "2026-05-26T08:00:00.000Z",
          },
        ]}
      />,
    );

    expect(screen.getByRole("button", { name: "继续生成" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "去创建任务" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "查看任务" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "去创建任务" }));
    expect(onSelectSummaryAction).toHaveBeenCalledWith(
      expect.objectContaining({ id: "batch-2" }),
      "review",
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

  it("renames a batch from the homepage", () => {
    const onRenameSummary = vi.fn();

    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onRenameSummary={onRenameSummary}
        onSelectSummary={() => undefined}
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
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "重命名" }));
    fireEvent.change(screen.getByLabelText("批次名称"), {
      target: { value: "New Name" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存名称" }));

    expect(onRenameSummary).toHaveBeenCalledWith(
      expect.objectContaining({ id: "batch-1" }),
      "New Name",
    );
  });

  it("deletes a batch from the homepage", () => {
    const onDeleteSummary = vi.fn();

    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onDeleteSummary={onDeleteSummary}
        onSelectSummary={() => undefined}
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

    fireEvent.click(screen.getByRole("button", { name: "删除" }));
    expect(onDeleteSummary).toHaveBeenCalledWith(
      expect.objectContaining({ id: "batch-1" }),
    );
  });

  it("supports multi-selecting recent batch cards and bulk updating store", () => {
    const onBulkUpdateStore = vi.fn();

    render(
      <SheinStudioRecentBatchesDashboard
        onBulkUpdateStore={onBulkUpdateStore}
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        storeOptions={[
          { id: "869", label: "US Store 1" },
          { id: "870", label: "US Store 2" },
        ]}
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
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Second Batch",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "second prompt",
            storeSummary: "跟随当前店铺",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T09:00:00.000Z",
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    expect(screen.getByText("已选择 2 个批次")).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("目标店铺"), {
      target: { value: "869" },
    });
    fireEvent.click(screen.getByRole("button", { name: "应用到已选批次" }));

    expect(onBulkUpdateStore).toHaveBeenCalledWith(["batch-1", "batch-2"], "869");
  });

  it("shows bulk queue actions when persisted batches are selected", () => {
    const onOpenBatchQueue = vi.fn();

    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onOpenBatchQueue={onOpenBatchQueue}
        onSelectSummary={() => undefined}
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
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Second Batch",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "second prompt",
            storeSummary: "跟随当前店铺",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T09:00:00.000Z",
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));

    expect(screen.getByRole("button", { name: "批量去创建任务 1 个" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "批量继续生成 1 个" })).toBeInTheDocument();
    expect(screen.getByText("待生成 1 个 / 待创建任务 1 个 / 已有任务 0 个")).toBeInTheDocument();
  });

  it("shows ready-to-work status badges for generated designs and created tasks", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Ready Batch",
            primaryProductName: "tee",
            productCount: 2,
            promptPreview: "retro cherries",
            storeSummary: "869",
            designCount: 3,
            createdTaskCount: 2,
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
        ]}
      />,
    );

    expect(screen.getByText("已有 3 张设计")).toBeInTheDocument();
    expect(screen.getByText("已建 2 个任务")).toBeInTheDocument();
  });

  it("emits selected persisted batch ids for continue-generate mode", () => {
    const onOpenBatchQueue = vi.fn();

    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onOpenBatchQueue={onOpenBatchQueue}
        onSelectSummary={() => undefined}
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
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            id: "draft-1",
            source: "local_draft",
            isRecoverableDraft: true,
            title: "Local Draft",
            primaryProductName: "mug",
            productCount: 1,
            promptPreview: "draft prompt",
            storeSummary: "跟随当前店铺",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T11:00:00.000Z",
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select draft-1" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 1 个" }));

    expect(onOpenBatchQueue).toHaveBeenCalledWith({
      batchIds: ["batch-1"],
      mode: "generate",
    });
  });

  it("routes selected persisted batches into review and tasks bulk queues by status", () => {
    const onOpenBatchQueue = vi.fn();

    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onOpenBatchQueue={onOpenBatchQueue}
        onSelectSummary={() => undefined}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Need Review",
            primaryProductName: "tee",
            productCount: 2,
            promptPreview: "retro cherries",
            storeSummary: "869",
            designCount: 2,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Need Tasks",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "second prompt",
            storeSummary: "跟随当前店铺",
            designCount: 2,
            createdTaskCount: 1,
            updatedAt: "2026-05-26T09:00:00.000Z",
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));

    fireEvent.click(screen.getByRole("button", { name: "批量去创建任务 1 个" }));
    expect(onOpenBatchQueue).toHaveBeenCalledWith({
      batchIds: ["batch-1"],
      mode: "create_tasks",
    });

    fireEvent.click(screen.getByRole("button", { name: "批量查看任务 1 个" }));
    expect(onOpenBatchQueue).toHaveBeenCalledWith({
      batchIds: ["batch-2"],
      mode: "create_tasks",
    });
  });
});
