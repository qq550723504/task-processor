import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { TenantStoreDirectoryPanel } from "@/components/listingkit/stores/tenant-store-directory-panel";

const mocks = vi.hoisted(() => ({
  getTenantListingStores: vi.fn(),
  createTenantListingStore: vi.fn(),
  updateTenantListingStore: vi.fn(),
  deleteTenantListingStore: vi.fn(),
  listSheinLoginAccounts: vi.fn(),
}));

vi.mock("@/lib/api/tenant-stores", () => ({
  getTenantListingStores: (...args: unknown[]) => mocks.getTenantListingStores(...args),
  createTenantListingStore: (...args: unknown[]) => mocks.createTenantListingStore(...args),
  updateTenantListingStore: (...args: unknown[]) => mocks.updateTenantListingStore(...args),
  deleteTenantListingStore: (...args: unknown[]) => mocks.deleteTenantListingStore(...args),
}));

vi.mock("@/lib/api/shein-login", () => ({
  listSheinLoginAccounts: (...args: unknown[]) => mocks.listSheinLoginAccounts(...args),
}));

describe("TenantStoreDirectoryPanel", () => {
  beforeEach(() => {
    mocks.getTenantListingStores.mockReset();
    mocks.createTenantListingStore.mockReset();
    mocks.updateTenantListingStore.mockReset();
    mocks.deleteTenantListingStore.mockReset();
    mocks.listSheinLoginAccounts.mockReset();

    mocks.getTenantListingStores.mockResolvedValue({
      items: [
        {
          id: 1,
          name: "SHEIN US",
          username: "shein-us",
          platform: "SHEIN",
          shopType: "semi",
          region: "US",
          storeId: "SHEIN-US-001",
          enableAutoListing: true,
        },
      ],
      total: 1,
      page: 1,
      page_size: 50,
    });
    mocks.createTenantListingStore.mockResolvedValue({
      id: 2,
      name: "SHEIN CA",
      username: "shein-ca",
      platform: "SHEIN",
      shopType: "semi",
      region: "CA",
      storeId: "SHEIN-CA-002",
    });
    mocks.listSheinLoginAccounts.mockResolvedValue([
      {
        account: {
          store_id: 1,
          tenant_id: 1,
          store_name: "SHEIN US",
        },
        has_cookie: true,
        cookie_ttl: 1800,
        waiting_for_verify_code: false,
        login_in_progress: false,
      },
    ]);
  });

  it("renders tenant store list", async () => {
    renderWithQueryClient(<TenantStoreDirectoryPanel />);

    expect(await screen.findByText("店铺主数据")).toBeInTheDocument();
    expect(await screen.findByText("SHEIN US")).toBeInTheDocument();
    expect(await screen.findByText("shein-us")).toBeInTheDocument();
  });

  it("creates a tenant store", async () => {
    const user = userEvent.setup();
    renderWithQueryClient(<TenantStoreDirectoryPanel />);

    await screen.findByText("店铺主数据");
    const form = screen.getByRole("form", { name: "租户店铺表单" });

    await user.type(within(form).getByLabelText("店铺名称"), "SHEIN CA");
    await user.type(within(form).getByLabelText("店铺 ID"), "SHEIN-CA-002");
    await user.type(within(form).getByLabelText("登录用户名"), "shein-ca");
    await user.type(within(form).getByLabelText("登录密码"), "secret");
    await user.clear(within(form).getByLabelText("地区"));
    await user.type(within(form).getByLabelText("地区"), "CA");
    await user.click(within(form).getByRole("button", { name: "保存店铺" }));

    await waitFor(() => {
      expect(mocks.createTenantListingStore).toHaveBeenCalledWith(
        expect.objectContaining({
          name: "SHEIN CA",
          storeId: "SHEIN-CA-002",
          username: "shein-ca",
          password: "secret",
          region: "CA",
          platform: "SHEIN",
          shopType: "0",
        }),
      );
    });
  });

  it("renders shein login status in store rows", async () => {
    mocks.getTenantListingStores.mockResolvedValue({
      items: [
        {
          id: 1,
          name: "SHEIN US",
          username: "shein-us",
          platform: "SHEIN",
          shopType: "0",
          region: "US",
          storeId: "SHEIN-US-001",
          enableAutoListing: true,
        },
        {
          id: 2,
          name: "TEMU US",
          username: "temu-us",
          platform: "TEMU",
          shopType: "2",
          region: "US",
          storeId: "TEMU-US-001",
          enableAutoListing: false,
        },
      ],
      total: 2,
      page: 1,
      page_size: 50,
    });
    mocks.listSheinLoginAccounts.mockResolvedValue([
      {
        account: {
          store_id: 1,
          tenant_id: 1,
          store_name: "SHEIN US",
        },
        has_cookie: true,
        cookie_ttl: 1800,
        waiting_for_verify_code: false,
        login_in_progress: false,
      },
    ]);

    renderWithQueryClient(<TenantStoreDirectoryPanel />);

    const sheinRow = (await screen.findByText("SHEIN US")).closest("tr");
    const temuRow = (await screen.findByText("TEMU US")).closest("tr");

    expect(sheinRow).not.toBeNull();
    expect(temuRow).not.toBeNull();
    expect(within(sheinRow as HTMLElement).getByText("已登录")).toBeInTheDocument();
    expect(
      within(sheinRow as HTMLElement).getByRole("link", { name: "去登录" }),
    ).toHaveAttribute("href", "/listing-kits/shein-login?store_id=1");
    expect(within(temuRow as HTMLElement).getAllByText("-").length).toBeGreaterThan(0);
    expect(within(temuRow as HTMLElement).queryByRole("link", { name: "去登录" })).toBeNull();
  });
});

function renderWithQueryClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}
