import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  evaluateGroupedSelectionCompatibility,
  SheinStudioGroupedSelectionPanel,
} from "@/components/listingkit/shein-studio/shein-studio-grouped-selection-panel";

const activeSelection = {
  productId: 1,
  parentProductId: 1,
  variantId: 100,
  prototypeGroupId: 200,
  layerId: "layer-1",
  productName: "tee",
  variantLabel: "M / black",
};

describe("evaluateGroupedSelectionCompatibility", () => {
  it("rejects candidates when selection details are missing", () => {
    expect(evaluateGroupedSelectionCompatibility(undefined, activeSelection)).toEqual({
      compatible: false,
      reason: "缺少 SDS 选择信息，暂时无法加入分组。",
    });
  });

  it("rejects the active selection as a duplicate", () => {
    expect(
      evaluateGroupedSelectionCompatibility(activeSelection, activeSelection),
    ).toEqual({
      compatible: false,
      reason: "这个商品已经在当前批次里，无需重复加入。",
    });
  });

  it("accepts a different variant candidate", () => {
    expect(
      evaluateGroupedSelectionCompatibility(activeSelection, {
        ...activeSelection,
        variantId: 101,
      }),
    ).toEqual({
      compatible: true,
      reason: "",
    });
  });
});

describe("SheinStudioGroupedSelectionPanel", () => {
  it("shows store name instead of ambiguous site-only labels", () => {
    const onBulkUpdateSelectionStore = vi.fn();
    const { container } = render(
        <SheinStudioGroupedSelectionPanel
        activeSelection={{
          productId: 1,
          parentProductId: 1,
          variantId: 100,
          prototypeGroupId: 200,
          layerId: "layer-1",
          productName: "tee",
          variantLabel: "M / black",
        }}
        activeSelectionBaselineReason=""
        activeSelectionBaselineStatus="ready"
        currentStoreId="869"
        currentStoreLabel="SHEIN US 1 (shein-us-1 / NA / US)"
        printableAreaLabel="5000 × 2554px"
        selectedColorCount={1}
        selectedSizeCount={2}
        selectedVariantCount={2}
        groupedSelections={[
          {
            selectionId: "1:200:101:layer-2:101",
            selection: {
              productId: 1,
              parentProductId: 1,
              variantId: 101,
              prototypeGroupId: 200,
              layerId: "layer-2",
              productName: "hoodie",
              variantLabel: "L / white",
            },
            baselineStatus: "ready",
            baselineReason: "",
            sheinStoreId: "870",
            eligible: true,
          },
          {
            selectionId: "1:200:102:layer-3:102",
            selection: {
              productId: 1,
              parentProductId: 1,
              variantId: 102,
              prototypeGroupId: 200,
              layerId: "layer-3",
              productName: "mug",
              variantLabel: "One size",
            },
            baselineStatus: "ready",
            baselineReason: "",
            sheinStoreId: "",
            eligible: true,
          },
        ]}
        onBulkUpdateSelectionStore={onBulkUpdateSelectionStore}
        onRemoveSelection={vi.fn()}
        onUpdateSelectionStore={vi.fn()}
        storeOptions={[
          {
            store_id: 869,
            store: {
              id: 869,
              name: "SHEIN US 1",
              region: "NA",
              store_id: "shein-us-1",
            },
            site: "US",
          },
          {
            store_id: 870,
            store: {
              id: 870,
              name: "SHEIN US 2",
              region: "NA",
              store_id: "shein-us-2",
            },
            site: "US",
          },
        ]}
      />,
    );

    expect(
      screen.getAllByRole("option", {
        name: "跟随批次店铺",
      }),
    ).toHaveLength(2);
    expect(
      screen.getAllByRole("option", { name: "SHEIN US 1 (shein-us-1 / NA / US)" }),
    ).toHaveLength(2);
    expect(
      screen.getAllByRole("option", { name: "SHEIN US 2 (shein-us-2 / NA / US)" }),
    ).toHaveLength(2);
    expect(
      screen.getByText("这些商品会随批次一起保存，并参与后续生成或创建 SHEIN 资料。"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "这款商品是创建该批次时最先带入的规格，用于记录批次起点，不代表当前批次只围绕它生成。",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText("入口商品状态")).not.toBeInTheDocument();
    expect(screen.queryByText("Baseline 状态")).not.toBeInTheDocument();
    expect(
      screen.getByText("当前入口商品已就绪，可作为批次起点继续生成。"),
    ).toBeInTheDocument();
    expect(screen.getByText("已指定店铺：SHEIN US 2 (shein-us-2 / NA / US)")).toBeInTheDocument();
    expect(screen.getByText("默认跟随批次店铺：SHEIN US 1 (shein-us-1 / NA / US)")).toBeInTheDocument();
    expect(screen.getByText("跨店铺")).toBeInTheDocument();
    expect(screen.getByText("跟随批次店铺 1 款")).toBeInTheDocument();
    expect(screen.getByText("跨店铺 1 款")).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "US" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("跨店铺 1 款"));
    expect(
      screen.getByText("已按店铺分发状态筛选显示，再点一次当前标签可恢复查看全部。"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("当前筛选命中 1 款商品，可统一改成同一家店，或改回跟随批次店铺。"),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByLabelText("应用到当前筛选"), {
      target: { value: "869" },
    });
    fireEvent.click(screen.getByText("批量应用到当前筛选"));
    expect(onBulkUpdateSelectionStore).toHaveBeenCalledWith(
      ["1:200:101:layer-2:101"],
      "869",
    );
    expect(
      screen.getByText("已把 1 款商品改到 SHEIN US 1 (shein-us-1 / NA / US)。"),
    ).toBeInTheDocument();

    const overviewMetricGrid = container.querySelector(
      ".grid.gap-2",
    ) as HTMLDivElement | null;
    expect(overviewMetricGrid).not.toBeNull();
    expect(overviewMetricGrid?.className).not.toContain("xl:min-w-[32rem]");

    const bulkStorePanel = screen.getByLabelText("应用到当前筛选").parentElement;
    expect(bulkStorePanel).not.toBeNull();
    expect(bulkStorePanel?.className).not.toContain("min-w-[15rem]");

    fireEvent.click(screen.getByText("跨店铺 1 款"));
    expect(
      screen.queryByText("已按店铺分发状态筛选显示，再点一次当前标签可恢复查看全部。"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("已把 1 款商品改到 SHEIN US 1 (shein-us-1 / NA / US)。"),
    ).not.toBeInTheDocument();
  });

  it("uses shorter store placeholder copy when the batch store is still missing", () => {
    render(
      <SheinStudioGroupedSelectionPanel
        activeSelection={{
          productId: 1,
          parentProductId: 1,
          variantId: 100,
          prototypeGroupId: 200,
          layerId: "layer-1",
          productName: "tee",
          variantLabel: "M / black",
        }}
        activeSelectionBaselineAction={{
          label: "去处理 baseline",
          onClick: vi.fn(),
        }}
        activeSelectionBaselineReason="当前商品的 baseline 还需要先处理。"
        activeSelectionBaselineStatus="blocked"
        currentStoreId=""
        currentStoreLabel=""
        printableAreaLabel="5000 × 2554px"
        selectedColorCount={1}
        selectedSizeCount={1}
        selectedVariantCount={1}
        groupedSelections={[
          {
            selectionId: "1:200:102:layer-3:102",
            selection: {
              productId: 1,
              parentProductId: 1,
              variantId: 102,
              prototypeGroupId: 200,
              layerId: "layer-3",
              productName: "mug",
              variantLabel: "One size",
            },
            baselineStatus: "ready",
            baselineReason: "",
            sheinStoreId: "",
            eligible: true,
          },
        ]}
        onBulkUpdateSelectionStore={vi.fn()}
        onRemoveSelection={vi.fn()}
        onUpdateSelectionStore={vi.fn()}
        storeOptions={[]}
      />,
    );

    expect(screen.getByText("当前商品的 baseline 还需要先处理。")).toBeInTheDocument();
    expect(screen.queryByText("Baseline 状态")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "去处理 baseline" })).toBeInTheDocument();
    expect(screen.getByText("无法跟随批次店铺，请先设置批次店铺")).toBeInTheDocument();
    expect(
      screen.getByRole("option", { name: "请先设置批次店铺" }),
    ).toBeInTheDocument();
  });
});
