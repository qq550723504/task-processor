import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuditPage, auditRowKey } from "./audit-page";

function stubAuditList(handler: (url: string, init: RequestInit) => Promise<Response>) {
  vi.stubGlobal("fetch", vi.fn((url, init) => String(url) === "/api/account/audit/summary"
    ? Promise.resolve(Response.json({ code: "SUMMARY_NOT_CONFIGURED", message: "", requestId: "", fieldErrors: [] }, { status: 503 }))
    : handler(String(url), init ?? {})));
}

const state = vi.hoisted(() => ({ context: { user: { id: "u1" }, effectiveOrganization: { id: "B" }, roles: ["listingkit_viewer"], isLoading: false, isSwitching: false, selectionRequired: false, error: null as { code: string } | null, blockingError: null as { code: string } | null } }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));
const reference = "0198d4f0-0000-7000-8000-000000000001";
const event = { eventType: "source_account.operation_committed", actor: "operator-B", time: "2026-09-12T00:00:00Z", objectType: "source_account", objectReference: reference, operation: "disable", result: "succeeded", relation: { type: "source_account_version", reference, version: "2" } };
const empty = { schemaVersion: "account-audit-v1", userId: "u1", effectiveOrganizationId: "B", source: "source_account_committed_operations", items: [], nextCursor: null };
const clients: QueryClient[] = [];
function mount() {
  const client = new QueryClient(); clients.push(client);
  const child = () => <QueryClientProvider client={client}><AuditPage expectedUserId="u1" /></QueryClientProvider>;
  const view = render(child()); return { ...view, update: () => view.rerender(child()) };
}
afterEach(() => { cleanup(); clients.splice(0).forEach(client => client.clear()); vi.unstubAllGlobals(); state.context = { user: { id: "u1" }, effectiveOrganization: { id: "B" }, roles: ["listingkit_viewer"], isLoading: false, isSwitching: false, selectionRequired: false, error: null, blockingError: null }; });
describe("audit page", () => {
  it("starts at thirty days and applies content, all-time and historical target ID to the full query", async () => {
    const calls: string[] = [];
    stubAuditList(async url => { calls.push(url); return Response.json(empty); });
    mount();
    await screen.findByRole("table", { name: "操作记录" });
    expect(new URL(calls[0], "http://localhost").searchParams.get("period")).toBe("30d");
    await userEvent.type(screen.getByRole("textbox", { name: /搜索操作内容/ }), "模型实际用量");
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "时间范围" }), "all");
    await userEvent.type(screen.getByRole("textbox", { name: /成员筛选/ }), "removed-member");
    await userEvent.click(screen.getByRole("button", { name: "应用筛选" }));
    await waitFor(() => expect(calls.length).toBeGreaterThan(1));
    const selected = new URL(calls.at(-1)!, "http://localhost");
    expect(selected.searchParams.get("query")).toBe("模型实际用量");
    expect(selected.searchParams.get("period")).toBe("all");
    expect(selected.searchParams.get("member")).toBe("removed-member");
  });
  it("shows the original member resource quantity and monthly cap in the audit table", async () => {
    const allocation = { eventType: "account_member_resource.changed", actor: "operator-B", time: "2026-09-29T08:00:00Z", objectType: "member_resource", objectReference: "member-1", operation: "allocate_member_resource", result: "succeeded", relation: { type: "organization_resource_operation", reference: "allocate-1", version: "1" }, resource: { type: "store_renewal_period", quantity: "1" } };
    const reclaim = { ...allocation, operation: "reclaim_member_resource", relation: { ...allocation.relation, reference: "reclaim-1", version: "2" }, resource: { type: "data_row", quantity: "100" } };
    const cap = { ...allocation, eventType: "account_member_ai_point_limit.changed", objectType: "member_ai_point_limit", operation: "set_member_ai_point_limit", relation: { ...allocation.relation, reference: "cap-1" }, resource: { type: "ai_point", quantity: "1000" } };
    stubAuditList(vi.fn().mockResolvedValue(Response.json({ ...empty, source: "source_account_committed_operations+member_resource_audit", items: [allocation, reclaim, cap] })));
    mount();
    const table = await screen.findByRole("table", { name: "操作记录" });
    expect(within(table).getByText("分配续费期数：1 期")).toBeVisible();
    expect(within(table).getByText("回收数据额度：100 条")).toBeVisible();
    expect(within(table).getByText("设置成员 AI 月度上限：1000 点/月")).toBeVisible();
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "操作类型" }), "reclaim_member_resource");
    await userEvent.click(screen.getByRole("button", { name: "应用筛选" }));
    await waitFor(() => expect(vi.mocked(fetch).mock.calls.at(-1)?.[0]).toContain("operation=reclaim_member_resource"));
  });
  it("shows image AI points separately from token consumption in the existing table", async () => {
    const debit = { eventType: "account_ai_points.committed", actor: "operator-B", time: "2026-09-26T01:00:00Z", objectType: "image_generation", objectReference: "run-1", operation: "consume", result: "succeeded", relation: { type: "organization_resource_event", reference, version: "" }, points: { memberId: "grant-1", quantity: "12", priceVersion: "price-1", intentId: "intent-1" } };
    stubAuditList(vi.fn().mockResolvedValue(Response.json({ ...empty, source: "source_account_committed_operations+image_ai_point_debits", items: [debit] })));
    mount();
    const table = await screen.findByRole("table", { name: "操作记录" });
    expect(within(table).getByText("图片 AI 点数已扣：12")).toBeVisible();
    expect(within(table).getByText("成员 grant-1 · 生成 run-1")).toBeVisible();
    expect(within(table).getByText("operator-B")).toBeVisible();
    expect(within(table).queryByText(/AI token 已结算/)).not.toBeInTheDocument();
  });
  it("shows the exact committed usage quantity, canonical member and invocation without an invented actor", async () => {
    const usage = { eventType: "ai_invocation.usage_observed", actor: "", time: "2026-09-25T01:00:00Z", objectType: "ai_invocation", objectReference: "inv-1", operation: "observe", result: "observed", relation: { type: "ai_invocation", reference, version: "" }, usage: { memberId: "grant-1", quantity: 7, metric: "model_tokens" } };
    stubAuditList(vi.fn().mockResolvedValue(Response.json({ ...empty, source: "source_account_committed_operations+ai_invocations", items: [usage] })));
    mount();
    const table = await screen.findByRole("table", { name: "操作记录" });
    expect(within(table).getByText("未记录操作人")).toBeVisible();
    expect(within(table).getByText("模型实际用量：7 Token")).toBeVisible();
    expect(within(table).getByText("成员 grant-1 · 调用 inv-1")).toBeVisible();
  });
  it("keeps audit row keys unique across relation types and actors", () => {
    const base = { eventType: "event", relation: { type: "relation", reference: "same", version: "1" } };
    expect(auditRowKey({ ...base, actor: "actor-a", eventType: "profile" })).not.toBe(auditRowKey({ ...base, actor: "actor-a", eventType: "resource" }));
    expect(auditRowKey({ ...base, actor: "actor-a", eventType: "membership" })).not.toBe(auditRowKey({ ...base, actor: "actor-b", eventType: "membership" }));
  });

  it("renders source facts and coverage without fake metrics or actions", async () => {
    stubAuditList(vi.fn().mockResolvedValue(Response.json({ ...empty, items: [event] })));
    mount(); const table = await screen.findByRole("table", { name: "操作记录" }); expect(table).toBeVisible();
    expect(within(table).getByText("operator-B")).toBeVisible(); expect(within(table).getByText("停用源账号")).toBeVisible();
    expect(screen.getByText("企业操作审计")).toBeVisible(); expect(screen.queryByText("86")).not.toBeInTheDocument();
    const metrics = screen.getByRole("region", { name: "审计汇总" });
    expect(within(metrics).getByText("近 30 天操作")).toBeVisible(); expect(await within(metrics).findAllByText("未配置")).toHaveLength(4);
    expect(screen.queryByRole("button", { name: /导出|邀请|续费/ })).not.toBeInTheDocument();
  });
  it("renders profile and membership audit facts", async () => {
    const profile = { eventType: "account_business_profile.updated", actor: "operator-B", time: "2026-09-12T00:00:00Z", objectType: "account_business_profile", objectReference: "u1", operation: "update", result: "succeeded", relation: { type: "account_business_profile_version", reference: "u1", version: "1" } };
    const member = { eventType: "organization_membership.changed", actor: "operator-B", time: "2026-09-11T00:00:00Z", objectType: "organization_member", objectReference: "member-1", operation: "role", result: "succeeded", relation: { type: "organization_membership_operation", reference: "00000000-0000-4000-8000-000000000001", version: "2" } };
    stubAuditList(vi.fn().mockResolvedValue(Response.json({ ...empty, source: "source_account_committed_operations+account_business_profile_audit+organization_member_audit", items: [profile, member] })));
    mount();
    const table = await screen.findByRole("table", { name: "操作记录" });
    expect(within(table).getByText("更新账户资料")).toBeVisible();
    expect(within(table).getByText("更新成员角色")).toBeVisible();
    expect(within(table).getByText("成员与权限")).toBeVisible();
  });
  it("distinguishes empty from dependency failure and allows retry", async () => {
    const fetch = vi.fn().mockResolvedValueOnce(Response.json({ code: "DEPENDENCY_UNAVAILABLE", message: "secret", requestId: "", fieldErrors: [] }, { status: 503 })).mockResolvedValue(Response.json(empty)); stubAuditList(fetch);
    mount(); expect(await screen.findByText("操作记录暂不可用")).toBeVisible(); expect(screen.queryByText("暂无操作记录")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "刷新记录" }));
    expect(await screen.findByText("当前范围内没有已提交的操作。")).toBeVisible(); expect(screen.queryByText("secret")).not.toBeInTheDocument();
    const table = screen.getByRole("table", { name: "操作记录" }); expect(table).toBeVisible();
    expect(within(table).getByRole("columnheader", { name: "时间" })).toBeVisible();
    expect(within(table).getByRole("columnheader", { name: "结果" })).toBeVisible();
  });
  it("cancels old organization requests and rejects late results", async () => {
    let release!: (response: Response) => void;
    const requests: RequestInit[] = [];
    stubAuditList(vi.fn((_url, init: RequestInit) => { requests.push(init); return requests.length === 1 ? new Promise<Response>(resolve => { release = resolve; }) : Promise.resolve(Response.json({ ...empty, effectiveOrganizationId: "C" })); }));
    const view = mount(); await waitFor(() => expect(requests).toHaveLength(1));
    state.context.isSwitching = true; view.update();
    expect(requests[0].signal?.aborted).toBe(true);
    state.context.isSwitching = false; state.context.effectiveOrganization = { id: "C" }; view.update();
    expect(await screen.findByText("当前范围内没有已提交的操作。")).toBeVisible();
    await act(async () => release(Response.json({ ...empty, items: [event] })));
    expect(screen.queryByText("operator-B")).not.toBeInTheDocument();
  });
  it.each(["AUTHENTICATION_REQUIRED", "PERMISSION_DENIED", "ORGANIZATION_ACCESS_REVOKED"])("clears visible facts when context reports %s", async code => {
    stubAuditList(vi.fn().mockResolvedValue(Response.json({ ...empty, items: [event] })));
    const view = mount(); await screen.findByText("operator-B");
    state.context.blockingError = { code }; view.update();
    expect(screen.queryByText("operator-B")).not.toBeInTheDocument(); expect(screen.getByRole("alert")).toBeVisible();
  });
});

const summary = { schemaVersion: "account-audit-summary-v1", userId: "u1", effectiveOrganizationId: "B", coverage: "current_account_audit_committed_events", window: { from: "2026-08-30T08:00:00Z", asOf: "2026-09-29T08:00:00Z" }, counts: { operations: "120", members: "3", permissions: "1", resources: "5" } };
describe("real audit summary", () => {
 it("keeps full totals while paging and filtering the existing list", async () => {
  const calls: string[] = [];
  vi.stubGlobal("fetch", vi.fn((url: string) => { calls.push(url); return Promise.resolve(Response.json(url === "/api/account/audit/summary" ? summary : { ...empty, items: [event], nextCursor: url.includes("cursor=") ? null : "YQ" })); }));
  mount();
  const metrics = await screen.findByRole("region", { name: "审计汇总" });
  expect(await within(metrics).findByText("120")).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "下一页" }));
  expect(await screen.findByText(/第 2 页/)).toBeVisible();
  await userEvent.selectOptions(screen.getByRole("combobox", { name: "操作类型" }), "role");
  await userEvent.click(screen.getByRole("button", { name: "应用筛选" }));
  expect(await screen.findByText(/第 1 页/)).toBeVisible();
  expect(within(metrics).getByText("120")).toBeVisible();
  expect(calls.filter(url => url === "/api/account/audit/summary")).toHaveLength(1);
 });
 it("shows true zero, then clears the page on revoked summary refresh", async () => {
  let revoked = false;
  vi.stubGlobal("fetch", vi.fn((url: string) => Promise.resolve(url === "/api/account/audit/summary" ? revoked ? Response.json({ code: "ORGANIZATION_ACCESS_REVOKED", message: "", requestId: "", fieldErrors: [] }, { status: 403 }) : Response.json({ ...summary, counts: { operations: "0", members: "0", permissions: "0", resources: "0" } }) : Response.json(empty))));
  mount();
  const metrics = await screen.findByRole("region", { name: "审计汇总" });
  await waitFor(() => expect(within(metrics).getAllByText("0")).toHaveLength(4));
  revoked = true; await userEvent.click(screen.getByRole("button", { name: "刷新记录" }));
  await screen.findByText("企业访问已撤销");
  expect(screen.queryByRole("region", { name: "审计汇总" })).not.toBeInTheDocument();
  expect(screen.queryByRole("table", { name: "操作记录" })).not.toBeInTheDocument();
 });
 it.each([
  [401, "AUTHENTICATION_REQUIRED"], [403, "PERMISSION_DENIED"],
  [403, "ORGANIZATION_ACCESS_DENIED"], [403, "ORGANIZATION_ACCESS_REVOKED"],
  [403, "ORGANIZATION_SUSPENDED"], [409, "IDENTITY_CONTEXT_CHANGED"],
  [409, "ORGANIZATION_CONTEXT_CHANGED"], [409, "ORGANIZATION_SELECTION_REQUIRED"],
 ])("clears successful totals when a list page rejects the scope with %s %s", async (status, code) => {
  vi.stubGlobal("fetch", vi.fn((url: string) => Promise.resolve(url === "/api/account/audit/summary"
   ? Response.json(summary) : url.includes("cursor=")
    ? Response.json({ code, message: "", requestId: "", fieldErrors: [] }, { status })
    : Response.json({ ...empty, items: [event], nextCursor: "YQ" }))));
  mount(); await screen.findByText("120");
  await userEvent.click(await screen.findByRole("button", { name: "下一页" }));
  await screen.findByRole("alert");
  await waitFor(() => expect(screen.queryByRole("region", { name: "审计汇总" })).not.toBeInTheDocument());
  expect(screen.queryByText("120")).not.toBeInTheDocument();
 });
 it("cancels a pending summary when the list rejects access and ignores its late result", async () => {
  let release!: (r: Response) => void; let signal: AbortSignal | undefined;
  vi.stubGlobal("fetch", vi.fn((url: string, init: RequestInit) => {
   if (url === "/api/account/audit/summary") { signal = init.signal as AbortSignal; return new Promise<Response>(resolve => { release = resolve; }); }
   return Promise.resolve(url.includes("cursor=")
    ? Response.json({ code: "ORGANIZATION_ACCESS_REVOKED", message: "", requestId: "", fieldErrors: [] }, { status: 403 })
    : Response.json({ ...empty, items: [event], nextCursor: "YQ" }));
  }));
  mount(); await userEvent.click(await screen.findByRole("button", { name: "下一页" }));
  await screen.findByText("企业访问已撤销");
  await waitFor(() => expect(signal?.aborted).toBe(true));
  await act(async () => release(Response.json(summary)));
  expect(screen.queryByText("120")).not.toBeInTheDocument();
  expect(screen.queryByRole("region", { name: "审计汇总" })).not.toBeInTheDocument();
 });
 it("keeps successful totals when only the list dependency fails", async () => {
  vi.stubGlobal("fetch", vi.fn((url: string) => Promise.resolve(url === "/api/account/audit/summary"
   ? Response.json(summary) : url.includes("cursor=")
    ? Response.json({ code: "DEPENDENCY_UNAVAILABLE", message: "", requestId: "", fieldErrors: [] }, { status: 503 })
    : Response.json({ ...empty, items: [event], nextCursor: "YQ" }))));
  mount(); await screen.findByText("120");
  await userEvent.click(await screen.findByRole("button", { name: "下一页" }));
  await screen.findByRole("alert");
  expect(within(screen.getByRole("region", { name: "审计汇总" })).getByText("120")).toBeVisible();
 });
 it("cancels a late summary across enterprise changes", async () => {
  let release!: (r: Response) => void; let signal: AbortSignal | undefined;
  vi.stubGlobal("fetch", vi.fn((url: string, init: RequestInit) => {
   if (url === "/api/account/audit/summary" && state.context.effectiveOrganization.id === "B") { signal = init.signal as AbortSignal; return new Promise<Response>(resolve => { release = resolve; }); }
   return Promise.resolve(Response.json(url === "/api/account/audit/summary" ? { ...summary, effectiveOrganizationId: "C", counts: { operations: "0", members: "0", permissions: "0", resources: "0" } } : { ...empty, effectiveOrganizationId: state.context.effectiveOrganization.id }));
  }));
  const view = mount(); await waitFor(() => expect(signal).toBeDefined());
  state.context.isSwitching = true; view.update(); expect(signal?.aborted).toBe(true);
  state.context.isSwitching = false; state.context.effectiveOrganization = { id: "C" }; view.update();
  await waitFor(() => expect(within(screen.getByRole("region", { name: "审计汇总" })).getAllByText("0")).toHaveLength(4));
  await act(async () => release(Response.json(summary)));
  expect(screen.queryByText("120")).not.toBeInTheDocument();
 });
});
