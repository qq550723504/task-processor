import { afterEach, describe, expect, it, vi } from "vitest";

import { loadSDSListingKitMetadata } from "@/lib/sds/product-metadata";

describe("loadSDSListingKitMetadata", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("expands selected variant ids into listing kit variant metadata", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue({
      ok: true,
      json: async () => ({
        id: 100,
        name: "Multi variant tee",
        sku: "PARENT-SKU",
        blankDesignUrl: "https://cdn.example.com/blank.jpg",
        subproducts: {
          items: [
            {
              id: 101,
              sku: "SKU-BLACK-S",
              size: "S",
              color_name: "Black",
              currentPrice: 19.8,
              weight: 180,
              productionCycle: 48,
              designPrototype: {
                prototypeGroupId: 10,
                prototypeResultGroups: [
                  {
                    resultImage: "https://cdn.example.com/black-main.jpg",
                    sort: 1,
                  },
                  {
                    resultImage: "https://cdn.example.com/black-size-chart.jpg",
                    sort: 2,
                  },
                ],
                prototypeLayerList: [
                  {
                    id: "layer-black",
                    isMasterMap: 1,
                    thumbnailUrl: "https://cdn.example.com/black-template.jpg",
                  },
                ],
              },
            },
            {
              id: 102,
              sku: "SKU-WHITE-M",
              size: "M",
              color_name: "White",
              currentPrice: 19.8,
              weight: 190,
              productionCycle: 48,
              designPrototype: {
                prototypeGroupId: 11,
                prototypeResultGroups: [
                  {
                    resultImage: "https://cdn.example.com/white-main.jpg",
                    sort: 1,
                  },
                  {
                    resultImage: "https://cdn.example.com/white-size-chart.jpg",
                    sort: 2,
                  },
                ],
                prototypeLayerList: [
                  {
                    id: "layer-white",
                    isMasterMap: 1,
                    thumbnailUrl: "https://cdn.example.com/white-template.jpg",
                  },
                ],
              },
            },
          ],
        },
      }),
    } as Response);

    const metadata = await loadSDSListingKitMetadata({
      parentProductId: 100,
      variantId: 101,
      selectedVariantIds: [101, 102],
    });

    expect(metadata.variants).toHaveLength(2);
    expect(metadata.variants?.map((variant) => variant.variant_sku)).toEqual([
      "SKU-BLACK-S",
      "SKU-WHITE-M",
    ]);
    expect(metadata.variants?.[1]).toMatchObject({
      variant_id: 102,
      size: "M",
      color: "White",
      layer_id: "layer-white",
      template_image_url: "https://cdn.example.com/white-template.jpg",
      size_reference_image_urls: ["https://cdn.example.com/white-size-chart.jpg"],
    });
  });
});
