import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { SheinAuthorizationCallback } from "./shein-authorization-callback";
import { StoreConnectionError } from "@/lib/api/store-connection";
const state = vi.hoisted(() => ({
  context: { user: { id: "actor" }, effectiveOrganization: { id: "org-1" } },
  complete: vi.fn(),
  query: vi.fn(),
  clear: vi.fn(),
  pending: vi.fn(),
}));
vi.mock("@/components/providers/workbench-context-provider", () => ({
  useWorkbenchContext: () => state.context,
}));
vi.mock("@/lib/api/store-connection", async (original) => ({
  ...(await original<typeof import("@/lib/api/store-connection")>()),
  readStoreAuthorization: state.pending,
  completeStoreConnection: state.complete,
  queryStoreConnection: state.query,
  clearStoreAuthorization: state.clear,
}));
const pending = {
  expectedUserId: "actor",
  expectedOrganizationId: "org-1",
  storeId: "11111111-1111-4111-8111-11111111111a",
  attemptId: "22222222-2222-4222-8222-22222222222b",
  expiresAt: new Date(Date.now() + 300000).toISOString(),
};
beforeEach(() => {
  state.context = {
    user: { id: "actor" },
    effectiveOrganization: { id: "org-1" },
  };
  state.pending.mockReturnValue(pending);
  state.complete.mockReset();
  state.query.mockReset();
  state.clear.mockReset();
  window.history.replaceState(
    null,
    "",
    "/workbench/stores/shein/callback?appid=app-1&state=synthetic-state&tempToken=synthetic-temp",
  );
});
afterEach(cleanup);
it("cleans callback secrets, waits for matching context and exchanges only by explicit POST", async () => {
  state.context.user.id = "other";
  const { rerender } = render(<SheinAuthorizationCallback />);
  expect(window.location.search).toBe("");
  expect(state.complete).not.toHaveBeenCalled();
  expect(await screen.findByRole("button", { name: "完成官方连接" })).toBeDisabled();
  state.context.user.id = "actor";
  rerender(<SheinAuthorizationCallback />);
  state.complete.mockRejectedValueOnce(
    new StoreConnectionError(503, "STORE_AUTHORIZATION_OUTCOME_UNKNOWN", true),
  );
  await userEvent.click(screen.getByRole("button", { name: "完成官方连接" }));
  await screen.findByText(/临时凭据交换结果未知/);
  expect(
    screen.queryByRole("button", { name: "完成官方连接" }),
  ).not.toBeInTheDocument();
  state.query.mockResolvedValueOnce({ connectionStatus: "connected" });
  await userEvent.click(screen.getByRole("button", { name: "核验原连接" }));
  await screen.findByText(/官方连接已完成/);
  expect(state.complete).toHaveBeenCalledTimes(1);
  expect(state.query).toHaveBeenCalledWith(
    pending,
    pending.storeId,
    pending.attemptId,
  );
});
it("rejects duplicate provider fields before any exchange", async () => {
  window.history.replaceState(
    null,
    "",
    "/workbench/stores/shein/callback?appid=app-1&state=a&state=b&tempToken=temp",
  );
  render(<SheinAuthorizationCallback />);
  await screen.findByText(/本次授权返回无法确认或已过期/);
  expect(
    screen.queryByRole("button", { name: "完成官方连接" }),
  ).not.toBeInTheDocument();
  expect(state.complete).not.toHaveBeenCalled();
  expect(window.location.search).toBe("");
});
