import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { recordListFixture } from "@/test/fixtures/shein-records";
import { SheinRecordListPage } from "./record-list-page";

const state = vi.hoisted(() => ({ context: {} as Record<string, unknown>, fetch: vi.fn() }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));
vi.mock("@/lib/api/shein-records-client", () => ({ fetchSheinRecords: state.fetch }));
let client: QueryClient;
const tree = (available = true) => <QueryClientProvider client={client}><SheinRecordListPage available={available} /></QueryClientProvider>;
function deferred<T>() { let resolve!: (value: T) => void; const promise = new Promise<T>((r) => { resolve = r; }); return { promise, resolve }; }
beforeEach(() => {
  state.fetch.mockReset();
  state.context = { user: { id: "reader" }, effectiveOrganization: { id: "200", name: "企业甲", roles: [] }, roles: [], retry: vi.fn() };
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});
afterEach(() => { cleanup(); client.clear(); });

it("shows exact metadata and links only server-returned IDs without per-row diagnostics", async () => {
  const records = recordListFixture();
  records.items[0].product_key = '<img src=x onerror="alert(1)">';
  state.fetch.mockResolvedValue(records);
  render(tree());
  expect(await screen.findByText(records.items[0].product_key)).toBeVisible();
  expect(screen.getByText("9007199254740993")).toBeVisible();
  expect(screen.queryByRole("img")).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "查看诊断" })).toHaveAttribute("href", `/workbench/shein-records/${records.items[0].record_id}/diagnostic`);
  expect(state.fetch).toHaveBeenCalledTimes(1);
  expect(state.fetch.mock.calls[0][0]).toMatchObject({ organizationId: "200", limit: 20 });
  expect(screen.queryByRole("button", { name: /新建|生成|发布|编辑/ })).not.toBeInTheDocument();
});

it("uses next/previous cursors and refresh resets to the first page", async () => {
  state.fetch.mockImplementation(async ({ cursor }) => cursor ? recordListFixture(null, "2") : recordListFixture("cursor-two"));
  render(tree());
  await screen.findByText("source-product-1");
  expect(screen.getByRole("button", { name: "上一页" })).toBeDisabled();
  await userEvent.click(screen.getByRole("button", { name: "下一页" }));
  await screen.findByText("source-product-2");
  expect(screen.getByText("第 2 页")).toHaveFocus();
  expect(state.fetch.mock.calls[1][0].cursor).toBe("cursor-two");
  expect(screen.getByRole("button", { name: "下一页" })).toBeDisabled();
  await userEvent.click(screen.getByRole("button", { name: "上一页" }));
  await screen.findByText("source-product-1");
  expect(state.fetch.mock.calls[2][0].cursor).toBeUndefined();
  await userEvent.click(screen.getByRole("button", { name: "下一页" }));
  await screen.findByText("source-product-2");
  await userEvent.click(screen.getByRole("button", { name: "刷新列表" }));
  await screen.findByText("source-product-1");
  expect(state.fetch.mock.calls.at(-1)[0].cursor).toBeUndefined();
  expect(screen.getByText("第 1 页")).toBeVisible();
});

it("does not request an unassembled capability or disguise it as an empty collection", () => {
  render(tree(false));
  expect(screen.getByText("暂未启用")).toBeVisible();
  expect(state.fetch).not.toHaveBeenCalled();
  expect(screen.queryByText("当前范围内暂无本地资料")).not.toBeInTheDocument();
});

it("shows a real empty collection with no invented create action", async () => {
  state.fetch.mockResolvedValue({ items: [], next_cursor: null });
  render(tree());
  expect(await screen.findByText("当前范围内暂无本地资料")).toBeVisible();
  expect(screen.queryByRole("link", { name: "查看诊断" })).not.toBeInTheDocument();
});

it.each([
  ["PERMISSION_DENIED", "没有读取本地资料的权限"],
  ["DEPENDENCY_UNAVAILABLE", "资料服务暂不可用"],
  ["DEADLINE_EXCEEDED", "读取资料超时"],
  ["INVALID_UPSTREAM_RESPONSE", "资料列表响应不合法"],
  ["invalid_request", "分页请求不合法"],
  ["ORGANIZATION_SUSPENDED", "当前企业已暂停访问"],
])("hides prior data on %s without retry or empty fallback", async (code, message) => {
  state.fetch.mockResolvedValueOnce(recordListFixture()).mockRejectedValue({ code, message: "private details" });
  render(tree());
  await screen.findByText("source-product-1");
  await userEvent.click(screen.getByRole("button", { name: "刷新列表" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(message);
  expect(screen.queryByText("source-product-1")).not.toBeInTheDocument();
  expect(screen.queryByText("当前范围内暂无本地资料")).not.toBeInTheDocument();
  expect(screen.queryByText("private details")).not.toBeInTheDocument();
  expect(state.fetch).toHaveBeenCalledTimes(2);
});

it("discards a delayed next page after refresh and cancels its signal", async () => {
  const late = deferred<ReturnType<typeof recordListFixture>>();
  state.fetch.mockResolvedValueOnce(recordListFixture("cursor-two")).mockReturnValueOnce(late.promise).mockResolvedValue(recordListFixture());
  render(tree());
  await userEvent.click(await screen.findByRole("button", { name: "下一页" }));
  await waitFor(() => expect(state.fetch).toHaveBeenCalledTimes(2));
  expect(screen.queryByText("source-product-1")).not.toBeInTheDocument();
  const signal = state.fetch.mock.calls[1][0].signal;
  await userEvent.click(screen.getByRole("button", { name: "刷新列表" }));
  await screen.findByText("source-product-1");
  expect(signal.aborted).toBe(true);
  await act(async () => late.resolve(recordListFixture(null, "2")));
  expect(screen.queryByText("source-product-2")).not.toBeInTheDocument();
});

it.each(["organization", "user", "roles", "logout", "revoked", "unmount"])("isolates pending responses and cursors on %s", async (kind) => {
  const late = deferred<ReturnType<typeof recordListFixture>>();
  state.fetch.mockResolvedValueOnce(recordListFixture("cursor-two")).mockReturnValueOnce(late.promise).mockResolvedValue(recordListFixture(null, "3"));
  const view = render(tree());
  await userEvent.click(await screen.findByRole("button", { name: "下一页" }));
  await waitFor(() => expect(state.fetch).toHaveBeenCalledTimes(2));
  const signal = state.fetch.mock.calls[1][0].signal;
  if (kind === "unmount") view.unmount();
  else {
    state.context = { ...state.context, ...(kind === "organization" ? { effectiveOrganization: { id: "100", name: "企业乙", roles: [] } } : kind === "user" ? { user: { id: "other" } } : kind === "roles" ? { roles: ["new-role"] } : kind === "logout" ? { user: null } : { blockingError: { code: "ORGANIZATION_ACCESS_REVOKED" } }) };
    view.rerender(tree());
  }
  expect(signal.aborted).toBe(true);
  await act(async () => late.resolve(recordListFixture(null, "2")));
  expect(screen.queryByText("source-product-2")).not.toBeInTheDocument();
  if (["organization", "user", "roles"].includes(kind)) {
    await screen.findByText("source-product-3");
    expect(state.fetch.mock.calls.at(-1)[0].cursor).toBeUndefined();
    expect(screen.getByText("第 1 页")).toBeVisible();
  } else await waitFor(() => expect(client.getQueryCache().getAll()).toHaveLength(0));
});

it("clears rendered data during organization switching before a new organization is known", async () => {
  state.fetch.mockResolvedValue(recordListFixture());
  const view = render(tree());
  await screen.findByText("source-product-1");
  state.context = { ...state.context, isSwitching: true };
  view.rerender(tree());
  expect(screen.getByRole("status")).toHaveTextContent("正在切换企业");
  expect(screen.queryByText("source-product-1")).not.toBeInTheDocument();
  await waitFor(() => expect(client.getQueryCache().getAll()).toHaveLength(0));
});

it.each([true, false])("continues only after a successful same-scope context recovery (%s)", async (success) => {
  const recovery = deferred<unknown>();
  state.context.retry = vi.fn(() => recovery.promise);
  state.fetch.mockRejectedValueOnce({ code: "ORGANIZATION_SUSPENDED" }).mockResolvedValue(recordListFixture());
  render(tree());
  await userEvent.click(await screen.findByRole("button", { name: "重新加载企业上下文" }));
  expect(screen.getByRole("button", { name: "正在恢复企业上下文…" })).toBeDisabled();
  expect(state.fetch).toHaveBeenCalledTimes(1);
  await act(async () => recovery.resolve(success ? { user: { id: "reader" }, effectiveOrganizationId: "200", selectionRequired: false, organizations: [{ id: "200", roles: [] }] } : null));
  if (success) {
    await screen.findByText("source-product-1");
    expect(state.fetch).toHaveBeenCalledTimes(2);
  } else expect(state.fetch).toHaveBeenCalledTimes(1);
});

it("does not restart an old scope when recovery returns late after an organization change", async () => {
  const recovery = deferred<unknown>();
  state.context.retry = vi.fn(() => recovery.promise);
  state.fetch.mockRejectedValueOnce({ code: "ORGANIZATION_SUSPENDED" }).mockResolvedValue(recordListFixture(null, "3"));
  const view = render(tree());
  await userEvent.click(await screen.findByRole("button", { name: "重新加载企业上下文" }));
  state.context = { ...state.context, effectiveOrganization: { id: "100", name: "企业乙", roles: [] } };
  view.rerender(tree());
  await screen.findByText("source-product-3");
  await act(async () => recovery.resolve({ user: { id: "reader" }, effectiveOrganizationId: "200", selectionRequired: false, organizations: [{ id: "200", roles: [] }] }));
  expect(state.fetch).toHaveBeenCalledTimes(2);
  expect(state.fetch.mock.calls[1][0].organizationId).toBe("100");
});

it("keeps pagination history and report cache bounded while allowing further forward navigation", async () => {
  state.fetch.mockImplementation(async ({ cursor }) => { const page = cursor ? Number(cursor) : 1; return recordListFixture(String(page + 1), String(page)); });
  render(tree());
  await screen.findByText("source-product-1");
  for (let page = 2; page <= 56; page++) {
    await userEvent.click(screen.getByRole("button", { name: "下一页" }));
    await screen.findByText(`source-product-${page}`);
  }
  await waitFor(() => expect(client.getQueryCache().getAll()).toHaveLength(1));
  for (let page = 55; page >= 7; page--) {
    await userEvent.click(screen.getByRole("button", { name: "上一页" }));
    await screen.findByText(`source-product-${page}`);
  }
  expect(screen.getByRole("button", { name: "上一页" })).toBeDisabled();
  expect(screen.getByText("更早的分页位置已释放，可刷新回到首页。")).toBeVisible();
  await userEvent.click(screen.getByRole("button", { name: "刷新列表" }));
  await screen.findByText("source-product-1");
}, 30_000);
