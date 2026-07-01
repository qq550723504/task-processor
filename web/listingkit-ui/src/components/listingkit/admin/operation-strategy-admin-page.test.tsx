import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { OperationStrategyAdminPage } from "@/components/listingkit/admin/operation-strategy-admin-page";
import * as adminOperationStrategiesApi from "@/lib/api/admin-operation-strategies";
import * as adminStoresApi from "@/lib/api/admin-stores";

describe("OperationStrategyAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads stores and renders ListingKit operation strategies", async () => {
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([
      { id: 11, name: "SHEIN US", platform: "SHEIN", region: "US" },
    ]);
    vi.spyOn(
      adminOperationStrategiesApi,
      "getListingOperationStrategies",
    ).mockResolvedValue({
      items: [
        {
          id: 1,
          tenantId: 101,
          storeId: 11,
          name: "SHEIN stock guard",
          platform: "SHEIN",
          status: 0,
          stockChangeThreshold: 10,
          stockChangeAction: "按比例更新",
          outOfStockAction: "自动下架",
          minProfitRate: 0.2,
          lowProfitAction: "暂停上架",
          priceUpdateMultiplier: 1.1,
          stockUpdateRatio: 0.8,
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
        <OperationStrategyAdminPage />
      </QueryClientProvider>,
    );

    expect(screen.getByRole("heading", { name: "运营策略" })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("SHEIN stock guard")).toBeInTheDocument();
    });
    expect(screen.getAllByText("SHEIN US (#11)").length).toBeGreaterThan(0);
    expect(screen.getAllByText("SHEIN").length).toBeGreaterThan(0);
  });
});
