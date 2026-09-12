import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { AccountOrganization, AccountProfile } from "@/lib/api/account";
import { AccountPage } from "./account-page";

const state = vi.hoisted(() => ({ context: { user: { id: "u1" } as { id: string } | null, homeOrganizationId: "A", effectiveOrganization: { id: "B", name: "企业乙", roles: ["viewer"] } as { id: string; name: string; roles: string[] } | null, roles: ["viewer"], isLoading: false, isSwitching: false, selectionRequired: false, error: null as { code: string } | null, blockingError: null as { code: string } | null } }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));
const profile: AccountProfile = { schemaVersion: "account-v1", userId: "u1", homeOrganizationId: "A", displayName: "本人甲", email: null, emailVerified: null, phoneNumber: "+8613800000000", phoneNumberVerified: false, source: "zitadel_userinfo", readAt: "2026-09-07T01:00:00Z" };
const organization: AccountOrganization = { schemaVersion: "account-v1", userId: "u1", homeOrganizationId: "A", effectiveOrganizationId: "B", name: "企业乙", roles: ["viewer"], source: "zitadel_project_authorizations", readAt: profile.readAt, authorizationMaxAgeSeconds: 60 };
const clients: QueryClient[] = [];
function mount(page: "profile" | "organization" = "profile", expectedUserId = "u1") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); clients.push(client);
  const child = (id = expectedUserId) => <QueryClientProvider client={client}><AccountPage page={page} expectedUserId={id} /></QueryClientProvider>;
  const view = render(child()); return { ...view, update: (id = expectedUserId) => view.rerender(child(id)) };
}
afterEach(() => { cleanup(); clients.splice(0).forEach(c => c.clear()); vi.unstubAllGlobals(); state.context = { user: { id: "u1" }, homeOrganizationId: "A", effectiveOrganization: { id: "B", name: "企业乙", roles: ["viewer"] }, roles: ["viewer"], isLoading: false, isSwitching: false, selectionRequired: false, error: null, blockingError: null }; });
describe("AccountPage read-only projection", () => {
  it("links the available resource page without claiming balances or activating other management cards", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(organization))); mount("organization");
    expect(await screen.findByRole("link", { name: "查看资源与额度" })).toHaveAttribute("href", "/workbench/account/organization/resources");
    expect(screen.getByText("资源余额读取暂未接入；已授予权益、已记录用量与源账号管理请进入资源与额度。")).toBeVisible();
    expect(screen.getAllByText("暂未接入")).toHaveLength(2);
    expect(screen.queryByRole("link", { name: /成员与权限|操作记录/ })).not.toBeInTheDocument();
  });
  it("offers an account return link in the breadcrumb", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(profile))); mount();
    expect(await screen.findByRole("heading", { name: "本人甲" })).toBeVisible();
    expect(screen.getByRole("link", { name: "我的账户" })).toHaveAttribute("href", "/workbench/account");
  });
  it("distinguishes undisclosed optional claims from a failed read", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...profile, displayName: null, phoneNumber: null, phoneNumberVerified: null }))); mount();
    expect(await screen.findByRole("heading", { name: "暂未提供个人资料" })).toBeVisible();
    expect(screen.getByText("账户 ID：u1")).toBeVisible();
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });
  it("offers login recovery for an expired profile session", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code: "AUTHENTICATION_REQUIRED", message: "", requestId: "", fieldErrors: [] }, { status: 401 }))); mount();
    expect(await screen.findByRole("link", { name: "重新登录" })).toHaveAttribute("href", "/login?returnTo=%2Fworkbench%2Faccount%2Fprofile");
  });
  it("retries a dependency failure only after user action and rereads facts", async () => {
    const fetcher = vi.fn().mockResolvedValueOnce(Response.json({ code: "DEPENDENCY_UNAVAILABLE", message: "", requestId: "", fieldErrors: [] }, { status: 503 })).mockImplementation(() => Promise.resolve(Response.json(profile)));
    vi.stubGlobal("fetch", fetcher); mount();
    expect(await screen.findByRole("alert")).toBeVisible(); expect(fetcher).toHaveBeenCalledTimes(1);
    await userEvent.click(screen.getByRole("button", { name: "刷新资料" }));
    expect(await screen.findByRole("heading", { name: "本人甲" })).toBeVisible(); expect(fetcher).toHaveBeenCalledTimes(2);
  });
  it.each(["no-org", "selection", "grant-error", "loading"])("reads the actual profile client independent of %s", async mode => {
    state.context.effectiveOrganization = null; state.context.user = null;
    state.context.selectionRequired = mode === "selection"; state.context.isLoading = mode === "loading";
    state.context.error = mode === "grant-error" ? { code: "DEPENDENCY_UNAVAILABLE" } : null;
    const fetcher = vi.fn().mockResolvedValue(Response.json(profile)); vi.stubGlobal("fetch", fetcher); mount();
    expect(await screen.findByRole("heading", { name: "本人甲" })).toBeVisible();
    expect(fetcher).toHaveBeenCalledTimes(1); expect(fetcher.mock.calls[0][0]).toBe("/api/account/profile");
    expect(new Headers(fetcher.mock.calls[0][1].headers).get("X-Expected-User-ID")).toBe("u1");
    expect(screen.getByText("未验证")).toBeVisible(); expect(screen.queryByText("已实名认证")).not.toBeInTheDocument();
    expect(screen.queryByText("未绑定")).not.toBeInTheDocument(); expect(screen.queryByRole("button", { name: /保存|修改|认证/ })).not.toBeInTheDocument();
  });
  it("keeps Home and Effective and project roles separate, with no fabricated commercial facts", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(organization))); mount("organization");
    expect(await screen.findByRole("heading", { name: "企业乙" })).toBeVisible();
    expect(screen.getByText("归属企业（Home）：A")).toBeVisible(); expect(screen.getByText("当前有效企业：B")).toBeVisible();
    expect(screen.getByText("viewer")).toBeVisible(); expect(screen.queryByText("8,650")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /管理成员|管理资源|邀请/ })).not.toBeInTheDocument();
  });
  it.each(["AUTHENTICATION_REQUIRED", "IDENTITY_CONTEXT_CHANGED", "ACCOUNT_NOT_CONFIGURED", "DEPENDENCY_UNAVAILABLE", "DEADLINE_EXCEEDED", "PERMISSION_DENIED", "ORGANIZATION_ACCESS_REVOKED", "unexpected"])("shows a safe %s state without data or raw error", async code => {
    const status = code === "AUTHENTICATION_REQUIRED" ? 401 : code === "IDENTITY_CONTEXT_CHANGED" ? 409 : code === "DEADLINE_EXCEEDED" ? 504 : code.includes("PERMISSION") || code.includes("REVOKED") ? 403 : 503;
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code, message: "private-secret", requestId: "", fieldErrors: [] }, { status }))); mount();
    expect(await screen.findByRole("alert")).toBeVisible(); expect(screen.queryByText("private-secret")).not.toBeInTheDocument(); expect(screen.queryByText("本人甲")).not.toBeInTheDocument();
  });
  it("cancels old org reads and ignores late success during switch", async () => {
    let release!: (r: Response) => void; const late = new Promise<Response>(r => { release = r; });
    const requests: RequestInit[] = []; vi.stubGlobal("fetch", vi.fn((_url, init: RequestInit) => { requests.push(init); return requests.length === 1 ? late : Promise.resolve(Response.json({ ...organization, effectiveOrganizationId: "C", name: "企业丙" })); }));
    const view = mount("organization"); await waitFor(() => expect(requests).toHaveLength(1));
    state.context.isSwitching = true; view.update(); expect(requests[0].signal?.aborted).toBe(true);
    state.context.isSwitching = false; state.context.effectiveOrganization = { id: "C", name: "企业丙", roles: ["viewer"] }; view.update();
    expect(await screen.findByRole("heading", { name: "企业丙" })).toBeVisible(); await act(async () => release(Response.json(organization)));
    expect(screen.queryByText("企业乙")).not.toBeInTheDocument(); expect(screen.queryByText("读取超时")).not.toBeInTheDocument();
  });
  it("clears profile immediately for a different known subject and ignores late response", async () => {
    let release!: (r: Response) => void; let signal: AbortSignal | undefined | null;
    vi.stubGlobal("fetch", vi.fn((_url, init: RequestInit) => { signal = init.signal; return new Promise<Response>(r => { release = r; }); }));
    const view = mount(); await waitFor(() => expect(signal).toBeDefined()); state.context.user = { id: "u2" }; view.update();
    expect(signal?.aborted).toBe(true); await act(async () => release(Response.json(profile))); expect(screen.queryByText("本人甲")).not.toBeInTheDocument(); expect(screen.getByRole("alert")).toBeVisible();
  });
  it("ignores late failed reads after an enterprise transition", async () => {
    let release!: (r: Response) => void;
    vi.stubGlobal("fetch", vi.fn().mockImplementationOnce(() => new Promise<Response>(r => { release = r; })).mockImplementation(() => Promise.resolve(Response.json({ ...organization, effectiveOrganizationId: "C", name: "企业丙" }))));
    const view = mount("organization"); await waitFor(() => expect(release).toBeDefined());
    state.context.effectiveOrganization = { id: "C", name: "企业丙", roles: ["viewer"] }; view.update();
    expect(await screen.findByRole("heading", { name: "企业丙" })).toBeVisible();
    await act(async () => release(Response.json({ code: "DEPENDENCY_UNAVAILABLE", message: "private", requestId: "", fieldErrors: [] }, { status: 503 })));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument(); expect(screen.getByRole("heading", { name: "企业丙" })).toBeVisible();
  });
  it.each([true, false, null])("renders nullable verification without inventing binding state: %s", async verified => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...profile, phoneNumberVerified: verified }))); mount();
    expect(await screen.findByRole("heading", { name: "本人甲" })).toBeVisible();
    const field = screen.getByText("手机验证").parentElement!;
    expect(field).toHaveTextContent(verified === true ? "已验证" : verified === false ? "未验证" : "未提供");
    expect(screen.queryByText("未绑定")).not.toBeInTheDocument();
  });
  it("clears visible profile on logout click and does not replay requests", async () => {
    const fetcher = vi.fn().mockImplementation(() => Promise.resolve(Response.json(profile))); vi.stubGlobal("fetch", fetcher);
    mount(); expect(await screen.findByRole("heading", { name: "本人甲" })).toBeVisible();
    const link = document.createElement("a"); link.href = "/api/zitadel-auth/logout"; link.textContent = "退出"; link.onclick = e => e.preventDefault(); document.body.append(link);
    await userEvent.click(link); expect(screen.queryByText("本人甲")).not.toBeInTheDocument(); expect(fetcher).toHaveBeenCalledTimes(1); link.remove();
  });
  it("clears visible profile and reauthorizes when role context changes", async () => {
    const fetcher = vi.fn().mockImplementationOnce(() => Promise.resolve(Response.json(profile))).mockImplementationOnce(() => Promise.resolve(Response.json({ code: "AUTHENTICATION_REQUIRED", message: "hidden", requestId: "", fieldErrors: [] }, { status: 401 }))); vi.stubGlobal("fetch", fetcher);
    const view = mount(); expect(await screen.findByRole("heading", { name: "本人甲" })).toBeVisible(); state.context.roles = []; view.update();
    expect(screen.queryByText("本人甲")).not.toBeInTheDocument(); expect(await screen.findByRole("alert")).toBeVisible();
  });
});
