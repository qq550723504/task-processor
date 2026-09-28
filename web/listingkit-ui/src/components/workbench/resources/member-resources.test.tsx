import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { MemberResources } from "./member-resources";
import { MemberResourceError } from "@/lib/api/member-resources";
import { commercialResourcesFixture } from "@/test/fixtures/commercial-resources";
const state = vi.hoisted(() => ({
  read: vi.fn(),
  write: vi.fn(),
  balance: vi.fn(),
  prices: vi.fn(),
}));
vi.mock("@/lib/api/member-resources", async (original) => ({
  ...(await original<typeof import("@/lib/api/member-resources")>()),
  getMemberResources: state.read,
  transferMemberResource: state.write,
  getMemberDataPrices: state.prices,
}));
vi.mock("@/lib/api/commercial-billing", async (original) => ({
  ...(await original<typeof import("@/lib/api/commercial-billing")>()),
  getCommercialResources: state.balance,
}));
const position = {
  free: "2",
  reserved: "1",
  consumed: "3",
  version: "1",
  recorded: true,
};
const member = {
  memberId: "member-1",
  userId: "user-1",
  displayName: "张琳",
  loginName: "member@example.test",
  state: "active",
  roles: ["listingkit_operator"],
  storeCount: "1",
  periods: position,
  dataRows: position,
};
function tree() {
  return (
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <MemberResources
        userId="actor"
        organizationId="org-1"
        canManage
        sequence={0}
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
  state.read.mockResolvedValue({
    schemaVersion: "member-resource-directory-v1",
    organizationId: "org-1",
    observedAt: "2026-09-29T00:00:00Z",
    members: [member],
  });
  state.balance.mockResolvedValue(commercialResourcesFixture("org-1"));
  state.prices.mockResolvedValue({ organizationId: "org-1", offers: [] });
  state.write.mockReset();
});
afterEach(cleanup);
it("retains the exact allocation command after response loss and verifies only that operation", async () => {
  state.write
    .mockRejectedValueOnce(
      new MemberResourceError(502, "RESULT_UNVERIFIED", "unknown"),
    )
    .mockResolvedValueOnce({ netCredit: "0", debtRepaid: "0" });
  render(tree());
  await screen.findByText("张琳");
  await userEvent.click(screen.getByRole("button", { name: "分配资源" }));
  const dialog = screen.getByRole("dialog", { name: "分配资源" });
  await userEvent.type(
    within(dialog).getByRole("textbox", { name: /本次期数/ }),
    "1",
  );
  await userEvent.click(
    within(dialog).getByRole("button", { name: "确认分配" }),
  );
  await screen.findByText(/操作结果未确认/);
  await userEvent.click(screen.getByRole("button", { name: "核验原操作" }));
  expect(state.write).toHaveBeenCalledTimes(2);
  expect(state.write.mock.calls[1]).toEqual(state.write.mock.calls[0]);
});
it("keeps departed member holdings visible and allows only reclaim", async () => {
  state.read.mockResolvedValue({
    schemaVersion: "member-resource-directory-v1",
    organizationId: "org-1",
    observedAt: "2026-09-29T00:00:00Z",
    members: [{ ...member, state: "departed" }],
  });
  render(tree());
  await screen.findByText("张琳");
  expect(screen.getByRole("button", { name: "分配资源" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "回收资源" })).toBeEnabled();
  expect(screen.queryByText("未提供")).not.toBeInTheDocument();
});
