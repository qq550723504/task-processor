import { describe, expect, it } from "vitest";

import {
  projectItemizedBatchDetail,
  projectItemizedTaskCreationResult,
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
