import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { renderHook, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  useExecuteSheinActivityEnrollment,
  useRefreshSheinActivityCandidates,
  useReviewSheinActivityCandidate,
  useSheinActivityStrategy,
  useSheinActivityCandidates,
  useSheinActivityEnrollmentRunItems,
  useSheinActivityEnrollmentRuns,
  useSheinEnrollmentDashboard,
  useSheinEnrollmentStoreSummary,
  useSheinSDSCostGroups,
  useSheinSyncedProducts,
  useTriggerSheinStoreSync,
  useUpdateSheinActivityStrategy,
  useUpdateSheinSDSCostGroup,
  useUpdateSheinSyncedProductCost,
  shouldPollSheinSyncSummary,
} from "@/lib/query/use-shein-enrollment";

const mocks = vi.hoisted(() => ({
  getSheinEnrollmentDashboard: vi.fn(),
  getSheinEnrollmentStoreSummary: vi.fn(),
  getSheinActivityStrategy: vi.fn(),
  getSheinSDSCostGroups: vi.fn(),
  getSheinSyncedProducts: vi.fn(),
  getSheinActivityCandidates: vi.fn(),
  getSheinActivityEnrollmentRuns: vi.fn(),
  getSheinActivityEnrollmentRunItems: vi.fn(),
  triggerSheinStoreSync: vi.fn(),
  updateSheinActivityStrategy: vi.fn(),
  updateSheinSDSCostGroup: vi.fn(),
  updateSheinSyncedProductCost: vi.fn(),
  refreshSheinActivityCandidates: vi.fn(),
  reviewSheinActivityCandidate: vi.fn(),
  executeSheinActivityEnrollment: vi.fn(),
}));

vi.mock("@/lib/api/shein-enrollment", () => ({
  getSheinEnrollmentDashboard: (...args: unknown[]) =>
    mocks.getSheinEnrollmentDashboard(...args),
  getSheinEnrollmentStoreSummary: (...args: unknown[]) =>
    mocks.getSheinEnrollmentStoreSummary(...args),
  getSheinActivityStrategy: (...args: unknown[]) =>
    mocks.getSheinActivityStrategy(...args),
  getSheinSDSCostGroups: (...args: unknown[]) =>
    mocks.getSheinSDSCostGroups(...args),
  getSheinSyncedProducts: (...args: unknown[]) =>
    mocks.getSheinSyncedProducts(...args),
  getSheinActivityCandidates: (...args: unknown[]) =>
    mocks.getSheinActivityCandidates(...args),
  getSheinActivityEnrollmentRuns: (...args: unknown[]) =>
    mocks.getSheinActivityEnrollmentRuns(...args),
  getSheinActivityEnrollmentRunItems: (...args: unknown[]) =>
    mocks.getSheinActivityEnrollmentRunItems(...args),
  triggerSheinStoreSync: (...args: unknown[]) =>
    mocks.triggerSheinStoreSync(...args),
  updateSheinActivityStrategy: (...args: unknown[]) =>
    mocks.updateSheinActivityStrategy(...args),
  updateSheinSDSCostGroup: (...args: unknown[]) =>
    mocks.updateSheinSDSCostGroup(...args),
  updateSheinSyncedProductCost: (...args: unknown[]) =>
    mocks.updateSheinSyncedProductCost(...args),
  refreshSheinActivityCandidates: (...args: unknown[]) =>
    mocks.refreshSheinActivityCandidates(...args),
  reviewSheinActivityCandidate: (...args: unknown[]) =>
    mocks.reviewSheinActivityCandidate(...args),
  executeSheinActivityEnrollment: (...args: unknown[]) =>
    mocks.executeSheinActivityEnrollment(...args),
}));

beforeEach(() => {
  vi.clearAllMocks();
});

function createWrapper(client: QueryClient) {
  const Wrapper = ({ children }: { children: React.ReactNode }) => (
    <QueryClientProvider client={client}>{children}</QueryClientProvider>
  );
  Wrapper.displayName = "SheinEnrollmentQueryTestWrapper";
  return Wrapper;
}

describe("use-shein-enrollment", () => {
  it("loads dashboard, store summary, activity strategy, products, candidates, and runs through react query", async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    mocks.getSheinEnrollmentDashboard.mockResolvedValue({ items: [], total: 0 });
    mocks.getSheinEnrollmentStoreSummary.mockResolvedValue({ summary: { store_id: 5 } });
    mocks.getSheinActivityStrategy.mockResolvedValue({
      configured: true,
      strategy: { store_id: 5, activity_discount_rate: 0.2 },
    });
    mocks.getSheinSDSCostGroups.mockResolvedValue({ items: [], total: 0 });
    mocks.getSheinSyncedProducts.mockResolvedValue({ items: [], total: 0 });
    mocks.getSheinActivityCandidates.mockResolvedValue({ items: [], total: 0 });
    mocks.getSheinActivityEnrollmentRuns.mockResolvedValue({ items: [], total: 0 });
    mocks.getSheinActivityEnrollmentRunItems.mockResolvedValue({ items: [], total: 0 });

    const { result: dashboard } = renderHook(
      () => useSheinEnrollmentDashboard({ activity_type: "PROMOTION" }),
      { wrapper: createWrapper(client) },
    );
    const { result: summary } = renderHook(
      () => useSheinEnrollmentStoreSummary(5, { activity_type: "PROMOTION" }),
      { wrapper: createWrapper(client) },
    );
    const { result: products } = renderHook(
      () =>
        useSheinSyncedProducts(5, {
          skc_name: "dress",
          page: 1,
          page_size: 20,
        }),
      { wrapper: createWrapper(client) },
    );
    const { result: activityStrategy } = renderHook(
      () => useSheinActivityStrategy(5),
      { wrapper: createWrapper(client) },
    );
    const { result: sdsCostGroups } = renderHook(
      () =>
        useSheinSDSCostGroups(5, {
          page: 1,
          page_size: 100,
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
    const { result: runs } = renderHook(
      () =>
        useSheinActivityEnrollmentRuns(5, {
          activity_type: "flash_sale",
          page: 1,
          page_size: 20,
        }),
      { wrapper: createWrapper(client) },
    );
    const { result: runItems } = renderHook(
      () =>
        useSheinActivityEnrollmentRunItems(5, 99, {
          page: 1,
          page_size: 50,
        }),
      { wrapper: createWrapper(client) },
    );

    await waitFor(() => expect(dashboard.current.isSuccess).toBe(true));
    await waitFor(() => expect(summary.current.isSuccess).toBe(true));
    await waitFor(() => expect(products.current.isSuccess).toBe(true));
    await waitFor(() => expect(activityStrategy.current.isSuccess).toBe(true));
    await waitFor(() => expect(sdsCostGroups.current.isSuccess).toBe(true));
    await waitFor(() => expect(candidates.current.isSuccess).toBe(true));
    await waitFor(() => expect(runs.current.isSuccess).toBe(true));
    await waitFor(() => expect(runItems.current.isSuccess).toBe(true));

    expect(mocks.getSheinEnrollmentDashboard).toHaveBeenCalledWith({
      activity_type: "PROMOTION",
    });
    expect(mocks.getSheinEnrollmentStoreSummary).toHaveBeenCalledWith(5, {
      activity_type: "PROMOTION",
    });
    expect(mocks.getSheinSyncedProducts).toHaveBeenCalledWith(5, {
      skc_name: "dress",
      page: 1,
      page_size: 20,
    });
    expect(mocks.getSheinActivityStrategy).toHaveBeenCalledWith(5);
    expect(mocks.getSheinSDSCostGroups).toHaveBeenCalledWith(5, {
      page: 1,
      page_size: 100,
    });
    expect(mocks.getSheinActivityCandidates).toHaveBeenCalledWith(5, {
      activity_type: "flash_sale",
      page: 1,
      page_size: 20,
    });
    expect(mocks.getSheinActivityEnrollmentRuns).toHaveBeenCalledWith(5, {
      activity_type: "flash_sale",
      page: 1,
      page_size: 20,
    });
    expect(mocks.getSheinActivityEnrollmentRunItems).toHaveBeenCalledWith(5, 99, {
      page: 1,
      page_size: 50,
    });
  });

  it("does not fetch tab-scoped store data when disabled", async () => {
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });

    renderHook(
      () =>
        useSheinSyncedProducts(
          5,
          {
            page: 1,
            page_size: 20,
          },
          { enabled: false },
        ),
      { wrapper: createWrapper(client) },
    );
    renderHook(
      () =>
        useSheinSDSCostGroups(
          5,
          {
            page: 1,
            page_size: 20,
          },
          { enabled: false },
        ),
      { wrapper: createWrapper(client) },
    );
    renderHook(
      () =>
        useSheinActivityCandidates(
          5,
          {
            activity_type: "PROMOTION",
            page: 1,
            page_size: 20,
          },
          { enabled: false },
        ),
      { wrapper: createWrapper(client) },
    );
    renderHook(
      () =>
        useSheinActivityEnrollmentRuns(
          5,
          {
            activity_type: "PROMOTION",
            page: 1,
            page_size: 20,
          },
          { enabled: false },
      ),
      { wrapper: createWrapper(client) },
    );
    renderHook(
      () =>
        useSheinActivityEnrollmentRunItems(
          5,
          99,
          {
            page: 1,
            page_size: 50,
          },
          { enabled: false },
        ),
      { wrapper: createWrapper(client) },
    );

    await new Promise((resolve) => setTimeout(resolve, 0));

    expect(mocks.getSheinSyncedProducts).not.toHaveBeenCalled();
    expect(mocks.getSheinSDSCostGroups).not.toHaveBeenCalled();
    expect(mocks.getSheinActivityCandidates).not.toHaveBeenCalled();
    expect(mocks.getSheinActivityEnrollmentRuns).not.toHaveBeenCalled();
    expect(mocks.getSheinActivityEnrollmentRunItems).not.toHaveBeenCalled();
  });

  it("invalidates store-scoped shein enrollment queries after sync, strategy update, and refresh", async () => {
    const client = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    const invalidateQueries = vi.spyOn(client, "invalidateQueries");
    mocks.triggerSheinStoreSync.mockResolvedValue({ job: { id: 1 } });
    mocks.updateSheinActivityStrategy.mockResolvedValue({
      configured: true,
      strategy: { activity_discount_rate: 0.18 },
    });
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
    const { result: updateStrategy } = renderHook(
      () => useUpdateSheinActivityStrategy(5),
      { wrapper: createWrapper(client) },
    );

    await sync.current.mutateAsync({ trigger_mode: "manual" });
    await updateStrategy.current.mutateAsync({
      activity_price_mode: "DISCOUNT",
      activity_discount_rate: 0.18,
      activity_stock_ratio: 0.4,
    });
    await refresh.current.mutateAsync({ activity_type: "flash_sale" });

    await waitFor(() =>
      expect(invalidateQueries).toHaveBeenCalledTimes(3),
    );
    expect(mocks.updateSheinActivityStrategy).toHaveBeenCalledWith(5, {
      activity_price_mode: "DISCOUNT",
      activity_discount_rate: 0.18,
      activity_stock_ratio: 0.4,
    });
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

  it("polls store summary while the latest sync job is still running", async () => {
    mocks.getSheinEnrollmentStoreSummary.mockReset();
    const client = new QueryClient({
      defaultOptions: {
        queries: { retry: false },
        mutations: { retry: false },
      },
    });
    mocks.getSheinEnrollmentStoreSummary
      .mockResolvedValueOnce({
        summary: {
          store_id: 5,
          last_sync_status: "running",
          last_sync_job: { status: "running" },
        },
      })
      .mockResolvedValueOnce({
        summary: {
          store_id: 5,
          last_sync_status: "failed",
          last_sync_job: {
            status: "failed",
            error_summary: "shein login failed: 登录等待验证码",
          },
        },
      });

    const { result } = renderHook(
      () => useSheinEnrollmentStoreSummary(5, { activity_type: "PROMOTION" }),
      { wrapper: createWrapper(client) },
    );

    await waitFor(() =>
      expect(result.current.data?.summary?.last_sync_status).toBe("running"),
    );

    await waitFor(
      () =>
        expect(result.current.data?.summary?.last_sync_job?.error_summary).toBe(
          "shein login failed: 登录等待验证码",
        ),
      { timeout: 3_500 },
    );
    expect(mocks.getSheinEnrollmentStoreSummary).toHaveBeenCalledTimes(2);
  }, 5_000);

  it("only polls SHEIN sync summaries while the latest job is non-terminal", () => {
    expect(shouldPollSheinSyncSummary({ summary: { last_sync_status: "pending" } })).toBe(true);
    expect(
      shouldPollSheinSyncSummary({
        summary: { last_sync_job: { status: "running" } },
      }),
    ).toBe(true);
    expect(shouldPollSheinSyncSummary({ summary: { last_sync_status: "failed" } })).toBe(false);
    expect(shouldPollSheinSyncSummary({ summary: { last_sync_status: "succeeded" } })).toBe(false);
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
    mocks.updateSheinSDSCostGroup.mockResolvedValue({
      group: { group_key: "style:B3195DA6", manual_cost_price: 46.8 },
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
    const { result: updateGroupCost } = renderHook(
      () => useUpdateSheinSDSCostGroup(5),
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
    await updateGroupCost.current.mutateAsync({
      groupKey: "style:B3195DA6",
      group_label: "B3195DA6",
      manual_cost_price: 46.8,
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
    expect(mocks.updateSheinSDSCostGroup).toHaveBeenCalledWith(5, "style:B3195DA6", {
      group_label: "B3195DA6",
      manual_cost_price: 46.8,
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
      expect(invalidateQueries).toHaveBeenCalledTimes(4),
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
    expect(invalidateQueries).toHaveBeenNthCalledWith(4, {
      queryKey: ["listingkit", "shein-enrollment", 5],
    });
  });
});
