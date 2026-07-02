import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { ScheduledTaskConfigAdminPage } from "@/components/listingkit/admin/scheduled-task-config-admin-page";
import * as scheduledTaskConfigsApi from "@/lib/api/admin-scheduled-task-configs";
import * as adminStoresApi from "@/lib/api/admin-stores";

describe("ScheduledTaskConfigAdminPage", () => {
  beforeEach(() => {
    vi.restoreAllMocks();
  });

  it("loads all task types by default", async () => {
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([]);
    const getConfigs = vi
      .spyOn(scheduledTaskConfigsApi, "getListingScheduledTaskConfigs")
      .mockResolvedValue({
        items: [],
        total: 0,
        page: 1,
        page_size: 50,
      });

    renderWithQueryClient(<ScheduledTaskConfigAdminPage />);

    await waitFor(() => {
      expect(getConfigs).toHaveBeenCalledWith(
        expect.not.objectContaining({
          platform: expect.any(String),
          taskType: expect.any(String),
        }),
      );
    });
    expect(screen.getAllByLabelText("平台")[0]).toHaveValue("");
    expect(screen.getAllByLabelText("任务")[0]).toHaveValue("");
  });

  it("renders scheduled inventory sync configs and saves store-level config", async () => {
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([
      { id: 962, name: "SHEIN 962", platform: "shein", region: "US" },
    ]);
    vi.spyOn(
      scheduledTaskConfigsApi,
      "getListingScheduledTaskConfigs",
    ).mockResolvedValue({
      items: [
        {
          id: 1,
          tenantId: 246,
          storeId: 962,
          platform: "shein",
          taskType: "inventory",
          enabled: true,
          intervalSeconds: 3600,
        },
      ],
      total: 1,
      page: 1,
      page_size: 20,
    });
    const upsert = vi
      .spyOn(scheduledTaskConfigsApi, "upsertListingScheduledTaskConfig")
      .mockResolvedValue({
        id: 1,
        tenantId: 246,
        storeId: 962,
        platform: "shein",
        taskType: "inventory",
        enabled: true,
        intervalSeconds: 3600,
      });

    renderWithQueryClient(<ScheduledTaskConfigAdminPage />);

    expect(
      screen.getByRole("heading", { name: "定时任务配置" }),
    ).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getAllByText("SHEIN 962 (#962)").length).toBeGreaterThan(0);
    });
    expect(screen.getAllByText("库存同步").length).toBeGreaterThan(0);
    expect(screen.getByText("1 小时")).toBeInTheDocument();

    await userEvent.selectOptions(screen.getByLabelText("店铺"), "962");
    await userEvent.click(screen.getByRole("button", { name: "保存配置" }));

    await waitFor(() => {
      expect(upsert).toHaveBeenCalledWith(
        expect.objectContaining({
          storeId: 962,
          platform: "shein",
          taskType: "inventory",
          enabled: true,
          intervalSeconds: 3600,
        }),
      );
    });
  });

  it("uses the selected store platform when saving a scheduled task config", async () => {
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([
      { id: 962, name: "SHEIN 962", platform: "shein", region: "US" },
      { id: 1201, name: "TEMU 1201", platform: "temu", region: "US" },
    ]);
    vi.spyOn(
      scheduledTaskConfigsApi,
      "getListingScheduledTaskConfigs",
    ).mockResolvedValue({
      items: [],
      total: 0,
      page: 1,
      page_size: 50,
    });
    const upsert = vi
      .spyOn(scheduledTaskConfigsApi, "upsertListingScheduledTaskConfig")
      .mockResolvedValue({
        id: 3,
        tenantId: 246,
        storeId: 1201,
        platform: "temu",
        taskType: "inventory",
        enabled: true,
        intervalSeconds: 3600,
      });

    renderWithQueryClient(<ScheduledTaskConfigAdminPage />);

    await waitFor(() => {
      expect(screen.getByText("TEMU 1201 (#1201)")).toBeInTheDocument();
    });
    await userEvent.selectOptions(screen.getByLabelText("店铺"), "1201");
    expect(screen.queryAllByLabelText("平台")).toHaveLength(1);
    await userEvent.click(screen.getByRole("button", { name: "保存配置" }));

    await waitFor(() => {
      expect(upsert).toHaveBeenCalledWith(
        expect.objectContaining({
          storeId: 1201,
          platform: "temu",
        }),
      );
    });
  });

  it("shows the saved config when it belongs to a different task filter", async () => {
    vi.spyOn(adminStoresApi, "getSimpleListingStores").mockResolvedValue([
      { id: 870, name: "SHEIN 870", platform: "shein", region: "US" },
    ]);
    const getConfigs = vi
      .spyOn(scheduledTaskConfigsApi, "getListingScheduledTaskConfigs")
      .mockImplementation(async (query = {}) => ({
        items:
          query.taskType === "productSync"
            ? [
                {
                  id: 2,
                  tenantId: 227,
                  storeId: 870,
                  platform: "shein",
                  taskType: "productSync",
                  enabled: true,
                  intervalSeconds: 3600,
                },
              ]
            : [],
        total: query.taskType === "productSync" ? 1 : 0,
        page: 1,
        page_size: 50,
      }));
    vi.spyOn(
      scheduledTaskConfigsApi,
      "upsertListingScheduledTaskConfig",
    ).mockResolvedValue({
      id: 2,
      tenantId: 227,
      storeId: 870,
      platform: "shein",
      taskType: "productSync",
      enabled: true,
      intervalSeconds: 3600,
    });

    renderWithQueryClient(<ScheduledTaskConfigAdminPage />);

    await waitFor(() => {
      expect(screen.getAllByText("SHEIN 870 (#870)").length).toBeGreaterThan(0);
    });
    await userEvent.selectOptions(screen.getByLabelText("店铺"), "870");
    await userEvent.selectOptions(screen.getAllByLabelText("任务")[1], "productSync");
    await userEvent.click(screen.getByRole("button", { name: "保存配置" }));

    await waitFor(() => {
      expect(getConfigs).toHaveBeenLastCalledWith(
        expect.objectContaining({ taskType: "productSync" }),
      );
    });
    expect(screen.getAllByText("产品同步").length).toBeGreaterThan(0);
    expect(screen.getByText("1 小时")).toBeInTheDocument();
  });
});

function renderWithQueryClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}
