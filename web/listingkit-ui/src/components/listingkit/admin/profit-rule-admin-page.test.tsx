import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ProfitRuleAdminPage } from "@/components/listingkit/admin/profit-rule-admin-page";
import * as adminProfitRulesApi from "@/lib/api/admin-profit-rules";
import * as adminStoresApi from "@/lib/api/admin-stores";

describe("ProfitRuleAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads stores and renders ListingKit profit rules", async () => {
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([
      { id: 11, name: "SHEIN US", platform: "SHEIN", region: "US" },
    ]);
    vi.spyOn(adminProfitRulesApi, "getListingProfitRules").mockResolvedValue({
      items: [
        {
          id: 1,
          tenantId: 101,
          name: "SHEIN margin",
          ruleCode: "PR-SHEIN",
          storeId: 11,
          salePriceMultiplier: 3,
          discountPriceMultiplier: 2.5,
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
        <ProfitRuleAdminPage />
      </QueryClientProvider>,
    );

    expect(screen.getByRole("heading", { name: "利润规则" })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("SHEIN margin")).toBeInTheDocument();
    });
    expect(screen.getByText("PR-SHEIN")).toBeInTheDocument();
    expect(screen.getAllByText("SHEIN US (#11)").length).toBeGreaterThan(0);
    expect(screen.getAllByText("启用").length).toBeGreaterThan(1);
  });

  it("creates new profit rules as enabled", async () => {
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([]);
    vi.spyOn(adminProfitRulesApi, "getListingProfitRules").mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    const createRule = vi
      .spyOn(adminProfitRulesApi, "createListingProfitRule")
      .mockResolvedValue({
        id: 1,
        name: "ND margin",
        ruleCode: "PR-ND",
        salePriceMultiplier: 3,
        discountPriceMultiplier: 1,
        status: 0,
      });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <ProfitRuleAdminPage />
      </QueryClientProvider>,
    );

    fireEvent.change(screen.getByLabelText("规则名称"), {
      target: { value: "ND margin" },
    });
    fireEvent.change(screen.getAllByLabelText("规则编码")[1], {
      target: { value: "PR-ND" },
    });
    fireEvent.click(screen.getByRole("button", { name: /保存规则/ }));

    await waitFor(() => {
      expect(createRule).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "ND margin",
          ruleCode: "PR-ND",
          status: 0,
        }),
      );
    });
  });
});
