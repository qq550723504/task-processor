import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { PricingRuleAdminPage } from "@/components/listingkit/admin/pricing-rule-admin-page";
import * as adminPricingRulesApi from "@/lib/api/admin-pricing-rules";
import * as adminStoresApi from "@/lib/api/admin-stores";

describe("PricingRuleAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads stores and renders ListingKit pricing rules", async () => {
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([
      { id: 11, name: "SHEIN US", platform: "SHEIN", region: "US" },
    ]);
    vi.spyOn(adminPricingRulesApi, "getListingPricingRules").mockResolvedValue({
      items: [
        {
          id: 1,
          tenantId: 101,
          name: "SHEIN auto price",
          ruleCode: "AR-SHEIN",
          storeId: 11,
          priceMin: 1,
          priceMax: 99,
          ruleType: "multiple_fixed",
          ruleValue: 1.8,
          fixedValue: 2.5,
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
        <PricingRuleAdminPage />
      </QueryClientProvider>,
    );

    expect(screen.getByRole("heading", { name: "核价规则" })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("SHEIN auto price")).toBeInTheDocument();
    });
    expect(screen.getByText("AR-SHEIN")).toBeInTheDocument();
    expect(screen.getAllByText("SHEIN US").length).toBeGreaterThan(0);
  });
});
