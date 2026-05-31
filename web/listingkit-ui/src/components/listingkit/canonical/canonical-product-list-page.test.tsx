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
    },
  }),
}));

describe("CanonicalProductListPage", () => {
  it("stacks top actions and row actions for narrow layouts", () => {
    render(<CanonicalProductListPage />);

    expect(screen.getByRole("button", { name: "刷新" })).toHaveClass("w-full");
    expect(screen.getByRole("link", { name: "详情" })).toHaveClass("w-full");
  });
});
