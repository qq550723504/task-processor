import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinEnrollmentStoreWorkbench } from "@/components/listingkit/shein-enrollment/shein-enrollment-store-workbench";

const mocks = vi.hoisted(() => ({
  useSheinEnrollmentStoreSummary: vi.fn(),
  useSheinSyncedProducts: vi.fn(),
  useSheinActivityCandidates: vi.fn(),
  useSheinActivityEnrollmentRuns: vi.fn(),
  useTriggerSheinStoreSync: vi.fn(),
  useRefreshSheinActivityCandidates: vi.fn(),
  useUpdateSheinSyncedProductCost: vi.fn(),
  useReviewSheinActivityCandidate: vi.fn(),
  useExecuteSheinActivityEnrollment: vi.fn(),
}));

vi.mock("@/lib/query/use-shein-enrollment", () => ({
  useSheinEnrollmentStoreSummary: (...args: unknown[]) =>
    mocks.useSheinEnrollmentStoreSummary(...args),
  useSheinSyncedProducts: (...args: unknown[]) => mocks.useSheinSyncedProducts(...args),
  useSheinActivityCandidates: (...args: unknown[]) => mocks.useSheinActivityCandidates(...args),
  useSheinActivityEnrollmentRuns: (...args: unknown[]) =>
    mocks.useSheinActivityEnrollmentRuns(...args),
  useTriggerSheinStoreSync: (...args: unknown[]) => mocks.useTriggerSheinStoreSync(...args),
  useRefreshSheinActivityCandidates: (...args: unknown[]) =>
    mocks.useRefreshSheinActivityCandidates(...args),
  useUpdateSheinSyncedProductCost: (...args: unknown[]) =>
    mocks.useUpdateSheinSyncedProductCost(...args),
  useReviewSheinActivityCandidate: (...args: unknown[]) =>
    mocks.useReviewSheinActivityCandidate(...args),
  useExecuteSheinActivityEnrollment: (...args: unknown[]) =>
    mocks.useExecuteSheinActivityEnrollment(...args),
}));

function resolvedMutation() {
  return {
    isPending: false,
    mutateAsync: vi.fn().mockResolvedValue(undefined),
  };
}

function renderWorkbench({
  initialTab,
  products = [],
  candidates = [],
}: {
  initialTab?: string;
  products?: Array<Record<string, unknown>>;
  candidates?: Array<Record<string, unknown>>;
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
    data: { items: products },
    isLoading: false,
  });
  mocks.useSheinActivityCandidates.mockReturnValue({
    data: { items: candidates },
    isLoading: false,
  });
  mocks.useSheinActivityEnrollmentRuns.mockReturnValue({
    data: { items: [] },
    isLoading: false,
  });
  mocks.useTriggerSheinStoreSync.mockReturnValue(resolvedMutation());
  mocks.useRefreshSheinActivityCandidates.mockReturnValue(resolvedMutation());
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
});
