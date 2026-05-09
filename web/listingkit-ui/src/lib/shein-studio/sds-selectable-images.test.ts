import { describe, expect, it } from "vitest";

import {
  buildDefaultSelectedSDSImages,
  buildSelectableSDSImages,
  normalizeSelectedSDSImages,
} from "@/lib/shein-studio/sds-selectable-images";

describe("buildSelectableSDSImages", () => {
  it("builds unique selectable SDS images from selection and variants", () => {
    const items = buildSelectableSDSImages({
      productId: 1,
      parentProductId: 1,
      variantId: 2,
      prototypeGroupId: 3,
      layerId: "layer-1",
      productName: "tee",
      variantLabel: "M / black",
      mockupImageUrls: [
        "https://example.com/global-main.jpg",
        "https://example.com/global-side.jpg",
      ],
      sizeReferenceImageUrls: ["https://example.com/global-size.jpg"],
      variants: [
        {
          variantId: 2,
          color: "Black",
          variantSku: "SKU-BLK",
          mockupImageUrls: [
            "https://example.com/global-main.jpg",
            "https://example.com/black-detail.jpg",
          ],
          sizeReferenceImageUrls: ["https://example.com/black-size.jpg"],
        },
      ],
    });

    expect(items).toHaveLength(5);
    expect(items[0].label).toContain("当前款式");
    expect(items[2]).toMatchObject({
      imageUrl: "https://example.com/global-size.jpg",
      kind: "size_reference",
    });
    expect(items[4]).toMatchObject({
      color: "Black",
      variantSku: "SKU-BLK",
      imageUrl: "https://example.com/black-size.jpg",
      kind: "size_reference",
    });
  });

  it("filters SDS preview images from mockup candidates", () => {
    const items = buildSelectableSDSImages({
      productId: 1,
      parentProductId: 1,
      variantId: 2,
      prototypeGroupId: 3,
      layerId: "layer-1",
      productName: "tee",
      variantLabel: "M / black",
      mockupImageUrls: [
        "https://cdn.sdspod.com/images/preview.jpg",
        "https://cdn.sdspod.com/out/rendered-main.jpg",
      ],
    });

    expect(items).toEqual([
      expect.objectContaining({
        imageUrl: "https://cdn.sdspod.com/out/rendered-main.jpg",
        kind: "mockup",
      }),
    ]);
  });

  it("preserves variant metadata when global mockups duplicate multi-variant images", () => {
    const items = buildSelectableSDSImages({
      productId: 1,
      parentProductId: 1,
      variantId: 2,
      prototypeGroupId: 3,
      layerId: "layer-1",
      productName: "tee",
      variantLabel: "M / black",
      mockupImageUrls: [
        "https://example.com/black-main.jpg",
        "https://example.com/white-main.jpg",
      ],
      variants: [
        {
          variantId: 2,
          color: "Black",
          variantSku: "SKU-BLK",
          mockupImageUrls: ["https://example.com/black-main.jpg"],
        },
        {
          variantId: 3,
          color: "White",
          variantSku: "SKU-WHT",
          mockupImageUrls: ["https://example.com/white-main.jpg"],
        },
      ],
    });

    expect(items).toEqual([
      expect.objectContaining({
        imageUrl: "https://example.com/black-main.jpg",
        color: "Black",
        variantSku: "SKU-BLK",
      }),
      expect.objectContaining({
        imageUrl: "https://example.com/white-main.jpg",
        color: "White",
        variantSku: "SKU-WHT",
      }),
    ]);
  });
});

describe("buildDefaultSelectedSDSImages", () => {
  it("uses the first mockup as main image and appends size references when enabled", () => {
    const items = buildSelectableSDSImages({
      productId: 1,
      parentProductId: 1,
      variantId: 2,
      prototypeGroupId: 3,
      layerId: "layer-1",
      productName: "tee",
      variantLabel: "M / black",
      mockupImageUrls: [
        "https://example.com/global-main.jpg",
        "https://example.com/global-side.jpg",
      ],
      sizeReferenceImageUrls: ["https://example.com/global-size.jpg"],
    });

    expect(
      buildDefaultSelectedSDSImages(items, { includeSizeReferenceImages: true }),
    ).toEqual([
      {
        imageUrl: "https://example.com/global-main.jpg",
        color: undefined,
        variantSku: undefined,
      },
      {
        imageUrl: "https://example.com/global-size.jpg",
        color: undefined,
        variantSku: undefined,
      },
    ]);
  });

  it("keeps only the SDS main image when size references are disabled", () => {
    const items = buildSelectableSDSImages({
      productId: 1,
      parentProductId: 1,
      variantId: 2,
      prototypeGroupId: 3,
      layerId: "layer-1",
      productName: "tee",
      variantLabel: "M / black",
      mockupImageUrls: ["https://example.com/global-main.jpg"],
      sizeReferenceImageUrls: ["https://example.com/global-size.jpg"],
    });

    expect(
      buildDefaultSelectedSDSImages(items, { includeSizeReferenceImages: false }),
    ).toEqual([
      {
        imageUrl: "https://example.com/global-main.jpg",
        color: undefined,
        variantSku: undefined,
      },
    ]);
  });

  it("selects one mockup per variant by default for multi-variant products", () => {
    const items = buildSelectableSDSImages({
      productId: 1,
      parentProductId: 1,
      variantId: 2,
      prototypeGroupId: 3,
      layerId: "layer-1",
      productName: "tee",
      variantLabel: "M / black",
      variants: [
        {
          variantId: 2,
          color: "Black",
          variantSku: "SKU-BLK",
          mockupImageUrls: [
            "https://example.com/black-main.jpg",
            "https://example.com/black-side.jpg",
          ],
        },
        {
          variantId: 3,
          color: "White",
          variantSku: "SKU-WHT",
          mockupImageUrls: [
            "https://example.com/white-main.jpg",
            "https://example.com/white-side.jpg",
          ],
        },
      ],
    });

    expect(
      buildDefaultSelectedSDSImages(items, { includeSizeReferenceImages: false }),
    ).toEqual([
      {
        imageUrl: "https://example.com/black-main.jpg",
        color: "Black",
        variantSku: "SKU-BLK",
      },
      {
        imageUrl: "https://example.com/white-main.jpg",
        color: "White",
        variantSku: "SKU-WHT",
      },
    ]);
  });
});

describe("normalizeSelectedSDSImages", () => {
  it("drops invalid and duplicate urls while preserving order", () => {
    expect(
      normalizeSelectedSDSImages([
        { imageUrl: "https://example.com/a.jpg", color: "Black" },
        { imageUrl: "https://example.com/a.jpg", color: "White" },
        { imageUrl: "https://cdn.sdspod.com/images/preview.jpg", color: "Preview" },
        { imageUrl: "   " },
        { imageUrl: "https://example.com/b.jpg", variantSku: "SKU-B" },
      ]),
    ).toEqual([
      { imageUrl: "https://example.com/a.jpg", color: "Black", variantSku: undefined },
      { imageUrl: "https://example.com/b.jpg", color: undefined, variantSku: "SKU-B" },
    ]);
  });
});
