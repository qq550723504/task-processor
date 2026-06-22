import { describe, expect, it } from "vitest";

import {
  createBatchQueueController,
  buildBatchQueueCompletionMessage,
  buildBatchQueueResumeState,
  getBatchQueueStartState,
  resolveNextQueuedBatch,
  resolveQueuedBatchStep,
} from "@/lib/shein-studio/batch-queue";
import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import type { SheinStudioSavedBatch } from "@/lib/types/shein-studio";

function buildBatch(
  overrides: Partial<SheinStudioSavedBatch> = {},
): SheinStudioSavedBatch {
  return {
    id: "batch-1",
    name: "Batch 1",
    prompt: "prompt",
    styleCount: "1",
    sheinStoreId: "869",
    designs: [],
    selectedIds: [],
    createdTasks: [],
    updatedAt: "2026-06-22T00:00:00.000Z",
    ...overrides,
  };
}

function buildHydratedBatch(
  batch: SheinStudioSavedBatch,
): SheinStudioWorkbenchHydratedBatch {
  return {
    savedBatch: batch,
    detail: {
      batch: {
        id: batch.id,
        status: "draft",
        prompt: batch.prompt,
        styleCount: batch.styleCount,
        sheinStoreId: Number(batch.sheinStoreId || "0"),
        createdAt: "2026-06-22T00:00:00.000Z",
        updatedAt: batch.updatedAt,
      },
      items: [],
    },
  };
}

function createControllerHarness({
  hydratedBatches = {},
  savedBatches = [
    buildBatch({ id: "batch-1" }),
    buildBatch({ id: "batch-2", name: "Batch 2" }),
  ],
}: {
  hydratedBatches?: Record<string, SheinStudioWorkbenchHydratedBatch>;
  savedBatches?: SheinStudioSavedBatch[];
} = {}) {
  const requestVersionRef = { current: 0 };
  const recentOpenVersionRef = { current: 0 };
  const calls: string[] = [];
  const state = {
    batchQueueMode: null as "generate" | "create_tasks" | null,
    queuedBatchIds: [] as string[],
    queuedBatchIndex: 0,
    queueResumeState: null as ReturnType<typeof buildBatchQueueResumeState>,
    queueMessage: "",
    effectiveStep: "",
  };
  const controller = createBatchQueueController({
    getSavedBatches: () => savedBatches,
    getSelectedHydrations: () => hydratedBatches,
    hydrateBatchSelection: async (batchIds) => {
      calls.push(`hydrate:${batchIds.join(",")}`);
      return Object.fromEntries(
        batchIds.flatMap((batchId) =>
          hydratedBatches[batchId] ? [[batchId, hydratedBatches[batchId]]] : [],
        ),
      );
    },
    loadBatch: (batch) => {
      calls.push(`load:${batch.id}`);
    },
    loadHydratedBatch: (batch) => {
      calls.push(`load-hydrated:${batch.savedBatch.id}`);
    },
    requestVersionRef,
    recentOpenVersionRef,
    setBatchQueueMode: (value) => {
      state.batchQueueMode = value;
    },
    setEffectiveStep: (value) => {
      state.effectiveStep = value;
    },
    setQueueMessage: (value) => {
      state.queueMessage = value;
    },
    setQueueResumeState: (value) => {
      state.queueResumeState = value;
    },
    setQueuedBatchIds: (value) => {
      state.queuedBatchIds = value;
    },
    setQueuedBatchIndex: (value) => {
      state.queuedBatchIndex = value;
    },
  });
  return {
    calls,
    controller,
    recentOpenVersionRef,
    requestVersionRef,
    state,
  };
}

describe("SHEIN Studio batch queue", () => {
  it("filters stale ids and clamps the requested start index", () => {
    expect(
      getBatchQueueStartState({
        batchIds: ["missing", "batch-1", "batch-2"],
        savedBatches: [
          buildBatch({ id: "batch-1" }),
          buildBatch({ id: "batch-2" }),
        ],
        startIndex: 5,
      }),
    ).toEqual({
      batchIds: ["batch-1", "batch-2"],
      startIndex: 1,
    });
  });

  it("reports an empty queue when no requested ids are still saved", () => {
    expect(
      getBatchQueueStartState({
        batchIds: ["missing"],
        savedBatches: [buildBatch({ id: "batch-1" })],
      }),
    ).toEqual({
      batchIds: [],
      startIndex: 0,
    });
  });

  it("resolves the next loadable queued batch and skips missing entries", () => {
    expect(
      resolveNextQueuedBatch({
        batchIds: ["missing", "batch-2"],
        savedBatches: [buildBatch({ id: "batch-2", name: "Batch 2" })],
        startIndex: 0,
      }),
    ).toEqual({
      batch: expect.objectContaining({ id: "batch-2" }),
      batchId: "batch-2",
      index: 1,
    });
  });

  it("returns null when no queued batch can be loaded", () => {
    expect(
      resolveNextQueuedBatch({
        batchIds: ["missing"],
        savedBatches: [buildBatch({ id: "batch-1" })],
        startIndex: 0,
      }),
    ).toBeNull();
  });

  it("routes queued batches to the right workbench step", () => {
    expect(
      resolveQueuedBatchStep(buildBatch({ createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }] }), "generate"),
    ).toBe("tasks");
    expect(
      resolveQueuedBatchStep(buildBatch({ designs: [{ id: "design-1" }] }), "create_tasks"),
    ).toBe("review");
    expect(resolveQueuedBatchStep(buildBatch(), "generate")).toBe("generate");
    expect(resolveQueuedBatchStep(buildBatch(), "create_tasks")).toBe("generate");
  });

  it("builds resume state only for active queues with queued ids", () => {
    expect(
      buildBatchQueueResumeState({
        batchIds: ["batch-1", "batch-2"],
        mode: "generate",
        startIndex: 1,
      }),
    ).toEqual({
      batchIds: ["batch-1", "batch-2"],
      mode: "generate",
      startIndex: 1,
      total: 2,
    });
    expect(
      buildBatchQueueResumeState({
        batchIds: [],
        mode: "generate",
        startIndex: 0,
      }),
    ).toBeNull();
    expect(
      buildBatchQueueResumeState({
        batchIds: ["batch-1"],
        mode: null,
        startIndex: 0,
      }),
    ).toBeNull();
  });

  it("builds queue completion messages for each mode", () => {
    expect(buildBatchQueueCompletionMessage("generate", 2)).toContain(
      "继续生成处理",
    );
    expect(buildBatchQueueCompletionMessage("create_tasks", 1)).toContain(
      "创建任务处理",
    );
    expect(buildBatchQueueCompletionMessage("generate", 0)).toContain(
      "当前没有可继续的已保存批次",
    );
  });

  it("starts a queue by filtering saved ids, hydrating, and loading the first valid batch", async () => {
    const batch = buildBatch({
      id: "batch-2",
      designs: [{ id: "design-1" }],
    });
    const harness = createControllerHarness({
      hydratedBatches: { "batch-2": buildHydratedBatch(batch) },
      savedBatches: [buildBatch({ id: "batch-1" }), batch],
    });

    await harness.controller.start({
      batchIds: ["missing", "batch-2"],
      mode: "create_tasks",
    });

    expect(harness.state.batchQueueMode).toBe("create_tasks");
    expect(harness.state.queuedBatchIds).toEqual(["batch-2"]);
    expect(harness.state.queuedBatchIndex).toBe(0);
    expect(harness.state.effectiveStep).toBe("review");
    expect(harness.calls).toEqual(["hydrate:batch-2", "load-hydrated:batch-2"]);
    expect(harness.requestVersionRef.current).toBe(1);
    expect(harness.recentOpenVersionRef.current).toBe(1);
  });

  it("completes and clears the queue when no valid batch ids remain", async () => {
    const harness = createControllerHarness({
      savedBatches: [buildBatch({ id: "batch-1" })],
    });

    await harness.controller.start({
      batchIds: ["missing"],
      mode: "generate",
    });

    expect(harness.state.batchQueueMode).toBeNull();
    expect(harness.state.queuedBatchIds).toEqual([]);
    expect(harness.state.queuedBatchIndex).toBe(0);
    expect(harness.state.queueResumeState).toBeNull();
    expect(harness.state.queueMessage).toContain("当前没有可继续的已保存批次");
  });

  it("advances from the current queued batch to the next loadable batch", async () => {
    const harness = createControllerHarness();
    harness.requestVersionRef.current = 3;

    await harness.controller.advance({
      batchIds: ["batch-1", "batch-2"],
      currentIndex: 0,
      mode: "generate",
    });

    expect(harness.calls).toEqual(["hydrate:batch-2", "load:batch-2"]);
    expect(harness.state.queuedBatchIndex).toBe(1);
    expect(harness.state.effectiveStep).toBe("generate");
  });

  it("stores resume state on exit and invalidates stale queued loads", () => {
    const harness = createControllerHarness();
    harness.requestVersionRef.current = 7;

    harness.controller.exit({
      batchIds: ["batch-1", "batch-2"],
      currentIndex: 1,
      mode: "generate",
    });

    expect(harness.state.queueResumeState).toEqual({
      batchIds: ["batch-1", "batch-2"],
      mode: "generate",
      startIndex: 1,
      total: 2,
    });
    expect(harness.state.batchQueueMode).toBeNull();
    expect(harness.state.queuedBatchIds).toEqual([]);
    expect(harness.requestVersionRef.current).toBe(8);
  });

  it("resumes from the stored queue state", async () => {
    const harness = createControllerHarness();

    await harness.controller.resume({
      batchIds: ["batch-1", "batch-2"],
      mode: "generate",
      startIndex: 1,
      total: 2,
    });

    expect(harness.calls).toEqual([
      "hydrate:batch-1,batch-2",
      "hydrate:batch-2",
      "load:batch-2",
    ]);
    expect(harness.state.queuedBatchIndex).toBe(1);
  });
});
