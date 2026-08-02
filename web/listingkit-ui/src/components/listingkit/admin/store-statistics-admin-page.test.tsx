import type { ReactNode } from "react";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { StoreStatisticsAdminPage } from "@/components/listingkit/admin/store-statistics-admin-page";
import * as adminStoreStatisticsApi from "@/lib/api/admin-store-statistics";
import type {
  ListingStoreStatistics,
  ListingStoreStatisticsPage,
} from "@/lib/api/admin-store-statistics";

function createQueryClient() {
  return new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
}

function renderPage(ui: ReactNode) {
  const queryClient = createQueryClient();
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}

function buildStatisticsItem(
  overrides: Partial<ListingStoreStatistics> = {},
): ListingStoreStatistics {
  return {
    id: 1,
    storeId: "SHEIN-US",
    tenantId: 101,
    name: "SHEIN US",
    platform: "SHEIN",
    dailyLimit: 10,
    dailyLimitType: "fixed",
    completedCount: 6,
    remainingCount: 2,
    holdCount: 1,
    queuedCount: 3,
    remainingQuota: 4,
    progressPercentage: 60,
    status: 0,
    ...overrides,
  };
}

function buildStatisticsPage(
  overrides: Partial<ListingStoreStatisticsPage> = {},
): ListingStoreStatisticsPage {
  return {
    items: [buildStatisticsItem()],
    total: 1,
    page: 1,
    page_size: 20,
    summary: {
      completed_count: 6,
      daily_limit: 10,
      remaining_count: 2,
      queued_count: 3,
      hold_count: 1,
    },
    ...overrides,
  };
}

describe("StoreStatisticsAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads and renders ListingKit store statistics", async () => {
    vi.spyOn(
      adminStoreStatisticsApi,
      "getListingStoreStatistics",
    ).mockResolvedValue(buildStatisticsPage());

    renderPage(<StoreStatisticsAdminPage />);

    expect(
      screen.getByRole("heading", { name: "上架统计" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("SHEIN US")).toBeInTheDocument();
    });
    expect(screen.getByText("6 / 10")).toBeInTheDocument();
    expect(screen.getByText("60%")).toBeInTheDocument();
  });

  it("keeps summary cards and the table mobile-friendly", async () => {
    vi.spyOn(
      adminStoreStatisticsApi,
      "getListingStoreStatistics",
    ).mockResolvedValue(buildStatisticsPage({ items: [], total: 0 }));

    const { container } = renderPage(<StoreStatisticsAdminPage />);

    await waitFor(() => {
      expect(adminStoreStatisticsApi.getListingStoreStatistics).toHaveBeenCalled();
    });

    expect(screen.getByRole("button", { name: "刷新" })).toHaveClass("w-full");
    expect(container.querySelector(".sm\\:grid-cols-2")).not.toBeNull();
    expect(container.querySelector(".overflow-x-auto")).not.toBeNull();
  });

  it("renders a tenant-facing statistics view without tenant IDs", async () => {
    vi.spyOn(
      adminStoreStatisticsApi,
      "getListingStoreStatistics",
    ).mockResolvedValue(buildStatisticsPage());

    renderPage(<StoreStatisticsAdminPage variant="tenant" />);

    expect(
      screen.getByRole("heading", { name: "我的上架统计" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("SHEIN US")).toBeInTheDocument();
    });
    expect(screen.getByText(/当前账号可见的自动上架店铺/)).toBeInTheDocument();
    expect(screen.queryByText("租户 101")).not.toBeInTheDocument();
  });

  it("uses full-scope totals and paginates the statistics page", async () => {
    const user = userEvent.setup();
    const getStatisticsSpy = vi
      .spyOn(adminStoreStatisticsApi, "getListingStoreStatistics")
      .mockResolvedValue(
        buildStatisticsPage({
          total: 41,
          page: 1,
          page_size: 20,
          summary: {
            completed_count: 33,
            daily_limit: 90,
            remaining_count: 40,
            queued_count: 5,
            hold_count: 2,
          },
        }),
      );

    renderPage(<StoreStatisticsAdminPage />);

    await screen.findByText(/共 41/);
    expect(screen.getByText("33")).toBeInTheDocument();
    expect(screen.getByText("40")).toBeInTheDocument();
    expect(screen.getByText("5")).toBeInTheDocument();
    expect(screen.getByText("2")).toBeInTheDocument();
    expect(
      screen.getByText("第 1 / 3 页 · 显示 1-20 / 41 条"),
    ).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "下一页" }));

    await waitFor(() => {
      expect(getStatisticsSpy).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 2, page_size: 20 }),
      );
    });
  });

  it("offers only 20, 50, and 100 as page-size options", async () => {
    vi.spyOn(
      adminStoreStatisticsApi,
      "getListingStoreStatistics",
    ).mockResolvedValue(
      buildStatisticsPage({
        total: 41,
        page: 1,
        page_size: 20,
      }),
    );

    renderPage(<StoreStatisticsAdminPage />);

    const pageSizeSelect = await screen.findByRole("combobox", { name: "每页" });
    const options = within(pageSizeSelect)
      .getAllByRole("option")
      .map((option) => option.getAttribute("value"));

    expect(options).toEqual(["20", "50", "100"]);
  });

  it("resets pagination to the first page when the date filter changes", async () => {
    const user = userEvent.setup();
    const getStatisticsSpy = vi
      .spyOn(adminStoreStatisticsApi, "getListingStoreStatistics")
      .mockResolvedValue(
        buildStatisticsPage({
          total: 41,
          page: 1,
          page_size: 20,
        }),
      );

    renderPage(<StoreStatisticsAdminPage />);

    const dateInput = screen.getByLabelText("日期");
    await screen.findByText(/共 41/);
    await user.click(screen.getByRole("button", { name: "下一页" }));

    await waitFor(() => {
      expect(getStatisticsSpy).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 2, page_size: 20 }),
      );
    });

    await user.clear(dateInput);
    await user.type(dateInput, "2026-08-01");

    await waitFor(() => {
      expect(getStatisticsSpy).toHaveBeenLastCalledWith(
        expect.objectContaining({
          date: "2026-08-01",
          page: 1,
          page_size: 20,
        }),
      );
    });
  });

  it("keeps the tenant title while using paginated statistics data", async () => {
    vi.spyOn(
      adminStoreStatisticsApi,
      "getListingStoreStatistics",
    ).mockResolvedValue(
      buildStatisticsPage({
        total: 41,
        page: 1,
        page_size: 20,
      }),
    );

    renderPage(<StoreStatisticsAdminPage variant="tenant" />);

    expect(
      screen.getByRole("heading", { name: "我的上架统计" }),
    ).toBeInTheDocument();
    await screen.findByText(/共 41/);
  });

  it("falls back to the last valid page after refresh returns an out-of-range page", async () => {
    const user = userEvent.setup();
    const getStatisticsSpy = vi
      .spyOn(adminStoreStatisticsApi, "getListingStoreStatistics")
      .mockResolvedValueOnce(
        buildStatisticsPage({
          total: 41,
          page: 1,
          page_size: 20,
        }),
      )
      .mockResolvedValueOnce(
        buildStatisticsPage({
          total: 41,
          page: 2,
          page_size: 20,
          items: [buildStatisticsItem({ id: 2, name: "SHEIN EU" })],
        }),
      )
      .mockResolvedValueOnce(
        buildStatisticsPage({
          total: 20,
          page: 2,
          page_size: 20,
          items: [],
        }),
      )
      .mockResolvedValueOnce(
        buildStatisticsPage({
          total: 20,
          page: 1,
          page_size: 20,
          items: [buildStatisticsItem({ id: 3, name: "Recovered Page 1" })],
        }),
      );

    renderPage(<StoreStatisticsAdminPage />);

    await screen.findByText(/共 41/);
    await user.click(screen.getByRole("button", { name: "下一页" }));
    await screen.findByText("SHEIN EU");

    await user.click(screen.getByRole("button", { name: "刷新" }));

    await waitFor(() => {
      expect(getStatisticsSpy).toHaveBeenNthCalledWith(
        4,
        expect.objectContaining({ page: 1, page_size: 20 }),
      );
    });
    await screen.findByText("Recovered Page 1");
  });
});
