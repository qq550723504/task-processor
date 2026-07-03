import { describe, expect, it } from "vitest";

import {
  resolveDedicatedBatchHydration,
  type DedicatedBatchLocalSnapshot,
} from "@/lib/shein-studio/batch-hydration";
import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import type {
  SheinStudioBatchDetail,
  SheinStudioDraft,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";
import type { SDSProductVariantSelection } from "@/lib/types/sds";

const selection: SDSProductVariantSelection = {
  productId: 100,
  parentProductId: 100,
  variantId: 101,
  prototypeGroupId: 1,
  layerId: "layer-1",
  productName: "T-shirt",
  variantLabel: "L / white",
};

function buildGroupedSelection(selectionId: string, sheinStoreId: string) {
  return {
    selectionId,
    selection,
    baselineStatus: "ready" as const,
    baselineReason: "",
    sheinStoreId,
    eligible: true,
  };
}

function buildSavedBatch(
  overrides: Partial<SheinStudioSavedBatch> = {},
): SheinStudioSavedBatch {
  return {
    id: "batch-1",
    name: "Remote batch",
    prompt: "remote prompt",
    styleCount: "1",
    variationIntensity: "medium",
    productImageCount: "5",
    productImagePrompt: "remote product prompt",
    productImagePrompts: [
      { role: "front", label: "Front", prompt: "remote product prompt" },
    ],
    artworkModel: "nanobanana",
    transparentBackground: false,
    sheinStoreId: "869",
    imageStrategy: "ai_generated",
    groupedImageMode: "shared_by_size",
    selectedSdsImages: [{ imageUrl: "remote.png" }],
    renderSizeImagesWithSds: true,
    groupedSelections: [
      buildGroupedSelection("remote-selection", "869"),
    ],
    groups: [
      {
        id: "remote-group",
        name: "Remote group",
        primarySelection: selection,
        sheinStoreId: "869",
        selectedSdsImages: [],
        groupedSelections: [],
        renderSizeImagesWithSds: true,
        currentPrompt: "remote group prompt",
        promptHistory: [],
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: "nanobanana",
        transparentBackground: false,
        variationIntensity: "medium",
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-06-22T00:00:00.000Z",
      },
    ],
    designs: [],
    selectedIds: ["remote-design"],
    createdTasks: [],
    draftUpdatedAt: "2026-06-22T00:00:00.000Z",
    updatedAt: "2026-06-22T00:00:00.000Z",
    ...overrides,
  };
}

function buildDetail(
  savedBatch: SheinStudioSavedBatch,
): SheinStudioBatchDetail {
  return {
    batch: {
      id: savedBatch.id,
      status: "draft",
      prompt: savedBatch.prompt,
      styleCount: savedBatch.styleCount,
      sheinStoreId: Number(savedBatch.sheinStoreId || "0"),
      variationIntensity: savedBatch.variationIntensity,
      artworkModel: savedBatch.artworkModel,
      transparentBackground: savedBatch.transparentBackground,
      groupedImageMode: savedBatch.groupedImageMode,
      selectedSdsImages: savedBatch.selectedSdsImages,
      selection: savedBatch.selection,
      groupedSelections: savedBatch.groupedSelections,
      createdAt: "2026-06-22T00:00:00.000Z",
      draftUpdatedAt: savedBatch.draftUpdatedAt,
      updatedAt: savedBatch.updatedAt,
    },
    items: [],
    createdTasks: [],
  };
}

function buildHydratedBatch(
  overrides: Partial<SheinStudioSavedBatch> = {},
): SheinStudioWorkbenchHydratedBatch {
  const savedBatch = buildSavedBatch(overrides);
  return {
    savedBatch,
    detail: buildDetail(savedBatch),
  };
}

function buildLocalDraft(overrides: Partial<SheinStudioDraft> = {}): SheinStudioDraft {
  return {
    prompt: "local prompt",
    styleCount: "3",
    variationIntensity: "strong",
    productImageCount: "8",
    productImagePrompt: "local product prompt",
    productImagePrompts: [
      { role: "front", label: "Front", prompt: "local product prompt" },
    ],
    artworkModel: "gpt-image-1",
    transparentBackground: true,
    sheinStoreId: "870",
    imageStrategy: "hybrid",
    groupedImageMode: "per_product",
    selectedSdsImages: [{ imageUrl: "local.png" }],
    renderSizeImagesWithSds: false,
    groupedSelections: [
      buildGroupedSelection("local-selection", "870"),
    ],
    groups: [
      {
        id: "local-group",
        name: "Local group",
        primarySelection: selection,
        sheinStoreId: "870",
        selectedSdsImages: [],
        groupedSelections: [],
        renderSizeImagesWithSds: false,
        currentPrompt: "local group prompt",
        promptHistory: [],
        productImageCount: "8",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: "gpt-image-1",
        transparentBackground: true,
        variationIntensity: "strong",
        designs: [],
        selectedIds: [],
        createdTasks: [],
        updatedAt: "2026-06-22T00:00:01.000Z",
      },
    ],
    designs: [],
    selectedIds: ["local-design"],
    createdTasks: [],
    updatedAt: "2026-06-22T00:00:01.000Z",
    ...overrides,
  };
}

function buildLocalSnapshot(
  overrides: Partial<DedicatedBatchLocalSnapshot> = {},
): DedicatedBatchLocalSnapshot {
  return {
    batchId: "batch-1",
    draft: buildLocalDraft(),
    ...overrides,
  };
}

describe("dedicated batch hydration", () => {
  it("keeps the remote batch when the local snapshot belongs to another batch", () => {
    const hydratedBatch = buildHydratedBatch();

    const result = resolveDedicatedBatchHydration({
      batchId: "batch-1",
      hydratedBatch,
      localSnapshot: buildLocalSnapshot({ batchId: "batch-2" }),
    });

    expect(result).toEqual(hydratedBatch);
  });

  it("merges a newer local snapshot into the dedicated batch without losing remote fallback fields", () => {
    const hydratedBatch = buildHydratedBatch({
      draftUpdatedAt: "2026-06-22T00:00:00.000Z",
    });

    const result = resolveDedicatedBatchHydration({
      batchId: "batch-1",
      hydratedBatch,
      localSnapshot: buildLocalSnapshot({
        draft: buildLocalDraft({
          prompt: "   ",
          sheinStoreId: "",
          groupedSelections: [],
          updatedAt: "2026-06-22T00:00:01.000Z",
        }),
      }),
    });

    expect(result.detail).toBe(hydratedBatch.detail);
    expect(result.savedBatch.prompt).toBe("remote prompt");
    expect(result.savedBatch.styleCount).toBe("3");
    expect(result.savedBatch.sheinStoreId).toBe("869");
    expect(result.savedBatch.groupedSelections).toEqual(
      hydratedBatch.savedBatch.groupedSelections,
    );
    expect(result.savedBatch.groups).toEqual([
      expect.objectContaining({ id: "local-group" }),
    ]);
  });

  it("merges hot style reference fields from a newer local snapshot", () => {
    const hydratedBatch = buildHydratedBatch({
      hotStyleReferenceImageUrls: ["https://example.com/remote-ref.png"],
      hotStyleReferenceBrief: "remote reference brief",
      hotStyleReferencePrompt: "remote reference prompt",
      draftUpdatedAt: "2026-06-22T00:00:00.000Z",
    });

    const result = resolveDedicatedBatchHydration({
      batchId: "batch-1",
      hydratedBatch,
      localSnapshot: buildLocalSnapshot({
        draft: buildLocalDraft({
          hotStyleReferenceImageUrls: ["https://example.com/local-ref.png"],
          hotStyleReferenceBrief: "local reference brief",
          hotStyleReferencePrompt: "local reference prompt",
          updatedAt: "2026-06-22T00:00:01.000Z",
        }),
      }),
    });

    expect(result.savedBatch.hotStyleReferenceImageUrls).toEqual([
      "https://example.com/local-ref.png",
    ]);
    expect(result.savedBatch.hotStyleReferenceBrief).toBe(
      "local reference brief",
    );
    expect(result.savedBatch.hotStyleReferencePrompt).toBe(
      "local reference prompt",
    );
  });

  it("keeps cleared hot style reference fields from a newer local snapshot", () => {
    const hydratedBatch = buildHydratedBatch({
      hotStyleReferenceImageUrls: ["https://example.com/remote-ref.png"],
      hotStyleReferenceBrief: "remote reference brief",
      hotStyleReferencePrompt: "remote reference prompt",
      draftUpdatedAt: "2026-06-22T00:00:00.000Z",
    });

    const result = resolveDedicatedBatchHydration({
      batchId: "batch-1",
      hydratedBatch,
      localSnapshot: buildLocalSnapshot({
        draft: buildLocalDraft({
          hotStyleReferenceImageUrls: [],
          hotStyleReferenceBrief: "",
          hotStyleReferencePrompt: "",
          updatedAt: "2026-06-22T00:00:01.000Z",
        }),
      }),
    });

    expect(result.savedBatch.hotStyleReferenceImageUrls).toEqual([]);
    expect(result.savedBatch.hotStyleReferenceBrief).toBe("");
    expect(result.savedBatch.hotStyleReferencePrompt).toBe("");
  });

  it("keeps the remote batch when the remote draft is newer", () => {
    const hydratedBatch = buildHydratedBatch({
      prompt: "new remote prompt",
      draftUpdatedAt: "2026-06-22T00:00:02.000Z",
    });

    const result = resolveDedicatedBatchHydration({
      batchId: "batch-1",
      hydratedBatch,
      localSnapshot: buildLocalSnapshot({
        draft: buildLocalDraft({
          prompt: "old local prompt",
          updatedAt: "2026-06-22T00:00:01.000Z",
        }),
      }),
    });

    expect(result).toEqual(hydratedBatch);
  });

  it("applies the dedicated prompt override even when the local snapshot is stale", () => {
    const hydratedBatch = buildHydratedBatch({
      prompt: "remote prompt",
      draftUpdatedAt: "2026-06-22T00:00:02.000Z",
    });

    const result = resolveDedicatedBatchHydration({
      batchId: "batch-1",
      hydratedBatch,
      localSnapshot: buildLocalSnapshot({
        draft: buildLocalDraft({
          prompt: "stale local prompt",
          updatedAt: "2026-06-22T00:00:01.000Z",
        }),
      }),
      promptOverride: "prompt from route override",
    });

    expect(result.savedBatch.prompt).toBe("prompt from route override");
    expect(result.savedBatch.styleCount).toBe("1");
  });
});
