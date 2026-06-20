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
        batchStatus: "generating",
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
      batchStatus: "generating",
      storeSummary: "869",
      alerts: [],
    });
  });

  it("does not double count the primary selection when legacy grouped selections duplicate it", () => {
    const summaries = buildRecentBatchSummaries([
      {
        id: "batch-legacy",
        name: "批次12",
        prompt: "",
        styleCount: "1",
        sheinStoreId: "",
        selection,
        groupedSelections: [
          {
            selectionId: "1:200:100:layer-1:",
            sheinStoreId: "",
            selection,
            eligible: true,
            baselineStatus: "baseline_cached",
            baselineReason: "基础模板已缓存，等待进一步校验",
          },
        ],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
    ]);

    expect(summaries[0]).toMatchObject({
      id: "batch-legacy",
      productCount: 1,
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

  it("does not render a separate local draft summary when the snapshot belongs to an existing saved batch", () => {
    const summaries = buildRecentBatchSummaries(
      [
        {
          id: "batch-1",
          name: "Saved Batch",
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
        draftBatchId: "batch-1",
      },
    );

    expect(summaries).toHaveLength(1);
    expect(summaries[0]).toMatchObject({
      id: "batch-1",
      source: "batch",
      title: "Saved Batch",
    });
  });

  it("derives grouped baseline validation and eligibility alerts from persisted batches", () => {
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
            baselineStatus: "blocked",
            baselineReason: "SDS 模板层未通过校验",
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
          label: "Baseline 校验未通过",
          detail: "SDS 模板层未通过校验",
        }),
        expect.objectContaining({
          label: "Grouped 商品待处理",
          detail: "印刷区域待确认",
        }),
      ]),
    );
  });

  it("surfaces cached baseline guidance when grouped products have not been validated yet", () => {
    const summaries = buildRecentBatchSummaries([
      {
        id: "batch-2",
        name: "Cached Batch",
        prompt: "cached prompt",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        groupedSelections: [
          {
            selectionId: "sel-2",
            sheinStoreId: "869",
            selection: hoodie,
            eligible: true,
            baselineStatus: "baseline_cached",
            baselineReason: "基础模板已缓存，等待进一步校验",
          },
        ],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:30:00.000Z",
      },
    ]);

    expect(summaries[0].alerts).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          label: "Baseline 待校验",
          detail: "基础模板已缓存，等待进一步校验",
        }),
      ]),
    );
  });

  it("falls back to baseline reason code when persisted grouped selections have no free-form reason", () => {
    const summaries = buildRecentBatchSummaries([
      {
        id: "batch-3",
        name: "Reason Code Batch",
        prompt: "reason code prompt",
        styleCount: "1",
        sheinStoreId: "869",
        selection,
        groupedSelections: [
          {
            selectionId: "sel-3",
            sheinStoreId: "869",
            selection: hoodie,
            eligible: false,
            baselineStatus: "blocked",
            baselineReason: "",
            baselineReasonCode: "prototype_group_mismatch",
          },
        ],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-26T10:40:00.000Z",
      },
    ]);

    expect(summaries[0].alerts).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          label: "Baseline 校验未通过",
          detail: "这款商品的 prototype group 与当前 SDS 设计面不匹配。",
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
        batchStatus: "generating",
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
