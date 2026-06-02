import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { CategoryAdminPage } from "@/components/listingkit/admin/category-admin-page";
import * as adminCategoriesApi from "@/lib/api/admin-categories";

describe("CategoryAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads and renders ListingKit categories", async () => {
    vi.spyOn(adminCategoriesApi, "getListingCategories").mockResolvedValue([
      {
        id: 1,
        tenantId: 101,
        name: "Apparel",
        code: "APPAREL",
        parentId: 0,
        level: 1,
        sort: 10,
        status: 1,
      },
      {
        id: 2,
        tenantId: 101,
        name: "Shirts",
        code: "SHIRTS",
        parentId: 1,
        level: 2,
        sort: 20,
        status: 1,
      },
    ]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <CategoryAdminPage />
      </QueryClientProvider>,
    );

    expect(screen.getByRole("heading", { name: "分类" })).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("Apparel")).toBeInTheDocument();
    });
    expect(screen.getByText("SHIRTS")).toBeInTheDocument();
    expect(screen.getByText("#1")).toBeInTheDocument();
  });

  it("uses a stacked mobile-first filter form and scrollable table", async () => {
    vi.spyOn(adminCategoriesApi, "getListingCategories").mockResolvedValue([]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const { container } = render(
      <QueryClientProvider client={queryClient}>
        <CategoryAdminPage />
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(adminCategoriesApi.getListingCategories).toHaveBeenCalled();
    });

    expect(screen.getByRole("button", { name: "查询" })).toHaveClass("w-full");
    expect(container.querySelector(".overflow-x-auto")).not.toBeNull();
  });
});
