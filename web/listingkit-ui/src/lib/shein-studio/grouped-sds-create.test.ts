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
  printableWidth: 1200,
  printableHeight: 1200,
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

const sameSizeDesign: SheinStudioGeneratedDesign = {
  ...baseDesign,
  id: "design-shared",
  targetGroupKey: "size:1200x1200",
  targetGroupLabel: "1200 x 1200",
};

const secondSizeDesign: SheinStudioGeneratedDesign = {
  ...baseDesign,
  id: "design-second-size",
  targetGroupKey: "size:1400x1000",
  targetGroupLabel: "1400 x 1000",
};

const perProductDesign: SheinStudioGeneratedDesign = {
  ...baseDesign,
  id: "design-product-101",
  targetGroupKey: "9001:7001:101:layer-1:101",
  targetGroupLabel: "Baseline Product",
};

const secondProductDesign: SheinStudioGeneratedDesign = {
  ...baseDesign,
  id: "design-product-102",
  targetGroupKey: "9001:7001:102:layer-1:102",
  targetGroupLabel: "Black / L",
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

  it("reuses the same designs for selections with the same printable size", async () => {
    const createReviewTasks = vi.fn().mockResolvedValue([]);

    await createGroupedSheinReviewTasks({
      prompt: "Grouped prompt",
      groupedImageMode: "shared_by_size",
      groups: [
        {
          sheinStoreId: "869",
          selections: [
            {
              selection: {
                ...baseSelection,
                printableWidth: 1200,
                printableHeight: 1200,
              },
              baselineStatus: "ready",
            },
            {
              selection: {
                ...secondSelection,
                printableWidth: 1200,
                printableHeight: 1200,
              },
              baselineStatus: "ready",
            },
          ],
          designs: [sameSizeDesign, secondSizeDesign],
          selectedIds: [sameSizeDesign.id, secondSizeDesign.id],
        },
      ],
      createReviewTasks,
    });

    expect(createReviewTasks).toHaveBeenCalledTimes(2);
    expect(createReviewTasks).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        selection: expect.objectContaining({ variantId: 101 }),
        designs: [sameSizeDesign],
        selectedIds: [sameSizeDesign.id],
      }),
    );
    expect(createReviewTasks).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        selection: expect.objectContaining({ variantId: 102 }),
        designs: [sameSizeDesign],
        selectedIds: [sameSizeDesign.id],
      }),
    );
  });

  it("uses product-specific designs when grouped image mode is per product", async () => {
    const createReviewTasks = vi.fn().mockResolvedValue([]);

    await createGroupedSheinReviewTasks({
      prompt: "Grouped prompt",
      groupedImageMode: "per_product",
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
          designs: [perProductDesign, secondProductDesign],
          selectedIds: [perProductDesign.id, secondProductDesign.id],
        },
      ],
      createReviewTasks,
    });

    expect(createReviewTasks).toHaveBeenCalledTimes(2);
    expect(createReviewTasks).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        selection: baseSelection,
        designs: [perProductDesign],
        selectedIds: [perProductDesign.id],
      }),
    );
    expect(createReviewTasks).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        selection: secondSelection,
        designs: [secondProductDesign],
        selectedIds: [secondProductDesign.id],
      }),
    );
  });
});
