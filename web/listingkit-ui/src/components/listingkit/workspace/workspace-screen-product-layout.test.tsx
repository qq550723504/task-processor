import { render, screen, within } from "@testing-library/react";
import type { ReactNode } from "react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { WorkspaceScreen } from "@/components/listingkit/workspace/workspace-screen";

const defaultWorkflowIssues = [
  {
    code: "attributes",
    severity: "blocking",
    message: "Material 缺失",
    detail: "SHEIN 提交前必须补充材质。",
  },
];

function canonicalProductFixture() {
  return {
    title: "Canvas Tote",
    brand: "ListingKit",
    category_path: ["Bags", "Totes"],
    images: [
      {
        url: "https://example.com/canvas-tote-front.jpg",
        alt: "Canvas Tote front",
        role: "主图",
      },
    ],
    variants: [],
    attributes: {},
  };
}

function sheinPlatformCardFixture() {
  return {
    platform: "shein",
    primary_navigation_target: {
      preview_query: { platform: "shein", section_key: "general_review" },
    },
    recovery_summary: {
      title: "Retry SHEIN preparation",
      severity: "high",
      urgency: "now",
      primary_descriptor: {
        platform: "shein",
        recovery_hint: "retry_dispatch",
        recovery_severity: "high",
        recovery_urgency: "now",
        recovery_cta_kind: "retry",
      },
    },
  };
}

function temuPlatformCardFixture() {
  return {
    platform: "temu",
    status: "failed",
    recovery_summary: {
      title: "Retry TEMU preparation",
      severity: "high",
      urgency: "now",
      primary_descriptor: {
        platform: "temu",
        recovery_hint: "retry_dispatch",
        recovery_severity: "high",
        recovery_urgency: "now",
        recovery_cta_kind: "retry",
      },
    },
  };
}

const mocks = vi.hoisted(() => ({
  executeActionMutate: vi.fn(),
  handlePlatformSelect: vi.fn(),
  handleRunSheinPrimaryAction: vi.fn(),
  handleSelectSheinBlockingItem: vi.fn(),
  dispatchTarget: vi.fn(),
  handleAction: vi.fn(),
  handleToolbarAction: vi.fn(),
  handleRecovery: vi.fn(),
  handlePlatformRecovery: vi.fn(),
  handleProductSelect: vi.fn(),
  handleHistorySelect: vi.fn(),
  routeSearch: "platform=shein",
  taskStatus: "needs_review" as string,
  taskResultLoading: false,
  canonicalProduct: undefined as ReturnType<typeof canonicalProductFixture> | undefined,
  navigationPlatformCards: [] as Array<
    | ReturnType<typeof sheinPlatformCardFixture>
    | ReturnType<typeof temuPlatformCardFixture>
    | {
        platform: string;
        status?: string;
      }
  >,
  workflowIssues: [
    {
      code: "attributes",
      severity: "blocking",
      message: "Material 缺失",
      detail: "SHEIN 提交前必须补充材质。",
    },
  ] as Array<{
    code: string;
    severity: string;
    message: string;
    detail?: string;
  }>,
}));

beforeEach(() => {
  vi.clearAllMocks();
  mocks.taskStatus = "needs_review";
  mocks.taskResultLoading = false;
  mocks.routeSearch = "platform=shein";
  mocks.canonicalProduct = canonicalProductFixture();
  mocks.navigationPlatformCards = [sheinPlatformCardFixture()];
  mocks.workflowIssues = defaultWorkflowIssues.map((issue) => ({ ...issue }));
});

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(mocks.routeSearch),
  useRouter: () => ({ replace: vi.fn() }),
}));

vi.mock("@/components/listingkit/workspace/use-workspace-data", () => ({
  useWorkspaceData: () => ({
    baseQuery: {},
    preview: {
      data: { shein: {}, asset_generation_overview: {} },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    },
    taskResult: {
      data: mocks.taskResultLoading
        ? undefined
        : {
            task_id: "task-1",
            status: mocks.taskStatus,
            result: {
              canonical_product: mocks.canonicalProduct,
              summary: { blocking_count: 1, warning_count: 0 },
              workflow_issues: mocks.workflowIssues,
            },
          },
      isLoading: mocks.taskResultLoading,
      isError: false,
      refetch: vi.fn(),
    },
    session: {
      data: { session: {}, recovery_summary: null },
      isLoading: false,
      isError: false,
      refetch: vi.fn(),
    },
    reviewPreview: { data: {} },
    sessionData: {
      selected_platform: "shein",
      overview: {},
      review_summary: { approved_sections: 2 },
    },
    platformCards: [sheinPlatformCardFixture()],
    navigationPlatformCards: mocks.navigationPlatformCards,
    focusedPreview: undefined,
    selectedPlatform: "shein",
    focusedScenePreset: undefined,
    suppressResolvedActionSummary: false,
    resolvedActionSummary: null,
    previewSuggestion: null,
    sheinImages: [],
    sheinAvailableImages: [],
    sheinMockupImages: [],
    sheinVariantCount: 0,
    sheinPreviewPayload: {},
    showSheinCategoryReview: false,
    showSheinAttributeReview: false,
    showSheinSaleAttributeReview: false,
    showSheinReviewDetails: false,
    shouldOpenSheinAdvancedDetails: false,
    isSheinFinalReviewMode: false,
    sheinFlowSteps: [],
    workspaceTitle: "Canvas Tote",
    workspaceStatusLabel: "待确认",
    workspaceUpdatedAt: "刚刚",
    workspaceSubtitle: "SHEIN · 商品审核",
  }),
}));

vi.mock("@/components/listingkit/workspace/use-shein-workspace-actions", () => ({
  useSheinWorkspaceActions: () => ({
    handleRefreshSheinCategory: vi.fn(),
    handleRegenerateSheinAttributes: vi.fn(),
    handleRegenerateSheinSaleAttributes: vi.fn(),
  }),
}));

vi.mock("@/components/listingkit/workspace/use-workspace-navigation-actions", () => ({
  useWorkspaceNavigationActions: () => ({
    dispatchTarget: mocks.dispatchTarget,
    handleAction: mocks.handleAction,
    handleToolbarAction: mocks.handleToolbarAction,
    handleRecovery: mocks.handleRecovery,
    handlePlatformSelect: mocks.handlePlatformSelect,
    handlePlatformRecovery: mocks.handlePlatformRecovery,
    handleProductSelect: mocks.handleProductSelect,
    handleHistorySelect: mocks.handleHistorySelect,
    handleSelectSheinBlockingItem: mocks.handleSelectSheinBlockingItem,
    handleRunSheinPrimaryAction: mocks.handleRunSheinPrimaryAction,
  }),
}));

vi.mock("@/components/listingkit/workspace/shein-workspace-view-props", () => ({
  buildSheinWorkspaceViewProps: () => ({
    imageGalleryProps: {},
    finalReviewProps: {},
    finalModeReadinessProps: {},
    timelineProps: {},
  }),
  buildSheinAdvancedReviewDetailsProps: () => null,
}));

vi.mock("@/components/listingkit/workspace/workspace-review-view-props", () => ({
  buildWorkspaceReviewViewProps: () => ({}),
}));

vi.mock("@/components/listingkit/workspace/workspace-screen-views", () => ({
  WorkspaceReviewView: () => <div>现有 SHEIN 审核内容</div>,
  SheinFinalReviewWorkspaceView: () => <div>SHEIN 最终审核内容</div>,
  WorkspaceAgentSurface: ({
    context,
    children,
  }: {
    context: { taskId: string; runId: string };
    children: ReactNode;
  }) => (
    <div
      data-testid="workspace-agent-surface"
      data-task-id={context.taskId}
      data-run-id={context.runId}
    >
      {children}
    </div>
  ),
}));

vi.mock("@/components/listingkit/tasks/task-status-panel", () => ({
  TaskStatusPanel: () => <div>执行状态详情</div>,
}));
vi.mock("@/components/listingkit/review/review-reasons-card", () => ({
  ReviewReasonsCard: () => <div>旧任务审核卡</div>,
}));
vi.mock("@/components/listingkit/tasks/task-progress-notice", () => ({
  TaskProgressNotice: () => <div>后台处理中</div>,
}));
vi.mock("@/components/listingkit/shared/platform-card-rail", () => ({
  PlatformCardRail: () => <div>旧平台卡片栏</div>,
}));
vi.mock("@/components/listingkit/shein/shein-flow-nav", () => ({
  SheinFlowNav: () => <div>SHEIN 流程状态</div>,
}));
vi.mock("@/components/listingkit/workspace/workspace-overview-panel", () => ({
  WorkspaceOverviewPanel: () => <div>商品概览内容</div>,
}));
vi.mock("@/components/listingkit/tasks/task-revision-history-panel", () => ({
  TaskRevisionHistoryPanel: () => <div>历史记录内容</div>,
}));
vi.mock("@/components/listingkit/workspace/sds-repair-panel", () => ({
  SDSRepairPanel: () => null,
}));
vi.mock("@/components/listingkit/workspace/shein-advanced-review-details", () => ({
  SheinAdvancedReviewDetails: () => <div>高级审核内容</div>,
}));

vi.mock("@/lib/query/use-apply-revision", () => ({
  useApplyRevision: () => ({ mutate: vi.fn(), isPending: false, error: null }),
}));
vi.mock("@/lib/query/use-submit-task", () => ({
  useSubmitTask: () => ({ mutate: vi.fn(), isPending: false, error: null }),
  useRefreshSubmissionStatus: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/query/use-shein-final-draft", () => ({
  useUpdateSheinFinalDraft: () => ({ mutate: vi.fn(), isPending: false }),
}));
vi.mock("@/lib/query/use-shein-resolution-cache", () => ({
  useClearSheinResolutionCache: () => ({ mutate: vi.fn(), isPending: false, variables: null }),
}));
vi.mock("@/lib/query/use-action", () => ({
  useExecuteAction: () => ({ mutate: mocks.executeActionMutate, isPending: false }),
}));
vi.mock("@/lib/query/use-child-task-retry", () => ({
  getTaskRetryVersion: () => undefined,
  useRetryChildTask: () => ({
    mutate: vi.fn(),
    isPending: false,
    variables: null,
    retryQueued: false,
    error: null,
  }),
}));

describe("WorkspaceScreen Product Workspace composition", () => {
  it("keeps the Image Agent surface around the merged Product Workspace", () => {
    mocks.routeSearch = "platform=shein&image_agent_run_id=run:agent-1";

    render(<WorkspaceScreen taskId="task-1" />);

    const surface = screen.getByTestId("workspace-agent-surface");
    expect(surface).toHaveAttribute("data-task-id", "task-1");
    expect(surface).toHaveAttribute("data-run-id", "run:agent-1");
    expect(within(surface).getByRole("heading", { name: "Canvas Tote" })).toBeInTheDocument();
  });

  it("renders product-first header and three-column workspace while keeping existing review content", () => {
    render(<WorkspaceScreen taskId="task-1" />);

    expect(screen.getByRole("link", { name: "返回商品中心" })).toHaveAttribute(
      "href",
      "/listing-kits/canonical-products",
    );
    expect(screen.getByRole("heading", { name: "Canvas Tote" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "查看执行记录" })).toHaveAttribute(
      "href",
      "/listing-kits/task-1/status",
    );

    const navigation = screen.getByRole("navigation", { name: "商品工作台导航" });
    expect(within(navigation).getByRole("button", { name: "概览" })).toBeInTheDocument();
    expect(
      within(navigation).getByRole("button", { name: "SHEIN · 未开始" }),
    ).toBeInTheDocument();

    const work = screen.getByRole("region", { name: "商品工作区" });
    expect(within(work).getByText("现有 SHEIN 审核内容")).toBeInTheDocument();

    const aiReview = screen.getByRole("complementary", { name: "AI 审核" });
    expect(within(aiReview).getByText("Material 缺失")).toBeInTheDocument();
    expect(within(aiReview).getByText("已通过").parentElement).toHaveTextContent("2");

    const actions = screen.getByRole("region", { name: "商品操作" });
    expect(within(actions).getByText("执行状态详情")).toBeInTheDocument();
    expect(within(actions).getByText("历史记录内容")).toBeInTheDocument();

    expect(screen.queryByText("旧任务审核卡")).not.toBeInTheDocument();
    expect(screen.queryByText("task-1")).not.toBeInTheDocument();
  });

  it("keeps platform navigation, AI issue routing, and layer action keys wired to existing handlers", async () => {
    const user = userEvent.setup();
    render(<WorkspaceScreen taskId="task-1" />);

    await user.click(screen.getByRole("button", { name: "SHEIN · 未开始" }));
    expect(mocks.dispatchTarget).toHaveBeenCalledWith({
      preview_query: { platform: "shein", section_key: "general_review" },
    });
    expect(mocks.handlePlatformSelect).toHaveBeenCalledWith("shein");

    await user.click(screen.getByRole("button", { name: "处理：Material 缺失" }));
    expect(mocks.handleRunSheinPrimaryAction).toHaveBeenCalledWith("attributes");

    await user.click(screen.getByRole("button", { name: "AI 生成商品" }));
    expect(mocks.executeActionMutate).toHaveBeenCalledWith({
      action_key: "run_standard_product_temporal",
    });

    await user.click(screen.getByRole("button", { name: "生成平台资料" }));
    expect(mocks.executeActionMutate).toHaveBeenCalledWith({
      action_key: "run_platform_adapt_temporal",
      target: {
        action_key: "run_platform_adapt_temporal",
        queue_query: { platform: "all" },
      },
    });
  });

  it("keeps a platform recovery action separate from opening platform review", async () => {
    const user = userEvent.setup();
    render(<WorkspaceScreen taskId="task-1" />);

    const recoveryButton = screen.getByRole("button", { name: "SHEIN · 立即重试" });
    expect(recoveryButton).toBeInTheDocument();

    await user.click(recoveryButton);

    expect(mocks.handlePlatformRecovery).toHaveBeenCalledWith(
      expect.objectContaining({
        platform: "shein",
        recovery_hint: "retry_dispatch",
      }),
      "shein",
    );
    expect(mocks.handlePlatformSelect).not.toHaveBeenCalled();
  });

  it("selects the clicked platform before running its action-style recovery", async () => {
    mocks.navigationPlatformCards = [
      sheinPlatformCardFixture(),
      temuPlatformCardFixture(),
    ];
    const user = userEvent.setup();
    render(<WorkspaceScreen taskId="task-1" />);

    await user.click(screen.getByRole("button", { name: "TEMU · 立即重试" }));

    expect(mocks.handlePlatformSelect).toHaveBeenCalledWith("temu");
    expect(mocks.handlePlatformRecovery).toHaveBeenCalledWith(
      expect.objectContaining({
        platform: "temu",
        recovery_hint: "retry_dispatch",
      }),
      "temu",
    );
  });

  it("switches the central work area between canonical product content and platform review", async () => {
    const user = userEvent.setup();
    render(<WorkspaceScreen taskId="task-1" />);

    const work = screen.getByRole("region", { name: "商品工作区" });
    expect(within(work).getByText("现有 SHEIN 审核内容")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "图片" }));

    expect(mocks.handleProductSelect).toHaveBeenCalledWith("images");

    expect(
      within(work).getByRole("img", { name: "Canvas Tote front" }),
    ).toHaveAttribute("src", "https://example.com/canvas-tote-front.jpg");
    expect(within(work).queryByText("现有 SHEIN 审核内容")).not.toBeInTheDocument();
    expect(within(work).queryByText("SHEIN 流程状态")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "SHEIN · 未开始" }));

    expect(within(work).getByText("现有 SHEIN 审核内容")).toBeInTheDocument();
    expect(
      within(work).queryByRole("img", { name: "Canvas Tote front" }),
    ).not.toBeInTheDocument();
  });

  it("keeps product overview and platform review as mutually exclusive destinations", async () => {
    const user = userEvent.setup();
    render(<WorkspaceScreen taskId="task-1" />);

    await user.click(screen.getByRole("button", { name: "SHEIN · 未开始" }));
    expect(screen.getAllByRole("button", { current: "page" })).toHaveLength(1);
    expect(screen.getByRole("button", { name: "SHEIN · 未开始" })).toHaveAttribute(
      "aria-current",
      "page",
    );

    await user.click(screen.getByRole("button", { name: "概览" }));

    expect(screen.getAllByRole("button", { current: "page" })).toHaveLength(1);
    expect(screen.getByRole("button", { name: "概览" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("region", { name: "商品工作区" })).toHaveTextContent(
      "商品概览内容",
    );
  });

  it("shows revision history in the central work area when history is selected", async () => {
    const user = userEvent.setup();
    render(<WorkspaceScreen taskId="task-1" />);

    const work = screen.getByRole("region", { name: "商品工作区" });
    expect(within(work).queryByText("历史记录内容")).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "历史" }));

    expect(mocks.handleHistorySelect).toHaveBeenCalledTimes(1);
    expect(within(work).getByText("历史记录内容")).toBeInTheDocument();
  });

  it("derives the history destination from the workspace route", () => {
    mocks.routeSearch = "workspace_view=history";

    render(<WorkspaceScreen taskId="task-1" />);

    expect(
      within(screen.getByRole("region", { name: "商品工作区" })).getByText(
        "历史记录内容",
      ),
    ).toBeInTheDocument();
  });

  it("leaves history before dispatching an AI issue action", async () => {
    const user = userEvent.setup();
    render(<WorkspaceScreen taskId="task-1" />);

    await user.click(screen.getByRole("button", { name: "历史" }));
    await user.click(screen.getByRole("button", { name: "处理：Material 缺失" }));

    expect(mocks.handleRunSheinPrimaryAction).toHaveBeenCalledWith("attributes");
    expect(within(screen.getByRole("region", { name: "商品工作区" })).getByText(
      "现有 SHEIN 审核内容",
    )).toBeInTheDocument();
  });

  it("leaves history and selects the platform work area before recovery", async () => {
    const user = userEvent.setup();
    render(<WorkspaceScreen taskId="task-1" />);

    await user.click(screen.getByRole("button", { name: "图片" }));
    await user.click(screen.getByRole("button", { name: "历史" }));
    await user.click(screen.getByRole("button", { name: "SHEIN · 立即重试" }));

    expect(mocks.handlePlatformRecovery).toHaveBeenCalledWith(
      expect.objectContaining({
        platform: "shein",
        recovery_hint: "retry_dispatch",
      }),
      "shein",
    );
    expect(
      within(screen.getByRole("region", { name: "商品工作区" })).getByText(
        "现有 SHEIN 审核内容",
      ),
    ).toBeInTheDocument();
  });

  it("uses the returned browser route after an optimistic product selection", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<WorkspaceScreen taskId="task-1" />);

    await user.click(screen.getByRole("button", { name: "图片" }));
    expect(screen.getByRole("img", { name: "Canvas Tote front" })).toBeInTheDocument();

    mocks.routeSearch = "product_section=images";
    rerender(<WorkspaceScreen taskId="task-1" />);
    expect(screen.getByRole("img", { name: "Canvas Tote front" })).toBeInTheDocument();

    mocks.routeSearch = "platform=shein";
    rerender(<WorkspaceScreen taskId="task-1" />);
    expect(screen.getByText("现有 SHEIN 审核内容")).toBeInTheDocument();
  });

  it.each(["processing", "pending", "queued", "running"])(
    "keeps AI review in a checking state while the task is %s",
    (status) => {
      mocks.taskStatus = status;
      mocks.workflowIssues = [];

      render(<WorkspaceScreen taskId="task-1" />);

      const aiReview = screen.getByRole("complementary", { name: "AI 审核" });
      expect(within(aiReview).getByText("AI 正在检查商品")).toBeInTheDocument();
      expect(
        within(aiReview).queryByText("AI 检查已完成，可以继续当前操作。"),
      ).not.toBeInTheDocument();
    },
  );

  it("keeps AI review checking while task-result data is loading", () => {
    mocks.taskResultLoading = true;
    mocks.workflowIssues = [];

    render(<WorkspaceScreen taskId="task-1" />);

    const aiReview = screen.getByRole("complementary", { name: "AI 审核" });
    expect(within(aiReview).getByText("AI 正在检查商品")).toBeInTheDocument();
    expect(
      within(aiReview).queryByText("AI 检查已完成，可以继续当前操作。"),
      ).not.toBeInTheDocument();
  });

  it("keeps canonical sections in a loading state until task-result data arrives", () => {
    mocks.routeSearch = "product_section=images";
    mocks.taskResultLoading = true;
    mocks.workflowIssues = [];

    render(<WorkspaceScreen taskId="task-1" />);

    const work = screen.getByRole("region", { name: "商品工作区" });
    expect(within(work).getByRole("status", { name: "正在加载商品资料" })).toBeInTheDocument();
    expect(within(work).queryByText("暂无商品图片")).not.toBeInTheDocument();
  });

  it("falls back to the workspace title when the canonical title is blank", async () => {
    mocks.canonicalProduct = {
      ...canonicalProductFixture(),
      title: "   ",
    };
    const user = userEvent.setup();
    render(<WorkspaceScreen taskId="task-1" />);

    await user.click(screen.getByRole("button", { name: "概览" }));

    expect(screen.getByRole("heading", { name: "Canvas Tote" })).toBeInTheDocument();
  });

  it("keeps all target platforms in navigation when the focused session has only one card", () => {
    mocks.navigationPlatformCards = [
      sheinPlatformCardFixture(),
      { platform: "temu", status: "ready" },
    ];

    render(<WorkspaceScreen taskId="task-1" />);

    const navigation = screen.getByRole("navigation", { name: "商品工作台导航" });
    expect(
      within(navigation).getByRole("button", { name: "SHEIN · 未开始" }),
    ).toBeInTheDocument();
    expect(within(navigation).getByRole("button", { name: /TEMU/ })).toBeInTheDocument();
  });

  it("returns non-canonical tasks to the task list instead of the product center", () => {
    mocks.canonicalProduct = undefined;

    render(<WorkspaceScreen taskId="task-1" />);

    expect(screen.getByRole("link", { name: "返回任务列表" })).toHaveAttribute(
      "href",
      "/listing-kits",
    );
    expect(screen.queryByRole("link", { name: "返回商品中心" })).not.toBeInTheDocument();
  });
});
