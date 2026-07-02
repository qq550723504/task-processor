import { renderHook } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  projectItemizedBatchDetail,
  projectItemizedDesignApprovalRequest,
  projectItemizedFailedRetryRequest,
  projectItemizedFailedRetryStep,
  projectItemizedReviewNoteUpdate,
  projectItemizedSelectionToggle,
  projectItemizedTaskCreationProgressEffects,
  projectItemizedTaskRecoveryState,
  projectItemizedTaskCreationProgress,
  projectItemizedTaskCreationResult,
  runItemizedDesignApproval,
  runItemizedFailedRetry,
  loadItemizedGenerationPollBatch,
  usePendingItemizedTaskDesignIds,
  useSheinStudioItemizedBatchContext,
} from "@/components/listingkit/shein-studio/shein-studio-task-creation-controller";
import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
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

function buildHydratedBatch(
  savedBatch: SheinStudioSavedBatch = buildCurrentBatch(),
): SheinStudioWorkbenchHydratedBatch {
  return {
    detail: buildCurrentDetail(),
    savedBatch,
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

describe("usePendingItemizedTaskDesignIds", () => {
  it("memoizes approved itemized designs that do not have tasks yet", () => {
    const result = buildTaskCreationResult();
    const detail: SheinStudioBatchDetail = {
      ...buildCurrentDetail(),
      items: [
        {
          ...result.items[0],
          designs: [
            ...result.items[0].designs,
            {
              ...result.items[0].designs[0],
              id: "design-2",
              reviewStatus: "approved",
            },
          ],
        },
      ],
      createdTasks: [{ id: "task-created", title: "Created", designId: "design-1" }],
      reusedTasks: [],
    };

    const { result: hookResult } = renderHook(() =>
      usePendingItemizedTaskDesignIds(detail),
    );

    expect(hookResult.current).toEqual(["design-2"]);
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

describe("projectItemizedReviewNoteUpdate", () => {
  it("updates itemized detail review notes when an active itemized batch exists", () => {
    const detail = {
      ...buildCurrentDetail(),
      items: buildTaskCreationResult().items,
    };

    const update = projectItemizedReviewNoteUpdate({
      activeBatchId: "batch-1",
      designs: [],
      detail,
      designId: "design-1",
      note: "needs crop",
    });

    expect(update).toMatchObject({
      kind: "itemized",
      detail: {
        items: [
          {
            designs: [
              {
                id: "design-1",
                reviewNote: "needs crop",
              },
            ],
          },
        ],
      },
    });
  });

  it("updates flat designs when no active itemized batch exists", () => {
    const update = projectItemizedReviewNoteUpdate({
      activeBatchId: "",
      designs: [{ id: "design-1" }, { id: "design-2" }],
      detail: buildCurrentDetail(),
      designId: "design-1",
      note: "needs crop",
    });

    expect(update).toEqual({
      designs: [
        { id: "design-1", reviewNote: "needs crop" },
        { id: "design-2" },
      ],
      kind: "flat",
    });
  });
});

describe("projectItemizedSelectionToggle", () => {
  it("toggles itemized design approval and returns selected ids for persistence", () => {
    const detail = {
      ...buildCurrentDetail(),
      items: buildTaskCreationResult().items,
    };

    const toggle = projectItemizedSelectionToggle({
      activeBatchId: "batch-1",
      detail,
      designId: "design-1",
      selectedIds: [],
    });

    expect(toggle).toMatchObject({
      kind: "itemized",
      selectedIds: ["design-1"],
      detail: {
        items: [
          {
            designs: [
              {
                id: "design-1",
                reviewStatus: "unreviewed",
              },
            ],
          },
        ],
      },
    });
  });

  it("toggles flat selected ids when no active itemized batch exists", () => {
    expect(
      projectItemizedSelectionToggle({
        activeBatchId: "",
        detail: null,
        designId: "design-1",
        selectedIds: ["design-1", "design-2"],
      }),
    ).toEqual({
      kind: "flat",
      selectedIds: ["design-2"],
    });
  });
});

describe("projectItemizedDesignApprovalRequest", () => {
  it("returns null without an active batch or itemized detail", () => {
    expect(
      projectItemizedDesignApprovalRequest({
        activeBatchId: "",
        currentActiveBatch: buildCurrentBatch(),
        detail: buildCurrentDetail(),
        selectedIds: ["design-1"],
      }),
    ).toBeNull();
    expect(
      projectItemizedDesignApprovalRequest({
        activeBatchId: "batch-1",
        currentActiveBatch: buildCurrentBatch(),
        detail: null,
        selectedIds: ["design-1"],
      }),
    ).toBeNull();
  });

  it("projects an approval request and prefers detail tenant id", () => {
    expect(
      projectItemizedDesignApprovalRequest({
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
        },
        selectedIds: ["design-1"],
      }),
    ).toEqual({
      batchId: "batch-1",
      selectedIds: ["design-1"],
      tenantId: "tenant-detail",
    });
  });

  it("falls back to active batch tenant id when detail tenant is absent", () => {
    expect(
      projectItemizedDesignApprovalRequest({
        activeBatchId: "batch-1",
        currentActiveBatch: {
          ...buildCurrentBatch(),
          tenantId: "tenant-active",
        },
        detail: {
          ...buildCurrentDetail(),
          batch: {
            ...buildCurrentDetail().batch,
            tenantId: undefined,
          },
        },
        selectedIds: ["design-1"],
      }),
    ).toEqual({
      batchId: "batch-1",
      selectedIds: ["design-1"],
      tenantId: "tenant-active",
    });
  });

  it("omits blank detail tenants without falling back", () => {
    expect(
      projectItemizedDesignApprovalRequest({
        activeBatchId: "batch-1",
        currentActiveBatch: {
          ...buildCurrentBatch(),
          tenantId: "tenant-active",
        },
        detail: {
          ...buildCurrentDetail(),
          batch: {
            ...buildCurrentDetail().batch,
            tenantId: " ",
          },
        },
        selectedIds: ["design-1"],
      }),
    ).toEqual({
      batchId: "batch-1",
      selectedIds: ["design-1"],
      tenantId: undefined,
    });
  });
});

describe("runItemizedDesignApproval", () => {
  it("does nothing when the approval request cannot be projected", async () => {
    const approveDesigns = vi.fn();

    await expect(
      runItemizedDesignApproval({
        activeBatchId: "",
        approveDesigns,
        currentActiveBatch: buildCurrentBatch(),
        detail: buildCurrentDetail(),
        selectedIds: ["design-1"],
      }),
    ).resolves.toBeNull();
    expect(approveDesigns).not.toHaveBeenCalled();
  });

  it("approves selected designs without tenant options when no tenant is available", async () => {
    const nextDetail = buildCurrentDetail();
    const approveDesigns = vi.fn().mockResolvedValue(nextDetail);

    await expect(
      runItemizedDesignApproval({
        activeBatchId: "batch-1",
        approveDesigns,
        currentActiveBatch: {
          ...buildCurrentBatch(),
          tenantId: undefined,
        },
        detail: {
          ...buildCurrentDetail(),
          batch: {
            ...buildCurrentDetail().batch,
            tenantId: undefined,
          },
        },
        selectedIds: ["design-1"],
      }),
    ).resolves.toBe(nextDetail);
    expect(approveDesigns).toHaveBeenCalledWith("batch-1", ["design-1"]);
  });

  it("passes tenant options when the approval request has a tenant", async () => {
    const nextDetail = buildCurrentDetail();
    const approveDesigns = vi.fn().mockResolvedValue(nextDetail);

    await expect(
      runItemizedDesignApproval({
        activeBatchId: "batch-1",
        approveDesigns,
        currentActiveBatch: buildCurrentBatch(),
        detail: {
          ...buildCurrentDetail(),
          batch: {
            ...buildCurrentDetail().batch,
            tenantId: "tenant-detail",
          },
        },
        selectedIds: ["design-1"],
      }),
    ).resolves.toBe(nextDetail);
    expect(approveDesigns).toHaveBeenCalledWith("batch-1", ["design-1"], {
      tenantId: "tenant-detail",
    });
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

  it("projects blocked task creation feedback when the batch returns to review ready", () => {
    expect(
      projectItemizedTaskCreationProgress({
        creatingMessage: "creating",
        detail: {
          ...buildCurrentDetail(),
          batch: {
            ...buildCurrentDetail().batch,
            status: "review_ready",
          },
          createdTasks: [],
          reusedTasks: [],
          rejectedTasks: [],
          failedTasks: [
            {
              designId: "design-1",
              message: "SDS credential bootstrap is missing merchant credentials.",
              reasonCode: "baseline_not_ready",
              title: "Style 1",
            },
          ],
        },
        isCreatingTasks: true,
      }),
    ).toEqual({
      completionSignature: "batch-1:review_ready:0:0:0:1",
      creatingMessage: "后台任务创建已结束，但本次没有成功创建任务。",
      creatingWarning:
        "部分任务被拒绝或创建失败：Style 1: baseline_not_ready · SDS credential bootstrap is missing merchant credentials.",
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

  it("projects persisted blocked task creation feedback after a page refresh", () => {
    expect(
      projectItemizedTaskCreationProgress({
        creatingMessage: "",
        detail: {
          ...buildCurrentDetail(),
          batch: {
            ...buildCurrentDetail().batch,
            status: "review_ready",
          },
          createdTasks: [],
          reusedTasks: [],
          rejectedTasks: [],
          failedTasks: [
            {
              designId: "design-1",
              message: "SDS credential bootstrap is missing merchant credentials.",
              reasonCode: "baseline_not_ready",
              title: "Style 1",
            },
          ],
        },
        isCreatingTasks: false,
      }),
    ).toEqual({
      completionSignature: "batch-1:review_ready:0:0:0:1",
      creatingMessage: "后台任务创建已结束，但本次没有成功创建任务。",
      creatingWarning:
        "部分任务被拒绝或创建失败：Style 1: baseline_not_ready · SDS credential bootstrap is missing merchant credentials.",
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

describe("projectItemizedTaskCreationProgressEffects", () => {
  it("does not apply effects for unchanged progress", () => {
    expect(
      projectItemizedTaskCreationProgressEffects({
        currentCompletionSignature: "batch-1:draft:0:0:0:0",
        progress: { kind: "unchanged" },
      }),
    ).toEqual({ kind: "unchanged" });
  });

  it("starts creation and records the completion signature without a toast", () => {
    expect(
      projectItemizedTaskCreationProgressEffects({
        currentCompletionSignature: "",
        progress: {
          completionSignature: "batch-1:tasks_creating:0:0:0:0",
          creatingMessage: "creating",
          isCreatingTasks: true,
          kind: "creating",
        },
      }),
    ).toEqual({
      completionSignature: "batch-1:tasks_creating:0:0:0:0",
      fields: {
        creatingMessage: "creating",
        isCreatingTasks: true,
      },
      kind: "apply",
    });
  });

  it("completes creation and emits a toast only for a new signature", () => {
    const progress = {
      completionSignature: "batch-1:tasks_created:1:0:0:0",
      creatingMessage: "done",
      creatingWarning: "",
      isCreatingTasks: false as const,
      kind: "completed" as const,
      toast: {
        duration: 7000,
        message: "created",
        title: "done",
        type: "success" as const,
      },
    };

    expect(
      projectItemizedTaskCreationProgressEffects({
        currentCompletionSignature: "",
        progress,
      }),
    ).toEqual({
      completionSignature: "batch-1:tasks_created:1:0:0:0",
      fields: {
        creatingMessage: "done",
        creatingWarning: "",
        isCreatingTasks: false,
      },
      kind: "apply",
      toast: progress.toast,
    });
    expect(
      projectItemizedTaskCreationProgressEffects({
        currentCompletionSignature: "batch-1:tasks_created:1:0:0:0",
        progress,
      }),
    ).toEqual({
      fields: {
        creatingMessage: "done",
        creatingWarning: "",
        isCreatingTasks: false,
      },
      kind: "apply",
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

describe("runItemizedFailedRetry", () => {
  it("does nothing when the retry request cannot be projected", async () => {
    const retryItems = vi.fn();

    await expect(
      runItemizedFailedRetry({
        activeBatchId: "",
        currentActiveBatch: buildCurrentBatch(),
        detail: buildCurrentDetail(),
        itemId: "item-1",
        retryItems,
      }),
    ).resolves.toBeNull();
    expect(retryItems).not.toHaveBeenCalled();
  });

  it("retries the failed item without tenant options when no tenant is available", async () => {
    const detail = {
      ...buildCurrentDetail(),
      batch: {
        ...buildCurrentDetail().batch,
        tenantId: undefined,
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            status: "failed" as const,
            targetGroupKey: "group-1",
            selectionCount: 1,
            createdAt: "2026-06-22T00:00:00.000Z",
            updatedAt: "2026-06-22T00:00:00.000Z",
          },
          designs: [],
        },
      ],
    };
    const retryItems = vi.fn().mockResolvedValue(detail);

    await expect(
      runItemizedFailedRetry({
        activeBatchId: "batch-1",
        currentActiveBatch: { ...buildCurrentBatch(), tenantId: undefined },
        detail,
        itemId: "item-1",
        retryItems,
      }),
    ).resolves.toBe(detail);
    expect(retryItems).toHaveBeenCalledWith("batch-1", ["item-1"]);
  });

  it("passes tenant options when retrying a failed item with tenant context", async () => {
    const detail = {
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
            status: "failed" as const,
            targetGroupKey: "group-1",
            selectionCount: 1,
            createdAt: "2026-06-22T00:00:00.000Z",
            updatedAt: "2026-06-22T00:00:00.000Z",
          },
          designs: [],
        },
      ],
    };
    const retryItems = vi.fn().mockResolvedValue(detail);

    await expect(
      runItemizedFailedRetry({
        activeBatchId: "batch-1",
        currentActiveBatch: buildCurrentBatch(),
        detail,
        itemId: "item-1",
        retryItems,
      }),
    ).resolves.toBe(detail);
    expect(retryItems).toHaveBeenCalledWith("batch-1", ["item-1"], {
      tenantId: "tenant-detail",
    });
  });
});

describe("loadItemizedGenerationPollBatch", () => {
  it("loads the hydrated batch for the active batch id", async () => {
    const hydratedBatch = buildHydratedBatch();
    const getHydratedBatch = vi.fn().mockResolvedValue(hydratedBatch);

    await expect(
      loadItemizedGenerationPollBatch({
        activeBatchId: "batch-1",
        getHydratedBatch,
      }),
    ).resolves.toBe(hydratedBatch);
    expect(getHydratedBatch).toHaveBeenCalledWith("batch-1");
  });

  it("returns null when polling the hydrated batch fails", async () => {
    await expect(
      loadItemizedGenerationPollBatch({
        activeBatchId: "batch-1",
        getHydratedBatch: vi.fn().mockRejectedValue(new Error("offline")),
      }),
    ).resolves.toBeNull();
  });
});

describe("projectItemizedFailedRetryStep", () => {
  it("returns generate while the retry result is still generating", () => {
    expect(
      projectItemizedFailedRetryStep({
        ...buildCurrentDetail(),
        batch: {
          ...buildCurrentDetail().batch,
          status: "generating",
        },
      }),
    ).toBe("generate");
  });

  it("returns generate while any retry item is still in flight", () => {
    expect(
      projectItemizedFailedRetryStep({
        ...buildCurrentDetail(),
        items: [
          {
            item: {
              id: "item-1",
              batchId: "batch-1",
              status: "awaiting_materialization",
              targetGroupKey: "group-1",
              selectionCount: 1,
              createdAt: "2026-06-22T00:00:00.000Z",
              updatedAt: "2026-06-22T00:00:00.000Z",
            },
            designs: [],
          },
        ],
      }),
    ).toBe("generate");
  });

  it("does not switch steps after retry settles outside generation", () => {
    expect(
      projectItemizedFailedRetryStep({
        ...buildCurrentDetail(),
        batch: {
          ...buildCurrentDetail().batch,
          status: "partially_failed",
        },
      }),
    ).toBeNull();
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
        hotStyleReferenceImageUrls: ["https://example.com/ref.png"],
        hotStyleReferenceBrief: "current hot style brief",
        hotStyleReferencePrompt: "current extracted prompt",
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
      savedBatch: expect.objectContaining({
        id: "batch-1",
        hotStyleReferenceImageUrls: ["https://example.com/ref.png"],
        hotStyleReferenceBrief: "current hot style brief",
        hotStyleReferencePrompt: "current extracted prompt",
      }),
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
