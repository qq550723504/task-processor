import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

  it("edits an existing ListingKit store", async () => {
    const user = userEvent.setup();
    const store = {
      id: 1,
      tenantId: 101,
      name: "SHEIN US",
      username: "shein-us",
      platform: "SHEIN",
      shopType: "semi",
      region: "US",
      dailyLimit: 200,
      dailyLimitType: "SPU",
      fixedStockCount: 999,
      skuGenerateStrategy: "0",
      enableAutoListing: true,
      status: 0,
    };

    vi.spyOn(adminStoresApi, "getListingStores").mockResolvedValue({
      items: [store],
      total: 1,
      page: 1,
      page_size: 20,
    });
    vi.spyOn(adminStoresApi, "getDeletedListingStores").mockResolvedValue([]);
    const updateStore = vi
      .spyOn(adminStoresApi, "updateListingStore")
      .mockResolvedValue({ ...store, name: "SHEIN US Edited" });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <StoreAdminPage />
      </QueryClientProvider>,
    );

    await screen.findByText("SHEIN US");
    await user.click(screen.getByRole("button", { name: "编辑 SHEIN US" }));

    expect(screen.getByRole("heading", { name: "编辑店铺" })).toBeInTheDocument();
    const form = screen.getByRole("form", { name: "店铺表单" });
    const nameInput = within(form).getByLabelText("店铺名称");
    expect(nameInput).toHaveValue("SHEIN US");

    await user.clear(nameInput);
    await user.type(nameInput, "SHEIN US Edited");
    await user.click(within(form).getByRole("button", { name: "保存修改" }));

    await waitFor(() => {
      expect(updateStore).toHaveBeenCalledWith(
        1,
        expect.objectContaining({ name: "SHEIN US Edited" }),
      );
    });
  });
});
