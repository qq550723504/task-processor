import { render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { StoreAdminPage } from "@/components/listingkit/admin/store-admin-page";
import * as adminStoresApi from "@/lib/api/admin-stores";

describe("StoreAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads and renders ListingKit stores", async () => {
    vi.spyOn(adminStoresApi, "getListingStores").mockResolvedValue({
      items: [
        {
          id: 1,
          tenantId: 101,
          name: "SHEIN US",
          username: "shein-us",
          platform: "SHEIN",
          shopType: "semi",
          region: "US",
          status: 0,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    });
    vi.spyOn(adminStoresApi, "getDeletedListingStores").mockResolvedValue([
      {
        id: 2,
        tenantId: 101,
        name: "Deleted SHEIN",
        username: "deleted",
        platform: "SHEIN",
        shopType: "semi",
        region: "US",
        status: 0,
      },
    ]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <StoreAdminPage />
      </QueryClientProvider>,
    );

    expect(screen.getByRole("heading", { name: "店铺管理" })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("SHEIN US")).toBeInTheDocument();
    });
    expect(screen.getByText("shein-us")).toBeInTheDocument();
    expect(screen.getAllByText("SHEIN").length).toBeGreaterThan(0);
    expect(screen.getByRole("button", { name: "回收站" })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "延长 SHEIN US 有效期" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("Deleted SHEIN")).toBeInTheDocument();
    });
    expect(
      screen.getByRole("button", { name: "恢复 Deleted SHEIN" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "彻底删除 Deleted SHEIN" }),
    ).toBeInTheDocument();
  });
});
