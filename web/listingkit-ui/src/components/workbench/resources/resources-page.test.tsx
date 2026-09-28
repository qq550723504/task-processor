import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { commercialOverviewFixture } from "@/test/fixtures/commercial-overview";
import { commercialResourcesFixture } from "@/test/fixtures/commercial-resources";
import { ResourcesPage } from "./resources-page";

const state = vi.hoisted(() => ({ context: {} as Record<string, unknown>, read: vi.fn(), resources: vi.fn(), stores: vi.fn(), pointLimits: vi.fn() }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));
vi.mock("@/lib/api/commercial", async original => ({ ...await original<typeof import("@/lib/api/commercial")>(), getCommercialOverview: state.read }));
vi.mock("@/lib/api/commercial-billing", async original => ({ ...await original<typeof import("@/lib/api/commercial-billing")>(), getCommercialResources: state.resources }));
vi.mock("@/lib/api/member-ai-point-limits", async original => ({ ...await original<typeof import("@/lib/api/member-ai-point-limits")>(), getMemberAIPointLimits: state.pointLimits }));
vi.mock("@/lib/api/workbench-stores", async original => ({ ...await original<typeof import("@/lib/api/workbench-stores")>(), listWorkbenchStores: state.stores }));
vi.mock("@/lib/api/member-resources",async original=>({...await original<typeof import("@/lib/api/member-resources")>(),getMemberResources:vi.fn().mockImplementation((scope:{expectedOrganizationId:string})=>Promise.resolve({schemaVersion:"member-resource-directory-v1",organizationId:scope.expectedOrganizationId,observedAt:"2026-09-29T00:00:00Z",members:[]}))}));
let client: QueryClient;
const tree = () => <QueryClientProvider client={client}><ResourcesPage /></QueryClientProvider>;
beforeEach(() => {
  state.context = { user: { id: "actor" }, effectiveOrganization: { id: "org-B", name: "企业乙" }, roles: ["listingkit_operator"], retry: vi.fn() };
  state.read.mockReset().mockResolvedValue(commercialOverviewFixture());
  state.resources.mockReset().mockImplementation((_user, org) => Promise.resolve(commercialResourcesFixture(org)));
  state.stores.mockReset().mockResolvedValue({ items: [], pagination: { total: 37, page: 1, pageSize: 1 } });
  state.pointLimits.mockReset().mockImplementation((scope: { expectedOrganizationId: string }) => ({ schemaVersion: "member-ai-point-monthly-limit-v1", organizationId: scope.expectedOrganizationId, resourceType: "ai_point", timezone: "UTC", members: [] }));
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});
afterEach(() => { cleanup(); client.clear(); vi.unstubAllGlobals(); });

it("hides the previous store total on refresh failure rather than projecting a zero", async () => {
  render(tree());
  const panel = screen.getByRole("region", { name: "店铺资源" });
  expect(await within(panel).findByText("37 家")).toBeVisible();
  state.stores.mockRejectedValue(new Error("private store detail"));
  await userEvent.click(screen.getByRole("button", { name: "刷新资源" }));
  expect(await within(panel).findByRole("alert")).toHaveTextContent("本次未取得店铺记录数");
  expect(within(panel).queryByText("37 家")).not.toBeInTheDocument();
  expect(within(panel).queryByText("0 家")).not.toBeInTheDocument();
  expect(within(panel).queryByText(/private store detail/)).not.toBeInTheDocument();
});

it("ignores a previous organization's late store count", async () => {
  let finish!: (value: unknown) => void;
  state.stores.mockReturnValueOnce(new Promise(resolve => { finish = resolve; })).mockResolvedValue({ items: [], pagination: { total: 2, page: 1, pageSize: 1 } });
  const view = render(tree());
  state.context.effectiveOrganization = { id: "org-C", name: "企业丙" };
  view.rerender(tree());
  expect(await screen.findByText("2 家")).toBeVisible();
  await act(async () => finish({ items: [], pagination: { total: 999, page: 1, pageSize: 1 } }));
  expect(screen.queryByText("999 家")).not.toBeInTheDocument();
});

it("projects real grants and usage while unknown resource and Store facts never become zero", async () => {
  render(tree());
  expect(await screen.findByText("企业实际合同")).toBeVisible();
  expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("资源与额度");
  expect(screen.getByRole("region", { name: "成员 AI 点数月度上限" })).toBeVisible();
  expect(screen.getByText("0 家")).toBeVisible(); // actual explicit store grant, not store count
  expect(screen.getByText("9007199254740993 字节")).toBeVisible();
  expect(screen.getAllByText(/未知（作业次）/).length).toBeGreaterThan(0);
  expect(await within(screen.getByRole("region", { name: "店铺资源" })).findByText("37 家")).toBeVisible();
  expect(state.stores).toHaveBeenCalledWith({ page: 1, pageSize: 1 }, "org-B", expect.any(AbortSignal));
  const topMetrics = screen.getByRole("region", { name: "企业资源权益摘要" });
  expect(within(topMetrics).getAllByRole("heading")).toHaveLength(3);
  const memberDirectory = await screen.findByRole("region", { name: "成员 AI Token 分配" });
  expect(topMetrics.compareDocumentPosition(memberDirectory) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(memberDirectory.compareDocumentPosition(screen.getByRole("region", { name: "源账号资源" })) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  const memberTable = within(memberDirectory).getByRole("table");
  expect(within(memberTable).getAllByRole("columnheader").map(header => header.textContent)).toEqual(["成员", "角色", "AI Token 额度", "操作"]);
  expect(topMetrics.compareDocumentPosition(screen.getByRole("region", { name: "源账号资源" })) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
  expect(screen.getByRole("group", { name: "成员资源筛选" })).toBeVisible();
  expect(screen.getByRole("link", { name: "管理源账号" })).toHaveAttribute("href", "/workbench/account/organization/resources/source-accounts");
  expect(screen.getByText(/可选企业资源/)).toBeVisible();
  expect(screen.queryByText(/8,650|¥3,000|0 家已绑定/)).not.toBeInTheDocument();
  expect(state.read).toHaveBeenCalledWith("org-B", expect.any(AbortSignal));
});

it("keeps pending member allocation reads distinct from unavailable and empty states", async () => {
  let resolveFetch!: (response: Response) => void;
  let resolveRefresh!: (response: Response) => void;
  const pendingResponse = new Promise<Response>(resolve => { resolveFetch = resolve; });
  const pendingRefresh = new Promise<Response>(resolve => { resolveRefresh = resolve; });
  vi.stubGlobal("fetch", vi.fn().mockReturnValueOnce(pendingResponse).mockReturnValueOnce(pendingRefresh));
  render(tree());

  await screen.findByText("企业实际合同");
  const directory = await screen.findByRole("region", { name: "成员 AI Token 分配" });
  expect(within(directory).getByText("正在读取成员 Token 分配")).toBeVisible();
  expect(within(directory).queryByText("当前成员额度 owner 未提供可展示的成员分配行。")).not.toBeInTheDocument();
  expect(within(directory).queryByText(/成员资源数据未提供/)).not.toBeInTheDocument();

  const emptySnapshot = { schemaVersion: "account-member-token-allocation-v1", organizationId: "org-B", metric: "token", windowStart: "2026-09-01T00:00:00Z", windowEnd: "2026-10-01T00:00:00Z", enterprise: { total: "9000", allocated: "0", unallocated: "9000", consumed: "0" }, members: [] };
  await act(async () => resolveFetch(Response.json(emptySnapshot)));
  expect(await screen.findByText("当前没有可展示的成员资源记录。")).toBeVisible();

  await userEvent.click(screen.getByRole("button", { name: "刷新资源" }));
  expect(await screen.findByText("正在读取成员 Token 分配")).toBeVisible();
  expect(screen.queryByText("当前没有可展示的成员资源记录。")).not.toBeInTheDocument();
  await act(async () => resolveRefresh(Response.json(emptySnapshot)));
  expect(await screen.findByText("当前没有可展示的成员资源记录。")).toBeVisible();
});

it("keeps member allocation errors separate from empty results", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ error: { code: "DEPENDENCY_UNAVAILABLE", message: "private failure detail" } }, { status: 503 })));
  render(tree());
  const directory = await screen.findByRole("region", { name: "成员 AI Token 分配" });
  expect(await within(directory).findByRole("alert")).toHaveTextContent("成员额度服务暂不可用");
  expect(within(directory).getByRole("alert")).toHaveTextContent("本次未能确认成员分配数据");
  expect(within(directory).queryByText("当前没有可展示的成员资源记录。")).not.toBeInTheDocument();
  expect(within(directory).queryByText(/private failure detail/)).not.toBeInTheDocument();
});

it.each(["PERMISSION_DENIED", "DEPENDENCY_UNAVAILABLE", "AUTHENTICATION_REQUIRED", "ORGANIZATION_ACCESS_REVOKED"])("hides stale facts on %s and preserves separate SourceAccount entry", async code => {
  state.read.mockResolvedValueOnce(commercialOverviewFixture()).mockRejectedValue({ code, message: "private token" });
  render(tree()); await screen.findByText("企业实际合同");
  await userEvent.click(screen.getByRole("button", { name: "刷新资源" }));
  expect(await screen.findByText(/本次未取得订阅权益与用量/)).toBeVisible();
  const fallbackMetrics = screen.getByRole("region", { name: "企业资源权益摘要" });
  expect(within(fallbackMetrics).getAllByRole("heading", { level: 3 })).toHaveLength(3);
  expect(await within(fallbackMetrics).findByText("9007199254740993 点")).toBeVisible();
  const allocationDirectory = screen.getByRole("region", { name: "成员 AI Token 分配" });
  expect(within(allocationDirectory).getByRole("alert")).toHaveTextContent("本次未能确认成员分配数据");
  const memberTable = screen.getByRole("region", { name: "成员 Token 表格，可横向滚动" });
  expect(within(memberTable).getAllByRole("columnheader").map(header => header.textContent)).toEqual(["成员", "角色", "AI Token 额度", "操作"]);
  expect(within(memberTable).queryByText("当前没有可展示的成员资源记录。")).not.toBeInTheDocument();
  expect(screen.queryByText("企业实际合同")).not.toBeInTheDocument();
  expect(screen.queryByText("无订阅")).not.toBeInTheDocument();
  expect(screen.queryByText(/private token/)).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "管理源账号" })).toBeVisible();
});

it("keeps Token allocation separate from Store and purchased resources", async () => {
  state.context.roles = ["admin"];
  const fetcher = vi.fn().mockResolvedValue(Response.json({ schemaVersion: "account-member-token-allocation-v1", organizationId: "org-B", metric: "token", windowStart: "2026-09-01T00:00:00Z", windowEnd: "2026-10-01T00:00:00Z", enterprise: { total: "9000", allocated: "4500", unallocated: "4500", consumed: "1200" }, members: [{ memberId: "member-1", userId: "user-1", displayName: "成员甲", loginName: "member@example.test", state: "active", allocation: { metric: "token", windowStart: "2026-09-01T00:00:00Z", windowEnd: "2026-10-01T00:00:00Z", allocated: "4500", consumed: "1200", remaining: "3300", version: "1", active: true } }] }));
  vi.stubGlobal("fetch", fetcher);
  state.pointLimits.mockResolvedValue({ schemaVersion: "member-ai-point-monthly-limit-v1", organizationId: "org-B", resourceType: "ai_point", timezone: "UTC", members: [{ organizationId: "org-B", memberId: "member-1", displayName: "成员甲", loginName: "member@example.test", roles: ["listingkit_operator"], configured: false, monthlyLimit: "0", consumed: "0", reserved: "0", remaining: "0", version: "0", monthStart: "2026-09-01T00:00:00Z", monthEnd: "2026-10-01T00:00:00Z" }] });
  render(tree());
  await waitFor(() => expect(within(screen.getByRole("region", { name: "成员 AI Token 分配" })).getByText("member@example.test")).toBeVisible());
  const directory = screen.getByRole("region", { name: "成员 AI Token 分配" });
  const table = within(directory).getByRole("table");
  const row = within(table).getByRole("row", { name: /成员甲/ });
  expect(await within(row).findByText("操作成员")).toBeVisible();
  expect(within(row).queryByText("未提供")).not.toBeInTheDocument();
  expect(within(row).getByText("4500")).toBeVisible();
  expect(within(row).getByText("已消费 1200 · 剩余 3300")).toBeVisible();
  expect(within(row).getByRole("button", { name: "保存目标" })).toBeVisible();
  expect(fetcher).toHaveBeenCalledWith("/api/account/member-allocations", expect.objectContaining({ headers: expect.objectContaining({ "X-Expected-User-ID": "actor", "X-Expected-Organization-ID": "org-B" }) }));
  await userEvent.click(within(directory).getByRole("button", { name: "刷新分配" }));
  expect(fetcher).toHaveBeenCalledTimes(2);
});

it.each(["another-member", "unavailable"])("does not borrow another member's role when the Directory result is %s", async kind => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ schemaVersion: "account-member-token-allocation-v1", organizationId: "org-B", metric: "token", windowStart: "2026-09-01T00:00:00Z", windowEnd: "2026-10-01T00:00:00Z", enterprise: { total: "9000", allocated: "4500", unallocated: "4500", consumed: "1200" }, members: [{ memberId: "member-1", userId: "user-1", displayName: "成员甲", loginName: "member@example.test", state: "active", allocation: { metric: "token", windowStart: "2026-09-01T00:00:00Z", windowEnd: "2026-10-01T00:00:00Z", allocated: "4500", consumed: "1200", remaining: "3300", version: "1", active: true } }] })));
  if (kind === "unavailable") state.pointLimits.mockRejectedValue(new Error("private directory detail"));
  else state.pointLimits.mockResolvedValue({ schemaVersion: "member-ai-point-monthly-limit-v1", organizationId: "org-B", resourceType: "ai_point", timezone: "UTC", members: [{ organizationId: "org-B", memberId: "member-2", displayName: "成员甲", loginName: "member@example.test", roles: ["listingkit_admin"], configured: false, monthlyLimit: "0", consumed: "0", reserved: "0", remaining: "0", version: "0", monthStart: "2026-09-01T00:00:00Z", monthEnd: "2026-10-01T00:00:00Z" }] });
  render(tree());
  await waitFor(() => {
    const row = within(screen.getByRole("region", { name: "成员 AI Token 分配" })).getByRole("row", { name: /成员甲/ });
    expect(within(row).getByText("角色暂不可用")).toBeVisible();
    expect(within(row).queryByText("企业管理员")).not.toBeInTheDocument();
  });
  expect(screen.queryByText(/private directory detail/)).not.toBeInTheDocument();
});

it.each(["organization", "actor", "roles", "switching", "logout", "revoke"])("clears facts immediately on %s", async kind => {
  const view = render(tree()); await screen.findByText("企业实际合同");
  state.read.mockReturnValue(new Promise(() => {}));
  if (kind === "organization") state.context.effectiveOrganization = { id: "org-C", name: "企业丙" };
  if (kind === "actor") state.context.user = { id: "other" };
  if (kind === "roles") state.context.roles = ["listingkit_viewer"];
  if (kind === "switching") state.context.isSwitching = true;
  if (kind === "logout") state.context.user = null;
  if (kind === "revoke") state.context.blockingError = { code: "ORGANIZATION_ACCESS_REVOKED" };
  view.rerender(tree());
  expect(screen.queryByText("企业实际合同")).not.toBeInTheDocument();
});

it("ignores late results after switching organizations", async () => {
  let finish!: (value: ReturnType<typeof commercialOverviewFixture>) => void;
  state.read.mockReturnValueOnce(new Promise(resolve => { finish = resolve; })).mockResolvedValue(commercialOverviewFixture("org-C"));
  const view = render(tree());
  state.context.effectiveOrganization = { id: "org-C", name: "企业丙" }; view.rerender(tree());
  await screen.findByText("企业实际合同");
  const stale = commercialOverviewFixture(); stale.subscription!.plan_name = "旧企业秘密";
  await act(async () => finish(stale));
  expect(screen.queryByText("旧企业秘密")).not.toBeInTheDocument();
});
