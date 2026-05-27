import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";

import ListingKitSDSPage from "@/app/listing-kits/sds/page";

const push = vi.fn();
const listSheinStudioBatches = vi.fn();
const buildRecentBatchSummaries = vi.fn();
const loadLocalSheinStudioDraftSnapshot = vi.fn();
const scrollIntoView = vi.fn();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push }),
  usePathname: () => "/listing-kits/sds",
}));

vi.mock("@/lib/utils/shein-studio-batches", () => ({
  listSheinStudioBatches: (...args: unknown[]) => listSheinStudioBatches(...args),
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
  }),
);

vi.mock(
  "@/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard",
  () => ({
    SheinStudioRecentBatchesDashboard: ({
      summaries,
    }: {
      summaries: Array<{ id: string; title: string }>;
    }) => (
      <section>
        <h2>最近批次</h2>
        <ul>
          {summaries.map((summary) => (
            <li key={summary.id}>{summary.title}</li>
          ))}
        </ul>
      </section>
    ),
  }),
);

describe("/listing-kits/sds page", () => {
  beforeEach(() => {
    push.mockReset();
    listSheinStudioBatches.mockReset();
    buildRecentBatchSummaries.mockReset();
    loadLocalSheinStudioDraftSnapshot.mockReset();
    scrollIntoView.mockReset();
    Object.defineProperty(HTMLElement.prototype, "scrollIntoView", {
      configurable: true,
      value: scrollIntoView,
    });

    listSheinStudioBatches.mockResolvedValue([]);
    loadLocalSheinStudioDraftSnapshot.mockReturnValue(null);
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
});
