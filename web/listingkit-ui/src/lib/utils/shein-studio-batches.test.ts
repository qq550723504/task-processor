import { beforeEach, describe, expect, it, vi } from "vitest";

import { ApiError } from "@/lib/api/client";
import {
  getSheinStudioBatch,
  getSheinStudioHydratedBatch,
  listSheinStudioBatches,
  saveSheinStudioBatch,
} from "@/lib/utils/shein-studio-batches";

const buildStudioBatchDraftSelectionKey = vi.fn();
const listSheinStudioBatchDrafts = vi.fn();
const upsertSheinStudioBatchDraft = vi.fn();
const deleteSheinStudioBatchDraft = vi.fn();
const getSheinStudioBatchDetail = vi.fn();

const selection = {
  productId: 1,
  parentProductId: 1,
  variantId: 100,
  prototypeGroupId: 200,
  layerId: "layer-1",
  productName: "tee",
  variantLabel: "M / black",
};

vi.mock("@/lib/api/shein-studio-batch-drafts", () => ({
  buildStudioBatchDraftSelectionKey: (...args: unknown[]) =>
    buildStudioBatchDraftSelectionKey(...args),
  listSheinStudioBatchDrafts: (...args: unknown[]) => listSheinStudioBatchDrafts(...args),
  upsertSheinStudioBatchDraft: (...args: unknown[]) => upsertSheinStudioBatchDraft(...args),
  deleteSheinStudioBatchDraft: (...args: unknown[]) => deleteSheinStudioBatchDraft(...args),
}));

vi.mock("@/lib/api/shein-studio-batches", () => ({
  getSheinStudioBatchDetail: (...args: unknown[]) => getSheinStudioBatchDetail(...args),
}));

describe("shein studio storage api", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    buildStudioBatchDraftSelectionKey.mockReset();
    buildStudioBatchDraftSelectionKey.mockReturnValue("");
    listSheinStudioBatchDrafts.mockReset();
    upsertSheinStudioBatchDraft.mockReset();
    deleteSheinStudioBatchDraft.mockReset();
    getSheinStudioBatchDetail.mockReset();
  });

  it("saves batch snapshots through server api", async () => {
    upsertSheinStudioBatchDraft.mockResolvedValue({
      id: "batch-1",
      name: "retro cherries",
      prompt: "retro cherries",
      styleCount: "3",
      sheinStoreId: "",
      selectionVariantId: 100,
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 100,
        prototypeGroupId: 200,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
      groupedSelections: [
        {
          selectionId: "1:200:101:layer-2:101",
          selection: {
            productId: 1,
            parentProductId: 1,
            variantId: 101,
            prototypeGroupId: 200,
            layerId: "layer-2",
            productName: "hoodie",
            variantLabel: "L / white",
          },
          baselineStatus: "ready",
          baselineReason: "",
          sheinStoreId: "store-9",
          eligible: true,
        },
      ],
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          currentPrompt: "prompt a",
          promptHistory: [
            {
              prompt: "prompt old",
              groupedImageMode: "shared_by_size",
              createdAt: "2026-05-26T00:00:00Z",
            },
          ],
          primarySelection: {
            productId: 1,
            parentProductId: 1,
            variantId: 100,
            prototypeGroupId: 200,
            layerId: "layer-1",
            productName: "tee",
            variantLabel: "M / black",
          },
          groupedSelections: [],
          sheinStoreId: "store-9",
          imageStrategy: "sds_official",
          groupedImageMode: "shared_by_size",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          generationJobs: [
            {
              jobId: "group-job-1",
              status: "queued",
            },
          ],
          updatedAt: "2026-05-26T00:00:00Z",
        },
      ],
      designs: [
        {
          id: "design-1",
          dataUrl: "data:image/png;base64,abc",
        },
      ],
      selectedIds: ["design-1"],
      createdTasks: [],
      generationJobs: [
        {
          jobId: "job-1",
          status: "queued",
        },
      ],
      updatedAt: "2026-04-24T00:00:00.000Z",
    });
    listSheinStudioBatchDrafts.mockResolvedValue([
      {
        id: "batch-1",
        name: "retro cherries",
        prompt: "retro cherries",
        styleCount: "3",
        sheinStoreId: "",
        selectionVariantId: 100,
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 100,
          prototypeGroupId: 200,
          layerId: "layer-1",
          productName: "tee",
          variantLabel: "M / black",
        },
        designs: [
          {
            id: "design-1",
            dataUrl: "data:image/png;base64,abc",
          },
        ],
        selectedIds: ["design-1"],
        createdTasks: [],
        generationJobs: [
          {
            jobId: "job-1",
            status: "queued",
          },
        ],
        updatedAt: "2026-04-24T00:00:00.000Z",
      },
    ]);

    const saved = await saveSheinStudioBatch({
      prompt: "retro cherries",
      styleCount: "3",
      sheinStoreId: "",
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 100,
        prototypeGroupId: 200,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
      groupedSelections: [
        {
          selectionId: "1:200:101:layer-2:101",
          selection: {
            productId: 1,
            parentProductId: 1,
            variantId: 101,
            prototypeGroupId: 200,
            layerId: "layer-2",
            productName: "hoodie",
            variantLabel: "L / white",
          },
          baselineStatus: "ready",
          baselineReason: "",
          sheinStoreId: "store-9",
          eligible: true,
        },
      ],
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          currentPrompt: "prompt a",
          promptHistory: [
            {
              prompt: "prompt old",
              groupedImageMode: "shared_by_size",
              createdAt: "2026-05-26T00:00:00Z",
            },
          ],
          primarySelection: {
            productId: 1,
            parentProductId: 1,
            variantId: 100,
            prototypeGroupId: 200,
            layerId: "layer-1",
            productName: "tee",
            variantLabel: "M / black",
          },
          groupedSelections: [],
          sheinStoreId: "store-9",
          imageStrategy: "sds_official",
          groupedImageMode: "shared_by_size",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          generationJobs: [
            {
              jobId: "group-job-1",
              status: "queued",
            },
          ],
          updatedAt: "2026-05-26T00:00:00Z",
        },
      ],
      designs: [
        {
          id: "design-1",
          dataUrl: "data:image/png;base64,abc",
        },
      ],
      selectedIds: ["design-1"],
      createdTasks: [],
      generationJobs: [
        {
          jobId: "job-1",
          status: "queued",
        },
      ],
    });

    expect(saved?.prompt).toBe("retro cherries");
    expect(upsertSheinStudioBatchDraft).toHaveBeenCalledTimes(1);
    const saveInput = upsertSheinStudioBatchDraft.mock.calls[0]?.[0];
    expect(saveInput).not.toHaveProperty("designs");
    expect(saveInput).not.toHaveProperty("selectedIds");
    expect(saveInput).not.toHaveProperty("createdTasks");
    expect(saveInput).not.toHaveProperty("generationJobs");
    expect(saveInput?.groups?.[0]).not.toHaveProperty("designs");
    expect(saveInput?.groups?.[0]).not.toHaveProperty("selectedIds");
    expect(saveInput?.groups?.[0]).not.toHaveProperty("createdTasks");
    expect(saveInput?.groups?.[0]).not.toHaveProperty("generationJobs");
    expect(upsertSheinStudioBatchDraft).toHaveBeenCalledWith(
      expect.objectContaining({
        groupedSelections: [
          expect.objectContaining({
            selectionId: "1:200:101:layer-2:101",
            sheinStoreId: "store-9",
          }),
        ],
        groups: [
          expect.objectContaining({
            id: "group-1",
            currentPrompt: "prompt a",
          }),
        ],
      }),
    );

    const batches = await listSheinStudioBatches();
    expect(batches).toHaveLength(1);
    expect(batches[0]).toMatchObject({
      prompt: "retro cherries",
      designs: [
        expect.objectContaining({
          id: "design-1",
        }),
      ],
      selectedIds: ["design-1"],
      createdTasks: [],
      generationJobs: [
        expect.objectContaining({
          jobId: "job-1",
        }),
      ],
    });
  });

  it("does not synthesize a default batch name on create", async () => {
    listSheinStudioBatchDrafts.mockResolvedValue([
      {
        id: "batch-1",
        name: "批次1",
        prompt: "legacy one",
        styleCount: "1",
        sheinStoreId: "",
        selectionVariantId: 100,
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 100,
          prototypeGroupId: 200,
          layerId: "layer-1",
          productName: "tee",
          variantLabel: "M / black",
        },
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-04-24T00:00:00.000Z",
      },
      {
        id: "batch-2",
        name: "节日专题",
        prompt: "legacy two",
        styleCount: "1",
        sheinStoreId: "",
        selectionVariantId: 101,
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 101,
          prototypeGroupId: 200,
          layerId: "layer-2",
          productName: "hoodie",
          variantLabel: "L / white",
        },
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-04-24T00:00:00.000Z",
      },
      {
        id: "batch-3",
        name: "批次7",
        prompt: "legacy seven",
        styleCount: "1",
        sheinStoreId: "",
        selectionVariantId: 102,
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 102,
          prototypeGroupId: 200,
          layerId: "layer-3",
          productName: "clock",
          variantLabel: "One size / black",
        },
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-04-24T00:00:00.000Z",
      },
    ]);
    upsertSheinStudioBatchDraft.mockResolvedValue({
      id: "batch-8",
      name: "批次8",
      prompt: "",
      styleCount: "1",
      sheinStoreId: "",
      selectionVariantId: 100,
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 100,
        prototypeGroupId: 200,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-04-24T00:00:00.000Z",
    });

    await saveSheinStudioBatch({
      prompt: "",
      styleCount: "1",
      sheinStoreId: "",
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 100,
        prototypeGroupId: 200,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
      groupedSelections: [],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
    });

    expect(upsertSheinStudioBatchDraft).toHaveBeenCalledWith(
      expect.objectContaining({
        prompt: "",
      }),
    );
    expect(upsertSheinStudioBatchDraft).toHaveBeenCalledWith(
      expect.not.objectContaining({
        name: expect.any(String),
      }),
    );
  });

  it("keeps a saved batch even when prompt is empty", async () => {
    upsertSheinStudioBatchDraft.mockResolvedValue({
      id: "batch-empty-prompt",
      name: "批次12",
      prompt: "",
      styleCount: "1",
      sheinStoreId: "",
      selectionVariantId: 100,
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 100,
        prototypeGroupId: 200,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-04-24T00:00:00.000Z",
    });

    const saved = await saveSheinStudioBatch({
      prompt: "",
      styleCount: "1",
      sheinStoreId: "",
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 100,
        prototypeGroupId: 200,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
      groupedSelections: [],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
    });

    expect(saved).toEqual(
      expect.objectContaining({
        id: "batch-empty-prompt",
        name: "批次12",
        prompt: "",
      }),
    );
  });

  it("retries batch save once with the latest updatedAt when the server reports a conflict", async () => {
    upsertSheinStudioBatchDraft
      .mockRejectedValueOnce(
        new ApiError("ListingKit API request failed: 409", 409, {
          error: "studio_batch_save_failed",
          message: "studio session has been updated by another request",
        }),
      )
      .mockResolvedValueOnce({
        id: "batch-1",
        name: "批次1",
        prompt: "retro cherries",
        styleCount: "3",
        sheinStoreId: "869",
        selectionVariantId: 100,
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        draftUpdatedAt: "2026-06-01T10:05:00.000Z",
        updatedAt: "2026-06-01T10:06:00.000Z",
      });
    listSheinStudioBatchDrafts.mockResolvedValue([
      {
        id: "batch-1",
        name: "批次1",
        prompt: "retro cherries",
        styleCount: "3",
        sheinStoreId: "869",
        selectionVariantId: 100,
        selection,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        draftUpdatedAt: "2026-06-01T10:05:00.000Z",
        updatedAt: "2026-06-01T10:05:00.000Z",
      },
    ]);
    getSheinStudioBatchDetail.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "draft",
        prompt: "retro cherries",
        styleCount: "3",
        sheinStoreId: 869,
        createdAt: "2026-06-01T10:00:00Z",
        draftUpdatedAt: "2026-06-01T10:05:00.000Z",
        updatedAt: "2026-06-01T10:06:00.000Z",
      },
      items: [],
    });

    const saved = await saveSheinStudioBatch({
      id: "batch-1",
      updatedAt: "2026-06-01T10:04:00.000Z",
      prompt: "retro cherries",
      styleCount: "3",
      sheinStoreId: "869",
      selection,
      groupedSelections: [],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
    });

    expect(saved).toEqual(
      expect.objectContaining({
        id: "batch-1",
        updatedAt: "2026-06-01T10:06:00.000Z",
      }),
    );
    expect(upsertSheinStudioBatchDraft).toHaveBeenCalledTimes(2);
    expect(upsertSheinStudioBatchDraft).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        id: "batch-1",
        expectedUpdatedAt: "2026-06-01T10:04:00.000Z",
      }),
    );
    expect(upsertSheinStudioBatchDraft).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        id: "batch-1",
        expectedUpdatedAt: "2026-06-01T10:05:00.000Z",
      }),
    );
  });

  it("maps itemized batch detail back into the saved batch shape", async () => {
    listSheinStudioBatchDrafts.mockResolvedValue([
      {
        id: "batch-1",
        name: "botanical legacy",
        prompt: "legacy prompt",
        styleCount: "5",
        sheinStoreId: "store-9",
        selectionVariantId: 100,
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 100,
          prototypeGroupId: 200,
          layerId: "layer-1",
          productName: "tee",
          variantLabel: "M / black",
        },
        groupedSelections: [
          {
            selectionId: "1:200:101:layer-2:101",
            selection: {
              productId: 1,
              parentProductId: 1,
              variantId: 101,
              prototypeGroupId: 200,
              layerId: "layer-2",
              productName: "hoodie",
              variantLabel: "L / white",
            },
            baselineStatus: "ready",
            baselineReason: "",
            sheinStoreId: "store-9",
            eligible: true,
          },
        ],
        groups: [
          {
            id: "group-1",
            name: "Group 1",
            currentPrompt: "prompt a",
            promptHistory: [],
            primarySelection: {
              productId: 1,
              parentProductId: 1,
              variantId: 100,
              prototypeGroupId: 200,
              layerId: "layer-1",
              productName: "tee",
              variantLabel: "M / black",
            },
            groupedSelections: [],
            sheinStoreId: "store-9",
            imageStrategy: "sds_official",
            groupedImageMode: "shared_by_size",
            selectedSdsImages: [],
            renderSizeImagesWithSds: true,
            productImageCount: "5",
            productImagePrompt: "",
            productImagePrompts: [],
            artworkModel: "",
            transparentBackground: false,
            variationIntensity: "medium",
            designs: [],
            selectedIds: [],
            createdTasks: [],
            updatedAt: "2026-06-01T10:00:00Z",
          },
        ],
        designs: [],
        selectedIds: [],
        createdTasks: [{ id: "task-0", title: "Existing Task", designId: "design-0" }],
        updatedAt: "2026-06-01T10:04:00Z",
      },
    ]);
    getSheinStudioBatchDetail.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "botanical",
        styleCount: "3",
        sheinStoreId: 7,
        createdAt: "2026-06-01T10:00:00Z",
        draftUpdatedAt: "2026-06-01T10:04:00Z",
        updatedAt: "2026-06-01T10:05:00Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "size:1200x1200",
            targetGroupLabel: "1200 x 1200",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-06-01T10:00:00Z",
            updatedAt: "2026-06-01T10:05:00Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1200x1200",
              targetGroupLabel: "1200 x 1200",
              imageUrl: "https://cdn.example.com/design-1.png",
              reviewStatus: "approved",
              reviewNote: "looks good",
              createdAt: "2026-06-01T10:01:00Z",
              updatedAt: "2026-06-01T10:05:00Z",
            },
            {
              id: "design-2",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "size:1200x1200",
              imageUrl: "https://cdn.example.com/design-2.png",
              reviewStatus: "rejected",
              createdAt: "2026-06-01T10:02:00Z",
              updatedAt: "2026-06-01T10:05:00Z",
            },
          ],
        },
      ],
    });

    await expect(getSheinStudioBatch("batch-1")).resolves.toMatchObject({
      id: "batch-1",
      name: "botanical legacy",
      prompt: "botanical",
      styleCount: "3",
      sheinStoreId: "7",
      selection: expect.objectContaining({
        variantId: 100,
        productName: "tee",
      }),
      groupedSelections: [
        expect.objectContaining({
          selectionId: "1:200:101:layer-2:101",
          sheinStoreId: "store-9",
        }),
      ],
      groups: [expect.objectContaining({ id: "group-1", name: "Group 1" })],
      createdTasks: [{ id: "task-0", title: "Existing Task", designId: "design-0" }],
      draftUpdatedAt: "2026-06-01T10:04:00Z",
      batchStatus: "review_ready",
      selectedIds: ["design-1"],
      designs: [
        expect.objectContaining({
          id: "design-1",
          imageUrl: "https://cdn.example.com/design-1.png",
          prompt: "botanical",
          targetGroupKey: "size:1200x1200",
          targetGroupLabel: "1200 x 1200",
          reviewNote: "looks good",
        }),
        expect.objectContaining({
          id: "design-2",
          imageUrl: "https://cdn.example.com/design-2.png",
          prompt: "botanical",
        }),
      ],
      updatedAt: "2026-06-01T10:05:00Z",
    });
  });

  it("keeps saved batch prompt and grouped selection context when detail is still uninitialized", async () => {
    listSheinStudioBatchDrafts.mockResolvedValue([
      {
        id: "batch-1",
        name: "869全品类",
        prompt: "server batch prompt",
        styleCount: "4",
        sheinStoreId: "869",
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 100,
          prototypeGroupId: 200,
          layerId: "layer-1",
          productName: "tee",
          variantLabel: "M / black",
        },
        groupedSelections: [
          {
            selectionId: "1:200:101:layer-2:101",
            selection: {
              productId: 1,
              parentProductId: 1,
              variantId: 101,
              prototypeGroupId: 200,
              layerId: "layer-2",
              productName: "hoodie",
              variantLabel: "L / white",
            },
            baselineStatus: "ready",
            baselineReason: "",
            sheinStoreId: "869",
            eligible: true,
          },
        ],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-06-01T10:04:00Z",
      },
    ]);
    getSheinStudioBatchDetail.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "draft",
        prompt: "",
        styleCount: "",
        sheinStoreId: 0,
        createdAt: "2026-06-01T10:00:00Z",
        updatedAt: "2026-06-01T10:05:00Z",
      },
      items: [],
    });

    await expect(getSheinStudioHydratedBatch("batch-1")).resolves.toMatchObject({
      savedBatch: expect.objectContaining({
        id: "batch-1",
        name: "869全品类",
        prompt: "server batch prompt",
        styleCount: "4",
        sheinStoreId: "869",
        selection: expect.objectContaining({
          variantId: 100,
          productName: "tee",
        }),
        groupedSelections: [
          expect.objectContaining({
            selectionId: "1:200:101:layer-2:101",
            sheinStoreId: "869",
          }),
        ],
      }),
    });
  });

  it("hydrates dedicated batch selection context from itemized detail when the saved batch summary is skeletal", async () => {
    listSheinStudioBatchDrafts.mockResolvedValue([
      {
        id: "batch-1",
        name: "869全品类",
        prompt: "server batch prompt",
        styleCount: "4",
        sheinStoreId: "",
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-06-01T10:04:00Z",
      },
    ]);
    getSheinStudioBatchDetail.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "generating",
        prompt: "server batch prompt",
        styleCount: "4",
        sheinStoreId: 869,
        groupedImageMode: "shared_by_size",
        transparentBackground: true,
        selectionVariantId: 100,
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 100,
          prototypeGroupId: 200,
          layerId: "layer-1",
          productName: "tee",
          variantLabel: "M / black",
        },
        groupedSelections: [
          {
            selectionId: "1:200:101:layer-2:101",
            selection: {
              productId: 1,
              parentProductId: 1,
              variantId: 101,
              prototypeGroupId: 200,
              layerId: "layer-2",
              productName: "hoodie",
              variantLabel: "L / white",
            },
            baselineStatus: "ready",
            baselineReason: "",
            sheinStoreId: "869",
            eligible: true,
          },
        ],
        selectedSdsImages: [
          {
            imageUrl: "https://cdn.example.com/sds-1.png",
            color: "black",
          },
        ],
        createdAt: "2026-06-01T10:00:00Z",
        updatedAt: "2026-06-01T10:05:00Z",
      },
      items: [],
    });

    await expect(getSheinStudioHydratedBatch("batch-1")).resolves.toMatchObject({
      savedBatch: expect.objectContaining({
        id: "batch-1",
        sheinStoreId: "869",
        groupedImageMode: "shared_by_size",
        transparentBackground: true,
        selection: expect.objectContaining({
          variantId: 100,
          productName: "tee",
        }),
        groupedSelections: [
          expect.objectContaining({
            selectionId: "1:200:101:layer-2:101",
            sheinStoreId: "869",
          }),
        ],
        selectedSdsImages: [
          expect.objectContaining({
            imageUrl: "https://cdn.example.com/sds-1.png",
          }),
        ],
      }),
    });
  });

  it("treats itemized batch detail as the primary saved-batch compatibility source", async () => {
    listSheinStudioBatchDrafts.mockResolvedValue([
      {
        id: "batch-1",
        name: "Legacy Batch",
        prompt: "stale saved prompt",
        styleCount: "9",
        sheinStoreId: "42",
        variationIntensity: "light",
        artworkModel: "legacy-model",
        transparentBackground: false,
        groupedImageMode: "shared_by_size",
        selectionVariantId: 100,
        selection,
        selectedSdsImages: [],
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-06-01T10:04:00Z",
      },
    ]);
    getSheinStudioBatchDetail.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "itemized prompt",
        styleCount: "2",
        sheinStoreId: 869,
        variationIntensity: "strong",
        artworkModel: "nanobanana",
        transparentBackground: true,
        groupedImageMode: "per_product",
        selectionVariantId: 101,
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 101,
          prototypeGroupId: 200,
          layerId: "layer-2",
          productName: "hoodie",
          variantLabel: "L / white",
        },
        selectedSdsImages: [
          {
            imageUrl: "https://cdn.example.com/sds-1.png",
            color: "black",
          },
        ],
        groupedSelections: [],
        createdAt: "2026-06-01T10:00:00Z",
        updatedAt: "2026-06-01T10:05:00Z",
      },
      items: [
        {
          item: {
            id: "item-1",
            batchId: "batch-1",
            targetGroupKey: "group-1",
            status: "review_ready",
            selectionCount: 1,
            createdAt: "2026-06-01T10:00:00Z",
            updatedAt: "2026-06-01T10:05:00Z",
          },
          designs: [
            {
              id: "design-1",
              batchId: "batch-1",
              itemId: "item-1",
              sourceAttemptId: "attempt-1",
              targetGroupKey: "group-1",
              imageUrl: "https://cdn.example.com/design-1.png",
              reviewStatus: "approved",
              createdAt: "2026-06-01T10:01:00Z",
              updatedAt: "2026-06-01T10:05:00Z",
            },
          ],
        },
      ],
    });

    await expect(getSheinStudioHydratedBatch("batch-1")).resolves.toMatchObject({
      savedBatch: expect.objectContaining({
        prompt: "itemized prompt",
        styleCount: "2",
        sheinStoreId: "869",
        variationIntensity: "strong",
        artworkModel: "nanobanana",
        transparentBackground: true,
        groupedImageMode: "per_product",
        selectionVariantId: 101,
        selection: expect.objectContaining({
          variantId: 101,
          productName: "hoodie",
        }),
        selectedIds: ["design-1"],
        designs: [expect.objectContaining({ id: "design-1" })],
      }),
    });
  });

  it("surfaces a hydration error when saved batch context cannot be loaded", async () => {
    listSheinStudioBatchDrafts.mockRejectedValueOnce(new Error("list failed"));
    getSheinStudioBatchDetail.mockResolvedValue({
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "botanical",
        styleCount: "3",
        sheinStoreId: 7,
        createdAt: "2026-06-01T10:00:00Z",
        updatedAt: "2026-06-01T10:05:00Z",
      },
      items: [],
    });

    await expect(getSheinStudioHydratedBatch("batch-1")).rejects.toThrow(
      "list failed",
    );
  });

  it("dedupes concurrent saved batch list requests", async () => {
    listSheinStudioBatchDrafts.mockImplementation(
      () =>
        new Promise((resolve) => {
          setTimeout(() => {
            resolve([
              {
                id: "batch-1",
                name: "Batch 1",
                prompt: "prompt",
                styleCount: "1",
                sheinStoreId: "",
                designs: [],
                selectedIds: [],
                createdTasks: [],
                updatedAt: "2026-06-01T10:04:00Z",
              },
            ]);
          }, 0);
        }),
    );

    const [first, second] = await Promise.all([
      listSheinStudioBatches(),
      listSheinStudioBatches(),
    ]);

    expect(listSheinStudioBatchDrafts).toHaveBeenCalledTimes(1);
    expect(first).toHaveLength(1);
    expect(second).toHaveLength(1);
  });

  it("dedupes concurrent hydrated batch requests for the same batch id", async () => {
    listSheinStudioBatchDrafts.mockImplementation(
      () =>
        new Promise((resolve) => {
          setTimeout(() => {
            resolve([
              {
                id: "batch-1",
                name: "Batch 1",
                prompt: "prompt",
                styleCount: "1",
                sheinStoreId: "",
                designs: [],
                selectedIds: [],
                createdTasks: [],
                updatedAt: "2026-06-01T10:04:00Z",
              },
            ]);
          }, 0);
        }),
    );
    getSheinStudioBatchDetail.mockImplementation(
      () =>
        new Promise((resolve) => {
          setTimeout(() => {
            resolve({
              batch: {
                id: "batch-1",
                status: "draft",
                prompt: "prompt",
                styleCount: "1",
                sheinStoreId: 7,
                createdAt: "2026-06-01T10:00:00Z",
                updatedAt: "2026-06-01T10:05:00Z",
              },
              items: [],
            });
          }, 0);
        }),
    );

    const [first, second] = await Promise.all([
      getSheinStudioHydratedBatch("batch-1"),
      getSheinStudioHydratedBatch("batch-1"),
    ]);

    expect(getSheinStudioBatchDetail).toHaveBeenCalledTimes(1);
    expect(listSheinStudioBatchDrafts).toHaveBeenCalledTimes(1);
    expect(first.savedBatch.id).toBe("batch-1");
    expect(second.savedBatch.id).toBe("batch-1");
  });

});

