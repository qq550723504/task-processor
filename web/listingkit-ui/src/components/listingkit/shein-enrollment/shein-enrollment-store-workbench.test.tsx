import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { SheinEnrollmentStoreWorkbench } from "@/components/listingkit/shein-enrollment/shein-enrollment-store-workbench";

const mocks = vi.hoisted(() => ({
  useSheinEnrollmentStoreSummary: vi.fn(),
  useSheinActivityStrategy: vi.fn(),
  useSheinSDSCostGroups: vi.fn(),
  useSheinSourceSDSCostGroups: vi.fn(),
  useSheinSyncedProducts: vi.fn(),
  useSheinActivityCandidates: vi.fn(),
  useSheinActivityEnrollmentRuns: vi.fn(),
  useSheinActivityEnrollmentRunItems: vi.fn(),
  useTriggerSheinStoreSync: vi.fn(),
  useSyncSheinSourceSDSProduct: vi.fn(),
  useRefreshSheinActivityCandidates: vi.fn(),
  useResetSheinActivityCandidates: vi.fn(),
  useUpdateSheinActivityStrategy: vi.fn(),
  useUpdateSheinSDSCostGroup: vi.fn(),
  useUpdateSheinSyncedProductCost: vi.fn(),
  useReviewSheinActivityCandidate: vi.fn(),
  useExecuteSheinActivityEnrollment: vi.fn(),
}));

vi.mock("@/lib/query/use-shein-enrollment", () => ({
  useSheinEnrollmentStoreSummary: (...args: unknown[]) =>
    mocks.useSheinEnrollmentStoreSummary(...args),
  useSheinActivityStrategy: (...args: unknown[]) =>
    mocks.useSheinActivityStrategy(...args),
  useSheinSDSCostGroups: (...args: unknown[]) =>
    mocks.useSheinSDSCostGroups(...args),
  useSheinSourceSDSCostGroups: (...args: unknown[]) =>
    mocks.useSheinSourceSDSCostGroups(...args),
  useSheinSyncedProducts: (...args: unknown[]) =>
    mocks.useSheinSyncedProducts(...args),
  useSheinActivityCandidates: (...args: unknown[]) =>
    mocks.useSheinActivityCandidates(...args),
  useSheinActivityEnrollmentRuns: (...args: unknown[]) =>
    mocks.useSheinActivityEnrollmentRuns(...args),
  useSheinActivityEnrollmentRunItems: (...args: unknown[]) =>
    mocks.useSheinActivityEnrollmentRunItems(...args),
  useTriggerSheinStoreSync: (...args: unknown[]) =>
    mocks.useTriggerSheinStoreSync(...args),
  useSyncSheinSourceSDSProduct: (...args: unknown[]) =>
    mocks.useSyncSheinSourceSDSProduct(...args),
  useRefreshSheinActivityCandidates: (...args: unknown[]) =>
    mocks.useRefreshSheinActivityCandidates(...args),
  useResetSheinActivityCandidates: (...args: unknown[]) =>
    mocks.useResetSheinActivityCandidates(...args),
  useUpdateSheinActivityStrategy: (...args: unknown[]) =>
    mocks.useUpdateSheinActivityStrategy(...args),
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
  initialActivityType,
  initialTab,
  products = [],
  productTotal,
  candidates = [],
  candidateTotal,
  runs = [],
  runItems = [],
  summary,
  activityStrategyResponse,
  updateStrategyMutation,
  resetCandidatesMutation,
  sdsCostGroups = [],
  sourceSDSCostGroups = [],
  runTotal,
  syncSourceMutation,
  enrollMutation,
}: {
  initialActivityType?: string;
  initialTab?: string;
  products?: Array<Record<string, unknown>>;
  productTotal?: number;
  candidates?: Array<Record<string, unknown>>;
  candidateTotal?: number;
  runs?: Array<Record<string, unknown>>;
  runItems?: Array<Record<string, unknown>>;
  summary?: Record<string, unknown>;
  activityStrategyResponse?: Record<string, unknown>;
  updateStrategyMutation?: ReturnType<typeof resolvedMutation>;
  resetCandidatesMutation?: ReturnType<typeof resolvedMutation>;
  sdsCostGroups?: Array<Record<string, unknown>>;
  sourceSDSCostGroups?: Array<Record<string, unknown>>;
  runTotal?: number;
  syncSourceMutation?: ReturnType<typeof resolvedMutation>;
  enrollMutation?: ReturnType<typeof resolvedMutation>;
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
  mocks.useSheinSourceSDSCostGroups.mockReturnValue({
    data: { items: sourceSDSCostGroups, total: sourceSDSCostGroups.length },
    isLoading: false,
  });
  mocks.useSheinActivityCandidates.mockReturnValue({
    data: { items: candidates, total: candidateTotal ?? candidates.length },
    isLoading: false,
  });
  mocks.useSheinActivityStrategy.mockReturnValue({
    data: activityStrategyResponse ?? {
      configured: true,
      strategy: {
        activity_price_mode: "DISCOUNT",
        activity_discount_rate: 0.2,
        activity_stock_ratio: 0.5,
        activity_min_profit_rate: 0.15,
        fixed_price_adjustment: 0,
      },
    },
    isLoading: false,
  });
  mocks.useSheinActivityEnrollmentRuns.mockReturnValue({
    data: { items: runs, total: runTotal ?? runs.length },
    isLoading: false,
  });
  mocks.useSheinActivityEnrollmentRunItems.mockReturnValue({
    data: { items: runItems, total: runItems.length },
    isLoading: false,
  });
  mocks.useTriggerSheinStoreSync.mockReturnValue(resolvedMutation());
  mocks.useSyncSheinSourceSDSProduct.mockReturnValue(
    syncSourceMutation ?? resolvedMutation(),
  );
  mocks.useRefreshSheinActivityCandidates.mockReturnValue(resolvedMutation());
  mocks.useResetSheinActivityCandidates.mockReturnValue(
    resetCandidatesMutation ?? resolvedMutation(),
  );
  mocks.useUpdateSheinActivityStrategy.mockReturnValue(
    updateStrategyMutation ?? resolvedMutation(),
  );
  mocks.useUpdateSheinSDSCostGroup.mockReturnValue(resolvedMutation());
  mocks.useUpdateSheinSyncedProductCost.mockReturnValue(resolvedMutation());
  mocks.useReviewSheinActivityCandidate.mockReturnValue(resolvedMutation());
  mocks.useExecuteSheinActivityEnrollment.mockReturnValue(
    enrollMutation ?? resolvedMutation(),
  );

  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  render(
    <QueryClientProvider client={client}>
      <SheinEnrollmentStoreWorkbench
        initialActivityType={initialActivityType}
        initialTab={initialTab}
        storeId={12}
      />
    </QueryClientProvider>,
  );
}

describe("SheinEnrollmentStoreWorkbench", () => {
  it("defaults to the candidates tab and carries activityType in links", async () => {
    renderWorkbench({});

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "候选池" })).toHaveAttribute(
      "href",
      "/listing-kits/shein-enrollment/12?tab=candidates&activityType=PROMOTION",
    );
    expect(
      screen.queryByRole("link", { name: "同步商品" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "成本价维护" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("option", { name: "混合活动" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "立即同步" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: "去检查登录" }),
    ).not.toBeInTheDocument();
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
          inventory_snapshot:
            '{"total_inventory":999,"saleable_inventory":999}',
          shelf_status: "ON_SHELF",
          created_at: "2026-06-01 01:38:43",
          publish_time: "2026-06-02 02:58:40",
          first_shelf_time: "2026-06-02 21:04:59",
        },
      ],
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/售价\s+\$29\.99/)).toBeInTheDocument();
    expect(screen.getByText(/SPU:\s+spu-123/)).toBeInTheDocument();
    expect(screen.getByText(/货号:\s+J0529021001/)).toBeInTheDocument();
    expect(screen.getByText(/总库存\s+999/)).toBeInTheDocument();
  });

  it("renders price snapshot in the candidates tab", async () => {
    renderWorkbench({
      initialTab: "candidates",
      candidates: [
        {
          id: 18,
          skc_name: "SKC-18",
          main_image_url: "https://example.com/skc-18.png",
          review_status: "pending_review",
          effective_cost_price: 12.5,
          price_snapshot: "USD 29.99",
        },
      ],
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/售价 \$29.99/)).toBeInTheDocument();
    expect(screen.getByRole("img", { name: "SKC-18 图片" })).toHaveAttribute(
      "src",
      "https://example.com/skc-18.png",
    );
    const thumbnail = screen.getByRole("img", { name: "SKC-18 图片" });
    expect(thumbnail.parentElement).toHaveClass("cursor-zoom-in");
    expect(screen.getByAltText("SKC-18 悬浮预览").parentElement).toHaveClass(
      "group-hover:block",
    );
  });

  it("filters candidates to executable enrollment items", async () => {
    renderWorkbench({
      initialTab: "candidates",
      candidates: [
        {
          id: 18,
          skc_name: "SKC-PENDING",
          review_status: "pending_review",
        },
      ],
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();
    expect(mocks.useSheinActivityCandidates).toHaveBeenLastCalledWith(
      12,
      {
        activity_type: "PROMOTION",
        executable_only: undefined,
        page: 1,
        page_size: 100,
      },
      { enabled: true },
    );

    fireEvent.click(screen.getByLabelText("只看可报名"));

    await waitFor(() => {
      expect(mocks.useSheinActivityCandidates).toHaveBeenLastCalledWith(
        12,
        {
          activity_type: "PROMOTION",
          executable_only: true,
          page: 1,
          page_size: 100,
        },
        { enabled: true },
      );
    });
  });

  it("does not expose manual approval from the candidates table", async () => {
    renderWorkbench({
      initialTab: "candidates",
      candidates: [
        {
          id: 18,
          skc_name: "SKC-PENDING",
          review_status: "pending_review",
        },
      ],
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "通过" }),
    ).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "驳回" })).toBeInTheDocument();
  });

  it("allows pending review candidates to be manually confirmed for enrollment", async () => {
    const enrollMutation = resolvedMutation();
    renderWorkbench({
      initialTab: "candidates",
      enrollMutation,
      candidates: [
        {
          id: 18,
          skc_name: "SKC-PENDING",
          review_status: "pending_review",
        },
        {
          id: 19,
          skc_name: "SKC-REJECTED",
          review_status: "rejected",
        },
      ],
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();

    expect(screen.getByLabelText("选择 SKC-PENDING")).not.toBeDisabled();
    fireEvent.click(screen.getByLabelText("选择 SKC-PENDING"));
    expect(screen.getByLabelText("选择 SKC-REJECTED")).toBeEnabled();
    fireEvent.click(screen.getByRole("button", { name: "报名活动" }));

    expect(enrollMutation.mutateAsync).toHaveBeenCalledWith({
      activity_type: "PROMOTION",
      activity_key: undefined,
      trigger_mode: "manual_confirmed",
      candidate_ids: [18],
    });
  });

  it("resets a selected candidate row without enabling it for enrollment", async () => {
    const resetCandidatesMutation = resolvedMutation();
    renderWorkbench({
      initialActivityType: "TIME_LIMITED",
      initialTab: "candidates",
      resetCandidatesMutation,
      candidates: [
        {
          id: 18,
          skc_name: "SKC-MISSING-COST",
          review_status: "failed",
          eligibility_status: "ineligible",
          eligibility_reason: "missing effective cost price",
        },
      ],
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();

    expect(screen.getByLabelText("选择 SKC-MISSING-COST")).toBeEnabled();
    fireEvent.click(screen.getByLabelText("选择 SKC-MISSING-COST"));
    expect(screen.getByRole("button", { name: "报名活动" })).toBeDisabled();
    fireEvent.click(
      screen.getByRole("button", { name: "重置 SKC-MISSING-COST 状态" }),
    );

    expect(resetCandidatesMutation.mutateAsync).toHaveBeenCalledWith({
      activity_type: "TIME_LIMITED",
      candidate_ids: [18],
    });
  });

  it("resets selected candidates in a single batch request", async () => {
    const resetCandidatesMutation = resolvedMutation();
    renderWorkbench({
      initialActivityType: "TIME_LIMITED",
      initialTab: "candidates",
      resetCandidatesMutation,
      candidates: [
        {
          id: 18,
          skc_name: "SKC-READY-1",
          review_status: "pending_review",
        },
        {
          id: 19,
          skc_name: "SKC-READY-2",
          review_status: "approved",
        },
      ],
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("选择 SKC-READY-1"));
    fireEvent.click(screen.getByLabelText("选择 SKC-READY-2"));
    fireEvent.click(screen.getByRole("button", { name: "批量重置已选" }));

    expect(resetCandidatesMutation.mutateAsync).toHaveBeenCalledWith({
      activity_type: "TIME_LIMITED",
      candidate_ids: [18, 19],
    });
  });

  it("selects all visible candidates for batch reset even when none are enrollable", async () => {
    const resetCandidatesMutation = resolvedMutation();
    renderWorkbench({
      initialActivityType: "TIME_LIMITED",
      initialTab: "candidates",
      resetCandidatesMutation,
      candidates: [
        {
          id: 18,
          skc_name: "SKC-FAILED",
          review_status: "failed",
        },
        {
          id: 19,
          skc_name: "SKC-REJECTED",
          review_status: "rejected",
        },
      ],
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "全选" }));
    expect(screen.getByLabelText("选择 SKC-FAILED")).toBeChecked();
    expect(screen.getByLabelText("选择 SKC-REJECTED")).toBeChecked();
    expect(screen.getByRole("button", { name: "报名活动" })).toBeDisabled();

    fireEvent.click(screen.getByRole("button", { name: "批量重置已选" }));

    expect(resetCandidatesMutation.mutateAsync).toHaveBeenCalledWith({
      activity_type: "TIME_LIMITED",
      candidate_ids: [18, 19],
    });
  });

  it("ignores duplicate manual enrollment clicks while a request is in flight", async () => {
    const enrollMutation = {
      isPending: false,
      mutateAsync: vi
        .fn()
        .mockReturnValue(new Promise<undefined>(() => undefined)),
    };
    renderWorkbench({
      initialTab: "candidates",
      enrollMutation,
      candidates: [
        {
          id: 18,
          skc_name: "SKC-PENDING",
          review_status: "pending_review",
        },
      ],
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("选择 SKC-PENDING"));
    const enrollButton = screen.getByRole("button", { name: "报名活动" });
    fireEvent.click(enrollButton);
    fireEvent.click(enrollButton);

    expect(enrollMutation.mutateAsync).toHaveBeenCalledTimes(1);
    expect(enrollMutation.mutateAsync).toHaveBeenCalledWith({
      activity_type: "PROMOTION",
      activity_key: undefined,
      trigger_mode: "manual_confirmed",
      candidate_ids: [18],
    });
  });

  it("selects all visible candidates while enrollment submits only executable candidates", async () => {
    const enrollMutation = resolvedMutation();
    renderWorkbench({
      initialTab: "candidates",
      enrollMutation,
      candidates: [
        {
          id: 18,
          skc_name: "SKC-PENDING",
          review_status: "pending_review",
        },
        {
          id: 19,
          skc_name: "SKC-REJECTED",
          review_status: "rejected",
        },
        {
          id: 20,
          skc_name: "SKC-FAILED",
          review_status: "failed",
          last_enrollment_error:
            "SHEIN rejected: current status can not enroll",
        },
      ],
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "全选" }));
    expect(screen.getByLabelText("选择 SKC-PENDING")).toBeChecked();
    expect(screen.getByLabelText("选择 SKC-REJECTED")).toBeChecked();
    expect(screen.getByLabelText("选择 SKC-REJECTED")).toBeEnabled();
    expect(screen.getByLabelText("选择 SKC-FAILED")).toBeChecked();
    expect(screen.getByLabelText("选择 SKC-FAILED")).toBeEnabled();
    expect(
      screen.getByText(/SHEIN rejected: current status can not enroll/),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "报名活动" }));

    expect(enrollMutation.mutateAsync).toHaveBeenCalledWith({
      activity_type: "PROMOTION",
      activity_key: undefined,
      trigger_mode: "manual_confirmed",
      candidate_ids: [18],
    });
  });

  it("requires activity strategy before manual enrollment", async () => {
    renderWorkbench({
      initialTab: "candidates",
      activityStrategyResponse: { configured: false, strategy: null },
      candidates: [
        {
          id: 18,
          skc_name: "SKC-18",
          review_status: "approved",
        },
      ],
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByLabelText("选择 SKC-18"));

    expect(screen.getByRole("button", { name: "报名活动" })).toBeDisabled();
    expect(screen.getByText("先完善活动报名设置")).toBeInTheDocument();
  });

  it("saves activity strategy from the candidates tab", async () => {
    const updateStrategyMutation = resolvedMutation();
    renderWorkbench({
      initialTab: "candidates",
      activityStrategyResponse: { configured: false, strategy: null },
      updateStrategyMutation,
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();

    fireEvent.change(screen.getByRole("spinbutton", { name: "常规折扣率" }), {
      target: { value: "0.18" },
    });
    fireEvent.change(screen.getByRole("spinbutton", { name: "固定调价" }), {
      target: { value: "1.2" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存活动设置" }));

    expect(updateStrategyMutation.mutateAsync).toHaveBeenCalledWith({
      activity_type: "PROMOTION",
      activity_price_mode: "DISCOUNT",
      activity_partake_type: "REGULAR",
      activity_discount_rate: 0.18,
      fixed_price_adjustment: 1.2,
    });
  });

  it("omits promotion partake settings when saving time-limited activity strategy", async () => {
    const updateStrategyMutation = resolvedMutation();
    renderWorkbench({
      initialActivityType: "TIME_LIMITED",
      initialTab: "candidates",
      activityStrategyResponse: { configured: false, strategy: null },
      updateStrategyMutation,
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();

    expect(
      screen.queryByRole("button", { name: "常规活动" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "限量活动" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "常规+限量" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("spinbutton", { name: "限量折扣率" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("spinbutton", { name: "活动库存比例" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "按利润" }));
    fireEvent.change(screen.getByRole("spinbutton", { name: "常规利润率" }), {
      target: { value: "0.12" },
    });
    fireEvent.change(screen.getByRole("spinbutton", { name: "固定调价" }), {
      target: { value: "0" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存活动设置" }));

    expect(updateStrategyMutation.mutateAsync).toHaveBeenCalledWith({
      activity_type: "TIME_LIMITED",
      activity_price_mode: "PROFIT",
      activity_min_profit_rate: 0.12,
      fixed_price_adjustment: 0,
    });
  });

  it("saves limited promotion activity strategy when selected", async () => {
    const updateStrategyMutation = resolvedMutation();
    renderWorkbench({
      initialTab: "candidates",
      activityStrategyResponse: { configured: false, strategy: null },
      updateStrategyMutation,
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "限量活动" }));
    fireEvent.change(screen.getByRole("spinbutton", { name: "活动库存比例" }), {
      target: { value: "0.4" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存活动设置" }));

    expect(updateStrategyMutation.mutateAsync).toHaveBeenCalledWith({
      activity_type: "PROMOTION",
      activity_price_mode: "DISCOUNT",
      activity_partake_type: "LIMITED",
      activity_discount_rate: 0.2,
      activity_stock_ratio: 0.4,
      fixed_price_adjustment: 0,
    });
  });

  it("saves both promotion activity strategy when selected", async () => {
    const updateStrategyMutation = resolvedMutation();
    renderWorkbench({
      initialTab: "candidates",
      activityStrategyResponse: { configured: false, strategy: null },
      updateStrategyMutation,
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "常规+限量" }));
    fireEvent.change(screen.getByRole("spinbutton", { name: "活动库存比例" }), {
      target: { value: "0.4" },
    });
    expect(
      screen.getByRole("spinbutton", { name: "限量折扣率" }),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: "保存活动设置" }));

    expect(updateStrategyMutation.mutateAsync).toHaveBeenCalledWith({
      activity_type: "PROMOTION",
      activity_price_mode: "DISCOUNT",
      activity_partake_type: "BOTH",
      activity_discount_rate: 0.2,
      activity_limited_discount_rate: 0.25,
      activity_stock_ratio: 0.4,
      fixed_price_adjustment: 0,
    });
  });

  it("saves both promotion profit strategy with a lower limited minimum profit", async () => {
    const updateStrategyMutation = resolvedMutation();
    renderWorkbench({
      initialTab: "candidates",
      activityStrategyResponse: { configured: false, strategy: null },
      updateStrategyMutation,
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "按利润" }));
    fireEvent.click(screen.getByRole("button", { name: "常规+限量" }));
    expect(
      screen.getByRole("spinbutton", { name: "限量利润率" }),
    ).toBeInTheDocument();
    fireEvent.change(screen.getByRole("spinbutton", { name: "活动库存比例" }), {
      target: { value: "0.4" },
    });
    fireEvent.change(screen.getByRole("spinbutton", { name: "常规利润率" }), {
      target: { value: "0.2" },
    });
    fireEvent.change(
      screen.getByRole("spinbutton", { name: "限量利润率" }),
      {
        target: { value: "0.1" },
      },
    );
    fireEvent.click(screen.getByRole("button", { name: "保存活动设置" }));

    expect(updateStrategyMutation.mutateAsync).toHaveBeenCalledWith({
      activity_type: "PROMOTION",
      activity_price_mode: "PROFIT",
      activity_partake_type: "BOTH",
      activity_stock_ratio: 0.4,
      activity_min_profit_rate: 0.2,
      activity_limited_min_profit_rate: 0.1,
      fixed_price_adjustment: 0,
    });
  });

  it("saves breakeven promotion strategy without discount or profit inputs", async () => {
    const updateStrategyMutation = resolvedMutation();
    renderWorkbench({
      initialTab: "candidates",
      activityStrategyResponse: { configured: false, strategy: null },
      updateStrategyMutation,
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "按保本" }));
    fireEvent.click(screen.getByRole("button", { name: "限量活动" }));
    fireEvent.change(screen.getByRole("spinbutton", { name: "活动库存比例" }), {
      target: { value: "0.4" },
    });

    expect(
      screen.queryByRole("spinbutton", { name: "常规折扣率" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("spinbutton", { name: "常规利润率" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "保存活动设置" }));

    expect(updateStrategyMutation.mutateAsync).toHaveBeenCalledWith({
      activity_type: "PROMOTION",
      activity_price_mode: "BREAKEVEN",
      activity_partake_type: "LIMITED",
      activity_stock_ratio: 0.4,
      fixed_price_adjustment: 0,
    });
  });

  it("shows only mode-specific activity strategy fields", async () => {
    const updateStrategyMutation = resolvedMutation();
    renderWorkbench({
      initialTab: "candidates",
      activityStrategyResponse: { configured: false, strategy: null },
      updateStrategyMutation,
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("spinbutton", { name: "常规折扣率" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("spinbutton", { name: "常规利润率" }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("spinbutton", { name: "活动库存比例" }),
    ).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "按利润" }));
    fireEvent.click(screen.getByRole("button", { name: "限量活动" }));

    expect(
      screen.queryByRole("spinbutton", { name: "常规折扣率" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("spinbutton", { name: "常规利润率" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("spinbutton", { name: "活动库存比例" }),
    ).toBeInTheDocument();

    const minProfitInput = screen.getByRole("spinbutton", {
      name: "常规利润率",
    });
    expect(minProfitInput).toHaveAttribute("min", "0");

    fireEvent.change(minProfitInput, {
      target: { value: "0" },
    });
    fireEvent.click(screen.getByRole("button", { name: "保存活动设置" }));

    expect(updateStrategyMutation.mutateAsync).toHaveBeenCalledWith({
      activity_type: "PROMOTION",
      activity_price_mode: "PROFIT",
      activity_partake_type: "LIMITED",
      activity_stock_ratio: 0.5,
      activity_min_profit_rate: 0,
      fixed_price_adjustment: 0,
    });
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
    expect(
      screen.getByText("shein login failed: 登录等待验证码"),
    ).toBeInTheDocument();
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

    expect(
      await screen.findByText("第 1 / 2 页 · 共 101 条"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "下一页" }));

    expect(mocks.useSheinActivityCandidates).toHaveBeenLastCalledWith(
      12,
      {
        activity_type: "PROMOTION",
        page: 2,
        page_size: 100,
      },
      {
        enabled: true,
      },
    );
  });

  it("searches candidates by exact SKC and resets backend pagination", async () => {
    renderWorkbench({
      initialTab: "candidates",
      candidateTotal: 101,
      candidates: [
        {
          id: 18,
          skc_name: "sg260618173737193036297",
          review_status: "pending_review",
        },
      ],
    });

    expect(
      await screen.findByText("第 1 / 2 页 · 共 101 条"),
    ).toBeInTheDocument();

    fireEvent.click(screen.getByRole("button", { name: "下一页" }));
    fireEvent.change(screen.getByPlaceholderText("输入完整 SKC 搜索候选商品"), {
      target: { value: " sg260618173737193036297 " },
    });

    await waitFor(() => {
      expect(mocks.useSheinActivityCandidates).toHaveBeenLastCalledWith(
        12,
        {
          activity_type: "PROMOTION",
          skc_name: "sg260618173737193036297",
          page: 1,
          page_size: 100,
        },
        {
          enabled: true,
        },
      );
    });
  });

  it("shows enrollment run item details from the runs tab", async () => {
    renderWorkbench({
      initialTab: "runs",
      runs: [
        {
          id: 99,
          activity_type: "PROMOTION",
          activity_key: "PROMOTION:12",
          trigger_mode: "manual_confirmed",
          status: "failed",
          candidate_count: 1,
          succeeded_count: 0,
          failed_count: 1,
          started_at: "2026-06-28T01:00:00Z",
        },
      ],
      runItems: [
        {
          id: 1001,
          run_id: 99,
          skc_name: "sg260618174087119533319",
          status: "failed",
          error_message: "current status can not enroll",
          updated_at: "2026-06-28T01:01:00Z",
        },
      ],
    });

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();
    expect(mocks.useSheinActivityEnrollmentRunItems).toHaveBeenLastCalledWith(
      12,
      0,
      {
        page: 1,
        page_size: 50,
      },
      { enabled: false },
    );

    const detailsButton = screen.getByRole("button", { name: "查看详情" });
    expect(detailsButton).toHaveClass("whitespace-nowrap");
    expect(detailsButton).toHaveClass("min-w-[4.5rem]");

    fireEvent.click(detailsButton);

    expect(mocks.useSheinActivityEnrollmentRunItems).toHaveBeenLastCalledWith(
      12,
      99,
      {
        page: 1,
        page_size: 50,
      },
      { enabled: true },
    );
    expect(
      await screen.findByText("sg260618174087119533319"),
    ).toBeInTheDocument();
    expect(
      screen.getByText("current status can not enroll"),
    ).toBeInTheDocument();
  });

  it("renders cost rows from source POD SDS groups in the cost tab", async () => {
    const syncSourceMutation = resolvedMutation();
    mocks.useSyncSheinSourceSDSProduct.mockReturnValue(syncSourceMutation);
    renderWorkbench({
      initialTab: "costs",
      syncSourceMutation,
      sourceSDSCostGroups: [
        {
          group_key: "source:XB0608021001",
          group_label: "XB0608021001",
          source_code: "XB0608021001",
          product_count: 2,
          manual_cost_price: 50,
          products: [
            {
              id: 8,
              skc_name: "sg260604223794143925005",
              supplier_code: "XB0608021001-DA578653",
            },
            {
              id: 9,
              skc_name: "sg260603162031320517713",
              supplier_code: "XB0608021001-DE93508C",
            },
          ],
        },
      ],
    });

    expect(
      await screen.findByText("XB0608021001 · 2 个商品"),
    ).toBeInTheDocument();
    expect(
      screen.getAllByText(/sg260604223794143925005/).length,
    ).toBeGreaterThan(0);
    expect(
      screen.getAllByText(/sg260603162031320517713/).length,
    ).toBeGreaterThan(0);
    expect(screen.getByDisplayValue("50")).toBeInTheDocument();

    fireEvent.click(
      screen.getByRole("button", { name: "同步该产品 XB0608021001" }),
    );

    expect(syncSourceMutation.mutateAsync).toHaveBeenCalledWith("XB0608021001");
  });

  it("delegates legacy cost tab links to the product workbench", async () => {
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

    expect(
      await screen.findByRole("heading", { name: "SHEIN US" }),
    ).toBeInTheDocument();
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
    expect(mocks.useSheinSDSCostGroups).toHaveBeenCalledWith(
      12,
      {
        page: 1,
        page_size: 100,
      },
      { enabled: false },
    );
    expect(mocks.useSheinActivityCandidates).not.toHaveBeenCalled();
    expect(mocks.useSheinActivityEnrollmentRuns).not.toHaveBeenCalled();
  });
});
