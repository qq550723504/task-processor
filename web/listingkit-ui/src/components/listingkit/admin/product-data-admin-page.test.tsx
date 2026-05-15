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
  });
});
