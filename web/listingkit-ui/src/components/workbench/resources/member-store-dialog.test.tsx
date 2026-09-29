import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { MemberStores } from "./member-store-dialog";
import {
  MemberResourceError,
  type MemberResourceEntry,
} from "@/lib/api/member-resources";
const state = vi.hoisted(() => ({ read: vi.fn(), write: vi.fn() }));
vi.mock("@/lib/api/member-resources", async (original) => ({
  ...(await original<typeof import("@/lib/api/member-resources")>()),
  getMemberStores: state.read,
  setMemberStoreGrant: state.write,
}));
const member = {
  memberId: "member",
  displayName: "保留成员",
  roles: [],
  state: "departed",
} as unknown as MemberResourceEntry;
const store = {
  id: "11111111-1111-4111-8111-11111111111a",
  name: "原授权店铺",
  grantVersion: "1",
  serviceExpiresAt: null,
};
function tree(org = "org-1") {
  return (
    <QueryClientProvider
      client={
        new QueryClient({ defaultOptions: { queries: { retry: false } } })
      }
    >
      <MemberStores
        member={member}
        scope={{ expectedUserId: "actor", expectedOrganizationId: org }}
        canManage
        onClose={vi.fn()}
        onChanged={vi.fn().mockResolvedValue(undefined)}
      />
    </QueryClientProvider>
  );
}
beforeEach(() => {
  sessionStorage.clear();
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
  state.read.mockResolvedValue({ items: [store], total: "1" });
  state.write.mockReset();
});
afterEach(cleanup);
it("restores a revoked grant command across reload without adopting a new grant version", async () => {
  state.write.mockRejectedValue(
    new MemberResourceError(502, "RESULT_UNVERIFIED", "unknown"),
  );
  const first = render(tree());
  await userEvent.click(
    await screen.findByRole("button", { name: "撤销 原授权店铺 授权" }),
  );
  await screen.findByText(/操作结果未确认/);
  const original = state.write.mock.calls[0];
  first.unmount();
  state.read.mockResolvedValue({
    items: [{ ...store, grantVersion: "2" }],
    total: "1",
  });
  render(tree());
  await userEvent.click(
    await screen.findByRole("button", { name: "核验原操作" }),
  );
  expect(state.write.mock.calls[1]).toEqual(original);
});
it("does not restore a pending grant into another organization", async () => {
  state.write.mockRejectedValue(
    new MemberResourceError(502, "RESULT_UNVERIFIED", "unknown"),
  );
  const first = render(tree());
  await userEvent.click(
    await screen.findByRole("button", { name: "撤销 原授权店铺 授权" }),
  );
  await screen.findByText(/操作结果未确认/);
  first.unmount();
  render(tree("org-2"));
  await screen.findByRole("button", { name: "撤销 原授权店铺 授权" });
  expect(
    screen.queryByRole("button", { name: "核验原操作" }),
  ).not.toBeInTheDocument();
  expect(state.write).toHaveBeenCalledTimes(1);
});
