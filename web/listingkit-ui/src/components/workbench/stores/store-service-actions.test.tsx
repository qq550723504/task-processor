import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { StoreServiceActions } from "./store-service-actions";
import {
  WorkbenchAPIError,
  type WorkbenchStore,
} from "@/lib/api/workbench-stores";
import { commercialResourcesFixture } from "@/test/fixtures/commercial-resources";
const state = vi.hoisted(() => ({
  renew: vi.fn(),
  balance: vi.fn(),
  members: vi.fn(),
}));
vi.mock("@/lib/api/workbench-stores", async (original) => ({
  ...(await original<typeof import("@/lib/api/workbench-stores")>()),
  renewWorkbenchStoreService: state.renew,
}));
vi.mock("@/lib/api/commercial-billing", async (original) => ({
  ...(await original<typeof import("@/lib/api/commercial-billing")>()),
  getCommercialResources: state.balance,
}));
vi.mock("@/lib/api/member-resources", async (original) => ({
  ...(await original<typeof import("@/lib/api/member-resources")>()),
  getMemberResources: state.members,
}));
const scope = { expectedUserId: "actor", expectedOrganizationId: "org-1" };
const store = {
  id: "11111111-1111-4111-8111-11111111111a",
  name: "测试店铺",
  recordStatus: "active",
  serviceStatus: "active",
  version: 2,
} as WorkbenchStore;
function tree(administrator = false, connected = true, record = store) {
  return (
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <StoreServiceActions
        store={record}
        scope={scope}
        administrator={administrator}
        canWrite
        connected={connected}
        onChanged={vi.fn().mockResolvedValue(null)}
      />
    </QueryClientProvider>
  );
}
beforeEach(() => {
  Object.defineProperty(HTMLDialogElement.prototype, "showModal", {
    configurable: true,
    value: function () {
      this.open = true;
    },
  });
  Object.defineProperty(HTMLDialogElement.prototype, "close", {
    configurable: true,
    value: function () {
      this.open = false;
    },
  });
  state.renew.mockReset();
  state.balance.mockResolvedValue(commercialResourcesFixture("org-1"));
  state.members.mockResolvedValue({
    organizationId: "org-1",
    members: [
      { memberId: "current-member", userId: "actor", periods: { free: "2" } },
    ],
  });
});
afterEach(cleanup);
it("member renewals retry only the original key, version and captured user", async () => {
  state.renew
    .mockRejectedValueOnce(
      new WorkbenchAPIError(
        503,
        "STORE_SERVICE_OUTCOME_UNKNOWN",
        "safe",
        "",
        [],
      ),
    )
    .mockResolvedValueOnce({
      quantity: "1",
      resourceBalanceAfter: "1",
      serviceExpiresAt: "2026-10-29T00:00:00Z",
    });
  render(tree());
  await userEvent.click(screen.getByRole("button", { name: "续费服务" }));
  const dialog = screen.getByRole("dialog");
  expect(await within(dialog).findByText("可用 2 期")).toBeVisible();
  await userEvent.click(
    within(dialog).getByRole("button", { name: "确认续费服务" }),
  );
  await screen.findByText(/续费结果未确认/);
  await userEvent.click(screen.getByRole("button", { name: "核验原操作" }));
  expect(state.renew).toHaveBeenCalledTimes(2);
  expect(state.renew.mock.calls[1]).toEqual(state.renew.mock.calls[0]);
  expect(state.renew.mock.calls[0]).toEqual([
    store.id,
    1,
    2,
    expect.any(String),
    "org-1",
    "actor",
  ]);
});
it("admin renewal uses unallocated balance and closes first activation when disconnected", async () => {
  const resource = commercialResourcesFixture("org-1");
  resource.resources[0].available = "0";
  resource.resources[0].allocated = "5";
  state.balance.mockResolvedValue(resource);
  const { unmount } = render(tree(true));
  await userEvent.click(screen.getByRole("button", { name: "续费服务" }));
  await screen.findByText("可用 0 期");
  expect(screen.getByRole("button", { name: "确认续费服务" })).toBeDisabled();
  expect(state.members).not.toHaveBeenCalled();
  unmount();
  render(tree(false, false, { ...store, serviceStatus: "pending_activation" }));
  expect(screen.getByRole("button", { name: "开通服务" })).toBeDisabled();
});
