import { afterEach, describe, expect, it } from "vitest";

import {
  hasSDSGroupedCandidate,
  loadSDSGroupedCandidates,
  removeSDSGroupedCandidate,
  saveSDSGroupedCandidate,
} from "@/lib/utils/sds-grouped-candidates";

describe("sds grouped candidates storage", () => {
  afterEach(() => {
    window.localStorage.clear();
  });

  it("deduplicates grouped candidates and keeps latest first", () => {
    saveSDSGroupedCandidate({
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
    });
    saveSDSGroupedCandidate({
      productId: 2,
      parentProductId: 2,
      variantId: 22,
      prototypeGroupId: 32,
      layerId: "layer-b",
      productName: "Product B",
      variantLabel: "L · white",
    });
    saveSDSGroupedCandidate({
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
    });

    expect(loadSDSGroupedCandidates()).toEqual([
      expect.objectContaining({ variantId: 11, productName: "Product A" }),
      expect.objectContaining({ variantId: 22, productName: "Product B" }),
    ]);
    expect(
      hasSDSGroupedCandidate({
        productId: 1,
        parentProductId: 1,
        variantId: 11,
        prototypeGroupId: 21,
        layerId: "layer-a",
        productName: "Product A",
        variantLabel: "M · black",
      }),
    ).toBe(true);
  });

  it("removes grouped candidates by stable selection identity", () => {
    saveSDSGroupedCandidate({
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
    });

    removeSDSGroupedCandidate({
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
    });

    expect(loadSDSGroupedCandidates()).toEqual([]);
    expect(
      hasSDSGroupedCandidate({
        productId: 1,
        parentProductId: 1,
        variantId: 11,
        prototypeGroupId: 21,
        layerId: "layer-a",
        productName: "Product A",
        variantLabel: "M · black",
      }),
    ).toBe(false);
  });

  it("does not collapse candidates that share a variant id but differ by grouped selection identity", () => {
    saveSDSGroupedCandidate({
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
      selectedVariantIds: [11],
    });
    saveSDSGroupedCandidate({
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-b",
      productName: "Product A alternate",
      variantLabel: "M · black",
      selectedVariantIds: [11, 12],
    });

    expect(loadSDSGroupedCandidates()).toHaveLength(2);
  });
});
