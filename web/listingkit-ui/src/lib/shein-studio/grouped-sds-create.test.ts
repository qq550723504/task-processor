import { describe, expect, it, vi } from "vitest";

import { createGroupedSheinReviewTasks } from "@/lib/shein-studio/create-review-tasks";
import {
  buildGroupedGenerationTargets,
  buildSharedCompatibilityGroupKey,
} from "@/lib/shein-studio/grouped-image-mode";
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
  designType: "material",
  productName: "Baseline Product",
  variantLabel: "Black / M",
  printableWidth: 1200,
  printableHeight: 1200,
  templateImageUrl: "https://cdn.example.com/template.png",
  maskImageUrl: "https://cdn.example.com/mask.png",
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
  targetGroupKey: "compat:0706f0914eb34a4c259c2b498fcbd160cf208d3c",
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
  it("groups shared generation targets by compatibility fingerprint instead of size only", () => {
    const sameCompatibility = {
      ...secondSelection,
      templateImageUrl: baseSelection.templateImageUrl,
      maskImageUrl: baseSelection.maskImageUrl,
    };
    const differentMask = {
      ...secondSelection,
      variantId: 103,
      selectedVariantIds: [103],
      maskImageUrl: "https://cdn.example.com/mask-b.png",
    };

    const targets = buildGroupedGenerationTargets({
      activeSelection: baseSelection,
      groupedSelections: [sameCompatibility, differentMask],
      groupedImageMode: "shared_by_size",
    });

    expect(targets).toHaveLength(2);
    expect(targets[0]).toMatchObject({
      key: "compat:0706f0914eb34a4c259c2b498fcbd160cf208d3c",
      selectionIds: [
        "9001:7001:101:layer-1:101",
        "9001:7001:102:layer-1:102",
      ],
    });
    expect(targets[1]).toMatchObject({
      key: "compat:0e695ecd33dccfcce7f4fe95171c62b97ee04f6b",
      selectionIds: ["9001:7001:103:layer-1:103"],
    });
  });

  it("falls shared generation back to one product when compatibility is incomplete", () => {
    const incomplete = { ...baseSelection, maskImageUrl: undefined };

    expect(buildSharedCompatibilityGroupKey(incomplete)).toBe("");
    expect(
      buildGroupedGenerationTargets({
        activeSelection: incomplete,
        groupedSelections: [],
        groupedImageMode: "shared_by_size",
      }),
    ).toEqual([
      expect.objectContaining({
        key: "9001:7001:101:layer-1:101",
        label: "Baseline Product · Black / M",
        selectionIds: ["9001:7001:101:layer-1:101"],
      }),
    ]);
  });

  it("rejects grouped create when a selection is not baseline validated", async () => {
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
    ).rejects.toThrow("baseline-validated");
  });

  it("rejects grouped create when a selection is only baseline cached", async () => {
    await expect(
      createGroupedSheinReviewTasks({
        prompt: "Grouped prompt",
        groups: [
          {
            sheinStoreId: "869",
            selections: [
              {
                selection: baseSelection,
                baselineStatus: "baseline_cached",
                baselineReason: "Baseline has been cached but not validated yet",
              },
            ],
            designs: [baseDesign],
            selectedIds: [baseDesign.id],
          },
        ],
      }),
    ).rejects.toThrow("Baseline has been cached but not validated yet");
  });

  it("calls through to createSheinReviewTasks for each ready grouped selection", async () => {
    const createReviewTasks = vi
      .fn()
      .mockResolvedValue([{ id: "task-1", title: "Style 1", designId: baseDesign.id }]);

    const result = await createGroupedSheinReviewTasks({
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
        prompt: "Grouped prompt",
        sheinStoreId: "869",
        selection: baseSelection,
        approvedDesigns: [perProductDesign],
      }),
    );
    expect(createReviewTasks).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        prompt: "Grouped prompt",
        sheinStoreId: "869",
        selection: secondSelection,
        approvedDesigns: [secondProductDesign],
      }),
    );
    expect(result).toEqual({
      created: [
        { id: "task-1", title: "Style 1", designId: baseDesign.id },
        { id: "task-1", title: "Style 1", designId: baseDesign.id },
      ],
      warnings: [],
    });
  });

  it("skips grouped selections that are marked ineligible", async () => {
    const createReviewTasks = vi
      .fn()
      .mockResolvedValue([{ id: "task-1", title: "Style 1", designId: baseDesign.id }]);

    const result = await createGroupedSheinReviewTasks({
      prompt: "Grouped prompt",
      groupedImageMode: "per_product",
      groups: [
        {
          sheinStoreId: "869",
          selections: [
            {
              selection: baseSelection,
              baselineStatus: "ready",
              eligible: true,
            },
            {
              selection: secondSelection,
              baselineStatus: "ready",
              eligible: false,
              eligibilityReason: "Duplicate product",
            },
          ],
          designs: [perProductDesign, secondProductDesign],
          selectedIds: [perProductDesign.id, secondProductDesign.id],
        },
      ],
      createReviewTasks,
    });

    expect(createReviewTasks).toHaveBeenCalledTimes(1);
    expect(createReviewTasks).toHaveBeenCalledWith(
      expect.objectContaining({
        selection: baseSelection,
        approvedDesigns: [perProductDesign],
      }),
    );
    expect(result).toEqual({
      created: [{ id: "task-1", title: "Style 1", designId: baseDesign.id }],
      warnings: [],
    });
  });

  it("reuses the same designs for selections with the same compatibility fingerprint", async () => {
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
                templateImageUrl: baseSelection.templateImageUrl,
                maskImageUrl: baseSelection.maskImageUrl,
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
        approvedDesigns: [sameSizeDesign],
      }),
    );
    expect(createReviewTasks).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        selection: expect.objectContaining({ variantId: 102 }),
        approvedDesigns: [sameSizeDesign],
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
        approvedDesigns: [perProductDesign],
      }),
    );
    expect(createReviewTasks).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        selection: secondSelection,
        approvedDesigns: [secondProductDesign],
      }),
    );
  });

  it("skips a grouped selection when no designs match its target instead of reusing every approved design", async () => {
    const createReviewTasks = vi.fn().mockResolvedValue([]);
    const onProgress = vi.fn();

    const result = await createGroupedSheinReviewTasks({
      prompt: "Grouped prompt",
      groupedImageMode: "per_product",
      onProgress,
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
          designs: [perProductDesign],
          selectedIds: [perProductDesign.id],
        },
      ],
      createReviewTasks,
    });

    expect(createReviewTasks).toHaveBeenCalledTimes(1);
    expect(createReviewTasks).toHaveBeenCalledWith(
      expect.objectContaining({
        selection: baseSelection,
        approvedDesigns: [perProductDesign],
      }),
    );
    expect(onProgress).toHaveBeenCalledWith(
      expect.stringContaining("no generated designs matched this product"),
    );
    expect(result.warnings).toEqual([
      expect.objectContaining({
        label: "Black / L",
        reason: "missing_design_match",
      }),
    ]);
  });

  it("skips legacy designs without explicit target metadata instead of treating them as a match", async () => {
    const createReviewTasks = vi.fn().mockResolvedValue([]);
    const onProgress = vi.fn();

    const result = await createGroupedSheinReviewTasks({
      prompt: "Grouped prompt",
      groupedImageMode: "per_product",
      onProgress,
      groups: [
        {
          sheinStoreId: "869",
          selections: [
            {
              selection: baseSelection,
              baselineStatus: "ready",
            },
          ],
          designs: [{ ...perProductDesign, targetGroupKey: undefined, targetGroupLabel: undefined }],
          selectedIds: [perProductDesign.id],
        },
      ],
      createReviewTasks,
    });

    expect(createReviewTasks).not.toHaveBeenCalled();
    expect(onProgress).toHaveBeenCalledWith(
      expect.stringContaining("no generated designs matched this product"),
    );
    expect(result.warnings).toEqual([
      expect.objectContaining({
        label: "Black / M",
        reason: "missing_design_match",
      }),
    ]);
  });
});
