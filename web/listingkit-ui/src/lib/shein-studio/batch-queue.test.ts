import { describe, expect, it } from "vitest";

import {
  buildBatchQueueCompletionMessage,
  buildBatchQueueResumeState,
  getBatchQueueStartState,
  resolveNextQueuedBatch,
  resolveQueuedBatchStep,
} from "@/lib/shein-studio/batch-queue";
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
});
