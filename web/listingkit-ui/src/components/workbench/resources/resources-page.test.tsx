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
  state.context = { user: { id: "actor" }, effectiveOrganization: { id: "org-B", name: "企业乙" }, roles: ["listingkit_operator"], permissions: ["product_sourcing.write","local_agent.write","listingkit.image_agent.read","listingkit.image_agent.write","workbench.agent.read","workbench.agent.use","workbench.chat.read","workbench.chat.use","workbench.task.read","workbench.knowledge.read","workbench.store.read","workbench.store.create","workbench.store.update","workbench.store.lifecycle","workbench.source_account.read","workbench.source_account.manage","workbench.organization_member.read","workbench.commercial.read"], retry: vi.fn() };
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

it("uses prepaid resource balances and monthly point limits without calling retired Token allocations",async()=>{const fetcher=vi.fn();vi.stubGlobal("fetch",fetcher);render(tree());expect(await screen.findByText("基础方案")).toBeVisible();expect(screen.getByRole("region",{name:"成员 AI 点数月度上限"})).toBeVisible();expect(screen.queryByText("成员 AI Token 分配")).not.toBeInTheDocument();expect(fetcher).not.toHaveBeenCalled();expect(await within(screen.getByRole("region",{name:"店铺资源"})).findByText("37 家")).toBeVisible();expect(screen.getByRole("link",{name:"管理源账号"})).toBeVisible();});

it.each(["PERMISSION_DENIED", "DEPENDENCY_UNAVAILABLE", "AUTHENTICATION_REQUIRED", "ORGANIZATION_ACCESS_REVOKED"])("hides stale facts on %s and preserves separate SourceAccount entry", async code => {
  state.read.mockResolvedValueOnce(commercialOverviewFixture()).mockRejectedValue({ code, message: "private token" });
  render(tree()); await screen.findByText("基础方案");
  await userEvent.click(screen.getByRole("button", { name: "刷新资源" }));
  expect(await screen.findByText(/本次未取得店铺服务与权益/)).toBeVisible();
  const fallbackMetrics = screen.getByRole("region", { name: "企业资源权益摘要" });
  expect(within(fallbackMetrics).getAllByRole("heading", { level: 3 })).toHaveLength(3);
  expect(await within(fallbackMetrics).findByText("9007199254740993 点")).toBeVisible();
  expect(screen.queryByText("基础方案")).not.toBeInTheDocument();
  expect(screen.queryByText("无订阅")).not.toBeInTheDocument();
  expect(screen.queryByText(/private token/)).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "管理源账号" })).toBeVisible();
});

it.each(["organization", "actor", "roles", "permissions", "switching", "logout", "revoke"])("clears facts immediately on %s", async kind => {
  const view = render(tree()); await screen.findByText("基础方案");
  state.read.mockReturnValue(new Promise(() => {}));
  if (kind === "organization") state.context.effectiveOrganization = { id: "org-C", name: "企业丙" };
  if (kind === "actor") state.context.user = { id: "other" };
  if (kind === "permissions") state.context.permissions = [];
  if (kind === "roles") state.context.roles = ["listingkit_viewer"]; state.context.permissions = ["workbench.task.read","workbench.store.read","workbench.source_account.read","workbench.organization_member.read","workbench.commercial.read"];
  if (kind === "switching") state.context.isSwitching = true;
  if (kind === "logout") state.context.user = null;
  if (kind === "revoke") state.context.blockingError = { code: "ORGANIZATION_ACCESS_REVOKED" };
  view.rerender(tree());
  expect(screen.queryByText("基础方案")).not.toBeInTheDocument();
});

it("ignores late results after switching organizations", async () => {
  let finish!: (value: ReturnType<typeof commercialOverviewFixture>) => void;
  state.read.mockReturnValueOnce(new Promise(resolve => { finish = resolve; })).mockResolvedValue(commercialOverviewFixture("org-C"));
  const view = render(tree());
  state.context.effectiveOrganization = { id: "org-C", name: "企业丙" }; view.rerender(tree());
  await screen.findByText("基础方案");
  const stale = commercialOverviewFixture(); if(stale.store_services.state==="available")stale.store_services.value.records=999;
  await act(async () => finish(stale));
  expect(screen.queryByText("999 家")).not.toBeInTheDocument();
});
