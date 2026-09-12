import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { SourceAccountAPIError, type SourceAccount } from "@/lib/api/source-accounts";
import { SourceAccountsPage } from "./source-accounts-page";

const state = vi.hoisted(() => ({ context: {} as Record<string, unknown>, list: vi.fn(), detail: vi.fn(), create: vi.fn(), enable: vi.fn(), disable: vi.fn() }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));
vi.mock("@/lib/api/source-accounts", async original => ({ ...await original<typeof import("@/lib/api/source-accounts")>(), listSourceAccounts: state.list, getSourceAccount: state.detail, createSourceAccount: state.create, enableSourceAccount: state.enable, disableSourceAccount: state.disable }));
const account: SourceAccount = { id: "01991e24-1001-7001-8001-000000000001", displayName: "企业乙源账号", platform: "1688", managementStatus: "enabled", connectionStatus: "pending_connection", version: "1", createdAt: "2026-09-12T00:00:00Z", updatedAt: "2026-09-12T00:00:00Z" };
const detail = (row = account) => ({ schemaVersion: 1, account: row, etag: `"${row.version}"` });
const failure = (code: string, outcome: "rejected" | "unknown" = "rejected") => new SourceAccountAPIError(503, code, "private token", "", null, outcome);
let client: QueryClient;
const tree = () => <QueryClientProvider client={client}><SourceAccountsPage /></QueryClientProvider>;
beforeEach(() => {
  state.context = { user: { id: "actor" }, effectiveOrganization: { id: "org-B", name: "企业乙" }, roles: ["listingkit_operator"], retry: vi.fn() };
  state.list.mockReset().mockResolvedValue({ schemaVersion: 1, items: [account], nextCursor: null });
  state.detail.mockReset().mockResolvedValue(detail());
  state.create.mockReset().mockResolvedValue({ ...detail(), replayed: false });
  state.enable.mockReset().mockResolvedValue({ ...detail(), replayed: false });
  state.disable.mockReset().mockResolvedValue({ ...detail(), replayed: false });
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});
afterEach(() => { cleanup(); client.clear(); });
async function openDetail() { await userEvent.click(await screen.findByRole("button", { name: "查看 企业乙源账号" })); await screen.findByRole("heading", { name: "源账号详情" }); }
async function register() { await userEvent.type(screen.getByLabelText("显示名称"), "新源账号"); await userEvent.click(screen.getByRole("button", { name: "登记源账号" })); }

it("lists current optional resources with separate management and connection facts", async () => {
  render(tree()); await screen.findByText("企业乙源账号");
  expect(screen.getByText("已启用")).toBeVisible(); expect(screen.getByText("待连接（未验证）")).toBeVisible();
  expect(screen.getByText(/匿名公开商品采集无需/)).toBeVisible();
  expect(screen.queryByRole("button", { name: /登录1688|连接1688/ })).not.toBeInTheDocument();
  expect(state.list).toHaveBeenCalledWith(expect.objectContaining({ expectedOrganizationId: "org-B", limit: 20, signal: expect.any(AbortSignal) }));
});

it("registers only on explicit submission then rereads server facts", async () => {
  render(tree()); await screen.findByText("企业乙源账号"); expect(state.create).not.toHaveBeenCalled();
  await register(); await waitFor(() => expect(state.detail).toHaveBeenCalled());
  expect(state.create).toHaveBeenCalledTimes(1);
  expect(state.create).toHaveBeenCalledWith(expect.objectContaining({ displayName: "新源账号", platform: "1688", expectedOrganizationId: "org-B", expectedActorSubject: "actor", idempotencyKey: expect.any(String) }));
  expect(await screen.findByRole("heading", { name: "源账号详情" })).toBeVisible();
});

it("uses exact server ETag and rereads instead of trusting the mutation payload", async () => {
  render(tree()); await openDetail();
  state.detail.mockResolvedValue(detail({ ...account, managementStatus: "disabled", version: "4" }));
  await userEvent.click(screen.getByRole("button", { name: "禁用源账号" }));
  await screen.findByText("4");
  expect(state.disable).toHaveBeenCalledWith(expect.objectContaining({ ifMatch: '"1"', sourceAccountId: account.id, expectedActorSubject: "actor", expectedOrganizationId: "org-B" }));
  expect(screen.getByRole("button", { name: "启用源账号" })).toBeVisible();
});

it("preserves an unknown operation and verifies only by an explicit same-identity submission", async () => {
  state.create.mockRejectedValueOnce(failure("OUTCOME_UNKNOWN", "unknown")).mockResolvedValue({ ...detail(), replayed: true });
  const view = render(tree()); await screen.findByText("企业乙源账号"); await register();
  expect(await screen.findByRole("alert")).toHaveTextContent("结果待核实");
  const intent = state.create.mock.calls[0][0];
  view.rerender(tree()); expect(state.create).toHaveBeenCalledTimes(1);
  expect(screen.getByRole("button", { name: "登记源账号" })).toBeDisabled();
  await userEvent.click(screen.getByRole("button", { name: "使用原请求重试" }));
  await waitFor(() => expect(state.create).toHaveBeenCalledTimes(2));
  expect(state.create.mock.calls[1][0]).toMatchObject({ idempotencyKey: intent.idempotencyKey, displayName: intent.displayName, expectedActorSubject: "actor", expectedOrganizationId: "org-B" });
});

it.each(["VERSION_CONFLICT", "PERMISSION_DENIED", "ORGANIZATION_ACCESS_REVOKED", "DEPENDENCY_UNAVAILABLE"])("projects rejected %s safely without retrying mutation", async code => {
  state.disable.mockRejectedValue(failure(code)); render(tree()); await openDetail();
  await userEvent.click(screen.getByRole("button", { name: "禁用源账号" }));
  expect(await screen.findByRole("alert")).toBeVisible(); expect(state.disable).toHaveBeenCalledTimes(1);
  expect(screen.queryByText(/private token/)).not.toBeInTheDocument();
});

it("keeps the original uncertainty when a verification request is rejected before obtaining its receipt", async () => {
  state.create.mockRejectedValueOnce(failure("OUTCOME_UNKNOWN", "unknown")).mockRejectedValue(failure("DEPENDENCY_UNAVAILABLE"));
  render(tree()); await screen.findByText("企业乙源账号"); await register(); await screen.findByText("结果待核实");
  await userEvent.click(screen.getByRole("button", { name: "使用原请求重试" }));
  await waitFor(() => expect(state.create).toHaveBeenCalledTimes(2));
  expect(screen.getByRole("button", { name: "登记源账号" })).toBeDisabled();
  expect(screen.getByRole("button", { name: "使用原请求重试" })).toBeVisible();
});

it.each(["enable", "disable"] as const)("retains exact %s request after response loss and a not-sent retry, then accepts replay", async action => {
  const fn = state[action];
  state.detail.mockResolvedValue(detail({ ...account, managementStatus: action === "enable" ? "disabled" : "enabled" }));
  fn.mockRejectedValueOnce(failure("OUTCOME_UNKNOWN", "unknown"))
    .mockRejectedValueOnce(new SourceAccountAPIError(0, "INVALID_REQUEST", "pre-cancel", "", null, "not_sent"))
    .mockResolvedValue({ ...detail(), replayed: true });
  render(tree()); await openDetail();
  await userEvent.click(screen.getByRole("button", { name: action === "enable" ? "启用源账号" : "禁用源账号" }));
  await screen.findByText("结果待核实"); const first = fn.mock.calls[0][0];
  await userEvent.click(screen.getByRole("button", { name: "使用原请求重试" }));
  await waitFor(() => expect(fn).toHaveBeenCalledTimes(2));
  expect(screen.getByRole("button", { name: "登记源账号" })).toBeDisabled();
  await userEvent.click(screen.getByRole("button", { name: "使用原请求重试" }));
  await waitFor(() => expect(fn).toHaveBeenCalledTimes(3));
  for (const call of fn.mock.calls) expect(call[0]).toMatchObject({ idempotencyKey: first.idempotencyKey, ifMatch: '"1"', sourceAccountId: account.id, expectedActorSubject: "actor", expectedOrganizationId: "org-B" });
});

it("allows viewer reads but hides manage controls", async () => {
  state.context.roles = ["listingkit_viewer"]; render(tree()); await openDetail();
  expect(screen.queryByRole("button", { name: "登记源账号" })).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "禁用源账号" })).not.toBeInTheDocument();
});

it("removes all old account facts and affordances when a mutation proves access revoked", async () => {
  state.disable.mockRejectedValue(failure("ORGANIZATION_ACCESS_REVOKED"));
  render(tree()); await openDetail();
  await userEvent.click(screen.getByRole("button", { name: "禁用源账号" }));
  await screen.findByRole("alert");
  expect(screen.queryByText("企业乙源账号")).not.toBeInTheDocument();
  expect(screen.queryByLabelText("显示名称")).not.toBeInTheDocument();
});

it("paginates with server cursor without claiming a global account count", async () => {
  state.list.mockResolvedValueOnce({ schemaVersion: 1, items: [account], nextCursor: "YQ" }).mockResolvedValue({ schemaVersion: 1, items: [{ ...account, displayName: "下一页账号" }], nextCursor: null });
  render(tree()); await screen.findByText("企业乙源账号");
  await userEvent.click(screen.getByRole("button", { name: "下一页" }));
  await screen.findByText("下一页账号");
  expect(state.list).toHaveBeenLastCalledWith(expect.objectContaining({ cursor: "YQ", limit: 20, expectedOrganizationId: "org-B" }));
  expect(screen.queryByText("企业乙源账号")).not.toBeInTheDocument();
});

it("separates unavailable from a successful empty list and retries reads", async () => {
  state.list.mockRejectedValueOnce(failure("DEPENDENCY_UNAVAILABLE")).mockResolvedValue({ schemaVersion: 1, items: [], nextCursor: null });
  render(tree()); await screen.findByRole("alert"); expect(screen.queryByText("尚未登记源账号")).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole("button", { name: "刷新列表" })); await screen.findByText("尚未登记源账号");
});

it.each(["organization", "actor", "roles", "switching", "logout", "revoke"])("clears details and pending intent on %s", async kind => {
  state.create.mockRejectedValue(failure("OUTCOME_UNKNOWN", "unknown"));
  const view = render(tree()); await openDetail(); await register(); await screen.findByText("结果待核实");
  state.list.mockReturnValue(new Promise(() => {}));
  if (kind === "organization") state.context.effectiveOrganization = { id: "org-C", name: "企业丙" };
  if (kind === "actor") state.context.user = { id: "other" };
  if (kind === "roles") state.context.roles = ["listingkit_viewer"];
  if (kind === "switching") state.context.isSwitching = true;
  if (kind === "logout") state.context.user = null;
  if (kind === "revoke") state.context.blockingError = { code: "ORGANIZATION_ACCESS_REVOKED" };
  view.rerender(tree());
  expect(screen.queryByText("企业乙源账号")).not.toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "使用原请求重试" })).not.toBeInTheDocument();
  expect(state.create).toHaveBeenCalledTimes(1);
});

it("ignores a late mutation from another organization", async () => {
  let finish!: (value: unknown) => void;
  state.create.mockReturnValue(new Promise(resolve => { finish = resolve; }));
  const view = render(tree()); await screen.findByText("企业乙源账号"); await register();
  state.context.effectiveOrganization = { id: "org-C", name: "企业丙" };
  state.list.mockResolvedValue({ schemaVersion: 1, items: [], nextCursor: null }); view.rerender(tree());
  await screen.findByText("尚未登记源账号");
  await act(async () => finish({ ...detail(), replayed: false }));
  expect(state.detail).not.toHaveBeenCalled();
  expect(screen.queryByRole("heading", { name: "源账号详情" })).not.toBeInTheDocument();
});

it("hides original intent in another organization and permits exact retry only after restoring its scope", async () => {
  state.create.mockRejectedValueOnce(failure("OUTCOME_UNKNOWN", "unknown")).mockResolvedValue({ ...detail(), replayed: true });
  const view = render(tree()); await screen.findByText("企业乙源账号"); await register(); await screen.findByText("结果待核实");
  const first = state.create.mock.calls[0][0];
  state.context.effectiveOrganization = { id: "org-C", name: "企业丙" }; view.rerender(tree());
  expect(screen.queryByRole("button", { name: "使用原请求重试" })).not.toBeInTheDocument();
  state.context.effectiveOrganization = { id: "org-B", name: "企业乙" }; view.rerender(tree());
  await userEvent.click(await screen.findByRole("button", { name: "使用原请求重试" }));
  await waitFor(() => expect(state.create).toHaveBeenCalledTimes(2));
  expect(state.create.mock.calls[1][0]).toMatchObject({ idempotencyKey: first.idempotencyKey, displayName: first.displayName, expectedActorSubject: "actor", expectedOrganizationId: "org-B" });
});
