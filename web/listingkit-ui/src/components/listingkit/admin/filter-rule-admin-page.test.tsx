import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { FilterRuleAdminPage } from "@/components/listingkit/admin/filter-rule-admin-page";
import * as adminFilterRulesApi from "@/lib/api/admin-filter-rules";
import * as adminStoresApi from "@/lib/api/admin-stores";

describe("FilterRuleAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads stores and renders ListingKit filter rules", async () => {
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([
      { id: 11, name: "SHEIN US", platform: "SHEIN", region: "US" },
    ]);
    vi.spyOn(adminFilterRulesApi, "getListingFilterRules").mockResolvedValue({
      items: [
        {
          id: 1,
          tenantId: 101,
          name: "Amazon basic",
          ruleCode: "FR-AMZ",
          storeId: 11,
          priceMin: 1,
          priceMax: 99,
          stockMin: 10,
          ratingMin: 4.2,
          reviewCountMin: 20,
          fulfillmentType: "FBA",
          status: 0,
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
        <FilterRuleAdminPage />
      </QueryClientProvider>,
    );

    expect(screen.getByRole("heading", { name: "筛选规则" })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("Amazon basic")).toBeInTheDocument();
    });
    expect(screen.getByText("FR-AMZ")).toBeInTheDocument();
    expect(screen.getAllByText("SHEIN US (#11)").length).toBeGreaterThan(0);
    expect(screen.getAllByText("启用").length).toBeGreaterThan(1);
  });

  it("creates new filter rules as enabled", async () => {
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([]);
    vi.spyOn(adminFilterRulesApi, "getListingFilterRules").mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    const createRule = vi
      .spyOn(adminFilterRulesApi, "createListingFilterRule")
      .mockResolvedValue({
        id: 1,
        name: "ND",
        ruleCode: "FR-ND",
        priceMin: 0,
        priceMax: 99999,
        stockMin: 10,
        ratingMin: 0,
        reviewCountMin: 0,
        status: 0,
      });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <FilterRuleAdminPage />
      </QueryClientProvider>,
    );

    fireEvent.change(screen.getByLabelText("规则名称"), {
      target: { value: "ND" },
    });
    fireEvent.change(screen.getAllByLabelText("规则编码")[1], {
      target: { value: "FR-ND" },
    });
    fireEvent.click(screen.getByRole("button", { name: /保存规则/ }));

    await waitFor(() => {
      expect(createRule).toHaveBeenCalledWith(
        expect.objectContaining({ name: "ND", ruleCode: "FR-ND", status: 0 }),
      );
    });
  });
});
