import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ProductDataAdminPage } from "@/components/listingkit/admin/product-data-admin-page";
import * as adminProductDataApi from "@/lib/api/admin-product-data";

describe("ProductDataAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads and renders ListingKit product data rows", async () => {
    vi.spyOn(adminProductDataApi, "getListingProductData").mockResolvedValue({
      items: [
        {
          id: 1,
          tenantId: 101,
          storeId: 11,
          platform: "SHEIN",
          region: "US",
          productId: "B001",
          parentProductId: "PARENT-001",
          title: "Cotton shirt",
          originalPrice: 19.99,
          specialPrice: 15.99,
          stock: "12",
          attributes: [
            {
              skc_name: "White",
              sku_info: [
                {
                  sku_code: "SKU-001",
                  mapping_info: { SKU: "seller-sku-1", CostPrice: 11 },
                  amazon_monitor_data: { price: "10.00" },
                  price_info_list: [
                    {
                      special_price: "15.00",
                      shop_price: "16.00",
                      currency: "USD",
                    },
                  ],
                },
                {
                  sku_code: "SKU-002",
                  mapping_info: { SKU: "seller-sku-2", CostPrice: 20 },
                  price_info_list: [
                    {
                      shop_price: "25.00",
                      currency: "USD",
                    },
                  ],
                },
              ],
            },
          ],
          status: 1,
          platformProductId: "SPU-001",
          shelfStatus: 2,
          lastSyncTime: "2026-05-15T09:00:00Z",
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <ProductDataAdminPage />
      </QueryClientProvider>,
    );

    expect(
      screen.getByRole("heading", { name: "商品数据" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("Cotton shirt")).toBeInTheDocument();
    });
    expect(screen.getByText("B001")).toBeInTheDocument();
    expect(screen.getByText("SPU-001")).toBeInTheDocument();
    expect(screen.getAllByText("SHEIN").length).toBeGreaterThan(0);
    expect(screen.getByText("利润率")).toBeInTheDocument();
    expect(screen.getByText("+25.0% ~ +50.0%")).toBeInTheDocument();
    expect(screen.getByText("(均 +37.5%)")).toBeInTheDocument();
    expect(screen.getByText("seller-sku-1")).toBeInTheDocument();
    expect(screen.getAllByText("+5.00 USD")).toHaveLength(2);
  });
});
