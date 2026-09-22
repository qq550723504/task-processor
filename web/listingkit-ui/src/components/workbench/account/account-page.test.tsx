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
function mount(page: "profile" | "profile-settings" | "profile-business" | "profile-verification" | "organization" = "profile", expectedUserId = "u1") {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } }); clients.push(client);
  const child = (id = expectedUserId) => <QueryClientProvider client={client}><AccountPage page={page} expectedUserId={id} /></QueryClientProvider>;
  const view = render(child()); return { ...view, update: (id = expectedUserId) => view.rerender(child(id)) };
}
afterEach(() => { cleanup(); clients.splice(0).forEach(c => c.clear()); vi.unstubAllGlobals(); state.context = { user: { id: "u1" }, homeOrganizationId: "A", effectiveOrganization: { id: "B", name: "企业乙", roles: ["viewer"] }, roles: ["viewer"], isLoading: false, isSwitching: false, selectionRequired: false, error: null, blockingError: null }; });
describe("AccountPage read-only projection", () => {
  it("shows all three entry cards without turning links into management authority", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(organization))); mount("organization");
    expect(await screen.findByRole("link", { name: "查看成员与权限" })).toHaveAttribute("href", "/workbench/account/organization/members");
    expect(screen.getByRole("link", { name: "查看资源与额度" })).toHaveAttribute("href", "/workbench/account/organization/resources");
    expect(screen.getByRole("link", { name: "查看操作记录" })).toHaveAttribute("href", "/workbench/account/organization/audit");
    expect(screen.getByText("可用操作以当前企业权限为准。")).toBeVisible();
    expect(screen.getByText("只展示已提交成功的业务事件；失败尝试及未提交的 provider 操作不纳入。")).toBeVisible();
    expect(screen.queryByRole("button", { name: /邀请|移除/ })).not.toBeInTheDocument();
  });
  it("links members with permission-qualified wording alongside delivered sibling cards", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(organization))); mount("organization");
    expect(await screen.findByRole("link", { name: "查看成员与权限" })).toHaveAttribute("href", "/workbench/account/organization/members");
    expect(screen.getByText("查看企业成员；获准管理员可邀请成员、调整角色和移除成员")).toBeVisible();
    expect(screen.getByText("可用操作以当前企业权限为准。")).toBeVisible();
    expect(screen.queryByText("暂未接入")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "查看资源与额度" })).toBeVisible();
    expect(screen.getByRole("link", { name: "查看操作记录" })).toBeVisible();
    expect(screen.queryByRole("button", { name: /邀请|移除/ })).not.toBeInTheDocument();
  });
  it("keeps resources and bounded audit cards available together alongside the permission-qualified member entry", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(organization))); mount("organization");
    expect(await screen.findByRole("link", { name: "查看操作记录" })).toHaveAttribute("href", "/workbench/account/organization/audit");
    expect(screen.getByRole("link", { name: "查看资源与额度" })).toHaveAttribute("href", "/workbench/account/organization/resources");
    expect(screen.getByText("只展示已提交成功的业务事件；失败尝试及未提交的 provider 操作不纳入。")).toBeVisible();
    expect(screen.getByText("企业额度与已记录用量进入资源与额度查看；成员 Token 分配使用当前企业 entitlement window。")).toBeVisible();
    expect(screen.queryByText("暂未接入")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "查看成员与权限" })).toBeVisible();
  });
  it("links the delivered audit slice with its bounded scope and retains the member entry", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(organization))); mount("organization");
    expect(await screen.findByRole("link", { name: "查看操作记录" })).toHaveAttribute("href", "/workbench/account/organization/audit");
    expect(screen.getByText("查看源账号、成员额度、经营画像和成员权限的已提交成功记录")).toBeVisible();
    expect(screen.getByText("只展示已提交成功的业务事件；失败尝试及未提交的 provider 操作不纳入。")).toBeVisible();
    expect(screen.queryByText("暂未接入")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "查看成员与权限" })).toBeVisible();
  });
  it("links the available resource page without claiming balances", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(organization))); mount("organization");
    expect(await screen.findByRole("link", { name: "查看资源与额度" })).toHaveAttribute("href", "/workbench/account/organization/resources");
    expect(screen.getByText("企业额度与已记录用量进入资源与额度查看；成员 Token 分配使用当前企业 entitlement window。")).toBeVisible();
    expect(screen.queryByText("暂未接入")).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "查看成员与权限" })).toBeVisible();
  });
  it("offers an account return link in the breadcrumb", async () => {
    const fetcher = vi.fn().mockResolvedValue(Response.json(profile)); vi.stubGlobal("fetch", fetcher); mount();
    expect(await screen.findByRole("heading", { name: "本人甲" })).toBeVisible();
    const businessCall = fetcher.mock.calls.find(([url]) => url === "/api/account/business-profile");
    expect(businessCall).toBeDefined();
    expect(new Headers(businessCall?.[1].headers).get("X-Expected-Organization-ID")).toBe("B");
    expect(screen.getByRole("link", { name: "我的账户" })).toHaveAttribute("href", "/workbench/account");
  });
  it.each(["profile-settings", "profile-verification"] as const)("does not read the business profile on the %s leaf", async page => {
    const identity = { schemaVersion: "account-identity-profile-v1", userId: "u1", firstName: "本人", lastName: "甲", nickName: "", displayName: "本人甲", preferredLanguage: "", gender: "", source: "zitadel_auth_v1" };
    const fetcher = vi.fn((url: string) => Promise.resolve(url === "/api/account/profile" ? Response.json(profile) : url === "/api/account/organization" ? Response.json(organization) : url === "/api/account/identity/profile" ? Response.json(identity) : Response.json({ code: "DEPENDENCY_UNAVAILABLE", message: "", requestId: "", fieldErrors: [] }, { status: 503 })));
    vi.stubGlobal("fetch", fetcher);
    mount(page);
    expect(await screen.findByRole("heading", { name: "本人甲" })).toBeVisible();
    await waitFor(() => expect(fetcher).toHaveBeenCalled());
    expect(fetcher.mock.calls.some(([url]) => url === "/api/account/business-profile")).toBe(false);
  });
  it("requires a selected organization before showing verification authorization", async () => {
    state.context.effectiveOrganization = null;
    vi.stubGlobal("fetch", vi.fn());
    mount("profile-verification");
    expect(await screen.findByRole("alert")).toHaveTextContent("请选择当前企业");
    expect(fetch).not.toHaveBeenCalled();
  });
  it("clears password inputs after the password provider confirms success", async () => {
    const identity = { schemaVersion: "account-identity-profile-v1", userId: "u1", firstName: "本人", lastName: "甲", nickName: "", displayName: "本人甲", preferredLanguage: "", gender: "", source: "zitadel_auth_v1" };
    const operation = { schemaVersion: "account-identity-operation-v1", operation: "password", state: "updated", source: "zitadel_auth_v1" };
    const fetcher = vi.fn((url: string) => Promise.resolve(url === "/api/account/profile" ? Response.json(profile) : url === "/api/account/identity/profile" ? Response.json(identity) : url === "/api/account/identity/password" ? Response.json(operation) : Response.json({ code: "DEPENDENCY_UNAVAILABLE", message: "", requestId: "", fieldErrors: [] }, { status: 503 })));
    vi.stubGlobal("fetch", fetcher);
    const user = userEvent.setup();
    mount("profile-settings");
    const oldPassword = await screen.findByLabelText("当前密码");
    const newPassword = screen.getByLabelText("新密码");
    await user.type(oldPassword, "old-secret");
    await user.type(newPassword, "new-secret");
    await user.click(screen.getByRole("button", { name: "修改密码" }));
    await waitFor(() => expect(fetcher.mock.calls.some(([url]) => url === "/api/account/identity/password")).toBe(true));
    await waitFor(() => { expect(oldPassword).toHaveValue(""); expect(newPassword).toHaveValue(""); });
  });
  it("resets the identity profile form when a refreshed provider profile changes", async () => {
    const identity = { schemaVersion: "account-identity-profile-v1", userId: "u1", firstName: "本人", lastName: "甲", nickName: "", displayName: "本人甲", preferredLanguage: "", gender: "", source: "zitadel_auth_v1" };
    const refreshedIdentity = { ...identity, firstName: "更新" };
    let identityReads = 0;
    const fetcher = vi.fn((url: string) => Promise.resolve(url === "/api/account/profile" ? Response.json(profile) : url === "/api/account/identity/profile" ? Response.json(identityReads++ === 0 ? identity : refreshedIdentity) : Response.json({ code: "DEPENDENCY_UNAVAILABLE", message: "", requestId: "", fieldErrors: [] }, { status: 503 })));
    vi.stubGlobal("fetch", fetcher);
    const user = userEvent.setup();
    mount("profile-settings");
    expect(await screen.findByLabelText("名")).toHaveValue("本人");
    await user.click(screen.getByRole("button", { name: "刷新资料" }));
    await waitFor(() => expect(screen.getByLabelText("名")).toHaveValue("更新"));
  });
  it("offers login recovery when the identity profile read expires", async () => {
    const fetcher = vi.fn((url: string) => Promise.resolve(url === "/api/account/profile" ? Response.json(profile) : Response.json({ code: "AUTHENTICATION_REQUIRED", message: "", requestId: "", fieldErrors: [] }, { status: 401 })));
    vi.stubGlobal("fetch", fetcher);
    mount("profile-settings");
    expect(await screen.findByText("个人资料暂时无法读取，请稍后重试。")).toBeVisible();
    expect(screen.getByRole("link", { name: "重新登录" })).toHaveAttribute("href", "/login?returnTo=%2Fworkbench%2Faccount%2Fprofile%2Fsettings");
  });
  it("surfaces a failed verification resend instead of hiding the mutation error", async () => {
    const fetcher = vi.fn((url: string) => Promise.resolve(url === "/api/account/profile" ? Response.json(profile) : url === "/api/account/organization" ? Response.json(organization) : Response.json({ code: "DEPENDENCY_UNAVAILABLE", message: "", requestId: "", fieldErrors: [] }, { status: 503 })));
    vi.stubGlobal("fetch", fetcher);
    const user = userEvent.setup();
    mount("profile-verification");
    await user.click((await screen.findAllByRole("button", { name: "重新发送" }))[0]);
    expect(await screen.findByText("操作失败，请稍后重试")).toBeVisible();
  });
  it("offers login recovery when a verification resend sees an expired session", async () => {
    const fetcher = vi.fn((url: string) => Promise.resolve(url === "/api/account/profile" ? Response.json(profile) : url === "/api/account/organization" ? Response.json(organization) : Response.json({ code: "AUTHENTICATION_REQUIRED", message: "", requestId: "", fieldErrors: [] }, { status: 401 })));
    vi.stubGlobal("fetch", fetcher);
    const user = userEvent.setup();
    mount("profile-verification");
    await user.click((await screen.findAllByRole("button", { name: "重新发送" }))[0]);
    expect(await screen.findByRole("link", { name: "重新登录" })).toHaveAttribute("href", "/login?returnTo=%2Fworkbench%2Faccount%2Fprofile%2Fverification");
  });
  it("keeps resend disabled after an unknown outcome while account facts reconcile", async () => {
    const identity = { schemaVersion: "account-identity-profile-v1", userId: "u1", firstName: "本人", lastName: "甲", nickName: "", displayName: "本人甲", preferredLanguage: "", gender: "", source: "zitadel_auth_v1" };
    const unknown = { code: "RESULT_UNVERIFIED", message: "", requestId: "", fieldErrors: [], outcome: "unknown" };
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === "/api/account/profile" || url === "/api/account/organization" || url === "/api/account/identity/profile") return Promise.resolve(Response.json(url === "/api/account/profile" ? profile : url === "/api/account/organization" ? organization : identity));
      if (url === "/api/account/identity/email/resend" && init?.method === "POST") return Promise.resolve(Response.json(unknown, { status: 504 }));
      throw new Error(`unexpected fetch: ${url}`);
    });
    vi.stubGlobal("fetch", fetcher);
    const user = userEvent.setup();
    mount("profile-verification");
    await user.click((await screen.findAllByRole("button", { name: "重新发送" }))[0]);
    expect(await screen.findByText("操作结果待核实，已刷新资料；请确认当前验证状态后，再点击“刷新资料”重新发送。")).toBeVisible();
    expect(screen.getAllByRole("button", { name: "重新发送" })[0]).toBeDisabled();
  });
  it("keeps contact updates disabled after an unknown outcome while account facts reconcile", async () => {
    const identity = { schemaVersion: "account-identity-profile-v1", userId: "u1", firstName: "本人", lastName: "甲", nickName: "", displayName: "本人甲", preferredLanguage: "", gender: "", source: "zitadel_auth_v1" };
    const unknown = { code: "RESULT_UNVERIFIED", message: "", requestId: "", fieldErrors: [], outcome: "unknown" };
    const fetcher = vi.fn((url: string, init?: RequestInit) => {
      if (url === "/api/account/profile" || url === "/api/account/identity/profile") return Promise.resolve(Response.json(url.endsWith("profile") && url !== "/api/account/profile" ? identity : profile));
      if (url === "/api/account/identity/email" && init?.method === "PUT") return Promise.resolve(Response.json(unknown, { status: 504 }));
      throw new Error(`unexpected fetch: ${url}`);
    });
    vi.stubGlobal("fetch", fetcher);
    const user = userEvent.setup();
    mount("profile-settings");
    const email = await screen.findByLabelText("邮箱地址");
    await user.type(email, "buyer@example.test");
    await user.click(screen.getByRole("button", { name: "修改邮箱" }));
    expect(await screen.findByText("操作结果待核实，已刷新资料；请确认当前联系方式状态后，再点击“刷新资料”重新提交。")).toBeVisible();
    expect(screen.getByRole("button", { name: "修改邮箱" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "修改手机号" })).toBeDisabled();
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
  it("requires the selected organization on the business-profile leaf", async () => {
    state.context.effectiveOrganization = null;
    vi.stubGlobal("fetch", vi.fn());
    mount("profile-business");
    expect(await screen.findByRole("alert")).toHaveTextContent("请选择当前企业");
    expect(fetch).not.toHaveBeenCalled();
  });
  it("surfaces business-profile organization failures instead of treating them as absent data", async () => {
    const fetcher = vi.fn((url: string) => url === "/api/account/profile"
      ? Promise.resolve(Response.json(profile))
      : Promise.resolve(Response.json({ code: "ORGANIZATION_ACCESS_REVOKED", message: "", requestId: "", fieldErrors: [] }, { status: 403 })));
    vi.stubGlobal("fetch", fetcher);
    mount("profile-business");
    expect(await screen.findByRole("alert")).toHaveTextContent("企业访问已撤销");
    expect(screen.queryByText("业务档案服务暂未接入")).not.toBeInTheDocument();
  });
  it("retries a dependency failure only after user action and rereads facts", async () => {
    const fetcher = vi.fn().mockResolvedValueOnce(Response.json({ code: "DEPENDENCY_UNAVAILABLE", message: "", requestId: "", fieldErrors: [] }, { status: 503 })).mockImplementation(() => Promise.resolve(Response.json(profile)));
    vi.stubGlobal("fetch", fetcher); mount();
    expect(await screen.findByRole("alert")).toBeVisible(); expect(fetcher).toHaveBeenCalledTimes(2);
    await userEvent.click(screen.getByRole("button", { name: "刷新资料" }));
    expect(await screen.findByRole("heading", { name: "本人甲" })).toBeVisible(); expect(fetcher).toHaveBeenCalledTimes(4);
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
    await userEvent.click(link); expect(screen.queryByText("本人甲")).not.toBeInTheDocument(); expect(fetcher).toHaveBeenCalledTimes(2); link.remove();
  });
  it("clears visible profile and reauthorizes when role context changes", async () => {
    const fetcher = vi.fn().mockImplementationOnce(() => Promise.resolve(Response.json(profile))).mockImplementationOnce(() => Promise.resolve(Response.json({ code: "AUTHENTICATION_REQUIRED", message: "hidden", requestId: "", fieldErrors: [] }, { status: 401 }))); vi.stubGlobal("fetch", fetcher);
    const view = mount(); expect(await screen.findByRole("heading", { name: "本人甲" })).toBeVisible(); state.context.roles = []; view.update();
    expect(screen.queryByText("本人甲")).not.toBeInTheDocument(); expect(await screen.findByRole("alert")).toBeVisible();
  });
});
