import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { SheinSyncedProductsTable } from "@/components/listingkit/shein-products/shein-synced-products-table";

describe("SheinSyncedProductsTable", () => {
  it("renders dense product operation details", () => {
    render(
      <SheinSyncedProductsTable
        isLoading={false}
        items={[
          {
            id: 1,
            main_image_url: "https://example.com/item.png",
            product_name_multi: "Round Drawer Knobs",
            spu_code: "spu-123",
            supplier_code: "J0529021001",
            sale_name: "White",
            skc_name: "skc-001",
            price_snapshot: "USD 34.17",
            effective_cost_price: 12.5,
            cost_price_source: "manual",
            inventory_snapshot:
              '{"total_inventory":999,"saleable_inventory":999}',
            shelf_status: "ON_SHELF",
            created_at: "2026-06-01 01:38:43",
            publish_time: "2026-06-02 02:58:40",
            first_shelf_time: "2026-06-02 21:04:59",
            last_sync_at: "2026-06-06 00:19:00",
          },
        ]}
      />,
    );

    expect(screen.getByText("Round Drawer Knobs")).toBeInTheDocument();
    expect(screen.getByText("SPU: spu-123")).toBeInTheDocument();
    expect(screen.getByText("货号: J0529021001")).toBeInTheDocument();
    expect(screen.getByText("White")).toBeInTheDocument();
    expect(screen.getByText("SKC: skc-001")).toBeInTheDocument();
    expect(screen.getByText("$34.17")).toBeInTheDocument();
    expect(screen.getByText("利润率 +173.4%")).toBeInTheDocument();
    expect(screen.getByText("利润 +21.67 USD")).toBeInTheDocument();
    expect(screen.getByText("来源 人工")).toBeInTheDocument();
    expect(screen.getByText("总库存 999")).toBeInTheDocument();
    expect(screen.getByText("可用库存 999")).toBeInTheDocument();
    expect(screen.getByText("创建 2026-06-01 01:38:43")).toBeInTheDocument();
  });

  it("separates SHEIN supply price from maintained cost", () => {
    render(
      <SheinSyncedProductsTable
        isLoading={false}
        items={[
          {
            id: 2,
            supplier_code: "sh260626230038058040685",
            product_name_multi: "SHEIN synced product",
            sale_name: "Default",
            skc_name: "sh260626230038058040685",
            price_snapshot:
              '{"sale_price":42.8,"currency":"USD","sub_site":"US"}',
            supply_price: 37.2,
            supply_price_currency: "USD",
            cost_price_source: "none",
            inventory_snapshot: '{"total":1,"available":1}',
            shelf_status: "ON_SHELF",
          },
        ]}
      />,
    );

    expect(screen.getByText("$42.80")).toBeInTheDocument();
    expect(screen.queryByText("供货价 37.20 USD")).not.toBeInTheDocument();
    expect(screen.getByText("待同步")).toBeInTheDocument();
    expect(screen.getByText("成本 -")).toBeInTheDocument();
    expect(screen.queryByText(/^利润率/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^利润 /)).not.toBeInTheDocument();
  });

  it("aligns every SKU sale and supply price in independent columns", () => {
    render(
      <SheinSyncedProductsTable
        isLoading={false}
        items={[
          {
            id: 4,
            product_name_multi: "Multi SKU SHEIN product",
            skc_name: "sh260627121580076835358",
            price_snapshot:
              '{"sale_price":28.1,"currency":"USD","sku_prices":[{"sku_code":"I5mqvuk7p0fzpi","sale_price":28.1,"currency":"USD"},{"sku_code":"I3mqvuk7pdcxlv","sale_price":31.5,"currency":"USD"}]}',
            supply_price_snapshot:
              '{"sku_supply_prices":[{"sku_code":"I5mqvuk7p0fzpi","supply_price":27.38,"currency":"USD"},{"sku_code":"I3mqvuk7pdcxlv","supply_price":31.62,"currency":"USD"}]}',
            cost_price_source: "none",
            shelf_status: "ON_SHELF",
          },
        ]}
      />,
    );

    expect(
      screen.getByRole("columnheader", { name: "SKU" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "同步售价" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("columnheader", { name: "SHEIN 供货价" }),
    ).toBeInTheDocument();
    expect(screen.getByText("I5mqvuk7p0fzpi")).toBeInTheDocument();
    expect(screen.getByText("$28.10")).toBeInTheDocument();
    expect(screen.getByText("$27.38")).toBeInTheDocument();
    expect(screen.getByText("I3mqvuk7pdcxlv")).toBeInTheDocument();
    expect(screen.getByText("$31.50")).toBeInTheDocument();
    expect(screen.getByText("$31.62")).toBeInTheDocument();
    expect(
      screen.queryByText("同步快照 SKU 价格（非实时后台价）"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("SKU 供货价（SHEIN 供货价接口）"),
    ).not.toBeInTheDocument();
    expect(
      screen.getByText("Multi SKU SHEIN product").closest("td"),
    ).toHaveAttribute("rowspan", "2");
  });

  it("shows every SKU supply price from the SHEIN supply price snapshot", () => {
    render(
      <SheinSyncedProductsTable
        isLoading={false}
        items={[
          {
            id: 5,
            product_name_multi: "Multi SKU SHEIN product",
            skc_name: "sh260627121580076835358",
            supply_price_snapshot:
              '{"sku_supply_prices":[{"sku_code":"I5mqvuk7p0fzpi","supply_price":12.5,"currency":"USD"},{"sku_code":"I3mqvuk7pdcxlv","supply_price":18.25,"currency":"USD"}]}',
            cost_price_source: "none",
            shelf_status: "ON_SHELF",
          },
        ]}
      />,
    );

    expect(screen.getByText("I5mqvuk7p0fzpi")).toBeInTheDocument();
    expect(screen.getByText("$12.50")).toBeInTheDocument();
    expect(screen.getByText("I3mqvuk7pdcxlv")).toBeInTheDocument();
    expect(screen.getByText("$18.25")).toBeInTheDocument();
  });

  it("shows inactive products as off shelf even when legacy shelf status is stale", () => {
    render(
      <SheinSyncedProductsTable
        isLoading={false}
        items={[
          {
            id: 3,
            product_name_multi: "Inactive SHEIN product",
            skc_name: "skc-inactive",
            cost_price_source: "none",
            shelf_status: "ON_SHELF",
            is_active: false,
          },
        ]}
      />,
    );

    expect(screen.getByText("已下架")).toBeInTheDocument();
    expect(screen.queryByText("已上架")).not.toBeInTheDocument();
  });
});
