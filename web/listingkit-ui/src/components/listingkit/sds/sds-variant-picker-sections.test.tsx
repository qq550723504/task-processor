import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SDSVariantSelectionSummary } from "@/components/listingkit/sds/sds-variant-picker-sections";

describe("SDSVariantSelectionSummary", () => {
  it("surfaces grouped-candidate action alongside variant actions", () => {
    const addSelectedVariantsToGroupedCandidates = vi.fn();

    render(
      <SDSVariantSelectionSummary
        addSelectedVariantsToGroupedCandidates={addSelectedVariantsToGroupedCandidates}
        clearFilteredVariants={() => {}}
        selectFilteredVariants={() => {}}
        selectedColorCount={2}
        selectedSizeCount={3}
        selectedVariantCount={4}
        useSelectedVariants={() => {}}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "加入批量候选池" }));
    expect(addSelectedVariantsToGroupedCandidates).toHaveBeenCalledTimes(1);
  });
});
