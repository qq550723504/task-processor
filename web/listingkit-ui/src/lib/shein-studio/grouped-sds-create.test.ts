import { describe, expect, it, vi } from "vitest";

import { createGroupedSheinReviewTasks } from "@/lib/shein-studio/create-review-tasks";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { SheinStudioGeneratedDesign } from "@/lib/types/shein-studio";

const baseSelection: SDSProductVariantSelection = {
  productId: 1001,
  parentProductId: 9001,
  variantId: 101,
  variants: [
    {
      variantId: 101,
      variantSku: "SKU-101",
      color: "Black",
    },
  ],
  selectedVariantIds: [101],
  prototypeGroupId: 7001,
  layerId: "layer-1",
  productName: "Baseline Product",
  variantLabel: "Black / M",
};

const secondSelection: SDSProductVariantSelection = {
  ...baseSelection,
  variantId: 102,
  selectedVariantIds: [102],
  variantLabel: "Black / L",
  variants: [
    {
      variantId: 102,
      variantSku: "SKU-102",
      color: "Black",
    },
  ],
};

const baseDesign: SheinStudioGeneratedDesign = {
  id: "design-1",
  imageUrl: "https://example.com/design-1.png",
  sourceWidth: 1200,
  sourceHeight: 1200,
};

describe("createGroupedSheinReviewTasks", () => {
  it("rejects grouped create when a selection is missing baseline readiness", async () => {
    await expect(
      createGroupedSheinReviewTasks({
        prompt: "Grouped prompt",
        groups: [
          {
            sheinStoreId: "869",
            selections: [
              {
                selection: baseSelection,
                baselineStatus: "missing",
                baselineReason: "Not ready yet",
              },
            ],
            designs: [baseDesign],
            selectedIds: [baseDesign.id],
          },
        ],
      }),
    ).rejects.toThrow("baseline ready");
  });

  it("calls through to createSheinReviewTasks for each ready grouped selection", async () => {
    const createReviewTasks = vi
      .fn()
      .mockResolvedValue([{ id: "task-1", title: "Style 1", designId: baseDesign.id }]);

    const result = await createGroupedSheinReviewTasks({
      prompt: "Grouped prompt",
      groups: [
        {
          sheinStoreId: "869",
          selections: [
            {
              selection: baseSelection,
              baselineStatus: "ready",
            },
            {
              selection: secondSelection,
              baselineStatus: "ready",
            },
          ],
          designs: [baseDesign],
          selectedIds: [baseDesign.id],
        },
      ],
      createReviewTasks,
    });

    expect(createReviewTasks).toHaveBeenCalledTimes(2);
    expect(createReviewTasks).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        prompt: "Grouped prompt",
        sheinStoreId: "869",
        selection: baseSelection,
        designs: [baseDesign],
        selectedIds: [baseDesign.id],
      }),
    );
    expect(createReviewTasks).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        prompt: "Grouped prompt",
        sheinStoreId: "869",
        selection: secondSelection,
        designs: [baseDesign],
        selectedIds: [baseDesign.id],
      }),
    );
    expect(result).toEqual([
      { id: "task-1", title: "Style 1", designId: baseDesign.id },
      { id: "task-1", title: "Style 1", designId: baseDesign.id },
    ]);
  });
});
