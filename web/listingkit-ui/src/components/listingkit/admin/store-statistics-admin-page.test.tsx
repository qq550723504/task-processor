import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { StoreStatisticsAdminPage } from "@/components/listingkit/admin/store-statistics-admin-page";
import * as adminStoreStatisticsApi from "@/lib/api/admin-store-statistics";

describe("StoreStatisticsAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads and renders ListingKit store statistics", async () => {
    vi.spyOn(
      adminStoreStatisticsApi,
      "getListingStoreStatistics",
    ).mockResolvedValue([
      {
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
      },
    ]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <StoreStatisticsAdminPage />
      </QueryClientProvider>,
    );

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
    ).mockResolvedValue([]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    const { container } = render(
      <QueryClientProvider client={queryClient}>
        <StoreStatisticsAdminPage />
      </QueryClientProvider>,
    );

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
    ).mockResolvedValue([
      {
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
      },
    ]);

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <StoreStatisticsAdminPage variant="tenant" />
      </QueryClientProvider>,
    );

    expect(
      screen.getByRole("heading", { name: "我的上架统计" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("SHEIN US")).toBeInTheDocument();
    });
    expect(screen.getByText(/当前账号可见的自动上架店铺/)).toBeInTheDocument();
    expect(screen.queryByText("租户 101")).not.toBeInTheDocument();
  });
});
