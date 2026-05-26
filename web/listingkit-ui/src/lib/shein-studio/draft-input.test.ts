import { describe, expect, it } from "vitest";

import { buildSheinStudioDraftInput } from "@/lib/shein-studio/draft-input";

describe("buildSheinStudioDraftInput", () => {
  it("builds a save payload with generated designs", () => {
    const payload = buildSheinStudioDraftInput({
      prompt: "retro cherries",
      styleCount: "2",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "hero image",
      productImagePrompts: [{ role: "main", label: "主图", prompt: "white bg" }],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "7",
      imageStrategy: "ai_generated",
      selectedSdsImages: [
        {
          imageUrl: "https://example.com/sds-main.jpg",
          color: "Black",
          variantSku: "SKU-BLK",
        },
      ],
      renderSizeImagesWithSds: true,
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 2,
        variants: [
          {
            variantId: 2,
            templateImageUrl: "https://example.com/template.jpg",
            mockupImageUrls: [
              "https://example.com/mockup-a.jpg",
              "https://example.com/mockup-b.jpg",
            ],
          },
        ],
        prototypeGroupId: 3,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
      groupedSelections: [
        {
          selectionId: "1:3:5:layer-2:5",
          selection: {
            productId: 4,
            parentProductId: 1,
            variantId: 5,
            prototypeGroupId: 3,
            layerId: "layer-2",
            productName: "hoodie",
            variantLabel: "L / white",
          },
          baselineStatus: "ready",
          baselineReason: "",
          sheinStoreId: "9",
          eligible: true,
        },
      ],
      designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
      selectedIds: ["design-1"],
      createdTasks: [],
    });

    expect(payload.designs).toEqual([
      { id: "design-1", imageUrl: "https://example.com/design.png" },
    ]);
    expect(payload.selectedIds).toEqual(["design-1"]);
    expect(payload.selectedSdsImages).toEqual([
      {
        imageUrl: "https://example.com/sds-main.jpg",
        color: "Black",
        variantSku: "SKU-BLK",
      },
    ]);
    expect(payload.selection?.variantId).toBe(2);
    expect(payload.selection?.variants).toEqual([
      {
        variantId: 2,
        variantSku: undefined,
        size: undefined,
        color: undefined,
        price: undefined,
        weight: undefined,
        boxLength: undefined,
        boxWidth: undefined,
        boxHeight: undefined,
        productionCycle: undefined,
        prototypeGroupId: undefined,
        layerId: undefined,
        mockupImageUrl: undefined,
      },
    ]);
    expect(payload.selection?.variants?.[0]).not.toHaveProperty("mockupImageUrls");
    expect(payload.selection?.variants?.[0]).not.toHaveProperty("templateImageUrl");
    expect(payload.groupedSelections).toEqual([
      expect.objectContaining({
        selectionId: "1:3:5:layer-2:5",
        sheinStoreId: "9",
        baselineStatus: "ready",
        selection: expect.objectContaining({
          variantId: 5,
          productName: "hoodie",
        }),
      }),
    ]);
  });
});
