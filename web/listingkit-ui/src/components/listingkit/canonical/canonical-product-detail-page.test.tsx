import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { CanonicalProductDetailPage } from "@/components/listingkit/canonical/canonical-product-detail-page";

vi.mock("@/lib/query/use-canonical-products", () => ({
  useCanonicalProductDetail: () => ({
    isLoading: false,
    isError: false,
    data: {
      taskId: "task-1",
      sourceReference: {
        key: "crawler:1688:888",
        type: "crawler",
        platform: "1688",
        id: "888",
        url: "https://detail.1688.com/offer/888.html",
      },
      workspaceHref: "/listing-kits/task-1/workspace?platform=shein",
      product: {
        title: "Canvas Tote",
        brand: "Studio",
        category_path: ["Bags"],
        images: [
          { url: "https://example.com/main.jpg", role: "main" },
          { url: "https://example.com/detail.jpg", role: "gallery" },
          { url: "https://example.com/main.jpg", role: "duplicate" },
        ],
      },
      summary: {
        taskId: "task-1",
        title: "Canvas Tote",
        categoryPath: ["Bags"],
        imageUrl: "https://example.com/main.jpg",
        platformLabels: ["shein"],
        needsReview: false,
        imageCount: 3,
        variantCount: 0,
      },
      reviewFieldCount: 0,
      trustedFieldCount: 0,
      fieldTraces: [],
    },
  }),
}));

describe("CanonicalProductDetailPage", () => {
  it("renders the full canonical image gallery instead of only the cover image", () => {
    render(<CanonicalProductDetailPage taskId="task-1" />);

    expect(screen.getByText("2 张")).toBeInTheDocument();
    expect(screen.getByText("main")).toBeInTheDocument();
    expect(screen.getByText("gallery")).toBeInTheDocument();
    expect(screen.getAllByRole("link", { name: /Canvas Tote/i })).toHaveLength(2);
  });

  it("shows the persisted source lineage", () => {
    render(<CanonicalProductDetailPage taskId="task-1" />);

    expect(screen.getByText("来源 1688 · 888")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "查看来源" })).toHaveAttribute(
      "href",
      "https://detail.1688.com/offer/888.html",
    );
  });

  it("uses product language and keeps execution identity secondary", () => {
    render(<CanonicalProductDetailPage taskId="task-1" />);

    expect(screen.getByRole("link", { name: "返回商品中心" })).toHaveAttribute(
      "href",
      "/listing-kits/canonical-products",
    );
    expect(screen.getByText("商品详情")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "编辑商品" })).toHaveAttribute(
      "href",
      "/listing-kits/task-1/workspace?platform=shein",
    );
    expect(screen.getByRole("link", { name: "查看执行记录" })).toHaveAttribute(
      "href",
      "/listing-kits/task-1/status",
    );
    expect(screen.getByText("需确认字段")).toBeInTheDocument();
    expect(screen.getByText("已校验")).toBeInTheDocument();
    expect(screen.getByText("字段依据")).toBeInTheDocument();
    expect(screen.queryByText("task-1")).not.toBeInTheDocument();
    expect(screen.queryByText(/canonical_product/i)).not.toBeInTheDocument();
  });

  it("preserves the existing workspace and execution routes", () => {
    render(<CanonicalProductDetailPage taskId="task-1" />);

    expect(screen.getByRole("link", { name: "查看执行记录" })).toHaveAttribute(
      "href",
      "/listing-kits/task-1/status",
    );
    expect(screen.getByRole("link", { name: "编辑商品" })).toHaveAttribute(
      "href",
      "/listing-kits/task-1/workspace?platform=shein",
    );
  });
});
