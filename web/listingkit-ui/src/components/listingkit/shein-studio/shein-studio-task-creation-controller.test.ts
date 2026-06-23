import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  projectItemizedBatchDetail,
  projectItemizedFailedRetryRequest,
  projectItemizedTaskRecoveryState,
  projectItemizedTaskCreationProgress,
  projectItemizedTaskCreationResult,
  useSheinStudioItemizedBatchContext,
} from "@/components/listingkit/shein-studio/shein-studio-task-creation-controller";
import type { SheinStudioBatchTaskCreationResult } from "@/lib/api/shein-studio-batches";
import type { SheinStudioBatchDetail, SheinStudioSavedBatch } from "@/lib/types/shein-studio";

function buildCurrentBatch(): SheinStudioSavedBatch {
  return {
    id: "batch-1",
    tenantId: "tenant-old",
    name: "Existing batch",
    prompt: "old prompt",
    styleCount: "1",
    sheinStoreId: "869",
    designs: [{ id: "old-design" }],
    selectedIds: ["old-design"],
    createdTasks: [],
    generationJobs: [{ jobId: "job-1", status: "succeeded" }],
    draftUpdatedAt: "2026-06-22T01:00:00.000Z",
    updatedAt: "2026-06-22T02:00:00.000Z",
  };
}

function buildCurrentDetail(): SheinStudioBatchDetail {
  return {
    batch: {
      id: "batch-1",
      tenantId: "tenant-detail",
      status: "draft",
      prompt: "old prompt",
      styleCount: "1",
      sheinStoreId: 869,
      createdAt: "2026-06-22T00:00:00.000Z",
      updatedAt: "2026-06-22T02:00:00.000Z",
    },
    items: [],
  };
}

function buildTaskCreationResult(): SheinStudioBatchTaskCreationResult {
  return {
    batch: {
      id: "batch-1",
      tenantId: "tenant-new",
      status: "draft",
      prompt: "prompt",
      styleCount: "2",
      sheinStoreId: 870,
      createdAt: "2026-06-22T00:00:00.000Z",
      updatedAt: "2026-06-22T03:00:00.000Z",
    },
    items: [
      {
        item: {
          id: "item-1",
          batchId: "batch-1",
          targetGroupKey: "group-1",
          targetGroupLabel: "Group 1",
          status: "review_ready",
          selectionCount: 1,
          createdAt: "2026-06-22T02:00:00.000Z",
          updatedAt: "2026-06-22T02:30:00.000Z",
        },
        designs: [
          {
            id: "design-1",
            batchId: "batch-1",
            itemId: "item-1",
            imageUrl: "https://example.test/design-1.png",
            sourceAttemptId: "attempt-1",
            targetGroupKey: "group-1",
            targetGroupLabel: "Group 1",
            reviewStatus: "approved",
            createdAt: "2026-06-22T02:30:00.000Z",
            updatedAt: "2026-06-22T02:45:00.000Z",
          },
        ],
      },
    ],
    createdTasks: [{ id: "task-created", title: "Created", designId: "design-1" }],
    reusedTasks: [{ id: "task-reused", title: "Reused", designId: "design-2" }],
    rejectedTasks: [],
    failedTasks: [],
  };
}

describe("projectItemizedTaskCreationResult", () => {
  it("projects a task creation response into hydrated batch state", () => {
    const result = projectItemizedTaskCreationResult({
      activeBatchId: "batch-1",
      activeSelection: {
        layerId: "layer-1",
        parentProductId: 1,
        productId: 10,
        productName: "Tee",
        prototypeGroupId: 20,
        variantId: 100,
        variantLabel: "Black / M",
      },
      artworkModel: "openai",
      currentActiveBatch: buildCurrentBatch(),
      currentDetail: buildCurrentDetail(),
      generationJobs: [{ jobId: "job-stale", status: "running" }],
      groupedImageMode: "shared_by_size",
      groupedSelections: [],
      groups: [],
      imageStrategy: "ai_generated",
      persistedUpdatedAt: "2026-06-22T00:30:00.000Z",
      productImageCount: "2",
      productImagePrompt: "product prompt",
      productImagePrompts: [],
      prompt: "new prompt",
      renderSizeImagesWithSds: true,
      result: buildTaskCreationResult(),
      selectedSdsImages: [],
      sheinStoreId: "870",
      styleCount: "2",
      transparentBackground: true,
      variationIntensity: "medium",
    });

    expect(result.detail).toMatchObject({
      batch: { id: "batch-1", tenantId: "tenant-new" },
      createdTasks: [{ id: "task-created" }],
      reusedTasks: [{ id: "task-reused" }],
    });
    expect(result.savedBatch).toMatchObject({
      id: "batch-1",
      tenantId: "tenant-new",
      name: "Existing batch",
      prompt: "new prompt",
      styleCount: "2",
      selectedIds: ["design-1"],
      createdTasks: [
        { id: "task-created" },
        { id: "task-reused" },
      ],
      generationJobs: [],
      draftUpdatedAt: "2026-06-22T01:00:00.000Z",
      updatedAt: "2026-06-22T02:00:00.000Z",
    });
  });
});

describe("projectItemizedBatchDetail", () => {
  it("projects a refreshed itemized detail while preserving requested tasks and generation jobs", () => {
    const result = projectItemizedBatchDetail({
      activeBatchId: "batch-1",
      activeSelection: {
        layerId: "layer-1",
        parentProductId: 1,
        productId: 10,
        productName: "Tee",
        prototypeGroupId: 20,
        variantId: 100,
        variantLabel: "Black / M",
      },
      artworkModel: "openai",
      createdTasks: [{ id: "task-existing", title: "Existing", designId: "design-1" }],
      currentActiveBatch: {
        ...buildCurrentBatch(),
        updatedAt: "",
      },
      detail: {
        ...buildCurrentDetail(),
        batch: {
          ...buildCurrentDetail().batch,
          tenantId: "tenant-detail-new",
          updatedAt: "2026-06-22T04:00:00.000Z",
        },
        items: buildTaskCreationResult().items,
      },
      generationJobs: [{ jobId: "job-running", status: "running" }],
      groupedImageMode: "shared_by_size",
      groupedSelections: [],
      groups: [],
      imageStrategy: "ai_generated",
      persistedUpdatedAt: "2026-06-22T00:30:00.000Z",
      productImageCount: "2",
      productImagePrompt: "product prompt",
      productImagePrompts: [],
      prompt: "new prompt",
      renderSizeImagesWithSds: true,
      selectedSdsImages: [],
      sheinStoreId: "870",
      styleCount: "2",
      transparentBackground: true,
      variationIntensity: "medium",
    });

    expect(result.savedBatch).toMatchObject({
      id: "batch-1",
      name: "Existing batch",
      selectedIds: ["design-1"],
      createdTasks: [{ id: "task-existing" }],
      generationJobs: [{ jobId: "job-running" }],
      updatedAt: "2026-06-22T04:00:00.000Z",
    });
    expect(result.detail.batch.tenantId).toBe("tenant-detail-new");
  });
});

describe("projectItemizedTaskCreationProgress", () => {
  it("projects in-flight task creation state", () => {
    expect(
      projectItemizedTaskCreationProgress({
        creatingMessage: "",
        detail: {
          ...buildCurrentDetail(),
          batch: {
            ...buildCurrentDetail().batch,
            status: "tasks_creating",
          },
        },
        isCreatingTasks: false,
      }),
    ).toEqual({
      completionSignature: "batch-1:tasks_creating:0:0:0:0",
      creatingMessage: "已开始在后台创建 SHEIN 资料，可离开当前页面，结果会自动刷新。",
      isCreatingTasks: true,
      kind: "creating",
    });
  });

  it("projects successful task creation completion feedback", () => {
    expect(
      projectItemizedTaskCreationProgress({
        creatingMessage: "creating",
        detail: {
          ...buildCurrentDetail(),
          batch: {
            ...buildCurrentDetail().batch,
            status: "tasks_created",
          },
          createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
          reusedTasks: [{ id: "task-2", title: "Task 2", designId: "design-2" }],
          failedTasks: [],
          rejectedTasks: [],
        },
        isCreatingTasks: true,
      }),
    ).toEqual({
      completionSignature: "batch-1:tasks_created:1:1:0:0",
      creatingMessage: "后台已完成创建，共生成或复用 2 个 SHEIN 任务。",
      creatingWarning: "",
      isCreatingTasks: false,
      kind: "completed",
      toast: {
        duration: 7000,
        message: "共生成或复用 2 个任务。",
        title: "SHEIN 资料创建完成",
        type: "success",
      },
    });
  });

  it("projects blocked task creation completion feedback", () => {
    expect(
      projectItemizedTaskCreationProgress({
        creatingMessage: "creating",
        detail: {
          ...buildCurrentDetail(),
          batch: {
            ...buildCurrentDetail().batch,
            status: "tasks_created",
          },
          createdTasks: [],
          reusedTasks: [],
          rejectedTasks: [
            {
              designId: "design-1",
              message: "bad image",
              reasonCode: "image_invalid",
              title: "Rejected",
            },
          ],
          failedTasks: [
            {
              designId: "design-2",
              message: "network",
              reasonCode: "timeout",
              title: "Failed",
            },
          ],
        },
        isCreatingTasks: true,
      }),
    ).toEqual({
      completionSignature: "batch-1:tasks_created:0:0:1:1",
      creatingMessage: "后台任务创建已结束，但本次没有成功创建任务。",
      creatingWarning:
        "部分任务被拒绝或创建失败：Rejected: image_invalid · bad image；Failed: timeout · network",
      isCreatingTasks: false,
      kind: "completed",
      toast: {
        duration: 8000,
        message: "本次没有成功创建任务。",
        title: "SHEIN 资料创建失败",
        type: "error",
      },
    });
  });
});

describe("projectItemizedTaskRecoveryState", () => {
  it("prioritizes retrying failed items for failed itemized batches", () => {
    expect(
      projectItemizedTaskRecoveryState({
        detail: {
          ...buildCurrentDetail(),
          batch: {
            ...buildCurrentDetail().batch,
            status: "partially_failed",
          },
          items: [
            {
              item: {
                id: "item-failed-1",
                batchId: "batch-1",
                status: "failed",
                targetGroupKey: "group-1",
                selectionCount: 1,
                createdAt: "2026-06-22T00:00:00.000Z",
                updatedAt: "2026-06-22T00:00:00.000Z",
              },
              designs: [],
            },
            {
              item: {
                id: "item-failed-2",
                batchId: "batch-1",
                status: "failed",
                targetGroupKey: "group-2",
                selectionCount: 1,
                createdAt: "2026-06-22T00:00:00.000Z",
                updatedAt: "2026-06-22T00:00:00.000Z",
              },
              designs: [],
            },
            {
              item: {
                id: "item-ready",
                batchId: "batch-1",
                status: "review_ready",
                targetGroupKey: "group-3",
                selectionCount: 1,
                createdAt: "2026-06-22T00:00:00.000Z",
                updatedAt: "2026-06-22T00:00:00.000Z",
              },
              designs: [],
            },
          ],
        },
        generationInFlight: false,
        pendingTaskDesignIds: ["design-pending"],
      }),
    ).toEqual({
      dedicatedGenerateButtonLabel: "重试失败款式 2 个",
      hasRetryableFailedItems: true,
      retryableFailedItemCount: 2,
      shouldPrioritizeTaskCreationRecovery: false,
    });
  });

  it("prioritizes continuing pending task creation when no failed items are retryable", () => {
    expect(
      projectItemizedTaskRecoveryState({
        detail: buildCurrentDetail(),
        generationInFlight: false,
        pendingTaskDesignIds: ["design-pending"],
      }),
    ).toEqual({
      dedicatedGenerateButtonLabel: "继续生成剩余款式",
      hasRetryableFailedItems: false,
      retryableFailedItemCount: 0,
      shouldPrioritizeTaskCreationRecovery: true,
    });
  });

  it("does not prioritize task recovery while itemized generation is in flight", () => {
    expect(
      projectItemizedTaskRecoveryState({
        detail: buildCurrentDetail(),
        generationInFlight: true,
        pendingTaskDesignIds: ["design-pending"],
      }).shouldPrioritizeTaskCreationRecovery,
    ).toBe(false);
  });
});

describe("projectItemizedFailedRetryRequest", () => {
  it("returns null without an active batch or detail", () => {
    expect(
      projectItemizedFailedRetryRequest({
        activeBatchId: "",
        currentActiveBatch: buildCurrentBatch(),
        detail: buildCurrentDetail(),
        itemId: "item-1",
      }),
    ).toBeNull();
    expect(
      projectItemizedFailedRetryRequest({
        activeBatchId: "batch-1",
        currentActiveBatch: buildCurrentBatch(),
        detail: null,
        itemId: "item-1",
      }),
    ).toBeNull();
  });

  it("returns null when the item is not failed", () => {
    expect(
      projectItemizedFailedRetryRequest({
        activeBatchId: "batch-1",
        currentActiveBatch: buildCurrentBatch(),
        detail: buildCurrentDetail(),
        itemId: "item-1",
      }),
    ).toBeNull();
  });

  it("projects retry request for a failed item and prefers detail tenant id", () => {
    expect(
      projectItemizedFailedRetryRequest({
        activeBatchId: "batch-1",
        currentActiveBatch: {
          ...buildCurrentBatch(),
          tenantId: "tenant-active",
        },
        detail: {
          ...buildCurrentDetail(),
          batch: {
            ...buildCurrentDetail().batch,
            tenantId: "tenant-detail",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                status: "failed",
                targetGroupKey: "group-1",
                targetGroupLabel: "Group 1",
                selectionCount: 1,
                createdAt: "2026-06-22T00:00:00.000Z",
                updatedAt: "2026-06-22T00:00:00.000Z",
              },
              designs: [],
            },
          ],
        },
        itemId: "item-1",
      }),
    ).toEqual({
      batchId: "batch-1",
      itemIds: ["item-1"],
      tenantId: "tenant-detail",
    });
  });
});

describe("useSheinStudioItemizedBatchContext", () => {
  it("returns undefined until both active batch id and detail are available", () => {
    const { result } = renderHook(() =>
      useSheinStudioItemizedBatchContext({
        activeBatchId: "",
        activeSelection: undefined,
        artworkModel: "openai",
        currentActiveBatch: buildCurrentBatch(),
        generationJobs: [],
        groupedImageMode: "shared_by_size",
        groupedSelections: [],
        groups: [],
        imageStrategy: "ai_generated",
        itemizedBatchDetail: buildCurrentDetail(),
        persistedUpdatedAt: "",
        productImageCount: "1",
        productImagePrompt: "",
        productImagePrompts: [],
        prompt: "prompt",
        renderSizeImagesWithSds: false,
        selectedSdsImages: [],
        setSavedBatches: vi.fn(),
        sheinStoreId: "869",
        styleCount: "1",
        transparentBackground: false,
        upsertSavedBatch: (current, savedBatch) => [savedBatch, ...current],
        variationIntensity: "medium",
        applyHydratedBatch: vi.fn(),
      }),
    );

    expect(result.current.itemizedBatchContext).toBeUndefined();
  });

  it("builds itemized task context and applies created results", () => {
    const setSavedBatches = vi.fn();
    const applyHydratedBatch = vi.fn();
    const { result } = renderHook(() =>
      useSheinStudioItemizedBatchContext({
        activeBatchId: "batch-1",
        activeSelection: undefined,
        artworkModel: "openai",
        currentActiveBatch: buildCurrentBatch(),
        generationJobs: [],
        groupedImageMode: "shared_by_size",
        groupedSelections: [],
        groups: [],
        imageStrategy: "ai_generated",
        itemizedBatchDetail: buildCurrentDetail(),
        persistedUpdatedAt: "2026-06-22T00:30:00.000Z",
        productImageCount: "1",
        productImagePrompt: "",
        productImagePrompts: [],
        prompt: "prompt",
        renderSizeImagesWithSds: false,
        selectedSdsImages: [],
        setSavedBatches,
        sheinStoreId: "869",
        styleCount: "1",
        transparentBackground: false,
        upsertSavedBatch: (current, savedBatch) => [savedBatch, ...current],
        variationIntensity: "medium",
        applyHydratedBatch,
      }),
    );

    expect(result.current.itemizedBatchContext).toMatchObject({
      batchId: "batch-1",
      tenantId: "tenant-detail",
      detail: buildCurrentDetail(),
    });

    result.current.itemizedBatchContext?.onCreated(buildTaskCreationResult());

    expect(setSavedBatches).toHaveBeenCalledWith(expect.any(Function));
    const updater = setSavedBatches.mock.calls[0][0] as (
      current: SheinStudioSavedBatch[],
    ) => SheinStudioSavedBatch[];
    expect(updater([])[0]).toMatchObject({
      id: "batch-1",
      createdTasks: [
        { id: "task-created" },
        { id: "task-reused" },
      ],
    });
    expect(applyHydratedBatch).toHaveBeenCalledWith({
      detail: expect.objectContaining({
        createdTasks: [{ id: "task-created", title: "Created", designId: "design-1" }],
      }),
      savedBatch: expect.objectContaining({ id: "batch-1" }),
    });
  });

  it("uses the shared recent batch upsert behavior by default", () => {
    const setSavedBatches = vi.fn();
    const { result } = renderHook(() =>
      useSheinStudioItemizedBatchContext({
        activeBatchId: "batch-1",
        activeSelection: undefined,
        artworkModel: "openai",
        currentActiveBatch: buildCurrentBatch(),
        generationJobs: [],
        groupedImageMode: "shared_by_size",
        groupedSelections: [],
        groups: [],
        imageStrategy: "ai_generated",
        itemizedBatchDetail: buildCurrentDetail(),
        persistedUpdatedAt: "2026-06-22T00:30:00.000Z",
        productImageCount: "1",
        productImagePrompt: "",
        productImagePrompts: [],
        prompt: "prompt",
        renderSizeImagesWithSds: false,
        selectedSdsImages: [],
        setSavedBatches,
        sheinStoreId: "869",
        styleCount: "1",
        transparentBackground: false,
        variationIntensity: "medium",
        applyHydratedBatch: vi.fn(),
      }),
    );

    result.current.itemizedBatchContext?.onCreated(buildTaskCreationResult());

    const updater = setSavedBatches.mock.calls[0][0] as (
      current: SheinStudioSavedBatch[],
    ) => SheinStudioSavedBatch[];
    expect(
      updater([
        {
          ...buildCurrentBatch(),
          id: "newer-batch",
          updatedAt: "2026-06-23T00:00:00.000Z",
        },
      ]).map((batch) => batch.id),
    ).toEqual(["newer-batch", "batch-1"]);
  });
});
