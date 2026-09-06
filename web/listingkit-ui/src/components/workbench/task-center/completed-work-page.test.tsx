import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { completedWorkFixture } from "@/test/fixtures/completed-work";
import { CompletedWorkPageContent } from "./completed-work-page";

const state = vi.hoisted(() => ({ context: {} as Record<string, unknown>, fetch: vi.fn() }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));
vi.mock("@/lib/api/completed-work-client", () => ({ fetchCompletedWork: state.fetch }));
let client: QueryClient;
const tree = (available = true) => <QueryClientProvider client={client}><CompletedWorkPageContent available={available} completed /></QueryClientProvider>;
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((r) => { resolve = r; }); return { promise, resolve }; }
beforeEach(() => {
  state.fetch.mockReset();
  state.context = { user: { id: "reader" }, effectiveOrganization: { id: "200", name: "企业甲", roles: [] }, roles: [], retry: vi.fn() };
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});
afterEach(() => { cleanup(); client.clear(); });

it("consumes the actual typed client once, keeps exact versions and opens only its returned result", async () => {
  const data = completedWorkFixture();
  data.items[0].product_key = '<img src=x onerror="alert(1)">';
  state.fetch.mockResolvedValue(data);
  render(tree());
  await userEvent.click(await screen.findByRole("button", { name: /<img src=x/ }));
  expect(screen.getByText("9007199254740993")).toBeVisible();
  expect(screen.queryByRole("img")).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "查看诊断" })).toHaveAttribute("href", data.items[0].result.href);
  expect(state.fetch).toHaveBeenCalledTimes(1);
  expect(state.fetch.mock.calls[0][0]).toMatchObject({ organizationId: "200", limit: 20 });
});

it("paginates opaque cursors, clears selection and refreshes at page one", async () => {
  state.fetch.mockImplementation(async ({ cursor }) => cursor ? completedWorkFixture(null, "2") : completedWorkFixture("opaque-next"));
  render(tree());
  await userEvent.click(await screen.findByRole("button", { name: /synthetic-product-1/ }));
  await userEvent.click(screen.getByRole("button", { name: "下一页" }));
  await screen.findByRole("button", { name: /synthetic-product-2/ });
  expect(screen.queryByRole("link", { name: "查看诊断" })).not.toBeInTheDocument();
  expect(screen.getByText("第 2 页")).toHaveFocus();
  expect(state.fetch.mock.calls[1][0].cursor).toBe("opaque-next");
  await userEvent.click(screen.getByRole("button", { name: "上一页" }));
  await screen.findByRole("button", { name: /synthetic-product-1/ });
  await userEvent.click(screen.getByRole("button", { name: "下一页" }));
  await screen.findByRole("button", { name: /synthetic-product-2/ });
  await userEvent.click(screen.getByRole("button", { name: "刷新记录" }));
  await screen.findByRole("button", { name: /synthetic-product-1/ });
  expect(state.fetch.mock.calls.at(-1)?.[0].cursor).toBeUndefined();
});

it("never fetches when unavailable and distinguishes a successful empty response", async () => {
  const view = render(tree(false));
  expect(screen.getByText("暂未启用")).toBeVisible(); expect(state.fetch).not.toHaveBeenCalled();
  state.fetch.mockResolvedValue({ ...completedWorkFixture(), items: [] });
  view.rerender(tree());
  expect(await screen.findByText("当前授权范围内暂无本地资料准备记录")).toBeVisible();
  expect(screen.getByText(/仅覆盖本地资料准备完成记录/)).toBeVisible();
});

it.each([
  ["PERMISSION_DENIED", "没有读取工作记录的权限"], ["DEPENDENCY_UNAVAILABLE", "工作记录服务暂不可用"],
  ["DEADLINE_EXCEEDED", "读取工作记录超时"], ["INVALID_UPSTREAM_RESPONSE", "工作记录响应不合法"],
  ["invalid_request", "分页请求不合法"], ["ORGANIZATION_ACCESS_REVOKED", "当前企业访问已撤销"],
])("clears results and selection on %s without an empty/fake fallback", async (code, message) => {
  state.fetch.mockResolvedValueOnce(completedWorkFixture()).mockRejectedValue({ code, message: "private raw details" });
  render(tree()); await userEvent.click(await screen.findByRole("button", { name: /synthetic-product-1/ }));
  await userEvent.click(screen.getByRole("button", { name: "刷新记录" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(message);
  expect(screen.queryByRole("link", { name: "查看诊断" })).not.toBeInTheDocument();
  expect(screen.queryByText("private raw details")).not.toBeInTheDocument();
});

it("cancels a delayed next page on refresh and rejects its late result", async () => {
  const late = deferred<ReturnType<typeof completedWorkFixture>>();
  state.fetch.mockResolvedValueOnce(completedWorkFixture("next")).mockReturnValueOnce(late.promise).mockResolvedValue(completedWorkFixture());
  render(tree()); await userEvent.click(await screen.findByRole("button", { name: "下一页" }));
  await waitFor(() => expect(state.fetch).toHaveBeenCalledTimes(2));
  const signal = state.fetch.mock.calls[1][0].signal;
  await userEvent.click(screen.getByRole("button", { name: "刷新记录" }));
  await screen.findByRole("button", { name: /synthetic-product-1/ }); expect(signal.aborted).toBe(true);
  await act(async () => late.resolve(completedWorkFixture(null, "2")));
  expect(screen.queryByRole("button", { name: /synthetic-product-2/ })).not.toBeInTheDocument();
});

it.each(["organization", "user", "roles", "switching", "revoked", "logout", "unmount"])("destroys old data/selection and cancels in-flight work on %s transition", async (kind) => {
  const late = deferred<ReturnType<typeof completedWorkFixture>>();
  state.fetch.mockResolvedValueOnce(completedWorkFixture("next")).mockReturnValueOnce(late.promise).mockResolvedValue(completedWorkFixture(null, "3"));
  const view = render(tree()); await userEvent.click(await screen.findByRole("button", { name: /synthetic-product-1/ }));
  await userEvent.click(screen.getByRole("button", { name: "下一页" }));
  await waitFor(() => expect(state.fetch).toHaveBeenCalledTimes(2)); const signal = state.fetch.mock.calls[1][0].signal;
  if (kind === "organization") state.context.effectiveOrganization = { id: "100", name: "企业乙", roles: [] };
  if (kind === "user") state.context.user = { id: "other" };
  if (kind === "roles") state.context.roles = ["new-role"];
  if (kind === "switching") state.context.isSwitching = true;
  if (kind === "revoked") state.context.blockingError = { code: "ORGANIZATION_ACCESS_REVOKED" };
  if (kind === "logout") state.context.user = null;
  if (kind === "unmount") view.unmount();
  else view.rerender(tree());
  expect(signal.aborted).toBe(true);
  await act(async () => late.resolve(completedWorkFixture(null, "2")));
  expect(screen.queryByRole("button", { name: /synthetic-product-2/ })).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "查看诊断" })).not.toBeInTheDocument();
  if (kind === "unmount") await waitFor(() => expect(client.getQueryCache().getAll()).toHaveLength(0));
});

it("bounds retained pagination and cache while allowing forward navigation and a fresh start", async () => {
  state.fetch.mockImplementation(async ({ cursor }) => {
    const page = cursor ? Number(cursor) : 1;
    return completedWorkFixture(String(page + 1), String(page));
  });
  render(tree());
  await screen.findByText("synthetic-product-1");
  for (let page = 2; page <= 56; page++) {
    await userEvent.click(screen.getByRole("button", { name: "下一页" }));
    await screen.findByText(`synthetic-product-${page}`);
  }
  await waitFor(() => expect(client.getQueryCache().getAll()).toHaveLength(1));
  for (let page = 55; page >= 7; page--) {
    await userEvent.click(screen.getByRole("button", { name: "上一页" }));
    await screen.findByText(`synthetic-product-${page}`);
  }
  expect(screen.getByRole("button", { name: "上一页" })).toBeDisabled();
  expect(screen.getByText("更早的分页位置已释放，可刷新回到第一页。")).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "刷新记录" }));
  await screen.findByText("synthetic-product-1");
  expect(state.fetch.mock.calls.at(-1)?.[0].cursor).toBeUndefined();
}, 30_000);
it.each([true, false])("continues only after a successful same-scope context recovery (%s)", async (success) => {
  const recovery = deferred<unknown>();
  state.context.retry = vi.fn(() => recovery.promise);
  state.fetch.mockRejectedValueOnce({ code: "ORGANIZATION_SUSPENDED" }).mockResolvedValue(completedWorkFixture());
  render(tree());
  await userEvent.click(await screen.findByRole("button", { name: "重新加载企业上下文" }));
  expect(screen.getByRole("button", { name: "正在恢复企业上下文…" })).toBeDisabled();
  expect(state.fetch).toHaveBeenCalledTimes(1);
  await act(async () => recovery.resolve(success ? { user: { id: "reader" }, effectiveOrganizationId: "200", selectionRequired: false, organizations: [{ id: "200", roles: [] }] } : null));
  if (success) {
    await screen.findByText("synthetic-product-1");
    expect(state.fetch).toHaveBeenCalledTimes(2);
  } else expect(state.fetch).toHaveBeenCalledTimes(1);
});

it("does not restart an old scope when recovery returns late after an organization change", async () => {
  const recovery = deferred<unknown>();
  state.context.retry = vi.fn(() => recovery.promise);
  state.fetch.mockRejectedValueOnce({ code: "ORGANIZATION_SUSPENDED" }).mockResolvedValue(completedWorkFixture(null, "3"));
  const view = render(tree());
  await userEvent.click(await screen.findByRole("button", { name: "重新加载企业上下文" }));
  state.context = { ...state.context, effectiveOrganization: { id: "100", name: "企业乙", roles: [] } };
  view.rerender(tree());
  await screen.findByText("synthetic-product-3");
  await act(async () => recovery.resolve({ user: { id: "reader" }, effectiveOrganizationId: "200", selectionRequired: false, organizations: [{ id: "200", roles: [] }] }));
  expect(state.fetch).toHaveBeenCalledTimes(2);
  expect(state.fetch.mock.calls[1][0].organizationId).toBe("100");
});


