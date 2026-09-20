import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { act, cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { commercialOverviewFixture } from "@/test/fixtures/commercial-overview";
import { ResourcesPage } from "./resources-page";

const state = vi.hoisted(() => ({ context: {} as Record<string, unknown>, read: vi.fn() }));
vi.mock("@/components/providers/workbench-context-provider", () => ({ useWorkbenchContext: () => state.context }));
vi.mock("@/lib/api/commercial", async original => ({ ...await original<typeof import("@/lib/api/commercial")>(), getCommercialOverview: state.read }));
let client: QueryClient;
const tree = () => <QueryClientProvider client={client}><ResourcesPage /></QueryClientProvider>;
beforeEach(() => {
  state.context = { user: { id: "actor" }, effectiveOrganization: { id: "org-B", name: "企业乙" }, roles: ["listingkit_operator"], retry: vi.fn() };
  state.read.mockReset().mockResolvedValue(commercialOverviewFixture());
  client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
});
afterEach(() => { cleanup(); client.clear(); });

it("projects real grants and usage while unknown resource and Store facts never become zero", async () => {
  render(tree());
  expect(await screen.findByText("企业实际合同")).toBeVisible();
  expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("资源与额度");
  expect(screen.getByText("0 家")).toBeVisible(); // actual explicit store grant, not store count
  expect(screen.getByText("9007199254740993 字节")).toBeVisible();
  expect(screen.getAllByText(/未知（作业次）/).length).toBeGreaterThan(0);
  expect(within(screen.getByRole("region", { name: "店铺资源" })).getByText(/实际店铺数量未提供/)).toBeVisible();
  expect(screen.getByRole("link", { name: "管理源账号" })).toHaveAttribute("href", "/workbench/account/organization/resources/source-accounts");
  expect(screen.getByText(/可选企业资源/)).toBeVisible();
  expect(screen.queryByText(/8,650|¥3,000|0 家已绑定/)).not.toBeInTheDocument();
  expect(state.read).toHaveBeenCalledWith("org-B", expect.any(AbortSignal));
});

it.each(["PERMISSION_DENIED", "DEPENDENCY_UNAVAILABLE", "AUTHENTICATION_REQUIRED", "ORGANIZATION_ACCESS_REVOKED"])("hides stale facts on %s and preserves separate SourceAccount entry", async code => {
  state.read.mockResolvedValueOnce(commercialOverviewFixture()).mockRejectedValue({ code, message: "private token" });
  render(tree()); await screen.findByText("企业实际合同");
  await userEvent.click(screen.getByRole("button", { name: "刷新资源" }));
  expect(await screen.findByRole("alert")).toBeVisible();
  expect(screen.queryByText("企业实际合同")).not.toBeInTheDocument();
  expect(screen.queryByText("无订阅")).not.toBeInTheDocument();
  expect(screen.queryByText(/private token/)).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "管理源账号" })).toBeVisible();
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
