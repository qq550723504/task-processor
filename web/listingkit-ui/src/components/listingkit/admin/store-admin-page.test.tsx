import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { StoreAdminPage } from "@/components/listingkit/admin/store-admin-page";
import * as adminStoresApi from "@/lib/api/admin-stores";
import * as sheinLoginApi from "@/lib/api/shein-login";

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
          shopType: "0",
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
        shopType: "0",
        region: "US",
        status: 0,
      },
    ]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    vi.spyOn(sheinLoginApi, "listSheinLoginAccounts").mockResolvedValue([
      {
        account: {
          store_id: 1,
          tenant_id: 101,
          store_name: "SHEIN US",
        },
        has_cookie: true,
        cookie_ttl: 1800,
        waiting_for_verify_code: false,
        login_in_progress: false,
      },
    ]);
    render(
      <QueryClientProvider client={queryClient}>
        <StoreAdminPage />
      </QueryClientProvider>,
    );

    expect(screen.getByRole("heading", { name: "平台店铺管理" })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("SHEIN US")).toBeInTheDocument();
    });
    expect(screen.getByText("shein-us")).toBeInTheDocument();
    expect(screen.getAllByText("SHEIN").length).toBeGreaterThan(0);
    expect(screen.getByText("启用")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "回收站" })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "延长 SHEIN US 有效期" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "禁用 SHEIN US" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("Deleted SHEIN")).toBeInTheDocument();
    });
    expect(screen.getByText("已登录")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "去登录" })).toHaveAttribute(
      "href",
      "/listing-kits/shein-login?store_id=1",
    );
    expect(
      screen.getByRole("button", { name: "恢复 Deleted SHEIN" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "彻底删除 Deleted SHEIN" }),
    ).toBeInTheDocument();
    expect(screen.queryByTestId("store-profile-admin-panel")).not.toBeInTheDocument();
  });

  it("uses mobile-first store filters and wide-table scroll containers", async () => {
    vi.spyOn(adminStoresApi, "getListingStores").mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    vi.spyOn(adminStoresApi, "getDeletedListingStores").mockResolvedValue([]);
    vi.spyOn(sheinLoginApi, "listSheinLoginAccounts").mockResolvedValue([]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <StoreAdminPage />
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(adminStoresApi.getListingStores).toHaveBeenCalled();
    });

    expect(screen.getByRole("button", { name: "回收站" })).toHaveClass("w-full");
    expect(screen.getByRole("button", { name: "查询" })).toHaveClass("w-full");
    expect(screen.getByPlaceholderText("搜索店铺")).toHaveClass("w-full");
  });

  it("edits an existing ListingKit store", async () => {
    const user = userEvent.setup();
    const store = {
      id: 1,
      tenantId: 101,
      name: "SHEIN US",
      username: "shein-us",
      platform: "SHEIN",
      shopType: "0",
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
    vi.spyOn(sheinLoginApi, "listSheinLoginAccounts").mockResolvedValue([
      {
        account: {
          store_id: 1,
          tenant_id: 101,
          store_name: "SHEIN US",
        },
        has_cookie: false,
        cookie_ttl: 0,
        waiting_for_verify_code: false,
        login_in_progress: false,
      },
    ]);
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
    expect(within(form).getByLabelText("启用自动登录")).not.toBeChecked();

    await user.clear(nameInput);
    await user.type(nameInput, "SHEIN US Edited");
    await user.click(within(form).getByLabelText("启用自动登录"));
    await user.click(within(form).getByRole("button", { name: "保存修改" }));

    await waitFor(() => {
      expect(updateStore).toHaveBeenCalledWith(
        1,
        expect.objectContaining({ name: "SHEIN US Edited", enableAutoLogin: true }),
      );
    });
  });

  it("uses a region dropdown when editing a platform store", async () => {
    const user = userEvent.setup();
    const store = {
      id: 1,
      tenantId: 101,
      name: "SHEIN US",
      username: "shein-us",
      platform: "SHEIN",
      shopType: "0",
      region: "US",
      status: 0,
    };

    vi.spyOn(adminStoresApi, "getListingStores").mockResolvedValue({
      items: [store],
      total: 1,
      page: 1,
      page_size: 20,
    });
    vi.spyOn(adminStoresApi, "getDeletedListingStores").mockResolvedValue([]);
    vi.spyOn(sheinLoginApi, "listSheinLoginAccounts").mockResolvedValue([]);

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

    const regionSelect = within(screen.getByRole("form", { name: "店铺表单" })).getByLabelText("地区");
    expect(regionSelect.tagName).toBe("SELECT");
    expect(regionSelect).toHaveValue("US");
    expect(within(screen.getByRole("form", { name: "店铺表单" })).getByLabelText("启用自动登录")).not.toBeChecked();
  });

  it("shows non-shein stores without login status", async () => {
    vi.spyOn(adminStoresApi, "getListingStores").mockResolvedValue({
      items: [
        {
          id: 8,
          tenantId: 101,
          name: "TEMU US",
          username: "temu-us",
          platform: "TEMU",
          shopType: "2",
          region: "US",
          status: 0,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    });
    vi.spyOn(adminStoresApi, "getDeletedListingStores").mockResolvedValue([]);
    vi.spyOn(sheinLoginApi, "listSheinLoginAccounts").mockResolvedValue([]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <StoreAdminPage />
      </QueryClientProvider>,
    );

    const temuRow = (await screen.findByText("TEMU US")).closest("tr");
    expect(temuRow).not.toBeNull();
    expect(within(temuRow as HTMLElement).getAllByText("-").length).toBeGreaterThan(0);
    expect(within(temuRow as HTMLElement).queryByRole("link", { name: "去登录" })).toBeNull();
  });

  it("toggles store status through the admin store status endpoint", async () => {
    const user = userEvent.setup();
    const store = {
      id: 1,
      tenantId: 101,
      name: "SHEIN US",
      username: "shein-us",
      platform: "SHEIN",
      shopType: "0",
      region: "US",
      status: 0,
    };

    vi.spyOn(adminStoresApi, "getListingStores").mockResolvedValue({
      items: [store],
      total: 1,
      page: 1,
      page_size: 20,
    });
    vi.spyOn(adminStoresApi, "getDeletedListingStores").mockResolvedValue([]);
    vi.spyOn(sheinLoginApi, "listSheinLoginAccounts").mockResolvedValue([]);
    const updateStatus = vi
      .spyOn(adminStoresApi, "updateListingStoreStatus")
      .mockResolvedValue({ ...store, status: 1 });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <StoreAdminPage />
      </QueryClientProvider>,
    );

    await screen.findByText("SHEIN US");
    await user.click(screen.getByRole("button", { name: "禁用 SHEIN US" }));

    await waitFor(() => {
      expect(updateStatus).toHaveBeenCalledWith(1, 1, "平台手动禁用店铺");
    });
  });
});
