import { describe, expect, it } from "vitest";

import {
  applySheinStudioWorkbenchBatch,
  applySheinStudioWorkbenchDraft,
  applySheinStudioWorkbenchHydratedBatch,
  buildInitialSheinStudioWorkbenchState,
  selectSheinStudioWorkbenchGroup,
  setSheinStudioWorkbenchField,
  sheinStudioWorkbenchReducer,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import {
  DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
  DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
  DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
  DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
  DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
} from "@/lib/shein-studio/storage-shared";
import { DEFAULT_SHEIN_STORE_ID } from "@/lib/shein-studio/create-review-tasks";

describe("buildInitialSheinStudioWorkbenchState", () => {
  it("centralizes the default values used by the workbench", () => {
    expect(buildInitialSheinStudioWorkbenchState()).toMatchObject({
      artworkModel: DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
      activeGroupId: "",
      batchQueueMode: null,
      imageStrategy: DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
      groups: [],
      groupedImageMode: DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
      productImageCount: DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
      queueMessage: "",
      queuedBatchIds: [],
      queuedBatchIndex: 0,
      variationIntensity: DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
      groupedSelections: [],
      prompt: "",
      styleCount: "1",
      transparentBackground: false,
      renderSizeImagesWithSds: true,
    });
  });
});

describe("sheinStudioWorkbenchReducer", () => {
  it("updates a single field without mutating the previous state", () => {
    const initial = buildInitialSheinStudioWorkbenchState();
    const next = sheinStudioWorkbenchReducer(
      initial,
      setSheinStudioWorkbenchField("prompt", "new prompt"),
    );

    expect(next.prompt).toBe("new prompt");
    expect(initial.prompt).toBe("");
    expect(next).not.toBe(initial);
  });

  it("supports functional updates for array fields", () => {
    const initial = buildInitialSheinStudioWorkbenchState();
    const next = sheinStudioWorkbenchReducer(
      initial,
      setSheinStudioWorkbenchField("selectedIds", (current) => [
        ...current,
        "design-1",
      ]),
    );

    expect(next.selectedIds).toEqual(["design-1"]);
  });

  it("stores batch queue metadata in workbench state", () => {
    const state = buildInitialSheinStudioWorkbenchState();
    const next = sheinStudioWorkbenchReducer(
      state,
      setSheinStudioWorkbenchField("batchQueueMode", "generate"),
    );

    expect(next.batchQueueMode).toBe("generate");
  });

  it("tracks queued batch ids and current index", () => {
    const state = buildInitialSheinStudioWorkbenchState();
    const withIds = sheinStudioWorkbenchReducer(
      state,
      setSheinStudioWorkbenchField("queuedBatchIds", ["batch-1", "batch-2"]),
    );
    const next = sheinStudioWorkbenchReducer(
      withIds,
      setSheinStudioWorkbenchField("queuedBatchIndex", 1),
    );

    expect(next.queuedBatchIds).toEqual(["batch-1", "batch-2"]);
    expect(next.queuedBatchIndex).toBe(1);
  });

  it("applies a loaded draft as a single reducer action", () => {
    const initial = buildInitialSheinStudioWorkbenchState();
    const next = sheinStudioWorkbenchReducer(
      initial,
      applySheinStudioWorkbenchDraft({
        prompt: "draft prompt",
        styleCount: "2",
        variationIntensity: "strong",
        productImageCount: "3",
        productImagePrompt: "product prompt",
        productImagePrompts: [],
        artworkModel: DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
        transparentBackground: true,
        sheinStoreId: "42",
        imageStrategy: "hybrid",
        groupedImageMode: "per_product",
        selectedSdsImages: [],
        groups: [
          {
            id: "group-1",
            name: "Group 1",
            currentPrompt: "draft prompt",
            promptHistory: [],
            primarySelection: {
              productId: 1,
              parentProductId: 1,
              variantId: 101,
              prototypeGroupId: 200,
              layerId: "layer-2",
              productName: "hoodie",
              variantLabel: "L / white",
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
                sheinStoreId: "88",
                eligible: true,
              },
            ],
            sheinStoreId: "42",
            imageStrategy: "hybrid",
            groupedImageMode: "per_product",
            selectedSdsImages: [],
            renderSizeImagesWithSds: false,
            productImageCount: "3",
            productImagePrompt: "product prompt",
            productImagePrompts: [],
            artworkModel: DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
            transparentBackground: true,
            variationIntensity: "strong",
            designs: [{ id: "design-1", imageUrl: "https://example.com/1.png" }],
            selectedIds: ["design-1"],
            createdTasks: [],
            updatedAt: "2026-05-26T00:00:00Z",
          },
        ],
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
            sheinStoreId: "88",
            eligible: true,
          },
        ],
        renderSizeImagesWithSds: false,
        designs: [{ id: "design-1", imageUrl: "https://example.com/1.png" }],
        selectedIds: ["design-1"],
        createdTasks: [],
        galleryRatioCheck: null,
      }),
    );

    expect(next.prompt).toBe("draft prompt");
    expect(next.activeGroupId).toBe("group-1");
    expect(next.designs).toHaveLength(1);
    expect(next.groups).toHaveLength(1);
    expect(next.groupedSelections).toHaveLength(1);
    expect(next.groupedImageMode).toBe("per_product");
    expect(next.renderSizeImagesWithSds).toBe(false);
  });

  it("normalizes an old saved batch through the reducer", () => {
    const initial = buildInitialSheinStudioWorkbenchState();
    const next = sheinStudioWorkbenchReducer(
      initial,
      applySheinStudioWorkbenchBatch({
        id: "batch-1",
        name: "Batch 1",
        prompt: "batch prompt",
        styleCount: "4",
        sheinStoreId: "",
        groups: [
          {
            id: "group-1",
            name: "Group 1",
            currentPrompt: "group 1 prompt",
            promptHistory: [],
            primarySelection: {
              productId: 1,
              parentProductId: 1,
              variantId: 101,
              prototypeGroupId: 200,
              layerId: "layer-2",
              productName: "hoodie",
              variantLabel: "L / white",
            },
            groupedSelections: [],
            sheinStoreId: "",
            imageStrategy: DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
            groupedImageMode: DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
            selectedSdsImages: [],
            renderSizeImagesWithSds: true,
            productImageCount: DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
            productImagePrompt: "",
            productImagePrompts: [],
            artworkModel: DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
            transparentBackground: false,
            variationIntensity: DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
            designs: [],
            selectedIds: [],
            createdTasks: [],
            updatedAt: "2026-05-26T00:00:00Z",
          },
          {
            id: "group-2",
            name: "Group 2",
            currentPrompt: "group 2 prompt",
            promptHistory: [],
            primarySelection: {
              productId: 1,
              parentProductId: 1,
              variantId: 102,
              prototypeGroupId: 200,
              layerId: "layer-3",
              productName: "mug",
              variantLabel: "One size",
            },
            groupedSelections: [
              {
                selectionId: "1:200:102:layer-3:102",
                selection: {
                  productId: 1,
                  parentProductId: 1,
                  variantId: 102,
                  prototypeGroupId: 200,
                  layerId: "layer-3",
                  productName: "mug",
                  variantLabel: "One size",
                },
                baselineStatus: "ready",
                baselineReason: "",
                sheinStoreId: "88",
                eligible: true,
              },
            ],
            sheinStoreId: "88",
            imageStrategy: DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
            groupedImageMode: DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
            selectedSdsImages: [],
            renderSizeImagesWithSds: true,
            productImageCount: DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
            productImagePrompt: "",
            productImagePrompts: [],
            artworkModel: DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
            transparentBackground: false,
            variationIntensity: DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
            designs: [],
            selectedIds: [],
            createdTasks: [],
            updatedAt: "2026-05-26T01:00:00Z",
          },
        ],
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
            sheinStoreId: "88",
            eligible: true,
          },
        ],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-15T00:00:00.000Z",
      }),
    );

    expect(next.sheinStoreId).toBe("88");
    expect(next.activeGroupId).toBe("group-2");
    expect(next.prompt).toBe("group 2 prompt");
    expect(next.imageStrategy).toBe(DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY);
    expect(next.groupedImageMode).toBe(DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE);
    expect(next.groupedSelections).toHaveLength(1);
  });

  it("projects hydrated batch detail into itemized review state", () => {
    const initial = buildInitialSheinStudioWorkbenchState();
    const next = sheinStudioWorkbenchReducer(
      initial,
      applySheinStudioWorkbenchHydratedBatch({
        savedBatch: {
          id: "batch-1",
          name: "Batch 1",
          prompt: "batch prompt",
          styleCount: "4",
          sheinStoreId: "88",
          selection: {
            productId: 1,
            parentProductId: 1,
            variantId: 101,
            prototypeGroupId: 200,
            layerId: "layer-2",
            productName: "hoodie",
            variantLabel: "L / white",
          },
          groupedSelections: [],
          groups: [],
          designs: [],
          selectedIds: [],
          createdTasks: [],
          draftUpdatedAt: "2026-05-15T00:00:00.000Z",
          updatedAt: "2026-05-15T00:05:00.000Z",
        },
        detail: {
          batch: {
            id: "batch-1",
            status: "review_ready",
            prompt: "batch prompt",
            styleCount: "4",
            sheinStoreId: 88,
            createdAt: "2026-05-15T00:00:00.000Z",
            draftUpdatedAt: "2026-05-15T00:00:00.000Z",
            updatedAt: "2026-05-15T00:05:00.000Z",
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
                createdAt: "2026-05-15T00:00:00.000Z",
                updatedAt: "2026-05-15T00:05:00.000Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1200x1200",
                  targetGroupLabel: "1200 x 1200",
                  imageUrl: "https://example.com/1.png",
                  reviewStatus: "approved",
                  createdAt: "2026-05-15T00:01:00.000Z",
                  updatedAt: "2026-05-15T00:05:00.000Z",
                },
                {
                  id: "design-2",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-2",
                  targetGroupKey: "size:1200x1200",
                  imageUrl: "https://example.com/2.png",
                  reviewStatus: "unreviewed",
                  createdAt: "2026-05-15T00:02:00.000Z",
                  updatedAt: "2026-05-15T00:05:00.000Z",
                },
              ],
            },
          ],
        },
      }),
    );

    expect(next.itemizedBatchDetail?.batch.id).toBe("batch-1");
    expect(next.designs).toHaveLength(2);
    expect(next.selectedIds).toEqual(["design-1"]);
    expect(next.persistedUpdatedAt).toBe("2026-05-15T00:00:00.000Z");
  });

  it("selects a saved group and projects it into the current editor state", () => {
    const initial = sheinStudioWorkbenchReducer(
      buildInitialSheinStudioWorkbenchState(),
      applySheinStudioWorkbenchDraft({
        prompt: "legacy prompt",
        styleCount: "2",
        variationIntensity: "medium",
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
        transparentBackground: false,
        sheinStoreId: "42",
        imageStrategy: "hybrid",
        groupedImageMode: "shared_by_size",
        selectedSdsImages: [],
        groups: [
          {
            id: "group-1",
            name: "Group 1",
            currentPrompt: "group 1 prompt",
            promptHistory: [],
            primarySelection: {
              productId: 1,
              parentProductId: 1,
              variantId: 101,
              prototypeGroupId: 200,
              layerId: "layer-2",
              productName: "hoodie",
              variantLabel: "L / white",
            },
            groupedSelections: [],
            sheinStoreId: "42",
            imageStrategy: "hybrid",
            groupedImageMode: "shared_by_size",
            selectedSdsImages: [],
            renderSizeImagesWithSds: true,
            productImageCount: "5",
            productImagePrompt: "",
            productImagePrompts: [],
            artworkModel: DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
            transparentBackground: false,
            variationIntensity: "medium",
            designs: [],
            selectedIds: [],
            createdTasks: [],
            updatedAt: "2026-05-26T00:00:00Z",
          },
          {
            id: "group-2",
            name: "Group 2",
            currentPrompt: "group 2 prompt",
            promptHistory: [],
            primarySelection: {
              productId: 1,
              parentProductId: 1,
              variantId: 102,
              prototypeGroupId: 200,
              layerId: "layer-3",
              productName: "mug",
              variantLabel: "One size",
            },
            groupedSelections: [],
            sheinStoreId: "88",
            imageStrategy: "sds_official",
            groupedImageMode: "per_product",
            selectedSdsImages: [],
            renderSizeImagesWithSds: false,
            productImageCount: "3",
            productImagePrompt: "product prompt",
            productImagePrompts: [],
            artworkModel: DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
            transparentBackground: true,
            variationIntensity: "strong",
            designs: [{ id: "design-2", imageUrl: "https://example.com/2.png" }],
            selectedIds: ["design-2"],
            createdTasks: [],
            updatedAt: "2026-05-26T01:00:00Z",
          },
        ],
        groupedSelections: [],
        renderSizeImagesWithSds: true,
        designs: [],
        selectedIds: [],
        createdTasks: [],
        galleryRatioCheck: null,
      }),
    );

    const next = sheinStudioWorkbenchReducer(
      initial,
      selectSheinStudioWorkbenchGroup("group-1"),
    );

    expect(next.activeGroupId).toBe("group-1");
    expect(next.prompt).toBe("group 1 prompt");
    expect(next.sheinStoreId).toBe("42");
  });

  it("keeps hydrated itemized designs and detail even when saved groups are stale shells", () => {
    const initial = sheinStudioWorkbenchReducer(
      buildInitialSheinStudioWorkbenchState(),
      applySheinStudioWorkbenchBatch({
        id: "batch-1",
        name: "batch",
        prompt: "legacy prompt",
        styleCount: "1",
        sheinStoreId: "42",
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 100,
          prototypeGroupId: 200,
          layerId: "layer-1",
          productName: "tee",
          variantLabel: "M / black",
        },
        groups: [
          {
            id: "group-1",
            name: "Group 1",
            currentPrompt: "legacy prompt",
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
            sheinStoreId: "42",
            imageStrategy: "sds_official",
            groupedImageMode: "shared_by_size",
            selectedSdsImages: [],
            renderSizeImagesWithSds: true,
            productImageCount: "5",
            productImagePrompt: "",
            productImagePrompts: [],
            artworkModel: DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
            transparentBackground: false,
            variationIntensity: "medium",
            designs: [],
            selectedIds: [],
            createdTasks: [],
            updatedAt: "2026-06-01T10:00:00Z",
          },
        ],
        groupedSelections: [],
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-06-01T10:00:00Z",
      }),
    );

    const next = sheinStudioWorkbenchReducer(
      initial,
      applySheinStudioWorkbenchHydratedBatch({
        savedBatch: {
          id: "batch-1",
          name: "batch",
          prompt: "hydrated prompt",
          styleCount: "2",
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
              selectionId: "sel-1",
              selection: {
                productId: 1,
                parentProductId: 1,
                variantId: 100,
                prototypeGroupId: 200,
                layerId: "layer-1",
                productName: "tee",
                variantLabel: "M / black",
                printableWidth: 1200,
                printableHeight: 1200,
              },
              baselineStatus: "ready",
              baselineReason: "",
              sheinStoreId: "869",
              eligible: true,
            },
          ],
          groups: initial.groups,
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-06-01T10:10:00Z",
        },
        detail: {
          batch: {
            id: "batch-1",
            status: "review_ready",
            prompt: "hydrated prompt",
            styleCount: "2",
            sheinStoreId: 869,
            createdAt: "2026-06-01T10:00:00Z",
            updatedAt: "2026-06-01T10:10:00Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1200x1200",
                status: "review_ready",
                selectionCount: 1,
                createdAt: "2026-06-01T10:00:00Z",
                updatedAt: "2026-06-01T10:10:00Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1200x1200",
                  imageUrl: "https://cdn.example.com/design-1.png",
                  reviewStatus: "approved",
                  createdAt: "2026-06-01T10:05:00Z",
                  updatedAt: "2026-06-01T10:10:00Z",
                },
              ],
            },
          ],
        },
      }),
    );

    expect(next.activeGroupId).toBe("group-1");
    expect(next.designs).toHaveLength(1);
    expect(next.designs[0]?.id).toBe("design-1");
    expect(next.selectedIds).toEqual(["design-1"]);
    expect(next.itemizedBatchDetail?.items).toHaveLength(1);
    expect(next.groups[0]?.designs).toHaveLength(1);
    expect(next.groups[0]?.selectedIds).toEqual(["design-1"]);
    expect(next.sheinStoreId).toBe("869");
    expect(next.selection?.variantId).toBe(100);
    expect(next.groupedSelections).toHaveLength(1);
    expect(next.groupedSelections[0]?.selectionId).toBe("sel-1");
  });

  it("ignores a stale hydrated batch update for the same batch", () => {
    const newer = sheinStudioWorkbenchReducer(
      buildInitialSheinStudioWorkbenchState(),
      applySheinStudioWorkbenchHydratedBatch({
        savedBatch: {
          id: "batch-1",
          name: "batch",
          prompt: "new prompt",
          styleCount: "2",
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
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-06-01T10:10:00Z",
        },
        detail: {
          batch: {
            id: "batch-1",
            status: "review_ready",
            prompt: "new prompt",
            styleCount: "2",
            sheinStoreId: 869,
            createdAt: "2026-06-01T10:00:00Z",
            updatedAt: "2026-06-01T10:10:00Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1200x1200",
                status: "review_ready",
                selectionCount: 1,
                createdAt: "2026-06-01T10:00:00Z",
                updatedAt: "2026-06-01T10:10:00Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1200x1200",
                  imageUrl: "https://cdn.example.com/design-1.png",
                  reviewStatus: "approved",
                  createdAt: "2026-06-01T10:05:00Z",
                  updatedAt: "2026-06-01T10:10:00Z",
                },
                {
                  id: "design-2",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-2",
                  targetGroupKey: "size:1200x1200",
                  imageUrl: "https://cdn.example.com/design-2.png",
                  reviewStatus: "unreviewed",
                  createdAt: "2026-06-01T10:06:00Z",
                  updatedAt: "2026-06-01T10:10:00Z",
                },
              ],
            },
          ],
        },
      }),
    );

    const next = sheinStudioWorkbenchReducer(
      newer,
      applySheinStudioWorkbenchHydratedBatch({
        savedBatch: {
          id: "batch-1",
          name: "batch",
          prompt: "old prompt",
          styleCount: "1",
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
          designs: [],
          selectedIds: [],
          createdTasks: [],
          updatedAt: "2026-06-01T10:05:00Z",
        },
        detail: {
          batch: {
            id: "batch-1",
            status: "review_ready",
            prompt: "old prompt",
            styleCount: "1",
            sheinStoreId: 869,
            createdAt: "2026-06-01T10:00:00Z",
            updatedAt: "2026-06-01T10:05:00Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1200x1200",
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
                  imageUrl: "https://cdn.example.com/design-1.png",
                  reviewStatus: "approved",
                  createdAt: "2026-06-01T10:05:00Z",
                  updatedAt: "2026-06-01T10:05:00Z",
                },
              ],
            },
          ],
        },
      }),
    );

    expect(next.prompt).toBe("new prompt");
    expect(next.designs).toHaveLength(2);
    expect(next.selectedIds).toEqual(["design-1"]);
    expect(next.persistedUpdatedAt).toBe("2026-06-01T10:10:00Z");
    expect(next.itemizedBatchDetail?.batch.updatedAt).toBe("2026-06-01T10:10:00Z");
  });

  it("refreshes saved batch snapshots from hydrated batch detail truth", () => {
    const initial = sheinStudioWorkbenchReducer(
      buildInitialSheinStudioWorkbenchState(),
      setSheinStudioWorkbenchField("savedBatches", [
        {
          id: "batch-1",
          name: "batch",
          prompt: "stale prompt",
          styleCount: "1",
          sheinStoreId: "869",
          designs: [{ id: "design-stale", imageUrl: "https://cdn.example.com/stale.png" }],
          selectedIds: [],
          createdTasks: [],
          generationJobs: [{ jobId: "job-1", status: "running" }],
          generationError: "stale error",
          generationJobId: "job-1",
          legacyCompatibilitySnapshot: {
            generationJobs: [{ jobId: "job-1", status: "running" }],
            generationError: "stale error",
            generationJobId: "job-1",
          },
          updatedAt: "2026-06-01T10:05:00Z",
        },
      ]),
    );

    const next = sheinStudioWorkbenchReducer(
      initial,
      applySheinStudioWorkbenchHydratedBatch({
        savedBatch: {
          id: "batch-1",
          name: "batch",
          prompt: "fresh prompt",
          styleCount: "2",
          sheinStoreId: "869",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          generationJobs: [{ jobId: "job-1", status: "running" }],
          generationError: "stale error",
          generationJobId: "job-1",
          legacyCompatibilitySnapshot: {
            generationJobs: [{ jobId: "job-1", status: "running" }],
            generationError: "stale error",
            generationJobId: "job-1",
          },
          draftUpdatedAt: "2026-06-01T10:05:00Z",
          updatedAt: "2026-06-01T10:10:00Z",
        },
        detail: {
          batch: {
            id: "batch-1",
            status: "review_ready",
            prompt: "fresh prompt",
            styleCount: "2",
            sheinStoreId: 869,
            createdAt: "2026-06-01T10:00:00Z",
            draftUpdatedAt: "2026-06-01T10:05:00Z",
            updatedAt: "2026-06-01T10:10:00Z",
          },
          items: [
            {
              item: {
                id: "item-1",
                batchId: "batch-1",
                targetGroupKey: "size:1200x1200",
                status: "review_ready",
                selectionCount: 1,
                createdAt: "2026-06-01T10:00:00Z",
                updatedAt: "2026-06-01T10:10:00Z",
              },
              designs: [
                {
                  id: "design-1",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-1",
                  targetGroupKey: "size:1200x1200",
                  imageUrl: "https://cdn.example.com/design-1.png",
                  reviewStatus: "approved",
                  createdAt: "2026-06-01T10:05:00Z",
                  updatedAt: "2026-06-01T10:10:00Z",
                },
                {
                  id: "design-2",
                  batchId: "batch-1",
                  itemId: "item-1",
                  sourceAttemptId: "attempt-2",
                  targetGroupKey: "size:1200x1200",
                  imageUrl: "https://cdn.example.com/design-2.png",
                  reviewStatus: "unreviewed",
                  createdAt: "2026-06-01T10:06:00Z",
                  updatedAt: "2026-06-01T10:10:00Z",
                },
              ],
            },
          ],
        },
      }),
    );

    expect(next.savedBatches).toHaveLength(1);
    expect(next.savedBatches[0]).toMatchObject({
      id: "batch-1",
      prompt: "fresh prompt",
      styleCount: "2",
      selectedIds: ["design-1"],
      generationJobs: [],
      generationError: "",
      generationJobId: "",
      updatedAt: "2026-06-01T10:10:00Z",
      batchStatus: "review_ready",
    });
    expect(next.savedBatches[0]?.designs).toHaveLength(2);
    expect(next.savedBatches[0]?.designs.map((design) => design.id)).toEqual([
      "design-1",
      "design-2",
    ]);
    expect(next.savedBatches[0]?.legacyCompatibilitySnapshot).toMatchObject({
      designs: [
        { id: "design-1", imageUrl: "https://cdn.example.com/design-1.png" },
        { id: "design-2", imageUrl: "https://cdn.example.com/design-2.png" },
      ],
      selectedIds: ["design-1"],
      generationJobs: [],
      generationError: "",
      generationJobId: "",
    });
  });
});
