import { describe, expect, it } from "vitest";

import { buildRecentBatchSummaries } from "@/lib/shein-studio/recent-batch-summaries";

const selection = {
  productId: 1,
  parentProductId: 1,
  variantId: 100,
  prototypeGroupId: 200,
  layerId: "layer-1",
  productName: "tee",
  variantLabel: "M / black",
  printableWidth: 1000,
  printableHeight: 1000,
};

const hoodie = {
  productId: 1,
  parentProductId: 1,
  variantId: 101,
  prototypeGroupId: 200,
  layerId: "layer-2",
  productName: "hoodie",
  variantLabel: "L / white",
  printableWidth: 1000,
  printableHeight: 1000,
};

describe("buildRecentBatchSummaries", () => {
  it("derives batch card summary from a persisted batch", () => {
    const summaries = buildRecentBatchSummaries([
      {
        id: "batch-1",
        name: "Retro Cherries",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        groupedSelections: [
          {
            selectionId: "sel-1",
            sheinStoreId: "869",
            selection: hoodie,
            eligible: true,
            baselineStatus: "ready",
            baselineReason: "",
          },
        ],
        designs: [{ id: "design-1" }],
        selectedIds: ["design-1"],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    expect(summaries[0]).toMatchObject({
      id: "batch-1",
      source: "batch",
      title: "Retro Cherries",
      primaryProductName: "tee",
      productCount: 2,
      promptPreview: "retro cherries",
      designCount: 1,
      createdTaskCount: 0,
      storeSummary: "869",
      alerts: [],
    });
  });

  it("marks local recovery drafts separately from persisted batches", () => {
    const summaries = buildRecentBatchSummaries([], {
      draft: {
        prompt: "draft prompt",
        styleCount: "1",
        sheinStoreId: "869",
        groups: [
          {
            id: "group-1",
            name: "Group 1",
            primarySelection: selection,
            groupedSelections: [],
            sheinStoreId: "869",
            currentPrompt: "prompt a",
            promptHistory: [],
            designs: [],
            selectedIds: [],
            createdTasks: [],
            updatedAt: "2026-05-26T11:00:00.000Z",
          },
        ],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T11:00:00.000Z",
      },
    });

    expect(summaries[0]).toMatchObject({
      source: "local_draft",
      isRecoverableDraft: true,
      title: "Group 1",
      primaryProductName: "tee",
      productCount: 1,
      promptPreview: "prompt a",
      alerts: [
        expect.objectContaining({
          label: "未保存草稿",
        }),
      ],
    });
  });

  it("sorts newest summaries first across local recovery and persisted batches", () => {
    const summaries = buildRecentBatchSummaries(
      [
        {
          id: "batch-1",
          name: "Older Batch",
          prompt: "older prompt",
          styleCount: "1",
          sheinStoreId: "",
          selection,
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T09:00:00.000Z",
        },
      ],
      {
        draft: {
          prompt: "draft prompt",
          styleCount: "1",
          sheinStoreId: "",
          groups: [
            {
              id: "group-1",
              name: "Draft Group",
              primarySelection: selection,
              groupedSelections: [],
              sheinStoreId: "",
              currentPrompt: "draft prompt",
              promptHistory: [],
              designs: [],
              selectedIds: [],
              createdTasks: [],
              updatedAt: "2026-05-26T11:00:00.000Z",
            },
          ],
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-05-26T11:00:00.000Z",
        },
      },
    );

    expect(summaries.map((item) => item.title)).toEqual([
      "Draft Group",
      "Older Batch",
    ]);
  });

  it("derives grouped baseline and eligibility alerts from persisted batches", () => {
    const summaries = buildRecentBatchSummaries([
      {
        id: "batch-1",
        name: "Blocked Batch",
        prompt: "blocked prompt",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        groupedSelections: [
          {
            selectionId: "sel-1",
            sheinStoreId: "869",
            selection: hoodie,
            eligible: false,
            eligibilityReason: "印刷区域待确认",
            baselineStatus: "missing",
            baselineReason: "尚未预热",
          },
        ],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    expect(summaries[0].alerts).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          label: "Baseline 未就绪",
          detail: "尚未预热",
        }),
        expect.objectContaining({
          label: "Grouped 商品待处理",
          detail: "印刷区域待确认",
        }),
      ]),
    );
  });

  it("flags persisted batches with designs but no selected styles", () => {
    const summaries = buildRecentBatchSummaries([
      {
        id: "batch-1",
        name: "Needs Review",
        prompt: "needs review",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [{ id: "design-1" }],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    expect(summaries[0].alerts).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          label: "待确认款式",
        }),
      ]),
    );
  });

  it("derives draft generation failure alerts", () => {
    const summaries = buildRecentBatchSummaries([], {
      draft: {
        prompt: "draft prompt",
        styleCount: "1",
        sheinStoreId: "869",
        generationError: "image generation timeout",
        groups: [
          {
            id: "group-1",
            name: "Group 1",
            primarySelection: selection,
            groupedSelections: [],
            sheinStoreId: "869",
            currentPrompt: "prompt a",
            promptHistory: [],
            designs: [],
            selectedIds: [],
            createdTasks: [],
            updatedAt: "2026-05-26T11:00:00.000Z",
          },
        ],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T11:00:00.000Z",
      },
    });

    expect(summaries[0].alerts).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          label: "未保存草稿",
        }),
        expect.objectContaining({
          label: "生成失败",
          detail: "image generation timeout",
        }),
      ]),
    );
  });

  it("derives recent processing results for generation and task creation", () => {
    const summaries = buildRecentBatchSummaries([
      {
        id: "batch-1",
        name: "Ready Batch",
        prompt: "ready prompt",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        designs: [{ id: "design-1" }, { id: "design-2" }],
        selectedIds: ["design-1"],
        createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    expect(summaries[0].recentResults).toEqual([
      expect.objectContaining({
        label: "最近生成成功",
        detail: "已生成 2 张设计。",
      }),
      expect.objectContaining({
        label: "最近任务已创建",
        detail: "已创建 1 个 SHEIN 资料任务。",
      }),
    ]);
  });

  it("derives in-progress and failed recent processing results from drafts", () => {
    const summaries = buildRecentBatchSummaries([], {
      draft: {
        prompt: "draft prompt",
        styleCount: "1",
        sheinStoreId: "869",
        generationError: "image generation timeout",
        sessionStatus: "generating",
        groups: [
          {
            id: "group-1",
            name: "Group 1",
            primarySelection: selection,
            groupedSelections: [],
            sheinStoreId: "869",
            currentPrompt: "prompt a",
            promptHistory: [],
            designs: [],
            selectedIds: [],
            createdTasks: [],
            updatedAt: "2026-05-26T11:00:00.000Z",
          },
        ],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T11:00:00.000Z",
      },
    });

    expect(summaries[0].recentResults).toEqual([
      expect.objectContaining({
        label: "最近生成中",
        detail: "当前仍在生成设计。",
      }),
    ]);
  });
});
