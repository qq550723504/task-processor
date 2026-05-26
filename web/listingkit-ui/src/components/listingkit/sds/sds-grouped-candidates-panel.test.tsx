import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SDSGroupedCandidatesPanel } from "@/components/listingkit/sds/sds-grouped-candidates-panel";

describe("SDSGroupedCandidatesPanel", () => {
  it("renders grouped candidates and routes select/remove actions", () => {
    const onRemove = vi.fn();
    const onSelect = vi.fn();
    const item = {
      productId: 1,
      parentProductId: 1,
      variantId: 11,
      prototypeGroupId: 21,
      layerId: "layer-a",
      productName: "Product A",
      variantLabel: "M · black",
      printableWidth: 1000,
      printableHeight: 1000,
      selectedVariantIds: [11],
    };

    render(
      <SDSGroupedCandidatesPanel
        activeSelection={item}
        items={[item]}
        onRemove={onRemove}
        onSelect={onSelect}
      />,
    );

    expect(screen.getByText("批量候选池")).toBeInTheDocument();
    expect(screen.getByText("1 款候选")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "当前已选" })).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "移除" }));
    expect(onRemove).toHaveBeenCalledWith(item);

    fireEvent.click(screen.getByRole("button", { name: "当前已选" }));
    expect(onSelect).toHaveBeenCalledWith(item);
  });
});
