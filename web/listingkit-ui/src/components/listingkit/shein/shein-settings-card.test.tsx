import { fireEvent, render, screen } from "@testing-library/react";

import { SheinSettingsCard } from "@/components/listingkit/shein/shein-settings-card";

const mocks = vi.hoisted(() => ({
  mutate: vi.fn(),
  useSheinSettings: vi.fn(),
  useUpdateSheinSettings: vi.fn(),
}));

vi.mock("@/lib/query/use-shein-settings", () => ({
  useSheinSettings: () => mocks.useSheinSettings(),
  useUpdateSheinSettings: () => mocks.useUpdateSheinSettings(),
}));

describe("SheinSettingsCard", () => {
  beforeEach(() => {
    mocks.mutate.mockReset();
    mocks.useSheinSettings.mockReset();
    mocks.useUpdateSheinSettings.mockReset();
    mocks.useSheinSettings.mockReturnValue({
      data: {
        default_store_id: 869,
        available_stores: [
          { id: 869, store_id: "SHEIN-US-869", name: "US 主店", region: "us" },
          { id: 870, store_id: "SHEIN-US-870", name: "US 备用店", region: "us" },
        ],
        site: "US",
        warehouse_code: "DEFAULT",
        default_stock: 100,
        default_submit_mode: "publish",
        pricing: {
          exchange_rate: 7.2,
          markup_multiplier: 2,
          minimum_price: 9.99,
          round_to: 0.01,
          price_ending: 0.99,
        },
      },
      isLoading: false,
      isError: false,
    });
    mocks.useUpdateSheinSettings.mockReturnValue({
      mutate: mocks.mutate,
      isPending: false,
      error: null,
    });
  });

  it("renders current tenant stores from settings payload", () => {
    render(<SheinSettingsCard />);

    expect(screen.getByRole("option", { name: "US 主店 (SHEIN-US-869 / us)" })).toBeInTheDocument();
    expect(screen.getByRole("option", { name: "US 备用店 (SHEIN-US-870 / us)" })).toBeInTheDocument();
  });

  it("saves selected store id from the tenant store list", () => {
    render(<SheinSettingsCard />);

    fireEvent.change(screen.getByRole("combobox", { name: "默认店铺" }), {
      target: { value: "870" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存配置" }));

    expect(mocks.mutate).toHaveBeenCalledWith(
      expect.objectContaining({
        default_store_id: 870,
      }),
    );
  });
});
