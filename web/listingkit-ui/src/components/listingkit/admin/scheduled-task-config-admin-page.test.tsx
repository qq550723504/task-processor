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
});

function renderWithQueryClient(ui: React.ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  return render(
    <QueryClientProvider client={queryClient}>{ui}</QueryClientProvider>,
  );
}
