import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { commercialOverviewFixture } from "@/test/fixtures/commercial-overview";
import { CommercialPage } from "./commercial-page";

const state = vi.hoisted(() => ({ context: {} as Record<string, unknown>, read: vi.fn(), wallet: vi.fn(), entries: vi.fn(), orders: vi.fn(), summary: vi.fn(), detail: vi.fn() }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));
vi.mock("@/lib/api/commercial", async (original) => ({ ...await original<typeof import("@/lib/api/commercial")>(), getCommercialOverview: state.read }));
vi.mock("@/lib/api/commercial-billing", async (original) => ({ ...await original<typeof import("@/lib/api/commercial-billing")>(), getCommercialWallet: state.wallet, getCommercialWalletEntries: state.entries, getCommercialOrders: state.orders, getCommercialOrderSummary: state.summary, getCommercialOrder: state.detail }));
let client: QueryClient;
const tree = (page: "options" | "entitlements" | "top-up" | "orders" | "order-detail" = "entitlements", orderId?: string) => <QueryClientProvider client={client}><CommercialPage page={page} orderId={orderId} /></QueryClientProvider>;
function deferred<T>() { let resolve!: (v: T) => void; let reject!: (e: unknown) => void; const promise = new Promise<T>((r, j) => { resolve = r; reject = j; }); return { promise, resolve, reject }; }
beforeEach(() => {
  state.context = { user: { id: "reader" }, effectiveOrganization: { id: "org-B", name: "企业乙", roles: ["listingkit_admin"] }, roles: ["listingkit_admin"], retry: vi.fn() };
  state.read.mockReset().mockResolvedValue(commercialOverviewFixture());
  state.wallet.mockReset().mockResolvedValue({ organization_id: "org-B", currency: "CNY", available_minor: "12000", reserved_minor: "0", debt_minor: "0", lifetime_topup_minor: "12000", lifetime_spend_minor: "0", version: "1", observed_at: "2026-09-23T10:00:00Z" });
  state.entries.mockReset().mockResolvedValue({ organization_id: "org-B", items: [], next_cursor: "" });
  state.summary.mockReset().mockResolvedValue({ organization_id: "org-B", currency: "CNY", from: "2026-08-24T10:00:00Z", until: "2026-09-23T10:00:00Z", spend_minor: "0", store_renewal_spend_minor: "0", ai_point_spend_minor: "0", data_row_spend_minor: "0", other_spend_minor: "0", observed_at: "2026-09-23T10:00:00Z" });
  state.orders.mockReset().mockResolvedValue({ organization_id: "org-B", items: [], next_cursor: "" });
  state.detail.mockReset().mockResolvedValue({ order_id: "order-1", organization_id: "org-B", kind: "RESOURCE_PURCHASE", description: "AI 点数 × 1", currency: "CNY", total_minor: "100", status: "FULFILLED", items: [], created_at: "2026-09-23T10:00:00Z", updated_at: "2026-09-23T10:00:00Z" });
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});

it("loads the wallet through the current identity and organization and keeps payment writes disabled", async () => {
  render(tree("top-up"));
  expect(within(await screen.findByRole("region", { name: "企业钱包余额" })).getAllByText("¥120.00")).toHaveLength(2);
  expect(state.wallet).toHaveBeenCalledWith("reader", "org-B", expect.any(AbortSignal));
  expect(state.entries).toHaveBeenCalledWith("reader", "org-B", expect.any(AbortSignal), undefined);
  expect(screen.getByRole("button", { name: "充值暂未开放" })).toBeDisabled();
});

it("loads organization-scoped order summaries and detail deep links", async () => {
  render(tree("orders"));
  expect(await screen.findByRole("heading", { name: "账单与订单" })).toBeVisible();
  expect(state.summary).toHaveBeenCalledWith("reader", "org-B", expect.any(AbortSignal));
  expect(state.orders).toHaveBeenCalledWith("reader", "org-B", expect.objectContaining({ cursor: undefined }), expect.any(AbortSignal));
  cleanup(); client.clear();
  render(tree("order-detail", "order-1"));
  expect(await screen.findByText("AI 点数 × 1")).toBeVisible();
  expect(state.detail).toHaveBeenCalledWith("reader", "org-B", "order-1", expect.any(AbortSignal));
});
afterEach(() => { cleanup(); client.clear(); });

it("separates actual subscription, grants, resource balances and ledger observations with exact units", async () => {
  render(tree());
  expect(await screen.findByText("企业实际合同")).toBeVisible();
  expect(screen.getByText(/当前有效企业：企业乙/)).toHaveTextContent("org-B");
  expect(screen.getByText("0 家")).toBeVisible();
  expect(screen.getByText("不限额（作业次）")).toBeVisible();
  expect(screen.getByText("9007199254740993 字节")).toBeVisible();
  expect(screen.getByText("-1 字节")).toBeVisible();
  for (const period of screen.getAllByText(/期末不含/)) expect(period).toHaveTextContent("2026-10-01 00:00:00 UTC");
  expect(screen.getByText(/当前存储观测，无统计起止/)).toBeVisible();
  expect(screen.getAllByText("未知").length).toBeGreaterThan(0);
  expect(screen.getByText(/仅展示已授予的限制/)).toBeVisible();
  expect(screen.getByText(/现金余额：尚未提供/)).toBeVisible();
  expect(screen.queryByText(/8,650|300,000|5 家使用中|¥168/)).not.toBeInTheDocument();
  expect(state.read).toHaveBeenCalledWith("org-B", expect.any(AbortSignal));
});

it.each([[1, "作业次"], [4, "字节"]] as const)("keeps the known unit visible when usage bucket %s is unknown", async (index, unit) => {
  const data = commercialOverviewFixture();
  data.usage[index] = { ...data.usage[index], state: "unknown", committed: null, reserved: null, updated_at: null };
  state.read.mockResolvedValue(data); render(tree());
  expect(await screen.findByText("企业实际合同")).toBeVisible();
  expect(screen.getAllByText(`未知（${unit}）`).length).toBeGreaterThanOrEqual(2);
});

it("keeps approved plan descriptions separate from subscription and never invents a selling price", async () => {
  render(tree("options"));
  expect(await screen.findByText("基础方案 · 按需使用")).toBeVisible();
  expect(screen.getByText("方案描述 · 暂不销售")).toBeVisible();
  expect(screen.getByText("价格未提供 · 币种未提供")).toBeVisible();
  expect(screen.queryByText("企业实际合同")).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "查看我的权益" })).toHaveAttribute("href", "/workbench/plans/entitlements");
  expect(screen.queryByRole("button", { name: /购买|充值|续费|退款|提现/ })).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: /钱包|充值|账单|用量明细/ })).not.toBeInTheDocument();
});

it("renders safe custom plan text without HTML and distinguishes no subscription from no grants", async () => {
  const data = commercialOverviewFixture();
  data.subscription = null; data.entitlements = [];
  state.read.mockResolvedValue(data);
  const view = render(tree());
  expect(await screen.findByText("无订阅")).toBeVisible();
  expect(screen.getByText("暂无已授予权益")).toBeVisible();
  view.unmount();
  const custom = commercialOverviewFixture(); custom.subscription!.plan_name = '<img src=x onerror="alert(1)">';
  state.read.mockResolvedValue(custom); render(tree());
  expect(await screen.findByText('<img src=x onerror="alert(1)">')).toBeVisible();
  expect(document.querySelector('img[src="x"]')).toBeNull();
});

it.each([ ["active", "生效中"], ["trialing", "试用中"], ["expired", "已过期"], ["disabled", "已停用"], ["not_started", "尚未开始"] ] as const)("shows backend effective state %s without computing it", async (status, label) => {
  const data = commercialOverviewFixture(); data.subscription!.effective_status = status; data.entitlements[0].effective_status = status;
  state.read.mockResolvedValue(data); render(tree());
  expect(await screen.findByText("企业实际合同")).toBeVisible();
  expect(within(screen.getByRole("region", { name: "实际订阅" })).getByText("生效状态").nextElementSibling).toHaveTextContent(label);
  expect(within(screen.getByRole("region", { name: "已授予权益" })).getAllByText(label).length).toBeGreaterThan(0);
});

it.each([
  ["PERMISSION_DENIED", "无查看权限"], ["AUTHENTICATION_REQUIRED", "登录已失效"],
  ["ORGANIZATION_ACCESS_REVOKED", "企业访问已撤销"], ["ORGANIZATION_CONTEXT_CHANGED", "企业上下文已变化"],
  ["DEPENDENCY_UNAVAILABLE", "商业数据依赖暂不可用"], ["DEADLINE_EXCEEDED", "商业数据读取超时"],
  ["INVALID_UPSTREAM_RESPONSE", "商业数据响应无效"],
])("hides prior success on refresh and displays safe %s without fallback", async (code, label) => {
  state.read.mockResolvedValueOnce(commercialOverviewFixture()).mockRejectedValue({ code, message: "private SQL/token" });
  render(tree()); await screen.findByText("企业实际合同");
  await userEvent.click(screen.getByRole("button", { name: "刷新数据" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(label);
  expect(screen.queryByText("企业实际合同")).not.toBeInTheDocument();
  expect(screen.queryByText("无订阅")).not.toBeInTheDocument();
  expect(screen.queryByText(/private SQL/)).not.toBeInTheDocument();
});

it.each(["organization", "subject", "roles", "logout", "switching", "contextError"])("clears successful sensitive state immediately on %s", async (change) => {
  const view = render(tree()); await screen.findByText("企业实际合同");
  state.read.mockReturnValue(new Promise(() => {}));
  if (change === "organization") state.context.effectiveOrganization = { id: "org-C", name: "企业丙" };
  if (change === "subject") state.context.user = { id: "other" };
  if (change === "roles") state.context.roles = ["listingkit_viewer"];
  if (change === "logout") state.context.user = null;
  if (change === "switching") state.context.isSwitching = true;
  if (change === "contextError") state.context.error = { code: "DEPENDENCY_UNAVAILABLE" };
  view.rerender(tree());
  expect(screen.queryByText("企业实际合同")).not.toBeInTheDocument();
  expect(screen.queryByText("9007199254740993 字节")).not.toBeInTheDocument();
});

it.each(["success", "error"])("aborts old scope and drops late %s through A→B→A", async outcome => {
  const late = deferred<ReturnType<typeof commercialOverviewFixture>>(); state.read.mockReturnValueOnce(late.promise);
  const view = render(tree()); await waitFor(() => expect(state.read).toHaveBeenCalledTimes(1));
  const signal = state.read.mock.calls[0][1] as AbortSignal;
  state.context.effectiveOrganization = { id: "org-C", name: "企业丙" };
  state.read.mockResolvedValue({ ...commercialOverviewFixture("org-C"), subscription: null });
  view.rerender(tree()); await screen.findByText("无订阅"); expect(signal.aborted).toBe(true);
  await act(async () => { if (outcome === "success") late.resolve(commercialOverviewFixture()); else late.reject({ code: "DEADLINE_EXCEEDED" }); });
  expect(screen.queryByText("企业实际合同")).not.toBeInTheDocument(); expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  state.context.effectiveOrganization = { id: "org-B", name: "企业乙" };
  state.read.mockResolvedValue({ ...commercialOverviewFixture(), subscription: null }); view.rerender(tree());
  await waitFor(() => expect(state.read).toHaveBeenCalledTimes(3));
  expect(screen.queryByText("企业实际合同")).not.toBeInTheDocument();
});

it("aborts pending request on unmount and refresh discards its late data", async () => {
  const late = deferred<ReturnType<typeof commercialOverviewFixture>>(); state.read.mockReturnValueOnce(late.promise);
  const view = render(tree()); await waitFor(() => expect(state.read).toHaveBeenCalledTimes(1));
  const firstSignal = state.read.mock.calls[0][1] as AbortSignal;
  state.read.mockResolvedValue({ ...commercialOverviewFixture(), subscription: null });
  await userEvent.click(screen.getByRole("button", { name: "刷新数据" }));
  expect(await screen.findByText("无订阅")).toBeVisible(); expect(firstSignal.aborted).toBe(true);
  await act(async () => late.resolve(commercialOverviewFixture()));
  expect(screen.queryByText("企业实际合同")).not.toBeInTheDocument();
  const pending = deferred<ReturnType<typeof commercialOverviewFixture>>(); state.read.mockReturnValue(pending.promise);
  await userEvent.click(screen.getByRole("button", { name: "刷新数据" }));
  const lastSignal = state.read.mock.calls.at(-1)![1] as AbortSignal; view.unmount(); expect(lastSignal.aborted).toBe(true);
  await act(async () => pending.resolve(commercialOverviewFixture()));
});
