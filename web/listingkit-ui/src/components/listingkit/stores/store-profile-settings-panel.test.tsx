import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";

import { StoreProfileSettingsPanel } from "@/components/listingkit/stores/store-profile-settings-panel";

const mocks = vi.hoisted(() => ({
  mutateAsync: vi.fn(),
  useStoreProfiles: vi.fn(),
  useUpsertStoreProfile: vi.fn(),
  useDeleteStoreProfile: vi.fn(),
  getTenantListingStores: vi.fn(),
}));

vi.mock("@/lib/query/use-store-profiles", () => ({
  useStoreProfiles: () => mocks.useStoreProfiles(),
  useUpsertStoreProfile: () => mocks.useUpsertStoreProfile(),
  useDeleteStoreProfile: () => mocks.useDeleteStoreProfile(),
}));

vi.mock("@/lib/api/tenant-stores", () => ({
  getTenantListingStores: (...args: unknown[]) => mocks.getTenantListingStores(...args),
}));

describe("StoreProfileSettingsPanel", () => {
  beforeEach(() => {
    mocks.mutateAsync.mockReset();
    mocks.useStoreProfiles.mockReset();
    mocks.useUpsertStoreProfile.mockReset();
    mocks.useDeleteStoreProfile.mockReset();
    mocks.getTenantListingStores.mockReset();

    mocks.useStoreProfiles.mockReturnValue({
      data: [
        {
          id: 1,
          store_id: 869,
          enabled: true,
          priority: 10,
          site: "US",
          warehouse_code: "WH-US-1",
          match_rules: [{ kind: "country", values: ["US", "CA"] }],
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
  });

  it("renders existing tenant store profiles", async () => {
    renderWithQueryClient(<StoreProfileSettingsPanel />);

    expect(await screen.findByText("我的店铺配置")).toBeInTheDocument();
    expect(screen.getByText("US 主店")).toBeInTheDocument();
    expect(screen.getByText("WH-US-1")).toBeInTheDocument();
    expect(screen.getByText("国家: US, CA")).toBeInTheDocument();
    expect(screen.getByText("已启用")).toBeInTheDocument();
  });

  it("creates a new tenant store profile from tenant-available stores", async () => {
    renderWithQueryClient(<StoreProfileSettingsPanel />);

    await screen.findByText("我的店铺配置");
    await screen.findByRole("option", { name: "US 备用店 (SHEIN-US-870 / US)" });

    fireEvent.change(screen.getByRole("combobox", { name: "SHEIN 店铺" }), {
      target: { value: "870" },
    });
    fireEvent.change(screen.getByLabelText("站点"), {
      target: { value: "CA" },
    });
    fireEvent.change(screen.getByLabelText("仓库编码"), {
      target: { value: "WH-CA-1" },
    });
    fireEvent.change(screen.getByLabelText("国家规则"), {
      target: { value: "CA, US" },
    });
    fireEvent.change(screen.getByLabelText("类目规则"), {
      target: { value: "shoes, jewelry" },
    });
    fireEvent.click(screen.getByRole("button", { name: "新增配置" }));

    await waitFor(() => {
      expect(mocks.mutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          store_id: 870,
          site: "CA",
          warehouse_code: "WH-CA-1",
          match_rules: [
            { kind: "country", values: ["CA", "US"] },
            { kind: "category", values: ["shoes", "jewelry"] },
          ],
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
