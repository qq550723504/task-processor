import { describe, expect, it } from "vitest";

import { buildDuplicatedSheinStudioBatchInput } from "@/lib/shein-studio/duplicate-batch";

describe("buildDuplicatedSheinStudioBatchInput", () => {
  it("preserves selections and prompt settings while clearing workflow progress", () => {
    const primarySelection = {
      productId: 456,
      parentProductId: 456,
      variantId: 123,
      prototypeGroupId: 901,
      layerId: "layer-primary",
      productName: "Primary",
      variantLabel: "Primary / Default",
    };
    const groupedSelection = {
      productId: 457,
      parentProductId: 457,
      variantId: 124,
      prototypeGroupId: 902,
      layerId: "layer-grouped",
      productName: "Grouped",
      variantLabel: "Grouped / Default",
    };

    const duplicated = buildDuplicatedSheinStudioBatchInput({
      id: "batch-1",
      name: "Batch One",
      prompt: "retro cherries",
      styleCount: "2",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "scene",
      productImagePrompts: [],
      artworkModel: "nano",
      transparentBackground: false,
      sheinStoreId: "869",
      imageStrategy: "sds_official",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [{ imageUrl: "https://example.com/1.png" }],
      renderSizeImagesWithSds: true,
      selection: primarySelection,
      groupedSelections: [
        {
          selectionId: "grouped-1",
          baselineStatus: "ready",
          baselineReason: "",
          sheinStoreId: "869",
          eligible: true,
          eligibilityReason: "",
          selection: groupedSelection,
        },
      ],
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          primarySelection,
          groupedSelections: [],
          sheinStoreId: "869",
          currentPrompt: "retro cherries",
          promptHistory: [],
          designs: [{ id: "design-1", imageUrl: "https://example.com/d.png" }],
          selectedIds: ["design-1"],
          createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
          updatedAt: "2026-05-29T21:10:00.000Z",
        },
      ],
      designs: [{ id: "design-1", imageUrl: "https://example.com/d.png" }],
      selectedIds: ["design-1"],
      createdTasks: [{ id: "task-1", title: "Task 1", designId: "design-1" }],
      updatedAt: "2026-05-29T21:10:00.000Z",
    });

    expect(duplicated).toMatchObject({
      id: undefined,
      name: "Batch One 副本",
      prompt: "retro cherries",
      selection: { variantId: 123 },
      groupedSelections: [
        {
          selection: { variantId: 124 },
        },
      ],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      groups: [
        {
          primarySelection: { variantId: 123 },
          currentPrompt: "retro cherries",
          designs: [],
          selectedIds: [],
          createdTasks: [],
        },
      ],
    });
  });
});
