import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ImportTaskAdminPage } from "@/components/listingkit/admin/import-task-admin-page";
import * as adminImportTasksApi from "@/lib/api/admin-import-tasks";
import * as adminStoresApi from "@/lib/api/admin-stores";

describe("ImportTaskAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads stores and renders ListingKit import tasks", async () => {
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([
      { id: 11, name: "SHEIN US", platform: "SHEIN", region: "US" },
    ]);
    vi.spyOn(adminImportTasksApi, "getListingImportTasks").mockResolvedValue({
      items: [
        {
          id: 1,
          tenantId: 101,
          storeId: 11,
          platform: "Amazon",
          region: "US",
          categoryId: 22,
          productId: "B001",
          status: 0,
          retryCount: 0,
          maxRetryCount: 3,
          priority: 8,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <ImportTaskAdminPage />
      </QueryClientProvider>,
    );

    expect(
      screen.getByRole("heading", { name: "任务导入" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("B001")).toBeInTheDocument();
    });
    expect(screen.getAllByText("SHEIN US").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Amazon").length).toBeGreaterThan(0);
  });
});
