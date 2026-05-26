import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SDSProductCard } from "@/components/listingkit/sds/sds-product-card";

describe("SDSProductCard", () => {
  it("shows grouped candidate hints when the product already has candidate variants", () => {
    render(
      <SDSProductCard
        groupedCandidateCount={2}
        hasGroupedCandidate
        isSelected={false}
        isVariantSelected={false}
        onOpenVariants={() => {}}
        product={{
          id: 1,
          name: "Canvas Tote",
          sku: "SKU-1",
          currentPrice: 12.5,
          min_price: 12.5,
        }}
      />,
    );

    expect(screen.getByText("候选池 2 款")).toBeInTheDocument();
    expect(
      screen.getByText("这款商品已有候选变体，可直接回选或继续补充更多子 SKU。"),
    ).toBeInTheDocument();
  });
});
