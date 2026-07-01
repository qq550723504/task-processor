import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
          reason_code: "no_capacity",
          stage: "dispatch",
          error_message: "Dispatch delayed: no_capacity",
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
    expect(screen.getAllByText("SHEIN US (#11)").length).toBeGreaterThan(0);
    expect(screen.getAllByText("Amazon").length).toBeGreaterThan(0);
    expect(screen.getByText("no_capacity")).toBeInTheDocument();
    expect(screen.getByText("dispatch")).toBeInTheDocument();
    expect(
      screen.getByText("Dispatch delayed: no_capacity"),
    ).toBeInTheDocument();
  });

  it("imports product ids from a CSV file before batch creating tasks", async () => {
    const user = userEvent.setup();
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([
      { id: 11, name: "SHEIN US", platform: "SHEIN", region: "US" },
    ]);
    vi.spyOn(adminImportTasksApi, "getListingImportTasks").mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 20,
    });
    const batchCreateSpy = vi
      .spyOn(adminImportTasksApi, "batchCreateListingImportTasks")
      .mockResolvedValue({ createdCount: 2, items: [] });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <ImportTaskAdminPage />
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getAllByText("SHEIN US (#11)").length).toBeGreaterThan(0);
    });

    await user.selectOptions(screen.getByLabelText("店铺"), "11");
    await user.type(screen.getByLabelText("类目 ID"), "22");
    const regionSelect = screen.getByLabelText("地区");
    expect(regionSelect.tagName).toBe("SELECT");
    await user.selectOptions(regionSelect, "CA");
    const file = new File(["product_id\nB001\nB002\nB001\n"], "tasks.csv", {
      type: "text/csv",
    });
    await user.upload(screen.getByLabelText("批量导入文件"), file);

    expect(await screen.findByText("已读取 2 个商品 ID")).toBeInTheDocument();
    expect(screen.getByText("已去重 1 个重复 ID")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "导入任务" }));

    expect(batchCreateSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        storeId: 11,
        categoryId: 22,
        region: "CA",
        productIds: ["B001", "B002"],
      }),
    );
  });
});
