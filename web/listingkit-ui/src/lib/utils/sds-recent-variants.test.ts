import { afterEach, describe, expect, it } from "vitest";

import {
  loadRecentSDSVariants,
  saveRecentSDSVariant,
} from "@/lib/utils/sds-recent-variants";

describe("sds recent variants storage", () => {
  afterEach(() => {
    window.localStorage.clear();
  });

  it("deduplicates variant history and keeps latest first", () => {
    saveRecentSDSVariant({
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
    });

    saveRecentSDSVariant({
      productId: 2,
      parentProductId: 2,
      variantId: 22,
      prototypeGroupId: 32,
      layerId: "layer-b",
      productName: "Product B",
      variantLabel: "L · white",
    });

    saveRecentSDSVariant({
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
    });

    expect(loadRecentSDSVariants()).toEqual([
      {
        productId: 1,
        parentProductId: 1,
        variantId: 11,
        prototypeGroupId: 21,
        layerId: "layer-a",
        productName: "Product A",
        variantLabel: "M · black",
      },
      {
        productId: 2,
        parentProductId: 2,
        variantId: 22,
        prototypeGroupId: 32,
        layerId: "layer-b",
        productName: "Product B",
        variantLabel: "L · white",
      },
    ]);
  });
});
