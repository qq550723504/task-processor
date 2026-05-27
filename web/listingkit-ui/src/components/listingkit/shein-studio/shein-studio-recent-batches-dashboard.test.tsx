import React from "react";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinStudioRecentBatchesDashboard } from "@/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard";
import { dispatchSheinStudioRecentBatchesFocus } from "@/lib/shein-studio/recent-batches-focus";
import { SHEIN_STUDIO_RECENT_BATCHES_FOCUS_EVENT } from "@/lib/shein-studio/recent-batches-focus";

const push = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
}));

describe("SheinStudioRecentBatchesDashboard", () => {
  beforeEach(() => {
    window.localStorage.clear();
    push.mockReset();
  });

  it("restores dashboard filter and selection context from local storage", async () => {
    window.localStorage.setItem(
      "listingkit:shein-studio:recent-batches-dashboard",
      JSON.stringify({
        statusFilter: "risk",
        resultFilter: "failure",
        activeRiskLabel: "生成失败",
        selectedSummaryIds: ["batch:batch-2"],
        lastBulkActionSummary:
          "上次已为 2 个待生成批次启动处理队列，另外还有 1 个待确认款式风险批次待处理。",
      }),
    );

    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Healthy Batch",
            primaryProductName: "tee",
            productCount: 1,
            promptPreview: "prompt one",
            storeSummary: "869",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-27T00:00:00.000Z",
            alerts: [],
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Failed Batch",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "prompt two",
            storeSummary: "869",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T23:00:00.000Z",
            alerts: [{ tone: "danger", label: "生成失败" }],
          },
        ]}
      />,
    );

    expect(
      await screen.findByText("当前只显示包含“生成失败”的风险批次。"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("已恢复上次的最近处理失败视图。"),
    ).toBeInTheDocument();
    expect(screen.getByText("已选择 1 个批次")).toBeInTheDocument();
    expect(screen.queryByText("Healthy Batch")).not.toBeInTheDocument();
    expect(
      screen.getByText(
        "上次已为 2 个待生成批次启动处理队列，另外还有 1 个待确认款式风险批次待处理。",
      ),
    ).toBeInTheDocument();
  });

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
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
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
    expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-2");
  });

  it("opens a recent batch on the dedicated batch route", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
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

    fireEvent.click(screen.getByRole("button", { name: "打开批次" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-1");
  });

  it("filters recent batches by status buckets", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
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
            updatedAt: "2026-05-27T00:00:00.000Z",
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Need Review",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "prompt two",
            storeSummary: "869",
            designCount: 2,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T23:00:00.000Z",
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
            designCount: 1,
            createdTaskCount: 2,
            updatedAt: "2026-05-26T22:00:00.000Z",
          },
        ]}
      />,
    );

    expect(screen.getByRole("button", { name: "全部 3" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "待生成 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "待创建任务 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "已有任务 1" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "待创建任务 1" }));
    expect(screen.getByText("Need Review")).toBeInTheDocument();
    expect(screen.queryByText("Need Generate")).not.toBeInTheDocument();
    expect(screen.queryByText("Has Tasks")).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "已有任务 1" }));
    expect(screen.getByText("Has Tasks")).toBeInTheDocument();
    expect(screen.queryByText("Need Review")).not.toBeInTheDocument();
  });

  it("filters recent batches to only risky batches", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Risky Batch",
            primaryProductName: "tee",
            productCount: 1,
            promptPreview: "prompt one",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-27T00:00:00.000Z",
            alerts: [
              {
                tone: "warning",
                label: "待确认款式",
              },
            ],
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Healthy Batch",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "prompt two",
            storeSummary: "869",
            designCount: 2,
            createdTaskCount: 1,
            updatedAt: "2026-05-26T23:00:00.000Z",
            alerts: [],
          },
        ]}
      />,
    );

    expect(screen.getByRole("button", { name: "有风险 1" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "有风险 1" }));
    expect(screen.getByText("Risky Batch")).toBeInTheDocument();
    expect(screen.queryByText("Healthy Batch")).not.toBeInTheDocument();
  });

  it("switches to the risk view when the homepage requests recent risky batches", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Risky Batch",
            primaryProductName: "tee",
            productCount: 1,
            promptPreview: "prompt one",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-27T00:00:00.000Z",
            alerts: [
              {
                tone: "danger",
                label: "Baseline 未就绪",
              },
            ],
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Healthy Batch",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "prompt two",
            storeSummary: "869",
            designCount: 2,
            createdTaskCount: 1,
            updatedAt: "2026-05-26T23:00:00.000Z",
            alerts: [],
          },
        ]}
      />,
    );

    fireEvent(
      window,
      new CustomEvent(SHEIN_STUDIO_RECENT_BATCHES_FOCUS_EVENT, {
        detail: { preferRisk: true },
      }),
    );

    expect(screen.getByText("Risky Batch")).toBeInTheDocument();
    expect(screen.queryByText("Healthy Batch")).not.toBeInTheDocument();
    expect(
      screen.getByText("已优先切到风险视图，建议先处理“Baseline 未就绪”相关批次。"),
    ).toBeInTheDocument();
  });

  it("lets the homepage risk guidance jump into the focused risk label", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Baseline Batch",
            primaryProductName: "tee",
            productCount: 1,
            promptPreview: "prompt one",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-27T00:00:00.000Z",
            alerts: [{ tone: "danger", label: "Baseline 未就绪" }],
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Other Risk Batch",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "prompt two",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T23:00:00.000Z",
            alerts: [{ tone: "warning", label: "待确认款式" }],
          },
        ]}
      />,
    );

    fireEvent(
      window,
      new CustomEvent(SHEIN_STUDIO_RECENT_BATCHES_FOCUS_EVENT, {
        detail: { preferRisk: true },
      }),
    );
    fireEvent.click(screen.getByRole("button", { name: "只看这一类风险" }));

    expect(
      screen.getByText("当前只显示包含“Baseline 未就绪”的风险批次。"),
    ).toBeInTheDocument();
    expect(screen.getByText("Baseline Batch")).toBeInTheDocument();
    expect(screen.queryByText("Other Risk Batch")).not.toBeInTheDocument();
  });

  it("follows the shared homepage risk flow into a focused risk label view", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Baseline Batch",
            primaryProductName: "tee",
            productCount: 1,
            promptPreview: "prompt one",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-27T00:00:00.000Z",
            alerts: [{ tone: "danger", label: "Baseline 未就绪" }],
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Other Risk Batch",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "prompt two",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T23:00:00.000Z",
            alerts: [{ tone: "warning", label: "待确认款式" }],
          },
        ]}
      />,
    );

    act(() => {
      dispatchSheinStudioRecentBatchesFocus({ preferRisk: true });
    });
    fireEvent.click(screen.getByRole("button", { name: "只看这一类风险" }));

    expect(
      screen.getByText("当前只显示包含“Baseline 未就绪”的风险批次。"),
    ).toBeInTheDocument();
    expect(screen.getByText("Baseline Batch")).toBeInTheDocument();
    expect(screen.queryByText("Other Risk Batch")).not.toBeInTheDocument();
  });

  it("shows risk summary counts and filters by the chosen risk label", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Baseline Batch",
            primaryProductName: "tee",
            productCount: 1,
            promptPreview: "prompt one",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-27T00:00:00.000Z",
            alerts: [
              {
                tone: "danger",
                label: "Baseline 未就绪",
              },
            ],
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Review Batch",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "prompt two",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T23:00:00.000Z",
            alerts: [
              {
                tone: "warning",
                label: "待确认款式",
              },
            ],
          },
          {
            id: "batch-3",
            source: "batch",
            isRecoverableDraft: false,
            title: "Failed Batch",
            primaryProductName: "mug",
            productCount: 1,
            promptPreview: "prompt three",
            storeSummary: "869",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T22:00:00.000Z",
            alerts: [
              {
                tone: "danger",
                label: "生成失败",
              },
            ],
          },
        ]}
      />,
    );

    expect(screen.getByRole("button", { name: "Baseline 未就绪 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "待确认款式 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "生成失败 1" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "Baseline 未就绪 1" }));
    expect(screen.getByText("Baseline Batch")).toBeInTheDocument();
    expect(screen.queryByText("Review Batch")).not.toBeInTheDocument();
    expect(screen.queryByText("Failed Batch")).not.toBeInTheDocument();
    expect(screen.getByText("当前只显示包含“Baseline 未就绪”的风险批次。")).toBeInTheDocument();
  });

  it("shows the empty state when no recent batches exist", () => {
    const onCreateBatch = vi.fn();

    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={onCreateBatch}
        onSelectSummary={() => undefined}
        summaries={[]}
      />,
    );

    expect(
      screen.getByText("还没有可继续的最近批次，建议先新建一个批次再开始选品。"),
    ).toBeInTheDocument();
    fireEvent.click(screen.getAllByRole("button", { name: "开始新建批次并选品" })[0]);
    expect(onCreateBatch).toHaveBeenCalled();
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

  it("shows recent result summary counts for selected batches", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Success Batch",
            primaryProductName: "tee",
            productCount: 1,
            promptPreview: "prompt one",
            storeSummary: "869",
            designCount: 2,
            createdTaskCount: 1,
            updatedAt: "2026-05-26T10:00:00.000Z",
            recentResults: [
              {
                tone: "success",
                label: "最近生成成功",
                detail: "已生成 2 张设计。",
              },
            ],
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Failed Batch",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "prompt two",
            storeSummary: "869",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T09:00:00.000Z",
            recentResults: [
              {
                tone: "danger",
                label: "最近生成失败",
                detail: "image generation timeout",
              },
            ],
          },
          {
            id: "batch-3",
            source: "batch",
            isRecoverableDraft: false,
            title: "Neutral Batch",
            primaryProductName: "mug",
            productCount: 1,
            promptPreview: "prompt three",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T08:00:00.000Z",
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-3" }));

    expect(
      screen.getByText("最近成功 1 个 / 最近失败 1 个 / 其他 1 个"),
    ).toBeInTheDocument();
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

  it("renders homepage risk alerts when a batch needs attention", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Attention Batch",
            primaryProductName: "tee",
            productCount: 2,
            promptPreview: "retro cherries",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T10:00:00.000Z",
            alerts: [
              {
                tone: "danger",
                label: "Baseline 未就绪",
                detail: "尚未预热",
              },
              {
                tone: "warning",
                label: "待确认款式",
                detail: "需要确认设计",
              },
            ],
          },
        ]}
      />,
    );

    expect(screen.getByText("Baseline 未就绪")).toBeInTheDocument();
    expect(screen.getByText("待确认款式")).toBeInTheDocument();
    expect(screen.getByText("Baseline 未就绪：尚未预热")).toBeInTheDocument();
    expect(screen.getByText("待确认款式：需要确认设计")).toBeInTheDocument();
  });

  it("renders recent processing results on batch cards", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Processed Batch",
            primaryProductName: "tee",
            productCount: 2,
            promptPreview: "retro cherries",
            storeSummary: "869",
            designCount: 2,
            createdTaskCount: 1,
            updatedAt: "2026-05-26T10:00:00.000Z",
            recentResults: [
              {
                tone: "success",
                label: "最近生成成功",
                detail: "已生成 2 张设计。",
              },
              {
                tone: "success",
                label: "最近任务已创建",
                detail: "已创建 1 个 SHEIN 资料任务。",
              },
            ],
          },
        ]}
      />,
    );

    expect(screen.getByText("最近处理结果")).toBeInTheDocument();
    expect(screen.getByText("最近生成成功")).toBeInTheDocument();
    expect(screen.getByText("已生成 2 张设计。")).toBeInTheDocument();
    expect(screen.getByText("最近任务已创建")).toBeInTheDocument();
    expect(screen.getByText("已创建 1 个 SHEIN 资料任务。")).toBeInTheDocument();
  });

  it("filters recent batches by recent processing result buckets", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Success Batch",
            primaryProductName: "tee",
            productCount: 1,
            promptPreview: "prompt one",
            storeSummary: "869",
            designCount: 2,
            createdTaskCount: 1,
            updatedAt: "2026-05-26T10:00:00.000Z",
            recentResults: [
              {
                tone: "success",
                label: "最近生成成功",
                detail: "已生成 2 张设计。",
              },
            ],
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Failed Batch",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "prompt two",
            storeSummary: "869",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T09:00:00.000Z",
            recentResults: [
              {
                tone: "danger",
                label: "最近生成失败",
                detail: "image generation timeout",
              },
            ],
          },
          {
            id: "batch-3",
            source: "batch",
            isRecoverableDraft: false,
            title: "Pending Batch",
            primaryProductName: "mug",
            productCount: 1,
            promptPreview: "prompt three",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T08:00:00.000Z",
            recentResults: [
              {
                tone: "warning",
                label: "待创建任务",
                detail: "已确认 1 个款式，尚未创建任务。",
              },
            ],
          },
        ]}
      />,
    );

    expect(screen.getByRole("button", { name: "全部结果 3" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "最近成功 1" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "最近失败 1" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "最近失败 1" }));
    expect(screen.getByText("Failed Batch")).toBeInTheDocument();
    expect(screen.queryByText("Success Batch")).not.toBeInTheDocument();
    expect(screen.queryByText("Pending Batch")).not.toBeInTheDocument();
    expect(screen.getByText("当前只显示最近处理失败的批次。")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "最近成功 1" }));
    expect(screen.getByText("Success Batch")).toBeInTheDocument();
    expect(screen.queryByText("Failed Batch")).not.toBeInTheDocument();
    expect(screen.getByText("当前只显示最近处理成功的批次。")).toBeInTheDocument();
  });

  it("routes risk alert actions to the dedicated batch route", () => {
    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Blocked Batch",
            primaryProductName: "tee",
            productCount: 2,
            promptPreview: "retro cherries",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T10:00:00.000Z",
            alerts: [
              {
                tone: "danger",
                label: "Baseline 未就绪",
              },
              {
                tone: "danger",
                label: "生成失败",
              },
              {
                tone: "warning",
                label: "待确认款式",
              },
            ],
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "去生成区处理" }));
    expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-1");

    fireEvent.click(screen.getByRole("button", { name: "回到生成区重试" }));
    expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-1");

    fireEvent.click(screen.getByRole("button", { name: "去确认设计" }));
    expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-1");
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

  it("shows bulk queue feedback and remaining bucket guidance after launching a state-aware bulk action", () => {
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
            title: "Need Generate",
            primaryProductName: "tee",
            productCount: 2,
            promptPreview: "retro cherries",
            storeSummary: "869",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T10:00:00.000Z",
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Need Review",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "second prompt",
            storeSummary: "跟随当前店铺",
            designCount: 2,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T09:00:00.000Z",
          },
          {
            id: "batch-3",
            source: "batch",
            isRecoverableDraft: false,
            title: "Need Tasks",
            primaryProductName: "mug",
            productCount: 1,
            promptPreview: "third prompt",
            storeSummary: "跟随当前店铺",
            designCount: 2,
            createdTaskCount: 1,
            updatedAt: "2026-05-26T08:00:00.000Z",
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-3" }));
    fireEvent.click(screen.getByRole("button", { name: "批量继续生成 1 个" }));

    expect(
      screen.getByText(
        "已为 1 个待生成批次启动处理队列。另外还有 1 个待创建任务批次，另外还有 1 个已有任务批次可继续处理。",
      ),
    ).toBeInTheDocument();
    expect(onOpenBatchQueue).toHaveBeenCalledWith({
      batchIds: ["batch-1"],
      mode: "generate",
    });
    expect(
      JSON.parse(
        window.localStorage.getItem(
          "listingkit:shein-studio:recent-batches-dashboard",
        ) || "{}",
      ).lastBulkActionSummary,
    ).toBe(
      "已为 1 个待生成批次启动处理队列。另外还有 1 个待创建任务批次，另外还有 1 个已有任务批次可继续处理。",
    );
  });

  it("warns when selected batches include risks and can queue only healthy batches", () => {
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
            title: "Healthy Generate Batch",
            primaryProductName: "tee",
            productCount: 1,
            promptPreview: "prompt one",
            storeSummary: "869",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-27T00:00:00.000Z",
            alerts: [],
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Risky Generate Batch",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "prompt two",
            storeSummary: "869",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T23:00:00.000Z",
            alerts: [
              {
                tone: "danger",
                label: "生成失败",
              },
            ],
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));

    expect(
      screen.getByText("本次选择里有 1 个风险批次，建议先处理后再进入队列。"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "仅处理可继续批次 1 个" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "仅处理可继续批次 1 个" }));
    expect(onOpenBatchQueue).toHaveBeenCalledWith({
      batchIds: ["batch-1"],
      mode: "generate",
    });
  });

  it("offers bulk risk repair actions for selected risky batches", () => {
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
            title: "Baseline Batch",
            primaryProductName: "tee",
            productCount: 1,
            promptPreview: "prompt one",
            storeSummary: "869",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-27T00:00:00.000Z",
            alerts: [
              { tone: "danger", label: "Baseline 未就绪" },
            ],
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Failed Batch",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "prompt two",
            storeSummary: "869",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T23:00:00.000Z",
            alerts: [
              { tone: "danger", label: "生成失败" },
            ],
          },
          {
            id: "batch-3",
            source: "batch",
            isRecoverableDraft: false,
            title: "Review Batch",
            primaryProductName: "mug",
            productCount: 1,
            promptPreview: "prompt three",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T22:00:00.000Z",
            alerts: [
              { tone: "warning", label: "待确认款式" },
            ],
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-1" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-2" }));
    fireEvent.click(screen.getByRole("checkbox", { name: "select batch-3" }));

    expect(
      screen.getByText("风险拆分：Baseline 未就绪 1 个 / 生成失败 1 个 / 待确认款式 1 个"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "批量去生成区处理 2 个" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "批量去确认设计 1 个" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "批量去生成区处理 2 个" }));
    expect(onOpenBatchQueue).toHaveBeenCalledWith({
      batchIds: ["batch-1", "batch-2"],
      mode: "generate",
    });
    expect(
      screen.getByText("已为 2 个风险批次启动生成处理队列。另外还有 1 个待确认款式风险批次可继续处理。"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "批量去确认设计 1 个" }));
    expect(onOpenBatchQueue).toHaveBeenCalledWith({
      batchIds: ["batch-3"],
      mode: "create_tasks",
    });
  });

  it("can keep only risky or only healthy selected batches", () => {
    const onSelectedSummaryIdsChange = vi.fn();

    render(
      <SheinStudioRecentBatchesDashboard
        onCreateBatch={() => undefined}
        onSelectSummary={() => undefined}
        onSelectedSummaryIdsChange={onSelectedSummaryIdsChange}
        selectedSummaryIds={["batch:batch-1", "batch:batch-2", "batch:batch-3"]}
        summaries={[
          {
            id: "batch-1",
            source: "batch",
            isRecoverableDraft: false,
            title: "Healthy Batch",
            primaryProductName: "tee",
            productCount: 1,
            promptPreview: "prompt one",
            storeSummary: "869",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-27T00:00:00.000Z",
            alerts: [],
          },
          {
            id: "batch-2",
            source: "batch",
            isRecoverableDraft: false,
            title: "Risky Batch A",
            primaryProductName: "hoodie",
            productCount: 1,
            promptPreview: "prompt two",
            storeSummary: "869",
            designCount: 0,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T23:00:00.000Z",
            alerts: [{ tone: "danger", label: "生成失败" }],
          },
          {
            id: "batch-3",
            source: "batch",
            isRecoverableDraft: false,
            title: "Risky Batch B",
            primaryProductName: "mug",
            productCount: 1,
            promptPreview: "prompt three",
            storeSummary: "869",
            designCount: 1,
            createdTaskCount: 0,
            updatedAt: "2026-05-26T22:00:00.000Z",
            alerts: [{ tone: "warning", label: "待确认款式" }],
          },
        ]}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "仅保留风险批次 2 个" }));
    expect(onSelectedSummaryIdsChange).toHaveBeenCalledWith([
      "batch:batch-2",
      "batch:batch-3",
    ]);

    fireEvent.click(screen.getByRole("button", { name: "仅保留可继续批次 1 个" }));
    expect(onSelectedSummaryIdsChange).toHaveBeenCalledWith(["batch:batch-1"]);
  });

  it("can restore the previous selection after using split shortcuts", () => {
    function Wrapper() {
      const [selectedSummaryIds, setSelectedSummaryIds] = React.useState([
        "batch:batch-1",
        "batch:batch-2",
        "batch:batch-3",
      ]);

      return (
        <SheinStudioRecentBatchesDashboard
          onCreateBatch={() => undefined}
          onSelectSummary={() => undefined}
          onSelectedSummaryIdsChange={setSelectedSummaryIds}
          selectedSummaryIds={selectedSummaryIds}
          summaries={[
            {
              id: "batch-1",
              source: "batch",
              isRecoverableDraft: false,
              title: "Healthy Batch",
              primaryProductName: "tee",
              productCount: 1,
              promptPreview: "prompt one",
              storeSummary: "869",
              designCount: 0,
              createdTaskCount: 0,
              updatedAt: "2026-05-27T00:00:00.000Z",
              alerts: [],
            },
            {
              id: "batch-2",
              source: "batch",
              isRecoverableDraft: false,
              title: "Risky Batch A",
              primaryProductName: "hoodie",
              productCount: 1,
              promptPreview: "prompt two",
              storeSummary: "869",
              designCount: 0,
              createdTaskCount: 0,
              updatedAt: "2026-05-26T23:00:00.000Z",
              alerts: [{ tone: "danger", label: "生成失败" }],
            },
            {
              id: "batch-3",
              source: "batch",
              isRecoverableDraft: false,
              title: "Risky Batch B",
              primaryProductName: "mug",
              productCount: 1,
              promptPreview: "prompt three",
              storeSummary: "869",
              designCount: 1,
              createdTaskCount: 0,
              updatedAt: "2026-05-26T22:00:00.000Z",
              alerts: [{ tone: "warning", label: "待确认款式" }],
            },
          ]}
        />
      );
    }

    render(<Wrapper />);

    fireEvent.click(screen.getByRole("button", { name: "仅保留风险批次 2 个" }));
    expect(screen.getByText("已选择 2 个批次")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "恢复上一次选择 3 个" }));
    expect(screen.getByText("已选择 3 个批次")).toBeInTheDocument();
  });
});
