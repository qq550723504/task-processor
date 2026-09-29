import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { commercialOverviewFixture } from "@/test/fixtures/commercial-overview";
import { commercialResourcesFixture } from "@/test/fixtures/commercial-resources";
import { CommercialPage } from "./commercial-page";
vi.mock("./wallet-topup", () => ({ WalletTopUpEntry: () => <button disabled>充值钱包</button>, TopUpPaymentPanel: () => <section>充值订单</section> }));

const state = vi.hoisted(() => ({ context: {} as Record<string, unknown>, read: vi.fn(), resources: vi.fn(), offers: vi.fn(), wallet: vi.fn(), entries: vi.fn(), orders: vi.fn(), summary: vi.fn(), detail: vi.fn(), events: vi.fn() }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));
vi.mock("@/lib/api/commercial", async (original) => ({ ...await original<typeof import("@/lib/api/commercial")>(), getCommercialOverview: state.read }));
vi.mock("@/lib/api/commercial-billing", async (original) => ({ ...await original<typeof import("@/lib/api/commercial-billing")>(), getCommercialResources: state.resources, getCommercialWallet: state.wallet, getCommercialWalletEntries: state.entries, getCommercialOrders: state.orders, getCommercialOrderSummary: state.summary, getCommercialOrder: state.detail }));
vi.mock("@/lib/api/resource-purchase", async original => ({...await original<typeof import("@/lib/api/resource-purchase")>(),getResourceOffers:state.offers}));
vi.mock("@/lib/api/resource-events", async original => ({...await original<typeof import("@/lib/api/resource-events")>(),getResourceEvents:state.events}));
let client: QueryClient;
const tree = (page: "options" | "entitlements" | "top-up" | "orders" | "order-detail" = "entitlements", orderId?: string) => <QueryClientProvider client={client}><CommercialPage page={page} orderId={orderId} /></QueryClientProvider>;
function deferred<T>() { let resolve!: (v: T) => void; let reject!: (e: unknown) => void; const promise = new Promise<T>((r, j) => { resolve = r; reject = j; }); return { promise, resolve, reject }; }

function fixture(org="org-B"){const data=commercialOverviewFixture(org);if(data.store_services.state==="available")data.store_services.value.records=137;return data;}
beforeEach(()=>{sessionStorage.clear();state.context={user:{id:"reader"},effectiveOrganization:{id:"org-B",name:"企业乙"},roles:["listingkit_admin"],retry:vi.fn()};state.read.mockReset().mockResolvedValue(fixture());state.resources.mockReset().mockImplementation((_user,org)=>Promise.resolve(commercialResourcesFixture(org)));state.offers.mockReset().mockResolvedValue({organization_id:"org-B",items:[]});state.events.mockReset().mockResolvedValue({organization_id:"org-B",items:[],next_cursor:null});state.wallet.mockReset().mockResolvedValue({organization_id:"org-B",currency:"CNY",available_minor:"12000",reserved_minor:"0",debt_minor:"0",lifetime_topup_minor:"12000",lifetime_spend_minor:"0",version:"1",observed_at:"2026-09-23T10:00:00Z"});state.entries.mockReset().mockResolvedValue({organization_id:"org-B",items:[],next_cursor:""});state.orders.mockReset().mockResolvedValue({organization_id:"org-B",items:[],next_cursor:""});state.summary.mockReset().mockResolvedValue({organization_id:"org-B",currency:"CNY",from:"2026-08-24T10:00:00Z",until:"2026-09-23T10:00:00Z",spend_minor:"0",store_renewal_spend_minor:"0",ai_point_spend_minor:"0",data_row_spend_minor:"0",other_spend_minor:"0",observed_at:"2026-09-23T10:00:00Z"});client=new QueryClient({defaultOptions:{queries:{retry:false}}});});
afterEach(()=>{cleanup();client.clear();sessionStorage.clear();});
it("keeps resource reads independent of an unavailable store summary",async()=>{state.read.mockRejectedValueOnce({code:"DEPENDENCY_UNAVAILABLE"});render(tree());expect(await within(screen.getByRole("region",{name:"AI 点数余额"})).findByText("9007199254740993 点")).toBeVisible();expect(await screen.findByRole("alert")).toHaveTextContent("本次未取得基础方案");expect(screen.queryByText("AI Token 分配额度")).not.toBeInTheDocument();});
it("reads the configured resource catalog and disables all unconfigured prices",async()=>{render(tree("options"));expect(await screen.findAllByText("价格未配置")).toHaveLength(3);expect(state.offers).toHaveBeenCalledWith("reader","org-B",expect.any(AbortSignal));expect(state.read).not.toHaveBeenCalled();});
it("does not render late resource prices after an enterprise switch",async()=>{const late=deferred<{organization_id:string;items:[]}>();state.offers.mockReturnValueOnce(late.promise);const view=render(tree("options"));await waitFor(()=>expect(state.offers).toHaveBeenCalledTimes(1));const signal=state.offers.mock.calls[0][2] as AbortSignal;state.context.effectiveOrganization={id:"org-C",name:"企业丙"};state.offers.mockResolvedValue({organization_id:"org-C",items:[]});view.rerender(tree("options"));expect(await screen.findAllByText("价格未配置")).toHaveLength(3);await act(async()=>late.resolve({organization_id:"org-B",items:[]}));expect(signal.aborted).toBe(true);expect(screen.getAllByText(/企业丙（org-C）/)).toHaveLength(2);});
it.each([
  ["PERMISSION_DENIED", "无查看权限"], ["AUTHENTICATION_REQUIRED", "登录已失效"],
  ["ORGANIZATION_ACCESS_REVOKED", "企业访问已撤销"], ["ORGANIZATION_CONTEXT_CHANGED", "企业上下文已变化"],
  ["DEPENDENCY_UNAVAILABLE", "商业数据依赖暂不可用"], ["DEADLINE_EXCEEDED", "商业数据读取超时"],
  ["INVALID_UPSTREAM_RESPONSE", "商业数据响应无效"],
])("hides prior success on refresh and displays safe %s without fallback", async (code, label) => {
  state.read.mockResolvedValueOnce(fixture()).mockRejectedValue({ code, message: "private SQL/token" });
  render(tree()); await screen.findByText("137 家");
  await userEvent.click(screen.getByRole("button", { name: "刷新数据" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(label);
  expect(screen.queryByText("137 家")).not.toBeInTheDocument();
  expect(screen.queryByText("基础方案")).not.toBeInTheDocument();
  expect(screen.queryByText(/private SQL/)).not.toBeInTheDocument();
});

it.each(["organization", "subject", "roles", "logout", "switching", "contextError"])("clears successful sensitive state immediately on %s", async (change) => {
  const view = render(tree()); await screen.findByText("137 家");
  state.read.mockReturnValue(new Promise(() => {}));
  if (change === "organization") state.context.effectiveOrganization = { id: "org-C", name: "企业丙" };
  if (change === "subject") state.context.user = { id: "other" };
  if (change === "roles") state.context.roles = ["listingkit_viewer"];
  if (change === "logout") state.context.user = null;
  if (change === "switching") state.context.isSwitching = true;
  if (change === "contextError") state.context.error = { code: "DEPENDENCY_UNAVAILABLE" };
  view.rerender(tree());
  expect(screen.queryByText("137 家")).not.toBeInTheDocument();
  expect(screen.queryByText("9007199254740993 点")).not.toBeInTheDocument();
});

it.each(["success", "error"])("aborts old scope and drops late %s through A→B→A", async outcome => {
  const late = deferred<ReturnType<typeof commercialOverviewFixture>>(); state.read.mockReturnValueOnce(late.promise);
  const view = render(tree()); await waitFor(() => expect(state.read).toHaveBeenCalledTimes(1));
  const signal = state.read.mock.calls[0][1] as AbortSignal;
  state.context.effectiveOrganization = { id: "org-C", name: "企业丙" };
  state.read.mockResolvedValue(commercialOverviewFixture("org-C"));
  view.rerender(tree()); await screen.findByText("基础方案"); expect(signal.aborted).toBe(true);
  await act(async () => { if (outcome === "success") late.resolve(fixture()); else late.reject({ code: "DEADLINE_EXCEEDED" }); });
  expect(screen.queryByText("137 家")).not.toBeInTheDocument(); expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  state.context.effectiveOrganization = { id: "org-B", name: "企业乙" };
  state.read.mockResolvedValue(commercialOverviewFixture()); view.rerender(tree());
  await waitFor(() => expect(state.read).toHaveBeenCalledTimes(3));
  expect(screen.queryByText("137 家")).not.toBeInTheDocument();
});

it("aborts pending request on unmount and refresh discards its late data", async () => {
  const late = deferred<ReturnType<typeof commercialOverviewFixture>>(); state.read.mockReturnValueOnce(late.promise);
  const view = render(tree()); await waitFor(() => expect(state.read).toHaveBeenCalledTimes(1));
  const firstSignal = state.read.mock.calls[0][1] as AbortSignal;
  state.read.mockResolvedValue(commercialOverviewFixture());
  await userEvent.click(screen.getByRole("button", { name: "刷新数据" }));
  expect(await screen.findByText("基础方案")).toBeVisible(); expect(firstSignal.aborted).toBe(true);
  await act(async () => late.resolve(fixture()));
  expect(screen.queryByText("137 家")).not.toBeInTheDocument();
  const pending = deferred<ReturnType<typeof commercialOverviewFixture>>(); state.read.mockReturnValue(pending.promise);
  await userEvent.click(screen.getByRole("button", { name: "刷新数据" }));
  const lastSignal = state.read.mock.calls.at(-1)![1] as AbortSignal; view.unmount(); expect(lastSignal.aborted).toBe(true);
  await act(async () => pending.resolve(fixture()));
});
