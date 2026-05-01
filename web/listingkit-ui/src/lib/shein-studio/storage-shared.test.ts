import { describe, expect, it } from "vitest";

import { buildSelectionSummary } from "@/lib/shein-studio/storage-shared";

describe("buildSelectionSummary", () => {
  it("drops heavy per-variant image arrays from saved draft payload", () => {
    const summary = buildSelectionSummary({
      productId: 1,
      parentProductId: 1,
      variantId: 2,
      prototypeGroupId: 3,
      layerId: "layer-1",
      productName: "tee",
      variantLabel: "M / black",
      printableWidth: 1000,
      printableHeight: 1000,
      mockupImageUrls: ["https://example.com/a.jpg", "https://example.com/b.jpg"],
      selectedVariantIds: [2, 3],
      variants: [
        {
          variantId: 2,
          variantSku: "SKU-2",
          size: "M",
          color: "black",
          mockupImageUrl: "https://example.com/m.jpg",
          mockupImageUrls: [
            "https://example.com/variant-a.jpg",
            "https://example.com/variant-b.jpg",
          ],
          templateImageUrl: "https://example.com/template.jpg",
        },
      ],
    });

    expect(summary?.mockupImageUrls).toEqual([
      "https://example.com/a.jpg",
      "https://example.com/b.jpg",
    ]);
    expect(summary?.variants).toEqual([
      {
        variantId: 2,
        variantSku: "SKU-2",
        size: "M",
        color: "black",
        price: undefined,
        weight: undefined,
        boxLength: undefined,
        boxWidth: undefined,
        boxHeight: undefined,
        productionCycle: undefined,
        prototypeGroupId: undefined,
        layerId: undefined,
        mockupImageUrl: "https://example.com/m.jpg",
      },
    ]);
    expect(summary?.variants?.[0]).not.toHaveProperty("mockupImageUrls");
    expect(summary?.variants?.[0]).not.toHaveProperty("templateImageUrl");
  });
});
