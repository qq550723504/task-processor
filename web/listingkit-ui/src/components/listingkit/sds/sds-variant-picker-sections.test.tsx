import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  SDSVariantGrid,
  SDSVariantSelectionSummary,
} from "@/components/listingkit/sds/sds-variant-picker-sections";

describe("SDSVariantSelectionSummary", () => {
  it("surfaces direct batch-add actions alongside variant actions", () => {
    const addSelectedVariantsToCurrentBatch = vi.fn();

    render(
      <SDSVariantSelectionSummary
        addSelectedVariantsToCurrentBatch={addSelectedVariantsToCurrentBatch}
        clearFilteredVariants={() => {}}
        createBatchFromSelectedVariants={() => {}}
        currentBatchLabel="Retro Cherries"
        openOtherBatchPicker={() => {}}
        selectFilteredVariants={() => {}}
        selectedColorCount={2}
        selectedSizeCount={3}
        selectedVariantCount={4}
        useSelectedVariants={() => {}}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "加入当前批次 · Retro Cherries" }));
    expect(addSelectedVariantsToCurrentBatch).toHaveBeenCalledTimes(1);
    expect(screen.getByRole("button", { name: "加入其他批次" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "创建新批次并加入" })).toBeInTheDocument();
  });

  it("collapses to a single primary confirm action in existing-batch mode", () => {
    render(
      <SDSVariantSelectionSummary
        addSelectedVariantsToCurrentBatch={() => {}}
        clearFilteredVariants={() => {}}
        createBatchFromSelectedVariants={() => {}}
        currentBatchLabel="TEST1"
        isTargetingExistingBatch
        openOtherBatchPicker={() => {}}
        selectFilteredVariants={() => {}}
        selectedColorCount={3}
        selectedSizeCount={1}
        selectedVariantCount={3}
        useSelectedVariants={() => {}}
      />,
    );

    expect(
      screen.getByText("已选 3 个 SKU，将加入批次 TEST1"),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "加入当前批次，继续选下一个" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "加入其他批次" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "创建新批次并加入" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "使用已选变体" }),
    ).not.toBeInTheDocument();
  });
});

describe("SDSVariantGrid", () => {
  const variants = [
    {
      id: 212095,
      sku: "MG17701061001",
      size: "10×10inch(25×25cm)",
      color_name: "White",
      currentPrice: 81,
      weight: 450,
      productionCycle: 48,
      issuingBayArea: { name: "美国" },
      designPrototype: { prototypeGroupId: 26098 },
    },
  ];

  it("hides primary-selection actions while targeting an existing batch", () => {
    render(
      <SDSVariantGrid
        allowPrimarySelection={false}
        filteredVariants={variants}
        onSelectAsPrimary={() => {}}
        selectedIds={[212095]}
        selectedVariantId={212095}
        toggleVariant={() => {}}
      />,
    );

    expect(screen.queryByRole("button", { name: "默认变体" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "设为默认变体" })).not.toBeInTheDocument();
  });
});
