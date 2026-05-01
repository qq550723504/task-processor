import { describe, expect, it } from "vitest";

import { buildSheinStudioDraftInput } from "@/lib/shein-studio/draft-input";

describe("buildSheinStudioDraftInput", () => {
  it("builds a save payload with generated designs", () => {
    const payload = buildSheinStudioDraftInput({
      prompt: "retro cherries",
      styleCount: "2",
      productImageCount: "5",
      productImagePrompt: "hero image",
      productImagePrompts: [{ role: "main", label: "主图", prompt: "white bg" }],
      artworkModel: "nanobanana",
      transparentBackground: false,
      sheinStoreId: "7",
      imageStrategy: "ai_generated",
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
      designs: [{ id: "design-1", imageUrl: "https://example.com/design.png" }],
      selectedIds: ["design-1"],
      createdTasks: [],
    });

    expect(payload.designs).toEqual([
      { id: "design-1", imageUrl: "https://example.com/design.png" },
    ]);
    expect(payload.selectedIds).toEqual(["design-1"]);
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
  });
});
