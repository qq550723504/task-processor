import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { CanonicalProductDetailPage } from "@/components/listingkit/canonical/canonical-product-detail-page";

vi.mock("@/lib/query/use-canonical-products", () => ({
  useCanonicalProductDetail: () => ({
    isLoading: false,
    isError: false,
    data: {
      taskId: "task-1",
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
});
