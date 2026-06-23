import { describe, expect, it } from "vitest";

import {
  buildRecentBatchSummaryKey,
  buildRecentBatchSummaryKeys,
  buildRecentBatchSaveInput,
  buildRecentBatchBulkStoreUpdateInputs,
  buildRecentBatchStoreUpdateInput,
  projectRecentBatchSelectionState,
  removeRecentBatchSummarySelection,
  selectFreshRecentBatchHydration,
  selectRecentBatchBulkDeleteFailure,
} from "@/components/listingkit/shein-studio/shein-studio-recent-batch-controller";
import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import type { SheinStudioSavedBatch } from "@/lib/types/shein-studio";

const selection = {
  productId: 1,
  parentProductId: 1,
  variantId: 100,
  prototypeGroupId: 200,
  layerId: "layer-1",
  productName: "tee",
  variantLabel: "M / black",
};

function buildBatch(
  overrides: Partial<SheinStudioSavedBatch> = {},
): SheinStudioSavedBatch {
  return {
    id: "batch-1",
    name: "Batch 1",
    prompt: "prompt",
    styleCount: "2",
    sheinStoreId: "store-old",
    designs: [],
    selectedIds: [],
    createdTasks: [],
    updatedAt: "2026-06-20T00:00:00.000Z",
    ...overrides,
  };
}

function buildHydratedBatch(
  savedBatch: SheinStudioSavedBatch,
): SheinStudioWorkbenchHydratedBatch {
  return {
    savedBatch,
    detail: {},
  } as SheinStudioWorkbenchHydratedBatch;
}

describe("buildRecentBatchSaveInput", () => {
  it("projects a saved batch for persistence and prefers draft updated time", () => {
    const input = buildRecentBatchSaveInput(
      buildBatch({
        draftUpdatedAt: "2026-06-21T00:00:00.000Z",
        generationJobId: "job-1",
      }),
      {
        name: "Renamed",
      },
    );

    expect(input).toMatchObject({
      id: "batch-1",
      name: "Renamed",
      prompt: "prompt",
      sheinStoreId: "store-old",
      generationJobId: "job-1",
      updatedAt: "2026-06-21T00:00:00.000Z",
    });
  });
});

describe("buildRecentBatchStoreUpdateInput", () => {
  it("updates the batch store and nested grouped store ids", () => {
    const input = buildRecentBatchStoreUpdateInput(
      buildBatch({
        groupedSelections: [
          {
            selectionId: "selection-1",
            selection,
            baselineStatus: "ready",
            baselineReason: "",
            sheinStoreId: "store-old",
            eligible: true,
          },
        ],
        groups: [
          {
            id: "group-1",
            name: "Group 1",
            primarySelection: selection,
            groupedSelections: [
              {
                selectionId: "selection-2",
                selection: { ...selection, variantId: 101 },
                baselineStatus: "ready",
                baselineReason: "",
                sheinStoreId: "store-old",
                eligible: true,
              },
            ],
            sheinStoreId: "store-old",
            currentPrompt: "group prompt",
            promptHistory: [],
            designs: [],
            selectedIds: [],
            createdTasks: [],
            updatedAt: "2026-06-20T00:00:00.000Z",
          },
        ],
      }),
      "store-new",
    );

    expect(input.sheinStoreId).toBe("store-new");
    expect(input.groupedSelections?.[0]?.sheinStoreId).toBe("store-new");
    expect(input.groups?.[0]?.sheinStoreId).toBe("store-new");
    expect(input.groups?.[0]?.groupedSelections[0]?.sheinStoreId).toBe(
      "store-new",
    );
  });
});

describe("buildRecentBatchBulkStoreUpdateInputs", () => {
  it("projects every selected batch into a store update save input", () => {
    const inputs = buildRecentBatchBulkStoreUpdateInputs(
      [
        buildBatch({ id: "batch-1", sheinStoreId: "store-old" }),
        buildBatch({ id: "batch-2", sheinStoreId: "store-old" }),
      ],
      "store-new",
    );

    expect(inputs).toEqual([
      expect.objectContaining({ id: "batch-1", sheinStoreId: "store-new" }),
      expect.objectContaining({ id: "batch-2", sheinStoreId: "store-new" }),
    ]);
  });
});

describe("projectRecentBatchSelectionState", () => {
  it("keeps only visible summary keys and returns persisted batch ids", () => {
    const projection = projectRecentBatchSelectionState({
      rawSelectedRecentBatchSummaryIds: [
        "batch:batch-1",
        "local_draft:local-draft:group-1",
        "batch:missing",
        "local_draft:missing",
      ],
      validRecentBatchSummaryKeys: buildRecentBatchSummaryKeys([
        { id: "batch-1", source: "batch" },
        { id: "local-draft:group-1", source: "local_draft" },
      ]),
    });

    expect([...projection.validRecentBatchSummaryKeys]).toEqual([
      "batch:batch-1",
      "local_draft:local-draft:group-1",
    ]);
    expect(projection.selectedRecentBatchSummaryIds).toEqual([
      "batch:batch-1",
      "local_draft:local-draft:group-1",
    ]);
    expect(projection.selectedPersistedRecentBatchIds).toEqual(["batch-1"]);
  });

  it("keeps batch ids with colons intact", () => {
    const projection = projectRecentBatchSelectionState({
      rawSelectedRecentBatchSummaryIds: ["batch:batch:1"],
      validRecentBatchSummaryKeys: buildRecentBatchSummaryKeys([
        { id: "batch:1", source: "batch" },
      ]),
    });

    expect(projection.selectedPersistedRecentBatchIds).toEqual(["batch:1"]);
  });
});

describe("removeRecentBatchSummarySelection", () => {
  it("removes the selected summary key without knowing its source format", () => {
    const summary = { id: "local-draft:group-1", source: "local_draft" } as const;

    expect(
      removeRecentBatchSummarySelection(
        [
          "batch:batch-1",
          buildRecentBatchSummaryKey(summary),
          "local_draft:other",
        ],
        summary,
      ),
    ).toEqual(["batch:batch-1", "local_draft:other"]);
  });
});

describe("selectFreshRecentBatchHydration", () => {
  it("keeps a cached hydration when it is at least as fresh as the saved batch", () => {
    const savedBatch = buildBatch({
      updatedAt: "2026-06-20T00:00:00.000Z",
    });
    const cachedHydratedBatch = buildHydratedBatch(
      buildBatch({ updatedAt: "2026-06-21T00:00:00.000Z" }),
    );

    expect(
      selectFreshRecentBatchHydration({ cachedHydratedBatch, savedBatch }),
    ).toBe(cachedHydratedBatch);
  });

  it("ignores a stale cached hydration", () => {
    const savedBatch = buildBatch({
      updatedAt: "2026-06-21T00:00:00.000Z",
    });
    const cachedHydratedBatch = buildHydratedBatch(
      buildBatch({ updatedAt: "2026-06-20T00:00:00.000Z" }),
    );

    expect(
      selectFreshRecentBatchHydration({ cachedHydratedBatch, savedBatch }),
    ).toBeNull();
  });
});

describe("selectRecentBatchBulkDeleteFailure", () => {
  it("ignores missing studio session delete failures", () => {
    const results: PromiseSettledResult<void>[] = [
      { status: "rejected", reason: new Error("studio session not found") },
      { status: "fulfilled", value: undefined },
    ];

    expect(selectRecentBatchBulkDeleteFailure(results)).toBeNull();
  });

  it("returns the first non-missing delete failure", () => {
    const failure = new Error("network failed");
    const results: PromiseSettledResult<void>[] = [
      { status: "rejected", reason: new Error("studio session not found") },
      { status: "rejected", reason: failure },
    ];

    expect(selectRecentBatchBulkDeleteFailure(results)).toBe(failure);
  });
});
