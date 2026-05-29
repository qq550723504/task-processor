import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";

import ListingKitSDSPage from "@/app/listing-kits/sds/page";
import { ApiError } from "@/lib/api/client";

const push = vi.fn();
const listSheinStudioBatches = vi.fn();
const getSheinStudioBatch = vi.fn();
const saveSheinStudioBatch = vi.fn();
const deleteSheinStudioBatch = vi.fn();
const buildRecentBatchSummaries = vi.fn();
const loadLocalSheinStudioDraftSnapshot = vi.fn();
const loadLocalSheinStudioDraftSnapshotDetail = vi.fn();
const clearLocalSheinStudioDraftSnapshot = vi.fn();
const scrollIntoView = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
  usePathname: () => "/listing-kits/sds",
}));

vi.mock("@/lib/utils/shein-studio-batches", () => ({
  listSheinStudioBatches: (...args: unknown[]) => listSheinStudioBatches(...args),
  getSheinStudioBatch: (...args: unknown[]) => getSheinStudioBatch(...args),
  saveSheinStudioBatch: (...args: unknown[]) => saveSheinStudioBatch(...args),
  deleteSheinStudioBatch: (...args: unknown[]) => deleteSheinStudioBatch(...args),
}));

vi.mock("@/lib/shein-studio/recent-batch-summaries", () => ({
  buildRecentBatchSummaries: (...args: unknown[]) =>
    buildRecentBatchSummaries(...args),
}));

vi.mock(
  "@/components/listingkit/shein-studio/shein-studio-workbench-hooks",
  () => ({
    loadLocalSheinStudioDraftSnapshot: (...args: unknown[]) =>
      loadLocalSheinStudioDraftSnapshot(...args),
    loadLocalSheinStudioDraftSnapshotDetail: (...args: unknown[]) =>
      loadLocalSheinStudioDraftSnapshotDetail(...args),
    clearLocalSheinStudioDraftSnapshot: (...args: unknown[]) =>
      clearLocalSheinStudioDraftSnapshot(...args),
  }),
);

vi.mock(
  "@/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard",
  () => ({
    SheinStudioRecentBatchesDashboard: ({
      onBulkDeleteSummaries,
      onDeleteSummary,
      onDuplicateSummary,
      onRenameSummary,
      summaries,
    }: {
      onBulkDeleteSummaries?: (summaryIds: string[]) => void;
      onDeleteSummary?: (summary: { id: string; title: string }) => void;
      onDuplicateSummary?: (summary: { id: string; title: string }) => void;
      onRenameSummary?: (summary: { id: string; title: string }, name: string) => void;
      summaries: Array<{ id: string; title: string }>;
    }) => (
      <section>
        <h2>最近批次</h2>
        <ul>
          {summaries.map((summary) => (
            <li key={summary.id}>
              <span>{summary.title}</span>
              {onRenameSummary ? (
                <button onClick={() => onRenameSummary(summary, `${summary.title} Renamed`)}>
                  重命名-{summary.id}
                </button>
              ) : null}
              {onDuplicateSummary ? (
                <button onClick={() => onDuplicateSummary(summary)}>
                  复制-{summary.id}
                </button>
              ) : null}
              {onDeleteSummary ? (
                <button onClick={() => onDeleteSummary(summary)}>
                  删除-{summary.id}
                </button>
              ) : null}
            </li>
          ))}
        </ul>
        {onBulkDeleteSummaries ? (
          <button onClick={() => onBulkDeleteSummaries(["batch-4", "batch-3"])}>
            批量删除-batch-4-batch-3
          </button>
        ) : null}
      </section>
    ),
  }),
);

describe("/listing-kits/sds page", () => {
  beforeEach(() => {
    push.mockReset();
    listSheinStudioBatches.mockReset();
    getSheinStudioBatch.mockReset();
    saveSheinStudioBatch.mockReset();
    deleteSheinStudioBatch.mockReset();
    buildRecentBatchSummaries.mockReset();
    loadLocalSheinStudioDraftSnapshot.mockReset();
    loadLocalSheinStudioDraftSnapshotDetail.mockReset();
    clearLocalSheinStudioDraftSnapshot.mockReset();
    scrollIntoView.mockReset();
    Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
      configurable: true,
      value: scrollIntoView,
    });

    listSheinStudioBatches.mockResolvedValue([]);
    getSheinStudioBatch.mockImplementation(async (id: string) => ({
      id,
      name: id === "batch-4" ? "Batch Four" : "Batch",
      prompt: "prompt",
      styleCount: "1",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "",
      transparentBackground: false,
      sheinStoreId: "",
      imageStrategy: "sds_official",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selectionVariantId: 123,
      selection: undefined,
      groupedSelections: [],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
      updatedAt: "2026-05-27T01:23:00.000Z",
    }));
    saveSheinStudioBatch.mockResolvedValue(null);
    deleteSheinStudioBatch.mockResolvedValue(undefined);
    loadLocalSheinStudioDraftSnapshot.mockReturnValue(null);
    loadLocalSheinStudioDraftSnapshotDetail.mockReturnValue(null);
    clearLocalSheinStudioDraftSnapshot.mockReturnValue(undefined);
    buildRecentBatchSummaries.mockReturnValue([
      {
        id: "batch-4",
        title: "Batch Four",
        source: "batch",
        primaryProductName: "Product Four",
        createdTaskCount: 0,
        designCount: 0,
        productCount: 4,
        storeSummary: "US",
        updatedAt: "2026-05-27T01:23:00.000Z",
        alerts: [],
      },
      {
        id: "batch-3",
        title: "Batch Three",
        source: "batch",
        primaryProductName: "Product Three",
        createdTaskCount: 1,
        designCount: 2,
        productCount: 3,
        storeSummary: "US",
        updatedAt: "2026-05-27T01:22:00.000Z",
        alerts: [],
      },
      {
        id: "batch-2",
        title: "Batch Two",
        source: "batch",
        primaryProductName: "Product Two",
        createdTaskCount: 0,
        designCount: 1,
        productCount: 2,
        storeSummary: "US",
        updatedAt: "2026-05-27T01:21:00.000Z",
        alerts: [],
      },
      {
        id: "batch-1",
        title: "Batch One",
        source: "batch",
        primaryProductName: "Product One",
        createdTaskCount: 0,
        designCount: 0,
        productCount: 1,
        storeSummary: "US",
        updatedAt: "2026-05-27T01:20:00.000Z",
        alerts: [],
      },
    ]);
  });

  it("renders the SDS homepage without the product browser", async () => {
    render(<ListingKitSDSPage />);

    expect(
      screen.getByRole("heading", { name: "从 POD 商品生成上架资料" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "新建批次并选品" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "快速单个生成" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "选择底版商品和子 SKU" }),
    ).not.toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("Batch Four")).toBeInTheDocument();
    });

    expect(screen.getByRole("heading", { name: "最近批次摘要" })).toBeInTheDocument();
    expect(screen.getByText("Batch Three")).toBeInTheDocument();
    expect(screen.getByText("Batch Two")).toBeInTheDocument();
    expect(screen.queryByText("Batch One")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "查看全部批次" }),
    ).toBeInTheDocument();
  });

  it("still shows the full-dashboard entry when only two recent batches exist", async () => {
    buildRecentBatchSummaries.mockReturnValueOnce([
      {
        id: "batch-2",
        title: "Batch Two",
        source: "batch",
        primaryProductName: "Product Two",
        createdTaskCount: 1,
        designCount: 2,
        productCount: 2,
        storeSummary: "US",
        updatedAt: "2026-05-27T01:21:00.000Z",
        alerts: [],
      },
      {
        id: "batch-1",
        title: "Batch One",
        source: "batch",
        primaryProductName: "Product One",
        createdTaskCount: 0,
        designCount: 0,
        productCount: 1,
        storeSummary: "US",
        updatedAt: "2026-05-27T01:20:00.000Z",
        alerts: [],
      },
    ]);

    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(screen.getByText("Batch Two")).toBeInTheDocument();
    });

    expect(
      screen.getByRole("button", { name: "查看全部批次" }),
    ).toBeInTheDocument();
  });

  it("navigates to the dedicated new-batch route from the homepage CTA", () => {
    render(<ListingKitSDSPage />);

    fireEvent.click(screen.getByRole("button", { name: "新建批次并选品" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/sds/new");
  });

  it("navigates to the quick single-generation path", () => {
    render(<ListingKitSDSPage />);

    fireEvent.click(screen.getByRole("button", { name: "快速单个生成" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/sds/new?entry=single");
  });

  it("routes continue recent to the latest persisted batch", async () => {
    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(screen.getByText("Batch Four")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "继续最近批次" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-4");
  });

  it("routes continue recent to the recoverable draft when no persisted batch exists", async () => {
    buildRecentBatchSummaries.mockReturnValue([
      {
        id: "local-draft:group-1",
        title: "Local Draft",
        source: "local_draft",
        isRecoverableDraft: true,
        primaryProductName: "Half Flag",
        createdTaskCount: 0,
        designCount: 2,
        productCount: 1,
        storeSummary: "US",
        updatedAt: "2026-05-27T01:23:00.000Z",
        alerts: [],
      },
    ]);

    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(screen.getByText("Local Draft")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "继续最近批次" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/sds/new?step=review");
  });

  it("opens a recoverable draft summary card in the draft workbench route", async () => {
    buildRecentBatchSummaries.mockReturnValue([
      {
        id: "local-draft:group-1",
        title: "Local Draft",
        source: "local_draft",
        isRecoverableDraft: true,
        primaryProductName: "Half Flag",
        createdTaskCount: 0,
        designCount: 0,
        productCount: 1,
        storeSummary: "US",
        updatedAt: "2026-05-27T01:23:00.000Z",
        alerts: [{ tone: "warning", label: "未保存草稿" }],
      },
    ]);

    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(screen.getByText("Local Draft")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: /Local Draft/ }));

    expect(push).toHaveBeenCalledWith("/listing-kits/sds/new?step=generate");
  });

  it("clears a recoverable local draft from the recent dashboard", async () => {
    buildRecentBatchSummaries
      .mockReturnValueOnce([
        {
          id: "local-draft:group-1",
          title: "Local Draft",
          source: "local_draft",
          isRecoverableDraft: true,
          primaryProductName: "Half Flag",
          createdTaskCount: 0,
          designCount: 0,
          productCount: 1,
          storeSummary: "US",
          updatedAt: "2026-05-27T01:23:00.000Z",
          alerts: [{ tone: "warning", label: "未保存草稿" }],
        },
        {
          id: "batch-2",
          title: "Batch Two",
          source: "batch",
          isRecoverableDraft: false,
          primaryProductName: "Half Flag",
          createdTaskCount: 0,
          designCount: 0,
          productCount: 1,
          storeSummary: "US",
          updatedAt: "2026-05-27T01:22:00.000Z",
          alerts: [],
        },
        {
          id: "batch-3",
          title: "Batch Three",
          source: "batch",
          isRecoverableDraft: false,
          primaryProductName: "Half Flag",
          createdTaskCount: 0,
          designCount: 0,
          productCount: 1,
          storeSummary: "US",
          updatedAt: "2026-05-27T01:21:00.000Z",
          alerts: [],
        },
        {
          id: "batch-4",
          title: "Batch Four",
          source: "batch",
          isRecoverableDraft: false,
          primaryProductName: "Half Flag",
          createdTaskCount: 0,
          designCount: 0,
          productCount: 1,
          storeSummary: "US",
          updatedAt: "2026-05-27T01:20:00.000Z",
          alerts: [],
        },
      ])
      .mockReturnValueOnce([]);

    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(screen.getByText("Local Draft")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "查看全部批次" }));
    fireEvent.click(screen.getByRole("button", { name: "删除-local-draft:group-1" }));

    await waitFor(() => {
      expect(clearLocalSheinStudioDraftSnapshot).toHaveBeenCalled();
    });
  });

  it("expands the full batches dashboard as a secondary view", async () => {
    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(screen.getByText("Batch Four")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "查看全部批次" }));

    expect(
      screen.getByRole("heading", { name: "全部批次看板" }),
    ).toBeInTheDocument();
    expect(screen.getByText("最近批次")).toBeInTheDocument();
    expect(
      screen.getByText("最近 3 个批次摘要已折叠，避免和下面的完整看板重复；处理完成后可以随时返回首页摘要。"),
    ).toBeInTheDocument();
    expect(screen.queryByText("Product Four")).not.toBeInTheDocument();
    expect(scrollIntoView).toHaveBeenCalled();
    expect(
      screen.getByRole("button", { name: "返回首页摘要（最近 3 个）" }),
    ).toBeInTheDocument();
  });

  it("wires rename and delete actions into the full recent batches dashboard", async () => {
    saveSheinStudioBatch.mockResolvedValue({
      id: "batch-4",
      name: "Batch Four Renamed",
    });

    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(screen.getByText("Batch Four")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "查看全部批次" }));
    fireEvent.click(screen.getByRole("button", { name: "重命名-batch-4" }));

    await waitFor(() => {
      expect(getSheinStudioBatch).toHaveBeenCalledWith("batch-4");
    });
    expect(saveSheinStudioBatch).toHaveBeenCalledWith(
      expect.objectContaining({
        id: "batch-4",
        name: "Batch Four Renamed",
      }),
      { makeActive: false },
    );

    fireEvent.click(screen.getByRole("button", { name: "删除-batch-4" }));
    await waitFor(() => {
      expect(deleteSheinStudioBatch).toHaveBeenCalledWith("batch-4");
    });
  });

  it("treats missing batches as benign during bulk delete and still refreshes", async () => {
    deleteSheinStudioBatch
      .mockRejectedValueOnce(new Error("studio session not found"))
      .mockResolvedValueOnce(undefined);
    listSheinStudioBatches.mockClear();

    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(screen.getByText("Batch Four")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "查看全部批次" }));
    fireEvent.click(screen.getByRole("button", { name: "批量删除-batch-4-batch-3" }));

    await waitFor(() => {
      expect(deleteSheinStudioBatch).toHaveBeenNthCalledWith(1, "batch-4");
      expect(deleteSheinStudioBatch).toHaveBeenNthCalledWith(2, "batch-3");
    });
    await waitFor(() => {
      expect(listSheinStudioBatches).toHaveBeenCalled();
    });
  });

  it("renders a lighter empty recent-batches state when there is nothing to continue", async () => {
    buildRecentBatchSummaries.mockReturnValueOnce([]);

    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "还没有可继续的最近批次" }),
      ).toBeInTheDocument();
    });

    expect(screen.queryByRole("heading", { name: "最近批次摘要" })).not.toBeInTheDocument();
    expect(
      screen.getByText("首页先保留为空态入口，等你创建第一个批次后，这里会显示最近批次摘要和完整看板入口。"),
    ).toBeInTheDocument();
  });

  it("shows a recoverable error state instead of pretending failed loads are empty", async () => {
    listSheinStudioBatches.mockRejectedValueOnce(
      new Error("ListingKit API request failed: 504"),
    );

    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "最近批次暂时加载失败" }),
      ).toBeInTheDocument();
    });

    expect(
      screen.getAllByText(
        "最近批次这次没有成功加载出来，请重试；如果持续失败，再检查登录态或后端服务。",
      ),
    ).toHaveLength(2);
    expect(
      screen.queryByRole("heading", { name: "还没有可继续的最近批次" }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "重新加载最近批次" }),
    ).toBeInTheDocument();
  });

  it("lets the user retry a failed recent-batches load", async () => {
    listSheinStudioBatches
      .mockRejectedValueOnce(new Error("ListingKit API request failed: 504"))
      .mockResolvedValueOnce([]);

    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "最近批次暂时加载失败" }),
      ).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "重新加载最近批次" }));

    await waitFor(() => {
      expect(screen.getByText("Batch Four")).toBeInTheDocument();
    });
    expect(listSheinStudioBatches).toHaveBeenCalledTimes(2);
  });

  it("does not mislabel endpoint-specific 401s as a global login expiry", async () => {
    listSheinStudioBatches.mockRejectedValueOnce(
      new ApiError("ListingKit API request failed: 401", 401),
    );

    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "最近批次暂时加载失败" }),
      ).toBeInTheDocument();
    });

    expect(
      screen.getAllByText(
        "最近批次接口这次请求被拒绝了。既然其他页面正常，这更像是这个接口自己的鉴权或会话透传有问题，请重试；如果持续失败，再单独排查这个接口。",
      ),
    ).toHaveLength(2);
    expect(screen.queryByText("登录状态可能已失效，请刷新页面或重新登录后再试。")).not.toBeInTheDocument();
  });

  it("surfaces inactive-token errors as a UI/API config mismatch hint", async () => {
    listSheinStudioBatches.mockRejectedValueOnce(
      new Error(
        "ZITADEL token introspection returned an inactive token; check whether the UI and API are using the same ZITADEL issuer/client configuration",
      ),
    );

    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "最近批次暂时加载失败" }),
      ).toBeInTheDocument();
    });

    expect(
      screen.getAllByText(
        "最近批次接口拿到的是一张当前后端不认可的 ZITADEL token。既然其他页面正常，这通常不是你没登录，而是前端和 API 用的 ZITADEL issuer 或 client 配置没对齐。",
      ),
    ).toHaveLength(2);
  });
});
