import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import { AuditPage } from "./audit-page";

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
  it("renders source facts and coverage without fake metrics or actions", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...empty, items: [event] })));
    mount(); const table = await screen.findByRole("table", { name: "操作记录" }); expect(table).toBeVisible();
    expect(within(table).getByText("operator-B")).toBeVisible(); expect(within(table).getByText("停用源账号")).toBeVisible();
    expect(screen.getByText(/源账号已提交操作/)).toBeVisible(); expect(screen.queryByText("86")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /导出|邀请|续费/ })).not.toBeInTheDocument();
  });
  it("renders profile and membership audit facts", async () => {
    const profile = { eventType: "account_business_profile.updated", actor: "operator-B", time: "2026-09-12T00:00:00Z", objectType: "account_business_profile", objectReference: "u1", operation: "update", result: "succeeded", relation: { type: "account_business_profile_version", reference: "u1", version: "1" } };
    const member = { eventType: "organization_membership.changed", actor: "operator-B", time: "2026-09-11T00:00:00Z", objectType: "organization_member", objectReference: "member-1", operation: "role", result: "succeeded", relation: { type: "organization_member_version", reference: "member-1", version: "2" } };
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...empty, source: "source_account_committed_operations+account_business_profile_audit+organization_member_audit", items: [profile, member] })));
    mount();
    const table = await screen.findByRole("table", { name: "操作记录" });
    expect(within(table).getByText("更新账户资料")).toBeVisible();
    expect(within(table).getByText("更新成员角色")).toBeVisible();
    expect(within(table).getByText("成员与权限")).toBeVisible();
  });
  it("distinguishes empty from dependency failure and allows retry", async () => {
    const fetch = vi.fn().mockResolvedValueOnce(Response.json({ code: "DEPENDENCY_UNAVAILABLE", message: "secret", requestId: "", fieldErrors: [] }, { status: 503 })).mockResolvedValue(Response.json(empty)); vi.stubGlobal("fetch", fetch);
    mount(); expect(await screen.findByText("操作记录暂不可用")).toBeVisible(); expect(screen.queryByText("暂无操作记录")).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "刷新记录" }));
    expect(await screen.findByText("暂无操作记录")).toBeVisible(); expect(screen.queryByText("secret")).not.toBeInTheDocument();
  });
  it("cancels old organization requests and rejects late results", async () => {
    let release!: (response: Response) => void;
    const requests: RequestInit[] = [];
    vi.stubGlobal("fetch", vi.fn((_url, init: RequestInit) => { requests.push(init); return requests.length === 1 ? new Promise<Response>(resolve => { release = resolve; }) : Promise.resolve(Response.json({ ...empty, effectiveOrganizationId: "C" })); }));
    const view = mount(); await waitFor(() => expect(requests).toHaveLength(1));
    state.context.isSwitching = true; view.update();
    expect(requests[0].signal?.aborted).toBe(true);
    state.context.isSwitching = false; state.context.effectiveOrganization = { id: "C" }; view.update();
    expect(await screen.findByText("暂无操作记录")).toBeVisible();
    await act(async () => release(Response.json({ ...empty, items: [event] })));
    expect(screen.queryByText("operator-B")).not.toBeInTheDocument();
  });
  it.each(["AUTHENTICATION_REQUIRED", "PERMISSION_DENIED", "ORGANIZATION_ACCESS_REVOKED"])("clears visible facts when context reports %s", async code => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...empty, items: [event] })));
    const view = mount(); await screen.findByText("operator-B");
    state.context.blockingError = { code }; view.update();
    expect(screen.queryByText("operator-B")).not.toBeInTheDocument(); expect(screen.getByRole("alert")).toBeVisible();
  });
});
