import { describe, expect, it } from "vitest";

import { buildSheinStudioDraftInput } from "@/lib/shein-studio/draft-input";

describe("buildSheinStudioDraftInput", () => {
  it("omits compatibility-era flat result fields from the top-level payload", () => {
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
      groupedImageMode: "shared_by_size",
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
      generationJobs: [{ jobId: "job-1", status: "queued" }],
    });

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
    expect(payload.groups).toEqual(undefined);
    expect(payload).not.toHaveProperty("designs");
    expect(payload).not.toHaveProperty("selectedIds");
    expect(payload).not.toHaveProperty("createdTasks");
    expect(payload).not.toHaveProperty("generationJobs");
  });

  it("omits compatibility-era flat result fields from grouped workspace payloads", () => {
    const payload = buildSheinStudioDraftInput({
      prompt: "top-level prompt",
      styleCount: "2",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "",
      transparentBackground: false,
      sheinStoreId: "7",
      imageStrategy: "sds_official",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selection: {
        productId: 1,
        parentProductId: 1,
        variantId: 100,
        prototypeGroupId: 3,
        layerId: "layer-1",
        productName: "tee",
        variantLabel: "M / black",
      },
      groupedSelections: [],
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          primarySelection: {
            productId: 1,
            parentProductId: 1,
            variantId: 100,
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
          sheinStoreId: "9",
          imageStrategy: "sds_official",
          groupedImageMode: "shared_by_size",
          selectedSdsImages: [],
          renderSizeImagesWithSds: true,
          currentPrompt: "prompt a",
          promptHistory: [
            {
              prompt: "prompt old",
              groupedImageMode: "shared_by_size",
              createdAt: "2026-05-26T00:00:00Z",
            },
          ],
          productImageCount: "5",
          productImagePrompt: "",
          productImagePrompts: [],
          artworkModel: "",
          transparentBackground: false,
          variationIntensity: "medium",
          designs: [],
          selectedIds: [],
          createdTasks: [],
          generationJobs: [{ jobId: "group-job-1", status: "queued" }],
          updatedAt: "2026-05-26T00:00:00Z",
        },
      ],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      generationJobs: [],
    });

    expect(payload.groups).toEqual([
      expect.objectContaining({
        id: "group-1",
        currentPrompt: "prompt a",
        promptHistory: [
          {
            prompt: "prompt old",
            groupedImageMode: "shared_by_size",
            createdAt: "2026-05-26T00:00:00Z",
          },
        ],
        primarySelection: expect.objectContaining({
          variantId: 100,
        }),
        groupedSelections: [
          expect.objectContaining({
            selectionId: "1:3:5:layer-2:5",
          }),
        ],
      }),
    ]);
    expect(payload.groups?.[0]).not.toHaveProperty("designs");
    expect(payload.groups?.[0]).not.toHaveProperty("selectedIds");
    expect(payload.groups?.[0]).not.toHaveProperty("createdTasks");
    expect(payload.groups?.[0]).not.toHaveProperty("generationJobs");
  });
});
