import { describe, expect, it } from "vitest";

import {
  buildSelectionSummary,
  normalizeBatch,
  normalizeDraft,
} from "@/lib/shein-studio/storage-shared";

describe("buildSelectionSummary", () => {
  it("preserves SDS product table fields for SHEIN generation", () => {
    const productSize =
      '[[{"content":"尺码","remark":""},{"content":"衣长(cm/in)","remark":""}],[{"content":"S","remark":""},{"content":"87.5/34.45 ","remark":""}]]';
    const packagingSpecification =
      '[[{"content":"尺码"},{"content":"包装尺寸（cm）"}],[{"content":"S"},{"content":"40.0*30.0*1.0"}]]';

    const summary = buildSelectionSummary({
      productId: 1,
      parentProductId: 1,
      variantId: 2,
      prototypeGroupId: 3,
      layerId: "layer-1",
      productName: "dress",
      productSize,
      packagingSpecification,
      variantLabel: "S / black",
    });

    expect(summary?.productSize).toBe(productSize);
    expect(summary?.packagingSpecification).toBe(packagingSpecification);
  });

  it("drops heavy per-variant image arrays from saved draft payload", () => {
    const summary = buildSelectionSummary({
      productId: 1,
      parentProductId: 1,
      variantId: 2,
      prototypeGroupId: 3,
      layerId: "layer-1",
      productName: "tee",
      variantLabel: "M / black",
      printableWidth: 1000,
      printableHeight: 1000,
      mockupImageUrls: ["https://example.com/a.jpg", "https://example.com/b.jpg"],
      selectedVariantIds: [2, 3],
      variants: [
        {
          variantId: 2,
          variantSku: "SKU-2",
          size: "M",
          color: "black",
          mockupImageUrl: "https://example.com/m.jpg",
          mockupImageUrls: [
            "https://example.com/variant-a.jpg",
            "https://example.com/variant-b.jpg",
          ],
          templateImageUrl: "https://example.com/template.jpg",
        },
      ],
    });

    expect(summary?.mockupImageUrls).toEqual([
      "https://example.com/a.jpg",
      "https://example.com/b.jpg",
    ]);
    expect(summary?.variants).toEqual([
      {
        variantId: 2,
        variantSku: "SKU-2",
        size: "M",
        color: "black",
        price: undefined,
        weight: undefined,
        boxLength: undefined,
        boxWidth: undefined,
        boxHeight: undefined,
        productionCycle: undefined,
        prototypeGroupId: undefined,
        layerId: undefined,
        mockupImageUrl: "https://example.com/m.jpg",
      },
    ]);
    expect(summary?.variants?.[0]).not.toHaveProperty("mockupImageUrls");
    expect(summary?.variants?.[0]).not.toHaveProperty("templateImageUrl");
  });
});

describe("normalizeDraft", () => {
  it("normalizes explicit multi-group drafts", () => {
    const draft = normalizeDraft({
      prompt: "legacy top-level prompt",
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
            variantId: 100,
            parentProductId: 1,
            productId: 1,
            prototypeGroupId: 200,
            layerId: "layer-1",
            productName: "tee",
            variantLabel: "M / black",
          },
          groupedSelections: [],
          sheinStoreId: "869",
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
          updatedAt: "2026-05-26T00:00:00Z",
        },
      ],
      updatedAt: "2026-05-26T00:00:00Z",
    });

    expect(draft?.groups).toHaveLength(1);
    expect(draft?.groups?.[0].currentPrompt).toBe("prompt a");
    expect(draft?.groups?.[0].promptHistory).toEqual([
      {
        prompt: "prompt old",
        groupedImageMode: "shared_by_size",
        createdAt: "2026-05-26T00:00:00Z",
      },
    ]);
  });

  it("synthesizes one group from legacy groupedSelections drafts", () => {
    const draft = normalizeDraft({
      prompt: "legacy prompt",
      groupedSelections: [
        {
          selectionId: "1:200:101:layer-2:101",
          selection: {
            variantId: 101,
            parentProductId: 1,
            productId: 1,
            prototypeGroupId: 200,
            layerId: "layer-2",
            productName: "hoodie",
            variantLabel: "L / white",
          },
          sheinStoreId: "869",
          baselineStatus: "ready",
          baselineReason: "",
          eligible: true,
        },
      ],
      selection: {
        variantId: 100,
        parentProductId: 1,
        productId: 1,
        prototypeGroupId: 200,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
      selectedIds: ["design-1"],
      designs: [],
      createdTasks: [],
      updatedAt: "2026-05-26T00:00:00Z",
    });

    expect(draft?.groups).toHaveLength(1);
    expect(draft?.groups?.[0].currentPrompt).toBe("legacy prompt");
    expect(draft?.groups?.[0].groupedSelections).toHaveLength(1);
    expect(draft?.groups?.[0].primarySelection.variantId).toBe(100);
  });

  it("restores hot style reference fields from persisted drafts", () => {
    const draft = normalizeDraft({
      prompt: "legacy prompt",
      hotStyleReferenceImageUrls: ["https://example.com/ref.png"],
      hotStyleReferenceBrief: "retro badge",
      hotStyleReferencePrompt: "original retro badge",
      updatedAt: "2026-05-26T00:00:00Z",
    });

    expect(draft?.hotStyleReferenceImageUrls).toEqual([
      "https://example.com/ref.png",
    ]);
    expect(draft?.hotStyleReferenceBrief).toBe("retro badge");
    expect(draft?.hotStyleReferencePrompt).toBe("original retro badge");
  });
});

describe("normalizeBatch", () => {
  it("preserves persisted design counts from server batch summaries", () => {
    const batch = normalizeBatch({
      id: "batch-1",
      name: "869全品类",
      prompt: "prompt",
      styleCount: "4",
      sheinStoreId: "869",
      persistedDesignCount: 58,
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-06-18T17:04:49.413822Z",
    });

    expect(batch).toMatchObject({
      id: "batch-1",
      persistedDesignCount: 58,
    });
  });

  it("restores hot style reference fields from persisted batches", () => {
    const batch = normalizeBatch({
      id: "batch-1",
      name: "869全品类",
      prompt: "prompt",
      styleCount: "4",
      hot_style_reference_image_urls: [
        "https://example.com/ref.png",
        "https://example.com/ref-2.png",
      ],
      hot_style_reference_brief: "retro badge",
      hot_style_reference_prompt: "original retro badge",
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-06-18T17:04:49.413822Z",
    } as never);

    expect(batch).toMatchObject({
      id: "batch-1",
      hotStyleReferenceImageUrls: ["https://example.com/ref.png"],
      hotStyleReferenceBrief: "retro badge",
      hotStyleReferencePrompt: "original retro badge",
    });
  });
});
