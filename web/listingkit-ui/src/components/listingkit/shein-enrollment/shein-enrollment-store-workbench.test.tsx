import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinEnrollmentStoreWorkbench } from "@/components/listingkit/shein-enrollment/shein-enrollment-store-workbench";

const mocks = vi.hoisted(() => ({
  useSheinEnrollmentStoreSummary: vi.fn(),
  useSheinSDSCostGroups: vi.fn(),
  useSheinSyncedProducts: vi.fn(),
  useSheinActivityCandidates: vi.fn(),
  useSheinActivityEnrollmentRuns: vi.fn(),
  useTriggerSheinStoreSync: vi.fn(),
  useRefreshSheinActivityCandidates: vi.fn(),
  useUpdateSheinSDSCostGroup: vi.fn(),
  useUpdateSheinSyncedProductCost: vi.fn(),
  useReviewSheinActivityCandidate: vi.fn(),
  useExecuteSheinActivityEnrollment: vi.fn(),
}));

vi.mock("@/lib/query/use-shein-enrollment", () => ({
  useSheinEnrollmentStoreSummary: (...args: unknown[]) =>
    mocks.useSheinEnrollmentStoreSummary(...args),
  useSheinSDSCostGroups: (...args: unknown[]) =>
    mocks.useSheinSDSCostGroups(...args),
  useSheinSyncedProducts: (...args: unknown[]) => mocks.useSheinSyncedProducts(...args),
  useSheinActivityCandidates: (...args: unknown[]) => mocks.useSheinActivityCandidates(...args),
  useSheinActivityEnrollmentRuns: (...args: unknown[]) =>
    mocks.useSheinActivityEnrollmentRuns(...args),
  useTriggerSheinStoreSync: (...args: unknown[]) => mocks.useTriggerSheinStoreSync(...args),
  useRefreshSheinActivityCandidates: (...args: unknown[]) =>
    mocks.useRefreshSheinActivityCandidates(...args),
  useUpdateSheinSDSCostGroup: (...args: unknown[]) =>
    mocks.useUpdateSheinSDSCostGroup(...args),
  useUpdateSheinSyncedProductCost: (...args: unknown[]) =>
    mocks.useUpdateSheinSyncedProductCost(...args),
  useReviewSheinActivityCandidate: (...args: unknown[]) =>
    mocks.useReviewSheinActivityCandidate(...args),
  useExecuteSheinActivityEnrollment: (...args: unknown[]) =>
    mocks.useExecuteSheinActivityEnrollment(...args),
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
  productTotal,
  candidates = [],
  candidateTotal,
  summary,
  sdsCostGroups = [],
  runTotal,
}: {
  initialTab?: string;
  products?: Array<Record<string, unknown>>;
  productTotal?: number;
  candidates?: Array<Record<string, unknown>>;
  candidateTotal?: number;
  summary?: Record<string, unknown>;
  sdsCostGroups?: Array<Record<string, unknown>>;
  runTotal?: number;
}) {
  mocks.useSheinEnrollmentStoreSummary.mockReturnValue({
    data: {
      summary: {
        store_id: 12,
        store_name: "SHEIN US",
        store_username: "shein-us",
        platform: "SHEIN",
        region: "US",
        ...summary,
      },
    },
  });
  mocks.useSheinSyncedProducts.mockReturnValue({
    data: { items: products, total: productTotal ?? products.length },
    isLoading: false,
  });
  mocks.useSheinSDSCostGroups.mockReturnValue({
    data: { items: sdsCostGroups },
    isLoading: false,
  });
  mocks.useSheinActivityCandidates.mockReturnValue({
    data: { items: candidates, total: candidateTotal ?? candidates.length },
    isLoading: false,
  });
  mocks.useSheinActivityEnrollmentRuns.mockReturnValue({
    data: { items: [], total: runTotal ?? 0 },
    isLoading: false,
  });
  mocks.useTriggerSheinStoreSync.mockReturnValue(resolvedMutation());
  mocks.useRefreshSheinActivityCandidates.mockReturnValue(resolvedMutation());
  mocks.useUpdateSheinSDSCostGroup.mockReturnValue(resolvedMutation());
  mocks.useUpdateSheinSyncedProductCost.mockReturnValue(resolvedMutation());
  mocks.useReviewSheinActivityCandidate.mockReturnValue(resolvedMutation());
  mocks.useExecuteSheinActivityEnrollment.mockReturnValue(resolvedMutation());

  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <SheinEnrollmentStoreWorkbench initialTab={initialTab} storeId={12} />
    </QueryClientProvider>,
  );
}

describe("SheinEnrollmentStoreWorkbench", () => {
  it("defaults to the candidates tab and carries activityType in links", async () => {
    renderWorkbench({});

    expect(await screen.findByRole("heading", { name: "SHEIN US" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "候选池" })).toHaveAttribute(
      "href",
      "/listing-kits/shein-enrollment/12?tab=candidates&activityType=PROMOTION",
    );
  });

  it("renders dense product details in the synced products tab", async () => {
    renderWorkbench({
      initialTab: "products",
      products: [
        {
          id: 8,
          main_image_url: "https://example.com/item.png",
          skc_name: "SKC-8",
          spu_code: "spu-123",
          product_name_multi: "Dress",
          supplier_code: "J0529021001",
          sale_name: "White",
          price_snapshot: "USD 29.99",
          effective_cost_price: 12.5,
          cost_price_source: "manual",
          inventory_snapshot: '{"total_inventory":999,"saleable_inventory":999}',
          shelf_status: "ON_SHELF",
          created_at: "2026-06-01 01:38:43",
          publish_time: "2026-06-02 02:58:40",
          first_shelf_time: "2026-06-02 21:04:59",
        },
      ],
    });

    expect(await screen.findByRole("heading", { name: "SHEIN US" })).toBeInTheDocument();
    expect(screen.getByText("$29.99")).toBeInTheDocument();
    expect(screen.getByText("SPU: spu-123")).toBeInTheDocument();
    expect(screen.getByText("货号: J0529021001")).toBeInTheDocument();
    expect(screen.getByText("总库存 999")).toBeInTheDocument();
  });

  it("renders price snapshot in the candidates tab", async () => {
    renderWorkbench({
      initialTab: "candidates",
      candidates: [
        {
          id: 18,
          skc_name: "SKC-18",
          review_status: "pending_review",
          effective_cost_price: 12.5,
          price_snapshot: "USD 29.99",
        },
      ],
    });

    expect(await screen.findByRole("heading", { name: "SHEIN US" })).toBeInTheDocument();
    expect(screen.getByText(/售价 \$29.99/)).toBeInTheDocument();
  });

  it("shows the latest async sync failure from the store summary", async () => {
    renderWorkbench({
      initialTab: "candidates",
      summary: {
        last_sync_status: "failed",
        last_sync_job: {
          status: "failed",
          error_summary: "shein login failed: 登录等待验证码",
        },
      },
    });

    expect(await screen.findByText("最近同步失败")).toBeInTheDocument();
    expect(screen.getByText("shein login failed: 登录等待验证码")).toBeInTheDocument();
  });

  it("paginates candidates with backend page parameters", async () => {
    renderWorkbench({
      initialTab: "candidates",
      candidateTotal: 101,
      candidates: [
        {
          id: 18,
          skc_name: "SKC-18",
          review_status: "pending_review",
        },
      ],
    });

    expect(await screen.findByText("第 1 / 2 页 · 共 101 条")).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "下一页" }));

    expect(mocks.useSheinActivityCandidates).toHaveBeenLastCalledWith(12, {
      activity_type: "PROMOTION",
      page: 2,
      page_size: 100,
    }, {
      enabled: true,
    });
  });

  it("groups cost rows by POD SDS source code in the cost tab", async () => {
    renderWorkbench({
      initialTab: "costs",
      products: [
        {
          id: 8,
          skc_name: "sg260604223794143925005",
          supplier_code: "XB0608021001-DA578653",
          auto_cost_price: 39.1,
          effective_cost_price: 39.1,
        },
        {
          id: 9,
          skc_name: "sg260603162031320517713",
          supplier_code: "XB0608021001-DE93508C",
          auto_cost_price: 46.8,
          effective_cost_price: 46.8,
        },
      ],
      sdsCostGroups: [
        {
          group_key: "style:DA578653",
          group_label: "DA578653",
          manual_cost_price: 50,
        },
      ],
    });

    expect(await screen.findByText("XB0608021001 · 2 个商品")).toBeInTheDocument();
    expect(screen.queryByText("DA578653 · 1 个商品")).not.toBeInTheDocument();
    expect(screen.queryByText("DE93508C · 1 个商品")).not.toBeInTheDocument();
    expect(screen.getByDisplayValue("50")).toBeInTheDocument();
  });

  it("only enables backend queries needed by the active costs tab", async () => {
    renderWorkbench({
      initialTab: "costs",
      products: [
        {
          id: 8,
          skc_name: "SKC-A",
          supplier_code: "MG8006905001-B3195DA6",
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
    expect(mocks.useSheinSyncedProducts).toHaveBeenNthCalledWith(
      2,
      12,
      {
        page: 1,
        page_size: 100,
      },
      { enabled: true },
    );
    expect(mocks.useSheinSDSCostGroups).toHaveBeenCalledWith(
      12,
      {
        page: 1,
        page_size: 100,
      },
      { enabled: true },
    );
    expect(mocks.useSheinActivityCandidates).toHaveBeenCalledWith(
      12,
      {
        activity_type: "PROMOTION",
        page: 1,
        page_size: 100,
      },
      { enabled: false },
    );
    expect(mocks.useSheinActivityEnrollmentRuns).toHaveBeenCalledWith(
      12,
      {
        activity_type: "PROMOTION",
        page: 1,
        page_size: 100,
      },
      { enabled: false },
    );
  });
});
