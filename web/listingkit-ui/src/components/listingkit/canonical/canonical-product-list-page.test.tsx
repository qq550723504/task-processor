import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { CanonicalProductListPage } from "@/components/listingkit/canonical/canonical-product-list-page";

vi.mock("next/navigation", () => ({
  useSearchParams: () => new URLSearchParams(),
}));

vi.mock("@/lib/query/use-canonical-products", () => ({
  useCanonicalProducts: () => ({
    isLoading: false,
    isError: false,
    refetch: vi.fn(),
    data: {
      items: [
        {
          taskId: "task-1",
          title: "Canvas Tote",
          brand: "Studio",
          categoryPath: ["Bags"],
          imageUrl: "https://example.com/main.jpg",
          platformLabels: ["shein"],
          needsReview: false,
          imageCount: 3,
          variantCount: 2,
          completedAt: "2026-05-01T00:00:00Z",
          createdAt: "2026-04-30T00:00:00Z",
        },
      ],
      total: 1,
    },
  }),
}));

describe("CanonicalProductListPage", () => {
  it("stacks top actions and row actions for narrow layouts", () => {
    render(<CanonicalProductListPage />);

    expect(screen.getByRole("button", { name: "刷新" })).toHaveClass("w-full");
    expect(screen.getByRole("link", { name: "打开商品" })).toHaveClass("w-full");
  });

  it("presents canonical products as the Product Center without execution-language copy", () => {
    render(<CanonicalProductListPage />);

    expect(screen.getByRole("heading", { name: "商品中心" })).toBeInTheDocument();
    expect(
      screen.getByText("管理 ListingKit 已整理的商品资料，并查看审核与平台准备情况。"),
    ).toBeInTheDocument();
    expect(screen.getByText("当前页 1 个商品")).toBeInTheDocument();
    expect(screen.getByText("共 1 个商品")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "打开商品" })).toHaveAttribute(
      "href",
      "/listing-kits/canonical-products/task-1",
    );
    expect(screen.queryByText(/canonical_product/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/task result/i)).not.toBeInTheDocument();
  });
});
