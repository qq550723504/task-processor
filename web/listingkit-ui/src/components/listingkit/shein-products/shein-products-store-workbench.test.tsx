import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinProductsStoreWorkbench } from "@/components/listingkit/shein-products/shein-products-store-workbench";

const mocks = vi.hoisted(() => ({
  useSheinEnrollmentStoreSummary: vi.fn(),
  useSheinSDSCostGroups: vi.fn(),
  useSheinSourceSDSCostGroups: vi.fn(),
  useSheinSyncedProducts: vi.fn(),
  useTriggerSheinStoreSync: vi.fn(),
  useSyncSheinSourceSDSProduct: vi.fn(),
  useUpdateSheinSDSCostGroup: vi.fn(),
  useUpdateSheinSyncedProductCost: vi.fn(),
}));

vi.mock("@/lib/query/use-shein-enrollment", () => ({
  useSheinEnrollmentStoreSummary: (...args: unknown[]) =>
    mocks.useSheinEnrollmentStoreSummary(...args),
  useSheinSDSCostGroups: (...args: unknown[]) =>
    mocks.useSheinSDSCostGroups(...args),
  useSheinSourceSDSCostGroups: (...args: unknown[]) =>
    mocks.useSheinSourceSDSCostGroups(...args),
  useSheinSyncedProducts: (...args: unknown[]) => mocks.useSheinSyncedProducts(...args),
  useTriggerSheinStoreSync: (...args: unknown[]) => mocks.useTriggerSheinStoreSync(...args),
  useSyncSheinSourceSDSProduct: (...args: unknown[]) =>
    mocks.useSyncSheinSourceSDSProduct(...args),
  useUpdateSheinSDSCostGroup: (...args: unknown[]) =>
    mocks.useUpdateSheinSDSCostGroup(...args),
  useUpdateSheinSyncedProductCost: (...args: unknown[]) =>
    mocks.useUpdateSheinSyncedProductCost(...args),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

function resolvedMutation() {
  return {
    isPending: false,
    mutateAsync: vi.fn().mockResolvedValue(undefined),
  };
}

function renderWorkbench({
  initialTab,
  products = [],
  sourceSDSCostGroups = [],
}: {
  initialTab?: string;
  products?: Array<Record<string, unknown>>;
  sourceSDSCostGroups?: Array<Record<string, unknown>>;
}) {
  mocks.useSheinEnrollmentStoreSummary.mockReturnValue({
    data: {
      summary: {
        store_id: 12,
        store_name: "SHEIN US",
        store_username: "shein-us",
        platform: "SHEIN",
        region: "US",
      },
    },
  });
  mocks.useSheinSyncedProducts.mockReturnValue({
    data: { items: products, total: products.length },
    isLoading: false,
  });
  mocks.useSheinSDSCostGroups.mockReturnValue({
    data: { items: [] },
    isLoading: false,
  });
  mocks.useSheinSourceSDSCostGroups.mockReturnValue({
    data: { items: sourceSDSCostGroups, total: sourceSDSCostGroups.length },
    isLoading: false,
  });
  mocks.useTriggerSheinStoreSync.mockReturnValue(resolvedMutation());
  mocks.useSyncSheinSourceSDSProduct.mockReturnValue(resolvedMutation());
  mocks.useUpdateSheinSDSCostGroup.mockReturnValue(resolvedMutation());
  mocks.useUpdateSheinSyncedProductCost.mockReturnValue(resolvedMutation());

  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <SheinProductsStoreWorkbench initialTab={initialTab} storeId={12} />
    </QueryClientProvider>,
  );
}

describe("SheinProductsStoreWorkbench", () => {
  it("renders product sync tabs without activity enrollment tabs", async () => {
    renderWorkbench({
      products: [
        {
          id: 8,
          main_image_url: "https://example.com/item.png",
          skc_name: "SKC-8",
          spu_code: "spu-123",
          supplier_code: "J0529021001",
          price_snapshot: "USD 29.99",
          inventory_snapshot: '{"total_inventory":999,"saleable_inventory":999}',
          shelf_status: "ON_SHELF",
        },
      ],
    });

    expect(await screen.findByRole("heading", { name: "SHEIN US" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "同步商品" })).toHaveAttribute(
      "href",
      "/listing-kits/shein-products/12?tab=products",
    );
    expect(screen.getByRole("link", { name: "成本价维护" })).toHaveAttribute(
      "href",
      "/listing-kits/shein-products/12?tab=costs",
    );
    expect(screen.queryByRole("link", { name: "候选池" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "报名记录" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "去检查登录" })).not.toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "同步售价" })).toBeInTheDocument();
    expect(screen.getByText("$29.99")).toBeInTheDocument();
  });

  it("keeps product and cost queries scoped to the active product tab", async () => {
    renderWorkbench({
      initialTab: "costs",
      sourceSDSCostGroups: [
        {
          group_key: "source:XB0608021001",
          group_label: "XB0608021001",
          source_code: "XB0608021001",
          product_count: 2,
          products: [],
        },
      ],
    });

    expect(await screen.findByRole("heading", { name: "SHEIN US" })).toBeInTheDocument();
    expect(mocks.useSheinSyncedProducts).toHaveBeenNthCalledWith(
      1,
      12,
      {
        skc_name: undefined,
        page: 1,
        page_size: 100,
      },
      { enabled: false },
    );
    expect(mocks.useSheinSourceSDSCostGroups).toHaveBeenCalledWith(
      12,
      {
        page: 1,
        page_size: 100,
      },
      { enabled: true },
    );
    expect(screen.getByText("XB0608021001 · 2 个商品")).toBeInTheDocument();
  });
});
