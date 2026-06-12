import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { StoreProfileSettingsPanel } from "@/components/listingkit/stores/store-profile-settings-panel";

const mocks = vi.hoisted(() => ({
  mutateAsync: vi.fn(),
  useStoreProfiles: vi.fn(),
  useUpsertStoreProfile: vi.fn(),
  useDeleteStoreProfile: vi.fn(),
  getTenantListingStores: vi.fn(),
  listSheinLoginAccounts: vi.fn(),
  listSheinStoreWarehouses: vi.fn(),
}));

vi.mock("@/lib/query/use-store-profiles", () => ({
  useStoreProfiles: () => mocks.useStoreProfiles(),
  useUpsertStoreProfile: () => mocks.useUpsertStoreProfile(),
  useDeleteStoreProfile: () => mocks.useDeleteStoreProfile(),
}));

vi.mock("@/lib/api/tenant-stores", () => ({
  getTenantListingStores: (...args: unknown[]) => mocks.getTenantListingStores(...args),
}));

vi.mock("@/lib/api/shein-login", () => ({
  listSheinLoginAccounts: (...args: unknown[]) => mocks.listSheinLoginAccounts(...args),
  listSheinStoreWarehouses: (...args: unknown[]) => mocks.listSheinStoreWarehouses(...args),
}));

describe("StoreProfileSettingsPanel", () => {
  beforeEach(() => {
    mocks.mutateAsync.mockReset();
    mocks.useStoreProfiles.mockReset();
    mocks.useUpsertStoreProfile.mockReset();
    mocks.useDeleteStoreProfile.mockReset();
    mocks.getTenantListingStores.mockReset();
    mocks.listSheinLoginAccounts.mockReset();
    mocks.listSheinStoreWarehouses.mockReset();

    mocks.useStoreProfiles.mockReturnValue({
      data: [
        {
          id: 1,
          store_id: 869,
          enabled: true,
          priority: 10,
          site: "US",
          warehouse_code: "WH-US-1",
          store: { id: 869, name: "US 主店", store_id: "SHEIN-US-869", region: "US" },
        },
      ],
      isLoading: false,
      isFetching: false,
      refetch: vi.fn(),
    });
    mocks.useUpsertStoreProfile.mockReturnValue({
      mutateAsync: mocks.mutateAsync,
      isPending: false,
    });
    mocks.useDeleteStoreProfile.mockReturnValue({
      mutateAsync: vi.fn(),
    });
    mocks.getTenantListingStores.mockResolvedValue({
      items: [
        { id: 869, name: "US 主店", storeId: "SHEIN-US-869", region: "US", platform: "SHEIN" },
        { id: 870, name: "US 备用店", storeId: "SHEIN-US-870", region: "US", platform: "SHEIN" },
      ],
      total: 2,
      page: 1,
      page_size: 200,
    });
    mocks.listSheinLoginAccounts.mockResolvedValue([
      {
        account: { store_id: 869, tenant_id: 1, store_name: "US 主店" },
        has_cookie: true,
        cookie_ttl: 1800,
        waiting_for_verify_code: false,
        login_in_progress: false,
      },
      {
        account: { store_id: 870, tenant_id: 1, store_name: "US 备用店" },
        has_cookie: true,
        cookie_ttl: 1800,
        waiting_for_verify_code: false,
        login_in_progress: false,
      },
    ]);
    mocks.listSheinStoreWarehouses.mockResolvedValue([
      {
        warehouse_code: "WH-CA-1",
        warehouse_name: "加拿大仓 1",
        sale_country_list: ["CA"],
      },
      {
        warehouse_code: "WH-US-1",
        warehouse_name: "美国仓 1",
        sale_country_list: ["US"],
      },
    ]);
  });

  it("renders existing tenant store profiles", async () => {
    renderWithQueryClient(<StoreProfileSettingsPanel />);

    expect(await screen.findByText("我的店铺配置")).toBeInTheDocument();
    expect(screen.getByText("US 主店")).toBeInTheDocument();
    expect(screen.getByText("WH-US-1")).toBeInTheDocument();
    expect(screen.getByText("已启用")).toBeInTheDocument();
    expect(screen.queryByText("匹配规则")).not.toBeInTheDocument();
  });

  it("creates a new tenant store profile from tenant-available stores", async () => {
    renderWithQueryClient(<StoreProfileSettingsPanel />);

    await screen.findByText("我的店铺配置");
    await screen.findByRole("option", { name: "US 备用店 (SHEIN-US-870 / US)" });
    expect(screen.getByRole("option", { name: "UK" })).toBeInTheDocument();

    fireEvent.change(screen.getByRole("combobox", { name: "SHEIN 店铺" }), {
      target: { value: "870" },
    });
    await screen.findByRole("option", { name: "加拿大仓 1 (WH-CA-1 / CA)" });
    fireEvent.change(screen.getByLabelText("站点"), {
      target: { value: "CA" },
    });
    const warehouseSelect = screen.getByLabelText("仓库编码") as HTMLSelectElement;
    const warehouseCA = screen.getByRole("option", { name: "加拿大仓 1 (WH-CA-1 / CA)" }) as HTMLOptionElement;
    const warehouseUS = screen.getByRole("option", { name: "美国仓 1 (WH-US-1 / US)" }) as HTMLOptionElement;
    warehouseCA.selected = true;
    warehouseUS.selected = true;
    fireEvent.change(warehouseSelect);
    fireEvent.click(screen.getByRole("button", { name: "新增配置" }));

    await waitFor(() => {
      expect(mocks.mutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          store_id: 870,
          site: "CA",
          warehouse_code: "WH-CA-1,WH-US-1",
        }),
      );
    });
  });
});

function renderWithQueryClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(<QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>);
}
