import { describe, expect, it } from "vitest";

import {
  parseSelectionFromSearchParams,
  parseSheinStudioStep,
} from "@/lib/shein-studio/url-state";

describe("parseSelectionFromSearchParams", () => {
  it("parses a live SHEIN studio selection from URL params", () => {
    const params = new URLSearchParams(
      "productId=124110&variantId=124111&parentProductId=124110&prototypeGroupId=18203&layerId=787532312015200256&printWidth=1000&printHeight=1000&variantIds=124111,124112,124113&step=select",
    );

    expect(parseSelectionFromSearchParams(params)).toEqual({
      productId: 124110,
      parentProductId: 124110,
      variantId: 124111,
      prototypeGroupId: 18203,
      layerId: "787532312015200256",
      productName: "已选择的 SDS 商品",
      variantLabel: "当前变体",
      printableWidth: 1000,
      printableHeight: 1000,
      templateImageUrl: undefined,
      maskImageUrl: undefined,
      blankDesignUrl: undefined,
      mockupImageUrl: undefined,
      mockupImageUrls: undefined,
      selectedVariantIds: [124111, 124112, 124113],
    });
  });
});

describe("parseSheinStudioStep", () => {
  it("falls back to select for unsupported step values", () => {
    expect(parseSheinStudioStep("bogus")).toBe("select");
  });
});
