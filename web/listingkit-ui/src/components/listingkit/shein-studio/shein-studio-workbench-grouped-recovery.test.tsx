import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import {
  resetDedicatedBatchPromptOverrides,
  SheinStudioWorkbench,
} from "@/components/listingkit/shein-studio/shein-studio-workbench";
import { saveSDSGroupedCandidateHandoff } from "@/lib/utils/sds-grouped-candidate-handoff";

const useQuery = vi.fn();
const getSDSBaselineReadiness = vi.fn();
const warmSDSBaselineForSelection = vi.fn();
const hydrateSDSVariantSelection = vi.fn();
const getSheinStudioHydratedBatch = vi.fn();
const loadSheinStudioDraft = vi.fn();
const listSheinStudioBatches = vi.fn();
const push = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => "/listing-kits/sds",
  useRouter: () => ({ push }),
  useSearchParams: () => new URLSearchParams("step=generate"),
}));

vi.mock("@tanstack/react-query", () => ({
  useQuery: (...args: unknown[]) => useQuery(...args),
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-progress-strip", () => ({
  SheinStudioProgressStrip: () => <div>progress strip</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-batch-run-progress", () => ({
  SheinStudioBatchRunProgress: () => <div>batch run progress</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-created-tasks-list", () => ({
  SheinCreatedTasksList: () => <div>created tasks</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-design-preview-grid", () => ({
  SheinDesignPreviewGrid: () => <div>review grid</div>,
}));

vi.mock("@/components/listingkit/shein-studio/shein-studio-generation-panel", () => ({
  SheinStudioGenerationPanel: () => <div id="shein-studio-generator">generation panel</div>,
}));

vi.mock("@/lib/api/shein-studio", () => ({
  generateSheinStudioDesigns: vi.fn(),
}));

vi.mock("@/lib/shein-studio/create-review-tasks", async () => {
  const actual = await vi.importActual<typeof import("@/lib/shein-studio/create-review-tasks")>(
    "@/lib/shein-studio/create-review-tasks",
  );
  return {
    ...actual,
    createSheinReviewTasks: vi.fn(),
  };
});

vi.mock("@/lib/shein-studio/hydrate-sds-selection", () => ({
  hydrateSDSVariantSelection: (...args: unknown[]) => hydrateSDSVariantSelection(...args),
}));

vi.mock("@/lib/api/sds-baseline", () => ({
  getSDSBaselineReadiness: (...args: unknown[]) => getSDSBaselineReadiness(...args),
  warmSDSBaselineForSelection: (...args: unknown[]) => warmSDSBaselineForSelection(...args),
}));

vi.mock("@/lib/api/shein-studio-batches", () => ({
  approveSheinStudioBatchDesigns: vi.fn(),
  createSheinStudioBatchTasks: vi.fn(),
  generateSheinStudioBatch: vi.fn(),
  retrySheinStudioBatchItems: vi.fn(),
}));

vi.mock("@/lib/api/shein-studio-batch-runs", () => ({
  startSheinStudioBatchRun: vi.fn(),
}));

vi.mock("@/lib/utils/shein-studio-batches", () => ({
  deleteSheinStudioBatch: vi.fn(),
  getSheinStudioBatch: vi.fn().mockResolvedValue(null),
  getSheinStudioHydratedBatch: (...args: unknown[]) => getSheinStudioHydratedBatch(...args),
  listSheinStudioBatches: (...args: unknown[]) => listSheinStudioBatches(...args),
  loadSheinStudioDraft: (...args: unknown[]) => loadSheinStudioDraft(...args),
  saveSheinStudioBatch: vi.fn().mockResolvedValue(null),
  saveSheinStudioDraftWithOptions: vi.fn().mockRejectedValue(new Error("timeout")),
  setActiveSheinStudioBatchId: vi.fn(),
}));

vi.mock("@/lib/query/use-shein-store-selector", () => ({
  useSheinStoreSelector: () => ({
    enabledProfiles: [],
    profiles: { isError: false },
    routing: { isError: false },
    recommendedStoreId: "",
  }),
}));

const selection = {
  layerId: "layer-1",
  parentProductId: 1,
  printableHeight: 1000,
  printableWidth: 1000,
  productId: 1,
  productName: "tee",
  prototypeGroupId: 200,
  variantId: 100,
  variantLabel: "M / black",
};

function buildHydratedBatch() {
  return {
    savedBatch: {
      id: "batch-1",
      name: "批次1",
      prompt: "retro cherries",
      styleCount: "1",
      sheinStoreId: "869",
      selection,
      groupedSelections: [],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-26T10:00:00.000Z",
    },
    detail: {
      batch: {
        id: "batch-1",
        status: "review_ready",
        prompt: "retro cherries",
        styleCount: "1",
        sheinStoreId: 869,
        createdAt: "2026-05-26T09:59:00.000Z",
        updatedAt: "2026-05-26T10:00:00.000Z",
      },
      items: [],
    },
  };
}

describe("SheinStudioWorkbench grouped recovery", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    window.localStorage.clear();
    resetDedicatedBatchPromptOverrides();
    useQuery.mockReturnValue({ data: undefined, error: null });
    getSDSBaselineReadiness.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "ready",
      reason: "",
    });
    warmSDSBaselineForSelection.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "ready",
      reason: "",
    });
    hydrateSDSVariantSelection.mockResolvedValue(selection);
    getSheinStudioHydratedBatch.mockResolvedValue(null);
    loadSheinStudioDraft.mockResolvedValue(null);
    listSheinStudioBatches.mockResolvedValue([]);
    push.mockReset();
  });

  it("shows grouped-candidate recovery guidance after returning from candidate pool", async () => {
    const scrollIntoView = vi.fn();
    Element.prototype.scrollIntoView = scrollIntoView;
    saveSDSGroupedCandidateHandoff({
      action: "focus_generate",
      actionLabel: "去生成并继续校验",
      message:
        "这款候选商品还没有 baseline 缓存。先在当前工作台完成一次生成或预热，再回来继续校验并加入 grouped 批量上品。",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(
        screen.getByText(
          "这款候选商品还没有 baseline 缓存。先在当前工作台完成一次生成或预热，再回来继续校验并加入 grouped 批量上品。",
        ),
      ).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "去生成并继续校验" }));
    await waitFor(() => expect(scrollIntoView).toHaveBeenCalled());
  });

  it("routes login-blocked grouped guidance to the SDS login page", async () => {
    saveSDSGroupedCandidateHandoff({
      action: "open_sds_login",
      actionLabel: "去处理 SDS 登录",
      message: "当前 SDS 登录缺少 access token。",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "去处理 SDS 登录" })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "去处理 SDS 登录" }));
    expect(push).toHaveBeenCalledWith("/listing-kits/sds-login");
  });

  it("routes the active-selection baseline action to the SDS login page when login is blocked", async () => {
    getSDSBaselineReadiness.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "blocked",
      reasonCode: "login_missing_credentials",
      reason: "",
    });
    getSheinStudioHydratedBatch.mockResolvedValue(buildHydratedBatch());

    render(<SheinStudioWorkbench activeStep="generate" initialBatchId="batch-1" />);

    const actionButton = await screen.findByRole("button", {
      name: "去处理 SDS 登录",
    });
    fireEvent.click(actionButton);

    expect(push).toHaveBeenCalledWith("/listing-kits/sds-login");
  });

  it("warms baseline directly from grouped-candidate guidance", async () => {
    saveSDSGroupedCandidateHandoff({
      action: "warm_baseline",
      actionLabel: "一键预热并校验 baseline",
      message:
        "这款候选商品还没有 baseline 缓存。先预热并校验 baseline，再回来加入 grouped 批量上品。",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "一键预热并校验 baseline" })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "一键预热并校验 baseline" }));

    await waitFor(() => expect(warmSDSBaselineForSelection).toHaveBeenCalledWith(selection));
    await waitFor(() =>
      expect(
        screen.getByText("这款 SDS 商品的 baseline 已通过校验，现在可以继续加入 grouped 批量上品。"),
      ).toBeInTheDocument(),
    );
  });

  it("keeps the baseline recovery action available when warmup still needs more validation", async () => {
    warmSDSBaselineForSelection.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "baseline_cached",
      reasonCode: "cache_unavailable",
      reason: "",
    });
    saveSDSGroupedCandidateHandoff({
      action: "warm_baseline",
      actionLabel: "一键预热并校验 baseline",
      message:
        "这款候选商品还没有 baseline 缓存。先预热并校验 baseline，再回来加入 grouped 批量上品。",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "一键预热并校验 baseline" })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "一键预热并校验 baseline" }));

    await waitFor(() =>
      expect(screen.getAllByRole("button", { name: "继续 baseline 校验" }).length).toBeGreaterThan(0),
    );
    expect(
      screen.getAllByText("当前 SDS 选择还没有可用的 baseline 缓存。").length,
    ).toBeGreaterThan(0);
  });

  it("shows a direct fallback message when warmup returns cached baseline without a reason", async () => {
    warmSDSBaselineForSelection.mockResolvedValue({
      baselineKey: "baseline-key",
      status: "baseline_cached",
      reasonCode: "",
      reason: "",
    });
    saveSDSGroupedCandidateHandoff({
      action: "warm_baseline",
      actionLabel: "一键预热并校验 baseline",
      message:
        "这款候选商品还没有 baseline 缓存。先预热并校验 baseline，再回来加入 grouped 批量上品。",
    });

    render(<SheinStudioWorkbench activeStep="generate" selection={selection} />);

    await waitFor(() =>
      expect(screen.getByRole("button", { name: "一键预热并校验 baseline" })).toBeInTheDocument(),
    );
    fireEvent.click(screen.getByRole("button", { name: "一键预热并校验 baseline" }));

    await waitFor(() =>
      expect(
        screen.getByText("这款 SDS 商品已经完成 baseline 缓存，当前没有更多校验结果。可以继续使用，必要时再手动复查。"),
      ).toBeInTheDocument(),
    );
  });
});
