import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import {
  buildSDSInternalFallbackHref,
  buildSDSSourceProductHref,
  SheinSourceProductPanel,
} from "@/components/listingkit/shein/shein-source-product-panel";

describe("SheinSourceProductPanel", () => {
  it("links to the SDS product when source identifiers are available", () => {
    render(
      <SheinSourceProductPanel
        shein={{
          source_product: {
            parent_product_id: "238915",
            variant_id: "238916",
            title: "SDS pants",
            sku: "NS6104229008",
          },
        }}
      />,
    );

    expect(screen.getByRole("link", { name: /打开 SDS 商品/ })).toHaveAttribute(
      "href",
      "https://www.sdsdiy.com/portal/detail/238915",
    );
    expect(screen.getByRole("link", { name: /打开 SDS 商品/ })).toHaveAttribute(
      "target",
      "_blank",
    );
  });

  it("builds an internal SKU search fallback when the SDS product id is missing", () => {
    expect(buildSDSSourceProductHref({ title: "SDS pants", sku: "NS6104229008" })).toBe(
      "",
    );
    expect(
      buildSDSInternalFallbackHref({
        title: "SDS pants",
        sku: "NS6104229008",
      }),
    ).toBe("/listing-kits/sds?keyword=NS6104229008");
  });
});
