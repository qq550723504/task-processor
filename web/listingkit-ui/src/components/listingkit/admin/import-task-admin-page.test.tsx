import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ImportTaskAdminPage } from "@/components/listingkit/admin/import-task-admin-page";
import * as adminCategoriesApi from "@/lib/api/admin-categories";
import * as adminImportTasksApi from "@/lib/api/admin-import-tasks";
import * as adminStoresApi from "@/lib/api/admin-stores";

describe("ImportTaskAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
    vi.spyOn(adminCategoriesApi, "getListingCategories").mockResolvedValue([]);
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
    vi.mocked(adminCategoriesApi.getListingCategories).mockResolvedValue([
      {
        id: 10,
        tenantId: 101,
        name: "服装",
        code: "apparel",
        parentId: 0,
        level: 1,
        sort: 1,
        status: 1,
      },
      {
        id: 22,
        tenantId: 101,
        name: "上衣",
        code: "tops",
        parentId: 10,
        level: 2,
        sort: 1,
        status: 1,
      },
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
    await waitFor(() => {
      expect(adminCategoriesApi.getListingCategories).toHaveBeenCalledWith({
        status: "1",
      });
    });

    await user.selectOptions(screen.getByLabelText("店铺"), "11");
    await user.selectOptions(screen.getByLabelText("分类"), "22");
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

  it("allows importing product ids without a category id", async () => {
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
      .mockResolvedValue({ createdCount: 1, items: [] });

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
    const file = new File(["product_id\nB001\n"], "tasks.csv", {
      type: "text/csv",
    });
    await user.upload(screen.getByLabelText("批量导入文件"), file);
    await user.click(screen.getByRole("button", { name: "导入任务" }));

    await waitFor(() => {
      expect(batchCreateSpy).toHaveBeenCalled();
    });
    expect(batchCreateSpy.mock.calls[0]?.[0].categoryId).toBeUndefined();
  });

  it("shows completed products skipped by a batch import", async () => {
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
      .mockResolvedValue({
        createdCount: 1,
        skippedCount: 1,
        skippedProductIds: ["B002"],
        items: [],
      });

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
    await user.type(screen.getByPlaceholderText("每行一个商品 ID"), "B001 B002");
    await user.click(screen.getByRole("button", { name: "导入任务" }));

    await waitFor(() => {
      expect(batchCreateSpy).toHaveBeenCalled();
    });
    expect(screen.getByRole("status")).toHaveTextContent(
      "已跳过 1 个已完成商品",
    );
    expect(screen.getByRole("status")).toHaveTextContent("B002");
  });

  it("clears the previous batch result when a later import fails", async () => {
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
      .mockResolvedValueOnce({
        createdCount: 1,
        skippedProductIds: ["B002"],
        items: [],
      })
      .mockRejectedValueOnce(new Error("active duplicate"));

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
    await user.type(screen.getByPlaceholderText("每行一个商品 ID"), "B001 B002");
    await user.click(screen.getByRole("button", { name: "导入任务" }));

    await waitFor(() => {
      expect(screen.getByRole("status")).toHaveTextContent("B002");
    });
    await user.type(screen.getByPlaceholderText("每行一个商品 ID"), "B003");
    await user.click(screen.getByRole("button", { name: "导入任务" }));

    await waitFor(() => {
      expect(batchCreateSpy).toHaveBeenCalledTimes(2);
      expect(screen.queryByText("B002")).not.toBeInTheDocument();
    });
  });

  it("requests the next page and resets pagination when page size changes", async () => {
    const user = userEvent.setup();
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([]);
    const getTasksSpy = vi
      .spyOn(adminImportTasksApi, "getListingImportTasks")
      .mockResolvedValue({
        items: [],
        total: 51,
        page: 1,
        page_size: 50,
      });

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <ImportTaskAdminPage />
      </QueryClientProvider>,
    );

    await screen.findByText("共 51 条，当前第 1 / 2 页");
    await user.click(screen.getByRole("button", { name: "下一页" }));

    await waitFor(() => {
      expect(getTasksSpy).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 2, page_size: 50 }),
      );
    });

    await user.selectOptions(screen.getByLabelText("每页"), "20");
    await waitFor(() => {
      expect(getTasksSpy).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 1, page_size: 20 }),
      );
    });
  });

  it("returns to a valid page when deleting the final task on the last page", async () => {
    const user = userEvent.setup();
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([]);
    const getTasksSpy = vi
      .spyOn(adminImportTasksApi, "getListingImportTasks")
      .mockResolvedValueOnce({
        items: [],
        total: 51,
        page: 1,
        page_size: 50,
      })
      .mockResolvedValueOnce({
        items: [
          {
            id: 51,
            platform: "Amazon",
            productId: "B051",
            status: 0,
          },
        ],
        total: 51,
        page: 2,
        page_size: 50,
      })
      .mockResolvedValueOnce({
        items: [],
        total: 50,
        page: 2,
        page_size: 50,
      })
      .mockResolvedValue({
        items: [],
        total: 50,
        page: 1,
        page_size: 50,
      });
    vi.spyOn(adminImportTasksApi, "deleteListingImportTask").mockResolvedValue();

    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    render(
      <QueryClientProvider client={queryClient}>
        <ImportTaskAdminPage />
      </QueryClientProvider>,
    );

    await screen.findByText("共 51 条，当前第 1 / 2 页");
    await user.click(screen.getByRole("button", { name: "下一页" }));
    await screen.findByText("B051");
    await user.click(screen.getByRole("button", { name: "删除 B051" }));

    await waitFor(() => {
      expect(getTasksSpy).toHaveBeenLastCalledWith(
        expect.objectContaining({ page: 1, page_size: 50 }),
      );
    });
    expect(screen.getByText("共 50 条，当前第 1 / 1 页")).toBeInTheDocument();
  });
});
