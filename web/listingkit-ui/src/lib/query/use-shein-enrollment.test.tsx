import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import {
  useExecuteSheinActivityEnrollment,
  useRefreshSheinActivityCandidates,
  useReviewSheinActivityCandidate,
  useSheinActivityCandidates,
  useSheinSyncedProducts,
  useTriggerSheinStoreSync,
  useUpdateSheinSyncedProductCost,
} from "@/lib/query/use-shein-enrollment";

const mocks = vi.hoisted(() => ({
  getSheinSyncedProducts: vi.fn(),
  getSheinActivityCandidates: vi.fn(),
  triggerSheinStoreSync: vi.fn(),
  updateSheinSyncedProductCost: vi.fn(),
  refreshSheinActivityCandidates: vi.fn(),
  reviewSheinActivityCandidate: vi.fn(),
  executeSheinActivityEnrollment: vi.fn(),
}));

vi.mock("@/lib/api/shein-enrollment", () => ({
  getSheinSyncedProducts: (...args: unknown[]) =>
    mocks.getSheinSyncedProducts(...args),
  getSheinActivityCandidates: (...args: unknown[]) =>
    mocks.getSheinActivityCandidates(...args),
  triggerSheinStoreSync: (...args: unknown[]) =>
    mocks.triggerSheinStoreSync(...args),
  updateSheinSyncedProductCost: (...args: unknown[]) =>
    mocks.updateSheinSyncedProductCost(...args),
  refreshSheinActivityCandidates: (...args: unknown[]) =>
    mocks.refreshSheinActivityCandidates(...args),
  reviewSheinActivityCandidate: (...args: unknown[]) =>
    mocks.reviewSheinActivityCandidate(...args),
  executeSheinActivityEnrollment: (...args: unknown[]) =>
    mocks.executeSheinActivityEnrollment(...args),
}));

function createWrapper(client: QueryClient) {
  const Wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  Wrapper.displayName = "SheinEnrollmentQueryTestWrapper";
  return Wrapper;
}

describe("use-shein-enrollment", () => {
  it("loads synced products and activity candidates through react query", async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    mocks.getSheinSyncedProducts.mockResolvedValue({ items: [], total: 0 });
    mocks.getSheinActivityCandidates.mockResolvedValue({ items: [], total: 0 });

    const { result: products } = renderHook(
      () =>
        useSheinSyncedProducts(5, {
          skc_name: "dress",
          page: 1,
          page_size: 20,
        }),
      { wrapper: createWrapper(client) },
    );
    const { result: candidates } = renderHook(
      () =>
        useSheinActivityCandidates(5, {
          activity_type: "flash_sale",
          page: 1,
          page_size: 20,
        }),
      { wrapper: createWrapper(client) },
    );

    await waitFor(() => expect(products.current.isSuccess).toBe(true));
    await waitFor(() => expect(candidates.current.isSuccess).toBe(true));

    expect(mocks.getSheinSyncedProducts).toHaveBeenCalledWith(5, {
      skc_name: "dress",
      page: 1,
      page_size: 20,
    });
    expect(mocks.getSheinActivityCandidates).toHaveBeenCalledWith(5, {
      activity_type: "flash_sale",
      page: 1,
      page_size: 20,
    });
  });

  it("invalidates store-scoped shein enrollment queries after sync and refresh", async () => {
    const client = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    const invalidateQueries = vi.spyOn(client, "invalidateQueries");
    mocks.triggerSheinStoreSync.mockResolvedValue({ job: { id: 1 } });
    mocks.refreshSheinActivityCandidates.mockResolvedValue({
      result: { processed_count: 1 },
    });

    const { result: sync } = renderHook(
      () => useTriggerSheinStoreSync(5),
      { wrapper: createWrapper(client) },
    );
    const { result: refresh } = renderHook(
      () => useRefreshSheinActivityCandidates(5),
      { wrapper: createWrapper(client) },
    );

    await sync.current.mutateAsync({ trigger_mode: "manual" });
    await refresh.current.mutateAsync({ activity_type: "flash_sale" });

    await waitFor(() =>
      expect(invalidateQueries).toHaveBeenCalledTimes(2),
    );
    expect(invalidateQueries).toHaveBeenNthCalledWith(1, {
      queryKey: ["listingkit", "shein-enrollment", 5],
    });
    expect(invalidateQueries).toHaveBeenNthCalledWith(2, {
      queryKey: ["listingkit", "shein-enrollment", 5],
    });
  });

  it("invalidates store-scoped queries after cost update, candidate review, and enrollment execution", async () => {
    const client = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    const invalidateQueries = vi.spyOn(client, "invalidateQueries");
    mocks.updateSheinSyncedProductCost.mockResolvedValue({
      id: 8,
      manual_cost_price: 12.5,
    });
    mocks.reviewSheinActivityCandidate.mockResolvedValue({
      candidate: { id: 21, review_status: "approved" },
    });
    mocks.executeSheinActivityEnrollment.mockResolvedValue({
      run: { id: 99, status: "running" },
    });

    const { result: updateCost } = renderHook(
      () => useUpdateSheinSyncedProductCost(5),
      { wrapper: createWrapper(client) },
    );
    const { result: reviewCandidate } = renderHook(
      () => useReviewSheinActivityCandidate(5),
      { wrapper: createWrapper(client) },
    );
    const { result: executeEnrollment } = renderHook(
      () => useExecuteSheinActivityEnrollment(5),
      { wrapper: createWrapper(client) },
    );

    await updateCost.current.mutateAsync({
      productId: 8,
      manual_cost_price: 12.5,
    });
    await reviewCandidate.current.mutateAsync({
      candidateId: 21,
      input: {
        store_id: 5,
        review_status: "approved",
      },
    });
    await executeEnrollment.current.mutateAsync({
      activity_type: "PROMOTION",
      candidate_ids: [21],
      trigger_mode: "manual_confirmed",
    });

    expect(mocks.updateSheinSyncedProductCost).toHaveBeenCalledWith(8, {
      manual_cost_price: 12.5,
    });
    expect(mocks.reviewSheinActivityCandidate).toHaveBeenCalledWith(21, {
      store_id: 5,
      review_status: "approved",
    });
    expect(mocks.executeSheinActivityEnrollment).toHaveBeenCalledWith(5, {
      activity_type: "PROMOTION",
      candidate_ids: [21],
      trigger_mode: "manual_confirmed",
    });
    await waitFor(() =>
      expect(invalidateQueries).toHaveBeenCalledTimes(3),
    );
    expect(invalidateQueries).toHaveBeenNthCalledWith(1, {
      queryKey: ["listingkit", "shein-enrollment", 5],
    });
    expect(invalidateQueries).toHaveBeenNthCalledWith(2, {
      queryKey: ["listingkit", "shein-enrollment", 5],
    });
    expect(invalidateQueries).toHaveBeenNthCalledWith(3, {
      queryKey: ["listingkit", "shein-enrollment", 5],
    });
  });
});
