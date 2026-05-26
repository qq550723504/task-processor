import { describe, expect, it } from "vitest";

import {
  applySheinStudioWorkbenchBatch,
  applySheinStudioWorkbenchDraft,
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
      imageStrategy: DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
      groups: [],
      groupedImageMode: DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
      productImageCount: DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
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
});
