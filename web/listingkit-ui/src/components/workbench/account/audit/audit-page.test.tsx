import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
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
    mount(); expect(await screen.findByRole("table", { name: "操作记录" })).toBeVisible();
    expect(screen.getByText("operator-B")).toBeVisible(); expect(screen.getByText("停用源账号")).toBeVisible();
    expect(screen.getByText(/源账号已提交操作/)).toBeVisible(); expect(screen.queryByText("86")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /导出|邀请|续费/ })).not.toBeInTheDocument();
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
