import { describe, expect, it, vi } from "vitest";

import {
  buildRecentBatchSummaryKey,
  buildRecentBatchSummaryKeys,
  buildRecentBatchSaveInput,
  buildRecentBatchBulkStoreUpdateInputs,
  buildRecentBatchStoreUpdateInput,
  deleteRecentBatchSummary,
  duplicateRecentBatchSummary,
  mergeRecentBatchHydrations,
  projectRecentBatchSelectionUpdate,
  projectRecentBatchSummaries,
  projectRecentBatchSelectionState,
  projectRecentBatchTargetStep,
  renameRecentBatchSummary,
  runRecentBatchBulkDelete,
  runRecentBatchBulkStoreUpdate,
  resolveRecentBatchSelectionTarget,
  removeRecentBatchSummarySelection,
  resolveRecentBatchForMutation,
  resolveRecentBatchMutationTargets,
  selectFreshRecentBatchHydration,
  selectRecentBatchBulkDeleteFailure,
  upsertRecentSavedBatch,
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

describe("renameRecentBatchSummary", () => {
  it("does nothing for non-persisted summaries", async () => {
    const resolveBatch = vi.fn();
    const saveBatch = vi.fn();
    const refreshSavedBatches = vi.fn();

    await renameRecentBatchSummary({
      name: "Renamed",
      refreshSavedBatches,
      resolveBatch,
      saveBatch,
      summary: { id: "local-draft", source: "local_draft" },
    });

    expect(resolveBatch).not.toHaveBeenCalled();
    expect(saveBatch).not.toHaveBeenCalled();
    expect(refreshSavedBatches).not.toHaveBeenCalled();
  });

  it("saves a renamed persisted summary and refreshes batches", async () => {
    const batch = buildBatch({ id: "batch-1", name: "Old" });
    const resolveBatch = vi.fn().mockResolvedValue(batch);
    const saveBatch = vi.fn().mockResolvedValue(null);
    const refreshSavedBatches = vi.fn().mockResolvedValue(undefined);

    await renameRecentBatchSummary({
      name: "Renamed",
      refreshSavedBatches,
      resolveBatch,
      saveBatch,
      summary: { id: "batch-1", source: "batch" },
    });

    expect(resolveBatch).toHaveBeenCalledWith("batch-1");
    expect(saveBatch).toHaveBeenCalledWith(
      expect.objectContaining({ id: "batch-1", name: "Renamed" }),
      { makeActive: false },
    );
    expect(refreshSavedBatches).toHaveBeenCalled();
  });
});

describe("duplicateRecentBatchSummary", () => {
  it("does nothing for non-persisted summaries", async () => {
    const resolveBatch = vi.fn();
    const saveBatch = vi.fn();
    const refreshSavedBatches = vi.fn();

    await duplicateRecentBatchSummary({
      refreshSavedBatches,
      resolveBatch,
      saveBatch,
      summary: { id: "local-draft", source: "local_draft" },
    });

    expect(resolveBatch).not.toHaveBeenCalled();
    expect(saveBatch).not.toHaveBeenCalled();
    expect(refreshSavedBatches).not.toHaveBeenCalled();
  });

  it("duplicates a persisted summary and refreshes batches", async () => {
    const batch = buildBatch({ id: "batch-1", name: "Original" });
    const resolveBatch = vi.fn().mockResolvedValue(batch);
    const saveBatch = vi.fn().mockResolvedValue(null);
    const refreshSavedBatches = vi.fn().mockResolvedValue(undefined);

    await duplicateRecentBatchSummary({
      refreshSavedBatches,
      resolveBatch,
      saveBatch,
      summary: { id: "batch-1", source: "batch" },
    });

    expect(resolveBatch).toHaveBeenCalledWith("batch-1");
    expect(saveBatch).toHaveBeenCalledWith(
      expect.objectContaining({
        name: expect.stringContaining("Original"),
      }),
      { makeActive: false },
    );
    expect(refreshSavedBatches).toHaveBeenCalled();
  });
});

describe("deleteRecentBatchSummary", () => {
  it("clears local draft state and selection for non-persisted summaries", async () => {
    const clearLocalDraft = vi.fn();
    const deleteBatch = vi.fn();
    const removeSelection = vi.fn();

    await deleteRecentBatchSummary({
      clearLocalDraft,
      deleteBatch,
      removeSelection,
      summary: { id: "local-draft", source: "local_draft" },
    });

    expect(clearLocalDraft).toHaveBeenCalled();
    expect(removeSelection).toHaveBeenCalledWith({
      id: "local-draft",
      source: "local_draft",
    });
    expect(deleteBatch).not.toHaveBeenCalled();
  });

  it("deletes persisted summaries through the batch delete runner", async () => {
    const clearLocalDraft = vi.fn();
    const deleteBatch = vi.fn().mockResolvedValue(undefined);
    const removeSelection = vi.fn();

    await deleteRecentBatchSummary({
      clearLocalDraft,
      deleteBatch,
      removeSelection,
      summary: { id: "batch-1", source: "batch" },
    });

    expect(deleteBatch).toHaveBeenCalledWith("batch-1");
    expect(clearLocalDraft).not.toHaveBeenCalled();
    expect(removeSelection).not.toHaveBeenCalled();
  });
});

describe("upsertRecentSavedBatch", () => {
  it("replaces an existing batch and keeps newest batches first", () => {
    const current = [
      buildBatch({
        id: "batch-old",
        name: "Old",
        updatedAt: "2026-06-20T00:00:00.000Z",
      }),
      buildBatch({
        id: "batch-newer",
        name: "Newer",
        updatedAt: "2026-06-22T00:00:00.000Z",
      }),
      buildBatch({
        id: "batch-1",
        name: "Original",
        updatedAt: "2026-06-21T00:00:00.000Z",
      }),
    ];

    const result = upsertRecentSavedBatch(
      current,
      buildBatch({
        id: "batch-1",
        name: "Updated",
        updatedAt: "2026-06-23T00:00:00.000Z",
      }),
    );

    expect(result.map((batch) => batch.id)).toEqual([
      "batch-1",
      "batch-newer",
      "batch-old",
    ]);
    expect(result[0]?.name).toBe("Updated");
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

describe("runRecentBatchBulkStoreUpdate", () => {
  it("does not save or refresh when no selected targets resolve", async () => {
    const resolveBatch = vi.fn().mockResolvedValue(null);
    const saveBatch = vi.fn();
    const refreshSavedBatches = vi.fn();

    await runRecentBatchBulkStoreUpdate({
      batchIds: ["missing"],
      refreshSavedBatches,
      resolveBatch,
      saveBatch,
      storeId: "store-new",
    });

    expect(resolveBatch).toHaveBeenCalledWith("missing");
    expect(saveBatch).not.toHaveBeenCalled();
    expect(refreshSavedBatches).not.toHaveBeenCalled();
  });

  it("saves resolved batches with the new store and refreshes batches", async () => {
    const resolveBatch = vi.fn(async (batchId: string) =>
      buildBatch({ id: batchId, sheinStoreId: "store-old" }),
    );
    const saveBatch = vi.fn().mockResolvedValue(undefined);
    const refreshSavedBatches = vi.fn().mockResolvedValue(undefined);

    await runRecentBatchBulkStoreUpdate({
      batchIds: ["batch-1", "batch-2"],
      refreshSavedBatches,
      resolveBatch,
      saveBatch,
      storeId: "store-new",
    });

    expect(saveBatch).toHaveBeenCalledTimes(2);
    expect(saveBatch).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({ id: "batch-1", sheinStoreId: "store-new" }),
      { makeActive: false },
    );
    expect(saveBatch).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({ id: "batch-2", sheinStoreId: "store-new" }),
      { makeActive: false },
    );
    expect(refreshSavedBatches).toHaveBeenCalled();
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

describe("projectRecentBatchSelectionUpdate", () => {
  it("filters direct selection updates to visible summary keys", () => {
    expect(
      projectRecentBatchSelectionUpdate({
        current: [],
        value: ["batch:batch-1", "batch:missing"],
        validRecentBatchSummaryKeys: new Set(["batch:batch-1"]),
      }),
    ).toEqual(["batch:batch-1"]);
  });

  it("filters functional selection updates to visible summary keys", () => {
    expect(
      projectRecentBatchSelectionUpdate({
        current: ["batch:batch-1"],
        value: (current) => [...current, "local_draft:missing"],
        validRecentBatchSummaryKeys: new Set(["batch:batch-1"]),
      }),
    ).toEqual(["batch:batch-1"]);
  });
});

describe("projectRecentBatchTargetStep", () => {
  it("maps recent batch actions to workbench steps", () => {
    expect(projectRecentBatchTargetStep()).toBe("generate");
    expect(projectRecentBatchTargetStep("generate")).toBe("generate");
    expect(projectRecentBatchTargetStep("review")).toBe("review");
    expect(projectRecentBatchTargetStep("tasks")).toBe("tasks");
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

describe("mergeRecentBatchHydrations", () => {
  it("adds hydrated batch entries without dropping existing cache", () => {
    const existing = buildHydratedBatch(buildBatch({ id: "batch-old" }));
    const next = buildHydratedBatch(buildBatch({ id: "batch-new" }));

    expect(
      mergeRecentBatchHydrations(
        {
          "batch-old": existing,
        },
        [["batch-new", next]],
      ),
    ).toEqual({
      "batch-old": existing,
      "batch-new": next,
    });
  });
});

describe("projectRecentBatchSummaries", () => {
  it("overlays hydrated saved batch summaries over stale saved batches", () => {
    const hydratedBatch = buildHydratedBatch(
      buildBatch({
        id: "batch-1",
        name: "Hydrated batch",
        prompt: "hydrated prompt",
        updatedAt: "2026-06-22T00:00:00.000Z",
      }),
    );

    const summaries = projectRecentBatchSummaries({
      draft: null,
      draftBatchId: undefined,
      savedBatches: [
        buildBatch({
          id: "batch-1",
          name: "Stale batch",
          prompt: "stale prompt",
          updatedAt: "2026-06-21T00:00:00.000Z",
        }),
      ],
      selectedRecentBatchHydrations: {
        "batch-1": hydratedBatch,
      },
    });

    expect(summaries[0]).toMatchObject({
      id: "batch-1",
      promptPreview: "hydrated prompt",
      title: "Hydrated batch",
    });
  });
});

describe("resolveRecentBatchForMutation", () => {
  it("returns a fresh cached hydration without loading remote detail", async () => {
    const savedBatch = buildBatch({
      id: "batch-1",
      updatedAt: "2026-06-20T00:00:00.000Z",
    });
    const cachedHydratedBatch = buildHydratedBatch(
      buildBatch({
        id: "batch-1",
        updatedAt: "2026-06-21T00:00:00.000Z",
      }),
    );
    const loadHydratedBatch = vi.fn();
    const cacheHydratedBatch = vi.fn();

    await expect(
      resolveRecentBatchForMutation({
        batchId: "batch-1",
        savedBatches: [savedBatch],
        selectedRecentBatchHydrations: {
          "batch-1": cachedHydratedBatch,
        },
        loadHydratedBatch,
        cacheHydratedBatch,
      }),
    ).resolves.toBe(cachedHydratedBatch.savedBatch);
    expect(loadHydratedBatch).not.toHaveBeenCalled();
    expect(cacheHydratedBatch).not.toHaveBeenCalled();
  });

  it("loads and caches hydrated batch detail when the cache is stale", async () => {
    const savedBatch = buildBatch({
      id: "batch-1",
      updatedAt: "2026-06-21T00:00:00.000Z",
    });
    const hydratedBatch = buildHydratedBatch(
      buildBatch({
        id: "batch-1",
        updatedAt: "2026-06-22T00:00:00.000Z",
      }),
    );
    const loadHydratedBatch = vi.fn().mockResolvedValue(hydratedBatch);
    const cacheHydratedBatch = vi.fn();

    await expect(
      resolveRecentBatchForMutation({
        batchId: "batch-1",
        savedBatches: [savedBatch],
        selectedRecentBatchHydrations: {},
        loadHydratedBatch,
        cacheHydratedBatch,
      }),
    ).resolves.toBe(hydratedBatch.savedBatch);
    expect(loadHydratedBatch).toHaveBeenCalledWith("batch-1");
    expect(cacheHydratedBatch).toHaveBeenCalledWith("batch-1", hydratedBatch);
  });

  it("falls back to the saved batch when hydration fails", async () => {
    const savedBatch = buildBatch({ id: "batch-1" });

    await expect(
      resolveRecentBatchForMutation({
        batchId: "batch-1",
        savedBatches: [savedBatch],
        selectedRecentBatchHydrations: {},
        loadHydratedBatch: vi.fn().mockRejectedValue(new Error("offline")),
        cacheHydratedBatch: vi.fn(),
      }),
    ).resolves.toBe(savedBatch);
  });

  it("falls back to the saved batch when mutation hydration returns null", async () => {
    const savedBatch = buildBatch({ id: "batch-1" });

    await expect(
      resolveRecentBatchForMutation({
        batchId: "batch-1",
        savedBatches: [savedBatch],
        selectedRecentBatchHydrations: {},
        loadHydratedBatch: vi.fn().mockResolvedValue(null),
        cacheHydratedBatch: vi.fn(),
      }),
    ).resolves.toBe(savedBatch);
  });
});

describe("resolveRecentBatchMutationTargets", () => {
  it("resolves selected batches in order and skips missing targets", async () => {
    const resolveBatch = vi.fn(async (batchId: string) =>
      batchId === "missing" ? null : buildBatch({ id: batchId }),
    );

    await expect(
      resolveRecentBatchMutationTargets({
        batchIds: ["batch-1", "missing", "batch-2"],
        resolveBatch,
      }),
    ).resolves.toEqual([
      expect.objectContaining({ id: "batch-1" }),
      expect.objectContaining({ id: "batch-2" }),
    ]);
    expect(resolveBatch).toHaveBeenCalledTimes(3);
  });
});

describe("resolveRecentBatchSelectionTarget", () => {
  it("returns hydrated batch detail for a persisted recent batch", async () => {
    const savedBatch = buildBatch({ id: "batch-1" });
    const hydratedBatch = buildHydratedBatch(savedBatch);

    await expect(
      resolveRecentBatchSelectionTarget({
        summary: { id: "batch-1", source: "batch" },
        savedBatches: [savedBatch],
        loadHydratedBatch: vi.fn().mockResolvedValue(hydratedBatch),
      }),
    ).resolves.toEqual({
      kind: "hydrated",
      hydratedBatch,
    });
  });

  it("falls back to the saved batch when hydration fails", async () => {
    const savedBatch = buildBatch({ id: "batch-1" });

    await expect(
      resolveRecentBatchSelectionTarget({
        summary: { id: "batch-1", source: "batch" },
        savedBatches: [savedBatch],
        loadHydratedBatch: vi.fn().mockRejectedValue(new Error("offline")),
      }),
    ).resolves.toEqual({
      batch: savedBatch,
      kind: "saved",
    });
  });

  it("falls back to the saved batch when hydration returns null", async () => {
    const savedBatch = buildBatch({ id: "batch-1" });

    await expect(
      resolveRecentBatchSelectionTarget({
        summary: { id: "batch-1", source: "batch" },
        savedBatches: [savedBatch],
        loadHydratedBatch: vi.fn().mockResolvedValue(null),
      }),
    ).resolves.toEqual({
      batch: savedBatch,
      kind: "saved",
    });
  });

  it("returns null when the summary has no saved batch target", async () => {
    await expect(
      resolveRecentBatchSelectionTarget({
        summary: { id: "local-draft:group-1", source: "local_draft" },
        savedBatches: [],
        loadHydratedBatch: vi.fn(),
      }),
    ).resolves.toBeNull();
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

describe("runRecentBatchBulkDelete", () => {
  it("does nothing when no summary ids are selected", async () => {
    const deleteBatch = vi.fn();

    await runRecentBatchBulkDelete([], deleteBatch);

    expect(deleteBatch).not.toHaveBeenCalled();
  });

  it("ignores missing batch delete failures", async () => {
    const deleteBatch = vi
      .fn()
      .mockRejectedValueOnce(new Error("studio session not found"))
      .mockResolvedValueOnce(undefined);

    await expect(
      runRecentBatchBulkDelete(["batch-missing", "batch-2"], deleteBatch),
    ).resolves.toBeUndefined();
    expect(deleteBatch).toHaveBeenCalledTimes(2);
  });

  it("throws the first non-missing delete failure", async () => {
    const failure = new Error("network failed");
    const deleteBatch = vi
      .fn()
      .mockRejectedValueOnce(new Error("studio session not found"))
      .mockRejectedValueOnce(failure);

    await expect(
      runRecentBatchBulkDelete(["batch-missing", "batch-failed"], deleteBatch),
    ).rejects.toBe(failure);
  });
});
