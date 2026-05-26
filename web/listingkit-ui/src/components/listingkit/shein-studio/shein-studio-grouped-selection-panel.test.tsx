import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinStudioGroupedSelectionPanel } from "@/components/listingkit/shein-studio/shein-studio-grouped-selection-panel";

describe("SheinStudioGroupedSelectionPanel", () => {
  it("shows store name instead of ambiguous site-only labels", () => {
    const onBulkUpdateSelectionStore = vi.fn();
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
        activeSelectionBaselineReason=""
        activeSelectionBaselineStatus="ready"
        candidates={[]}
        currentStoreId="869"
        currentStoreLabel="SHEIN US 1 (shein-us-1 / NA / US)"
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
        onAddSelection={vi.fn()}
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
        name: "跟随当前店铺（SHEIN US 1 (shein-us-1 / NA / US)）",
      }),
    ).toHaveLength(2);
    expect(
      screen.getAllByRole("option", { name: "SHEIN US 1 (shein-us-1 / NA / US)" }),
    ).toHaveLength(2);
    expect(
      screen.getAllByRole("option", { name: "SHEIN US 2 (shein-us-2 / NA / US)" }),
    ).toHaveLength(2);
    expect(screen.getByText("已指定店铺：SHEIN US 2 (shein-us-2 / NA / US)")).toBeInTheDocument();
    expect(screen.getByText("当前跟随：SHEIN US 1 (shein-us-1 / NA / US)")).toBeInTheDocument();
    expect(screen.getByText("跨店铺")).toBeInTheDocument();
    expect(screen.getByText("跟随当前店铺 1 款")).toBeInTheDocument();
    expect(screen.getByText("跨店铺 1 款")).toBeInTheDocument();
    expect(screen.queryByRole("option", { name: "US" })).not.toBeInTheDocument();

    fireEvent.click(screen.getByText("跨店铺 1 款"));
    expect(
      screen.getByText("已按店铺分发状态筛选显示，再点一次当前标签可恢复查看全部。"),
    ).toBeInTheDocument();
    expect(screen.getByText("当前筛选命中 1 款商品，可以统一改成同一家店，或者改回跟随当前店铺。")).toBeInTheDocument();

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

    fireEvent.click(screen.getByText("跨店铺 1 款"));
    expect(
      screen.queryByText("已按店铺分发状态筛选显示，再点一次当前标签可恢复查看全部。"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("已把 1 款商品改到 SHEIN US 1 (shein-us-1 / NA / US)。"),
    ).not.toBeInTheDocument();
  });
});
