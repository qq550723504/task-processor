import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinEnrollmentStoreWorkbench } from "@/components/listingkit/shein-enrollment/shein-enrollment-store-workbench";

const mocks = vi.hoisted(() => ({
  getTenantListingStores: vi.fn(),
  useSheinSyncedProducts: vi.fn(),
  useSheinActivityCandidates: vi.fn(),
  useTriggerSheinStoreSync: vi.fn(),
  useRefreshSheinActivityCandidates: vi.fn(),
  useUpdateSheinSyncedProductCost: vi.fn(),
  useReviewSheinActivityCandidate: vi.fn(),
  useExecuteSheinActivityEnrollment: vi.fn(),
}));

vi.mock("@/lib/api/tenant-stores", () => ({
  getTenantListingStores: (...args: unknown[]) => mocks.getTenantListingStores(...args),
}));

vi.mock("@/lib/query/use-shein-enrollment", () => ({
  useSheinSyncedProducts: (...args: unknown[]) => mocks.useSheinSyncedProducts(...args),
  useSheinActivityCandidates: (...args: unknown[]) => mocks.useSheinActivityCandidates(...args),
  useTriggerSheinStoreSync: (...args: unknown[]) => mocks.useTriggerSheinStoreSync(...args),
  useRefreshSheinActivityCandidates: (...args: unknown[]) => mocks.useRefreshSheinActivityCandidates(...args),
  useUpdateSheinSyncedProductCost: (...args: unknown[]) => mocks.useUpdateSheinSyncedProductCost(...args),
  useReviewSheinActivityCandidate: (...args: unknown[]) => mocks.useReviewSheinActivityCandidate(...args),
  useExecuteSheinActivityEnrollment: (...args: unknown[]) => mocks.useExecuteSheinActivityEnrollment(...args),
}));

function resolvedMutation() {
  return {
    isPending: false,
    mutateAsync: vi.fn().mockResolvedValue(undefined),
  };
}

describe("SheinEnrollmentStoreWorkbench", () => {
  it("defaults to the candidates tab and carries activityType in links", async () => {
    mocks.getTenantListingStores.mockResolvedValue({
      items: [
        {
          id: 12,
          name: "SHEIN US",
          username: "shein-us",
          platform: "SHEIN",
          region: "US",
        },
      ],
      total: 1,
      page: 1,
      page_size: 100,
    });
    mocks.useSheinSyncedProducts.mockReturnValue({ data: { items: [] }, isLoading: false });
    mocks.useSheinActivityCandidates.mockReturnValue({ data: { items: [] }, isLoading: false });
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
        <SheinEnrollmentStoreWorkbench storeId={12} />
      </QueryClientProvider>,
    );

    expect(await screen.findByRole("heading", { name: "SHEIN US" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "候选池" })).toHaveAttribute(
      "href",
      "/listing-kits/shein-enrollment/12?tab=candidates&activityType=PROMOTION",
    );
    expect(screen.getByText("当前活动类型下暂无候选商品。")).toBeInTheDocument();
  });
});
