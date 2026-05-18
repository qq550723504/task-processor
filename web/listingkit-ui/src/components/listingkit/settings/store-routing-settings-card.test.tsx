import { fireEvent, render, screen } from "@testing-library/react";

import { StoreRoutingSettingsCard } from "@/components/listingkit/settings/store-routing-settings-card";

const mocks = vi.hoisted(() => ({
  mutate: vi.fn(),
  useStoreProfiles: vi.fn(),
  useStoreRouting: vi.fn(),
  useUpdateStoreRouting: vi.fn(),
}));

vi.mock("@/lib/query/use-store-profiles", () => ({
  useStoreProfiles: () => mocks.useStoreProfiles(),
}));

vi.mock("@/lib/query/use-store-routing", () => ({
  useStoreRouting: () => mocks.useStoreRouting(),
  useUpdateStoreRouting: () => mocks.useUpdateStoreRouting(),
}));

describe("StoreRoutingSettingsCard", () => {
  beforeEach(() => {
    mocks.mutate.mockReset();
    mocks.useStoreProfiles.mockReset();
    mocks.useStoreRouting.mockReset();
    mocks.useUpdateStoreRouting.mockReset();

    mocks.useStoreProfiles.mockReturnValue({
      data: [
        {
          id: 1,
          store_id: 869,
          enabled: true,
          site: "US",
          store: { name: "US 主店", store_id: "SHEIN-US-869", region: "US" },
        },
        {
          id: 2,
          store_id: 870,
          enabled: true,
          site: "US",
          store: { name: "US 备用店", store_id: "SHEIN-US-870", region: "US" },
        },
      ],
      isError: false,
    });
    mocks.useStoreRouting.mockReturnValue({
      data: {
        selection_strategy: "priority",
        fallback_store_id: 869,
        allow_manual_override: true,
        allow_fallback: true,
      },
      isError: false,
    });
    mocks.useUpdateStoreRouting.mockReturnValue({
      mutate: mocks.mutate,
      isPending: false,
      error: null,
    });
  });

  it("renders enabled store profiles as fallback options", () => {
    render(<StoreRoutingSettingsCard />);

    expect(screen.getByRole("option", { name: "US 主店 (US / US)" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "US 备用店 (US / US)" })).toBeInTheDocument();
    expect(
      screen.getByText(
        "`manual` 先尊重任务里显式指定的店铺；`priority` 按启用 profile 的优先级选；`country` 会优先匹配 profile 里的国家规则。",
      ),
    ).toBeInTheDocument();
  });

  it("saves store routing settings", () => {
    render(<StoreRoutingSettingsCard />);

    fireEvent.change(screen.getByRole("combobox", { name: "fallback 店铺" }), {
      target: { value: "870" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存路由策略" }));

    expect(mocks.mutate).toHaveBeenCalledWith({
      selection_strategy: "priority",
      fallback_store_id: 870,
      allow_manual_override: true,
      allow_fallback: true,
    });
  });
});
