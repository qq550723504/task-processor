import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ProductImportMappingAdminPage } from "@/components/listingkit/admin/product-import-mapping-admin-page";
import * as adminProductImportMappingsApi from "@/lib/api/admin-product-import-mappings";

describe("ProductImportMappingAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads and renders ListingKit product import mappings", async () => {
    vi.spyOn(
      adminProductImportMappingsApi,
      "getListingProductImportMappings",
    ).mockResolvedValue({
      items: [
        {
          id: 1,
          tenantId: 101,
          importTaskId: 1001,
          storeId: 11,
          platform: "SHEIN",
          region: "US",
          productId: "B001",
          sku: "SKU-001",
          salePriceMultiplier: 1.8,
          discountPriceMultiplier: 1.2,
          status: 1,
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
        <ProductImportMappingAdminPage />
      </QueryClientProvider>,
    );

    expect(
      screen.getByRole("heading", { name: "导入映射" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("B001")).toBeInTheDocument();
    });
    expect(screen.getByText("SKU-001")).toBeInTheDocument();
    expect(screen.getAllByText("SHEIN").length).toBeGreaterThan(0);
  });
});
