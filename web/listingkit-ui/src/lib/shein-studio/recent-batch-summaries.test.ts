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
});
