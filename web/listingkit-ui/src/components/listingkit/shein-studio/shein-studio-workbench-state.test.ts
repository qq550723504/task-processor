import { describe, expect, it } from "vitest";

import {
  applySheinStudioWorkbenchBatch,
  applySheinStudioWorkbenchDraft,
  buildInitialSheinStudioWorkbenchState,
  setSheinStudioWorkbenchField,
  sheinStudioWorkbenchReducer,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import {
  DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
  DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
  DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
  DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
} from "@/lib/shein-studio/storage-shared";
import { DEFAULT_SHEIN_STORE_ID } from "@/lib/shein-studio/create-review-tasks";

describe("buildInitialSheinStudioWorkbenchState", () => {
  it("centralizes the default values used by the workbench", () => {
    expect(buildInitialSheinStudioWorkbenchState()).toMatchObject({
      artworkModel: DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
      imageStrategy: DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
      productImageCount: DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
      variationIntensity: DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
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
        selectedSdsImages: [],
        renderSizeImagesWithSds: false,
        designs: [{ id: "design-1", imageUrl: "https://example.com/1.png" }],
        selectedIds: ["design-1"],
        createdTasks: [],
        galleryRatioCheck: null,
      }),
    );

    expect(next.prompt).toBe("draft prompt");
    expect(next.designs).toHaveLength(1);
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
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-05-15T00:00:00.000Z",
      }),
    );

    expect(next.prompt).toBe("batch prompt");
    expect(next.sheinStoreId).toBe(DEFAULT_SHEIN_STORE_ID);
    expect(next.imageStrategy).toBe(DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY);
  });
});
