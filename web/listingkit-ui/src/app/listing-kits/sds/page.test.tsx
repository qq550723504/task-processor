import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach } from "vitest";

import ListingKitSDSPage from "@/app/listing-kits/sds/page";

const push = vi.fn();
const listSheinStudioBatches = vi.fn();
const buildRecentBatchSummaries = vi.fn();
const loadLocalSheinStudioDraftSnapshot = vi.fn();

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

    listSheinStudioBatches.mockResolvedValue([]);
    loadLocalSheinStudioDraftSnapshot.mockReturnValue(null);
    buildRecentBatchSummaries.mockReturnValue([
      { id: "batch-4", title: "Batch Four", source: "batch" },
      { id: "batch-3", title: "Batch Three", source: "batch" },
      { id: "batch-2", title: "Batch Two", source: "batch" },
      { id: "batch-1", title: "Batch One", source: "batch" },
    ]);
  });

  it("renders the SDS homepage without the product browser", async () => {
    render(<ListingKitSDSPage />);

    expect(
      screen.getByRole("heading", { name: "从 POD 商品生成上架资料" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "最近批次" })).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "新建批次并选品" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("heading", { name: "选择底版商品和子 SKU" }),
    ).not.toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByText("Batch Four")).toBeInTheDocument();
    });

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

  it("routes continue recent to the latest persisted batch", async () => {
    render(<ListingKitSDSPage />);

    await waitFor(() => {
      expect(screen.getByText("Batch Four")).toBeInTheDocument();
    });

    fireEvent.click(screen.getByRole("button", { name: "继续最近批次" }));

    expect(push).toHaveBeenCalledWith("/listing-kits/sds/batches/batch-4");
  });
});
