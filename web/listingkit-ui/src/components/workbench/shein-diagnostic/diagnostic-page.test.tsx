import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { diagnosticFixture } from "@/test/fixtures/shein-diagnostic";
import { SheinDiagnosticPage } from "./diagnostic-page";

const state = vi.hoisted(() => ({ context: {} as Record<string, unknown>, fetch: vi.fn() }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));
// Only the client boundary is replaced; these tests do not claim BFF/Go integration.
vi.mock("@/lib/api/shein-diagnostic-client", () => ({ fetchSheinDiagnostic: state.fetch }));
const recordId = "c78285de-a1a8-4cc5-8aae-93ad6851da11";
let client: QueryClient;
function tree() {
  return <QueryClientProvider client={client}><SheinDiagnosticPage recordId={recordId} /></QueryClientProvider>;
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((r) => { resolve = r; });
  return { promise, resolve };
}

beforeEach(() => {
  state.fetch.mockReset();
  state.context = { user: { id: "reader" }, effectiveOrganization: { id: "org-a", name: "企业甲", roles: [] }, roles: [], isLoading: false, isSwitching: false, selectionRequired: false, error: null, blockingError: null, retry: vi.fn() };
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});
afterEach(() => { cleanup(); client.clear(); });

describe("diagnostic page request lifecycle (contract fixture)", () => {
  it("loads explicitly as publish, refreshes, binds same-content checks, and resets digest on action changes", async () => {
    const user = userEvent.setup();
    state.fetch.mockImplementation(async ({ action }) => diagnosticFixture(action));
    render(tree());
    expect(await screen.findByText("发现需要处理的问题")).toBeVisible();
    expect(state.fetch.mock.calls[0][0]).toMatchObject({ recordId, organizationId: "org-a", action: "publish" });
    expect(state.fetch.mock.calls[0][0].expectedDigest).toBeUndefined();
    await user.click(screen.getByRole("button", { name: "重新检查" }));
    await waitFor(() => expect(state.fetch).toHaveBeenCalledTimes(2));
    expect(state.fetch.mock.calls[1][0].expectedDigest).toBeUndefined();
    await user.click(await screen.findByRole("button", { name: "复核同一内容" }));
    await waitFor(() => expect(state.fetch).toHaveBeenCalledTimes(3));
    expect(state.fetch.mock.calls[2][0].expectedDigest).toBe(diagnosticFixture().input.actual_digest);
    await user.selectOptions(screen.getByLabelText("检查动作"), "save_draft");
    await waitFor(() => expect(state.fetch).toHaveBeenCalledTimes(4));
    expect(state.fetch.mock.calls[3][0]).toMatchObject({ action: "save_draft", organizationId: "org-a" });
    expect(state.fetch.mock.calls[3][0].expectedDigest).toBeUndefined();
  });

  it.each([
    ["permission_denied", "没有读取这份资料的权限"],
    ["AUTHENTICATION_REQUIRED", "登录状态已失效"],
    ["not_found", "资料不存在或当前账号不可读取"],
    ["deadline_exceeded", "检查超时"],
    ["DEADLINE_EXCEEDED", "检查超时"],
    ["stale_input", "资料内容或有效性已变化"],
    ["INVALID_UPSTREAM_RESPONSE", "诊断响应不合法"],
    ["DEPENDENCY_UNAVAILABLE", "诊断服务暂不可用"],
    ["ORGANIZATION_CONTEXT_CHANGED", "企业上下文已变化"],
    ["ORGANIZATION_ACCESS_REVOKED", "当前企业访问已撤销"],
  ])("hides prior success on %s with no automatic retry", async (code, message) => {
    state.fetch.mockResolvedValueOnce(diagnosticFixture()).mockRejectedValue({ code, status: 403 });
    render(tree());
    expect(await screen.findByText("发现需要处理的问题")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "重新检查" }));
    expect(await screen.findByRole("alert")).toHaveTextContent(message);
    expect(screen.queryByText("缺少商品名称")).not.toBeInTheDocument();
    expect(state.fetch).toHaveBeenCalledTimes(2);
    expect(screen.queryByRole("button", { name: "复核同一内容" })).not.toBeInTheDocument();
  });

  it("does not silently retry a same-content mismatch without its digest", async () => {
    state.fetch.mockResolvedValueOnce(diagnosticFixture()).mockRejectedValue({ code: "stale_input", status: 409 });
    render(tree());
    await userEvent.click(await screen.findByRole("button", { name: "复核同一内容" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("资料内容或有效性已变化");
    expect(state.fetch).toHaveBeenCalledTimes(2);
    expect(state.fetch.mock.calls[1][0].expectedDigest).toBe(diagnosticFixture().input.actual_digest);
    expect(screen.getByRole("button", { name: "检查当前内容" })).toBeVisible();
  });

  it("discards late publish response after fast action change even if transport ignores cancellation", async () => {
    const old = deferred<ReturnType<typeof diagnosticFixture>>();
    state.fetch.mockReturnValueOnce(old.promise).mockResolvedValue(diagnosticFixture("save_draft"));
    render(tree());
    expect(screen.getByRole("status")).toHaveTextContent("正在检查");
    await waitFor(() => expect(state.fetch).toHaveBeenCalledTimes(1));
    const signal = state.fetch.mock.calls[0][0].signal;
    await userEvent.selectOptions(screen.getByLabelText("检查动作"), "save_draft");
    expect(await screen.findByText("发现需要处理的问题")).toBeVisible();
    expect(signal.aborted).toBe(true);
    const late = diagnosticFixture();
    late.offline_checks.blockers[0].message = "旧动作敏感内容";
    await act(async () => old.resolve(late));
    expect(screen.queryByText("旧动作敏感内容")).not.toBeInTheDocument();
    expect(screen.getByLabelText("检查动作")).toHaveValue("save_draft");
  });

  it("unmounts sensitive results during organization switch and isolates late responses and cache", async () => {
    const old = deferred<ReturnType<typeof diagnosticFixture>>();
    state.fetch.mockReturnValueOnce(old.promise).mockResolvedValue(diagnosticFixture());
    const view = render(tree());
    await waitFor(() => expect(state.fetch).toHaveBeenCalledTimes(1));
    const signal = state.fetch.mock.calls[0][0].signal;
    state.context = { ...state.context, isSwitching: true };
    view.rerender(tree());
    expect(signal.aborted).toBe(true);
    expect(screen.getByRole("status")).toHaveTextContent("正在切换企业");
    state.context = { ...state.context, isSwitching: false, effectiveOrganization: { id: "org-b", name: "企业乙", roles: [] } };
    view.rerender(tree());
    expect(await screen.findByText("发现需要处理的问题")).toBeVisible();
    expect(state.fetch.mock.calls[1][0].organizationId).toBe("org-b");
    const late = diagnosticFixture();
    late.offline_checks.blockers[0].message = "企业甲敏感内容";
    await act(async () => old.resolve(late));
    expect(screen.queryByText("企业甲敏感内容")).not.toBeInTheDocument();
    await waitFor(() => expect(client.getQueryCache().getAll().every((query) => query.queryKey.includes("org-b"))).toBe(true));
  });

  it.each(["logout", "revoked", "unmount"])("cancels pending requests on %s", async (kind) => {
    const request = deferred<ReturnType<typeof diagnosticFixture>>();
    state.fetch.mockReturnValue(request.promise);
    const view = render(tree());
    await waitFor(() => expect(state.fetch).toHaveBeenCalledTimes(1));
    const signal = state.fetch.mock.calls[0][0].signal;
    if (kind === "unmount") view.unmount();
    else {
      state.context = { ...state.context, ...(kind === "logout" ? { user: null } : { blockingError: { code: "ORGANIZATION_ACCESS_REVOKED" } }) };
      view.rerender(tree());
    }
    expect(signal.aborted).toBe(true);
    await act(async () => request.resolve(diagnosticFixture()));
    expect(screen.queryByText("缺少商品名称")).not.toBeInTheDocument();
  });

  it("removes rendered sensitive data on logout, role changes, and denied context", async () => {
    state.fetch.mockResolvedValue(diagnosticFixture());
    const view = render(tree());
    expect(await screen.findByText("发现需要处理的问题")).toBeVisible();
    const pending = deferred<ReturnType<typeof diagnosticFixture>>();
    state.fetch.mockReturnValue(pending.promise);
    state.context = { ...state.context, roles: ["changed-role"] };
    view.rerender(tree());
    expect(screen.queryByText("发现需要处理的问题")).not.toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("正在检查");
    await act(async () => pending.resolve(diagnosticFixture()));
    expect(await screen.findByText("发现需要处理的问题")).toBeVisible();
    state.context = { ...state.context, user: null };
    view.rerender(tree());
    expect(screen.queryByText("缺少商品名称")).not.toBeInTheDocument();
    await waitFor(() => expect(client.getQueryCache().getAll()).toHaveLength(0));
    state.context = { ...state.context, user: { id: "reader" }, error: { code: "ORGANIZATION_ACCESS_DENIED" } };
    view.rerender(tree());
    expect(screen.queryByText("缺少商品名称")).not.toBeInTheDocument();
    expect(state.fetch).toHaveBeenCalledTimes(2);
  });

  it("clears success while refreshing and renders a native network failure without raw errors", async () => {
    state.fetch.mockResolvedValueOnce(diagnosticFixture());
    const pending = deferred<ReturnType<typeof diagnosticFixture>>();
    state.fetch.mockReturnValueOnce(pending.promise);
    render(tree());
    expect(await screen.findByText("发现需要处理的问题")).toBeVisible();
    await userEvent.click(screen.getByRole("button", { name: "重新检查" }));
    expect(screen.queryByText("缺少商品名称")).not.toBeInTheDocument();
    expect(screen.getByRole("status")).toHaveTextContent("正在检查");
    await act(async () => pending.resolve(diagnosticFixture()));
    expect(await screen.findByText("发现需要处理的问题")).toBeVisible();
    state.fetch.mockRejectedValueOnce(new Error("secret SQL/token/origin"));
    await userEvent.click(screen.getByRole("button", { name: "重新检查" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("诊断请求失败");
    expect(screen.queryByText(/secret SQL/)).not.toBeInTheDocument();
  });

  it("uses context recovery for organization drift instead of a blind diagnostic retry", async () => {
    state.fetch.mockRejectedValue({ code: "ORGANIZATION_CONTEXT_CHANGED" });
    render(tree());
    await userEvent.click(await screen.findByRole("button", { name: "重新加载企业上下文" }));
    expect(state.context.retry).toHaveBeenCalledTimes(1);
    expect(state.fetch).toHaveBeenCalledTimes(1);
  });

  it.each(["action", "organization", "unmount"])("discards delayed recovery after %s changes", async (change) => {
    const recovery = deferred<unknown>();
    state.context.retry = vi.fn(() => recovery.promise);
    state.fetch.mockRejectedValueOnce({ code: "ORGANIZATION_SUSPENDED" }).mockImplementation(async ({ action }) => diagnosticFixture(action));
    const view = render(tree());
    await userEvent.click(await screen.findByRole("button", { name: "重新加载企业上下文" }));
    expect(screen.getByRole("button", { name: "正在恢复企业上下文…" })).toBeDisabled();
    if (change === "action") await userEvent.selectOptions(screen.getByLabelText("检查动作"), "save_draft");
    else if (change === "organization") {
      state.context = { ...state.context, effectiveOrganization: { id: "org-b", name: "企业乙", roles: [] } };
      view.rerender(tree());
    } else view.unmount();
    if (change !== "unmount") expect(await screen.findByText("发现需要处理的问题")).toBeVisible();
    const count = state.fetch.mock.calls.length;
    await act(async () => recovery.resolve({ user: { id: "reader" }, effectiveOrganizationId: "org-a", selectionRequired: false, organizations: [{ id: "org-a", roles: [] }] }));
    expect(state.fetch).toHaveBeenCalledTimes(count);
    if (change === "action") expect(screen.getByLabelText("检查动作")).toHaveValue("save_draft");
  });
});
