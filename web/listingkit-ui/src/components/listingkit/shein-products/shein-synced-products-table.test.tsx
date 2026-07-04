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
            inventory_snapshot: '{"total_inventory":999,"saleable_inventory":999}',
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
    expect(screen.getByText("售价 $34.17")).toBeInTheDocument();
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
            price_snapshot: '{"sale_price":42.8,"currency":"USD","sub_site":"US"}',
            supply_price: 37.2,
            supply_price_currency: "USD",
            cost_price_source: "none",
            inventory_snapshot: '{"total":1,"available":1}',
            shelf_status: "ON_SHELF",
          },
        ]}
      />,
    );

    expect(screen.getByText("售价 $42.80")).toBeInTheDocument();
    expect(screen.getByText("供货价 37.20 USD")).toBeInTheDocument();
    expect(screen.getByText("成本 -")).toBeInTheDocument();
    expect(screen.queryByText(/^利润率/)).not.toBeInTheDocument();
    expect(screen.queryByText(/^利润 /)).not.toBeInTheDocument();
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
