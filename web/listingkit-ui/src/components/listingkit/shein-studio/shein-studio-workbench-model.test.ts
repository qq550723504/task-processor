import { describe, expect, it } from "vitest";

import {
  buildSheinStudioGenerateRequest,
  buildSheinStudioSelectedVariants,
  getSheinStudioCreateActionDisabledReason,
  mergeSheinStudioDraftState,
  sheinStudioBusyMessage,
  summarizeSheinStudioSelection,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";

describe("shein studio workbench model", () => {
  it("summarizes explicit SDS variants before falling back to selected ids", () => {
    const summary = summarizeSheinStudioSelection({
      productId: 1,
      parentProductId: 1,
      variantId: 10,
      variants: [
        { variantId: 10, color: "Black", size: "M" },
        { variantId: 11, color: "White", size: "L" },
      ],
      selectedVariantIds: [10, 11, 12],
      prototypeGroupId: 2,
      layerId: "layer",
      productName: "tee",
      variantLabel: "M / Black",
      printableWidth: 1200,
      printableHeight: 1600,
    });

    expect(summary.printableAreaLabel).toBe("1200 × 1600px");
    expect(summary.selectedVariants).toHaveLength(2);
    expect(summary.selectedColorCount).toBe(2);
    expect(summary.selectedSizeCount).toBe(2);
  });

  it("builds a single fallback variant from the selected SDS variant", () => {
    expect(
      buildSheinStudioSelectedVariants({
        productId: 1,
        parentProductId: 1,
        variantId: 10,
        prototypeGroupId: 2,
        layerId: "layer",
        productName: "tee",
        variantLabel: "M / Black",
      }),
    ).toEqual([{ variantId: 10, size: "M / Black", color: "默认" }]);
  });

  it("merges a gallery handoff into an existing draft once", () => {
    const state = mergeSheinStudioDraftState({
      draft: {
        prompt: "saved prompt",
        styleCount: "2",
        variationIntensity: "medium",
        productImageCount: "5",
        productImagePrompt: "",
        productImagePrompts: [],
        artworkModel: "nanobanana",
        transparentBackground: false,
        sheinStoreId: "869",
        imageStrategy: "hybrid",
        selectedSdsImages: [{ imageUrl: "https://example.com/sds.jpg" }],
        renderSizeImagesWithSds: true,
        designs: [{ id: "design-1", imageUrl: "https://example.com/a.png" }],
        selectedIds: ["design-1"],
        createdTasks: [],
        updatedAt: "2026-05-10T00:00:00.000Z",
      },
      galleryDesign: {
        id: "gallery-1",
        imageUrl: "https://example.com/gallery.png",
      },
      galleryPrompt: "gallery prompt",
    });

    expect(state.prompt).toBe("saved prompt");
    expect(state.designs.map((item) => item.id)).toEqual(["design-1", "gallery-1"]);
    expect(state.selectedIds).toEqual(["design-1", "gallery-1"]);
    expect(state.hasCustomizedSdsSelection).toBe(true);
    expect(state.importedGalleryDesign).toBe(true);
  });

  it("returns the create-task disabled reason from the first blocking condition", () => {
    expect(
      getSheinStudioCreateActionDisabledReason({
        selectedIds: ["design-1"],
      }),
    ).toBe("请先选择 SDS 商品变体。生成 SHEIN 资料前需要锁定商品模板。");

    expect(
      getSheinStudioCreateActionDisabledReason({
        selection: {
          productId: 1,
          parentProductId: 1,
          variantId: 10,
          prototypeGroupId: 2,
          layerId: "layer",
          productName: "tee",
          variantLabel: "M / Black",
        },
        galleryRatioCheck: { status: "blocking", message: "比例不匹配" },
        selectedIds: ["design-1"],
      }),
    ).toBe("比例不匹配");
  });

  it("builds generation requests with transparent-background model override", () => {
    expect(
      buildSheinStudioGenerateRequest({
        artworkModel: "nanobanana",
        prompt: " retro cherries ",
        printableWidth: 1000,
        printableHeight: 1200,
        productReferenceImageUrls: ["https://example.com/reference.jpg"],
        styleCount: 2,
        transparentBackground: true,
        variationIntensity: "strong",
      }),
    ).toEqual({
      prompt: "retro cherries\n\nprintable size: 1000x1200px.",
      count: 2,
      variationIntensity: "strong",
      printableWidth: 1000,
      printableHeight: 1200,
      productReferenceImageUrls: ["https://example.com/reference.jpg"],
      imageModel: "gpt-image-2",
      transparentBackground: true,
    });
  });

  it("leaves image model empty so backend default can apply", () => {
    expect(
      buildSheinStudioGenerateRequest({
        artworkModel: "   ",
        prompt: "retro cherries",
        styleCount: 1,
        transparentBackground: false,
        variationIntensity: "medium",
      }),
    ).toEqual({
      prompt: "retro cherries",
      count: 1,
      variationIntensity: "medium",
      printableWidth: undefined,
      printableHeight: undefined,
      productReferenceImageUrls: undefined,
      imageModel: undefined,
      transparentBackground: false,
    });
  });

  it("does not append the SDS size twice when the prompt already contains it", () => {
    expect(
      buildSheinStudioGenerateRequest({
        artworkModel: "nanobanana",
        prompt: "retro cherries printable size: 1000x1200px.",
        printableWidth: 1000,
        printableHeight: 1200,
        styleCount: 1,
        transparentBackground: false,
        variationIntensity: "medium",
      }),
    ).toEqual({
      prompt: "retro cherries printable size: 1000x1200px.",
      count: 1,
      variationIntensity: "medium",
      printableWidth: 1000,
      printableHeight: 1200,
      productReferenceImageUrls: undefined,
      imageModel: "nanobanana",
      transparentBackground: false,
    });
  });

  it("prioritizes busy messages by active operation", () => {
    expect(
      sheinStudioBusyMessage({
        isCreatingTasks: true,
        isGenerating: true,
        regeneratingId: "design-1",
      }),
    ).toBe("正在生成款式图");
    expect(
      sheinStudioBusyMessage({
        isCreatingTasks: true,
        isGenerating: false,
        regeneratingId: "design-1",
      }),
    ).toBe("正在重新生成图片");
  });
});
