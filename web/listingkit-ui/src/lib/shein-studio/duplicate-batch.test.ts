import { describe, expect, it } from "vitest";

import { buildDuplicatedSheinStudioBatchInput } from "@/lib/shein-studio/duplicate-batch";

describe("buildDuplicatedSheinStudioBatchInput", () => {
  it("preserves selections and prompt settings while clearing workflow progress", () => {
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
      selection: { variantId: 123, productId: 456, title: "Primary" },
      groupedSelections: [
        {
          id: "grouped-1",
          sheinStoreId: "869",
          eligible: true,
          eligibilityReason: "",
          selection: { variantId: 124, productId: 457, title: "Grouped" },
        },
      ],
      groups: [
        {
          id: "group-1",
          name: "Group 1",
          primarySelection: { variantId: 123, productId: 456, title: "Primary" },
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
