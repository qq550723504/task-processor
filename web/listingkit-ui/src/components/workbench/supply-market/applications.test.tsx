import {cleanup, render, screen, waitFor, within} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {QueryClient, QueryClientProvider} from "@tanstack/react-query";
import {afterEach, beforeEach, expect, it, vi} from "vitest";
import {listMarketRecords} from "@/lib/api/supply-market";
import {SupplyApplicationsPage} from "./applications";

const context = vi.hoisted(() => ({user:{id:"member-a"}, effectiveOrganization:{id:"org-a"}, isLoading:false, isSwitching:false, error:null, blockingError:null}));
vi.mock("@/components/providers/workbench-context-provider", () => ({useWorkbenchContext:() => context}));
vi.mock("@/lib/api/supply-market", async importOriginal => ({...await importOriginal<typeof import("@/lib/api/supply-market")>(), listMarketRecords:vi.fn()}));
afterEach(cleanup);
beforeEach(() => {
  context.user.id = "member-a";
  context.isLoading = false;
  context.effectiveOrganization.id = "org-a";
  vi.mocked(listMarketRecords).mockReset().mockResolvedValue({items:[], total:0});
});
function page(view?:"progress"|"records", admin=false) {
  const client = new QueryClient({defaultOptions:{queries:{retry:false, gcTime:0}}});
  return render(<QueryClientProvider client={client}><SupplyApplicationsPage view={view} admin={admin} expectedUserId={admin?"specialist":undefined}/></QueryClientProvider>);
}
it.each([["progress", "申请进度", false], ["records", "申请记录", true]] as const)("binds %s to the existing selected/ended contract and complete breadcrumb", async (view, label, ended) => {
  page(view);
  await screen.findByText("暂无申请记录");
  expect(screen.getByRole("heading", {name:label, level:1})).toBeVisible();
  const breadcrumbs = within(screen.getByRole("navigation", {name:"面包屑"}));
  expect(breadcrumbs.getByText("供应市场")).toBeVisible();
  expect(breadcrumbs.getByRole("link", {name:"优选申请"})).toHaveAttribute("href", "/workbench/supply/applications");
  expect(breadcrumbs.getByText(label)).toBeVisible();
  expect(listMarketRecords).toHaveBeenCalledWith({userId:"member-a", organizationId:"org-a"}, {kind:"selected", ended, after:undefined}, false, expect.any(AbortSignal));
  expect(screen.queryByRole("combobox", {name:"申请类型"})).not.toBeInTheDocument();
  expect(screen.getByRole("link", {name:"申请进度"})).toHaveAttribute("href", "/workbench/supply/applications/progress");
  expect(screen.getByRole("link", {name:"申请记录"})).toHaveAttribute("href", "/workbench/supply/applications/records");
});
it("preserves ended/cursor while paging history", async () => {
  vi.mocked(listMarketRecords).mockResolvedValue({items:[], total:0, nextCursor:"next-history"});
  page("records");
  await screen.findByText("暂无申请记录");
  await userEvent.setup().click(screen.getByRole("button", {name:"下一页"}));
  await waitFor(() => expect(listMarketRecords).toHaveBeenLastCalledWith({userId:"member-a", organizationId:"org-a"}, {kind:"selected", ended:true, after:"next-history"}, false, expect.any(AbortSignal)));
});
it("keeps parent and platform type/state filters available", async () => {
  context.user.id = "specialist";
  page(undefined, true);
  await screen.findByText("暂无申请记录");
  await userEvent.setup().selectOptions(screen.getByRole("combobox", {name:"申请类型"}), "connection");
  await userEvent.setup().click(screen.getByRole("button", {name:"已结束"}));
  await waitFor(() => expect(listMarketRecords).toHaveBeenLastCalledWith({userId:"specialist", organizationId:""}, {kind:"connection", ended:true, after:undefined}, true, expect.any(AbortSignal)));
});
it("keeps the original member parent filters and member-scoped query", async () => {
  page();
  await screen.findByText("暂无申请记录");
  expect(screen.getByRole("heading", {name:"优选申请", level:1})).toBeVisible();
  await userEvent.setup().selectOptions(screen.getByRole("combobox", {name:"申请类型"}), "connection");
  await userEvent.setup().click(screen.getByRole("button", {name:"已结束"}));
  await waitFor(() => expect(listMarketRecords).toHaveBeenLastCalledWith({userId:"member-a", organizationId:"org-a"}, {kind:"connection", ended:true, after:undefined}, false, expect.any(AbortSignal)));
});
it("resets the cursor when changing the selected-application leaf", async () => {
  vi.mocked(listMarketRecords).mockResolvedValue({items:[], total:0, nextCursor:"history-cursor"});
  const client=new QueryClient({defaultOptions:{queries:{retry:false, gcTime:0}}});
  const view=render(<QueryClientProvider client={client}><SupplyApplicationsPage view="records"/></QueryClientProvider>);
  await screen.findByText("暂无申请记录");
  await userEvent.setup().click(screen.getByRole("button", {name:"下一页"}));
  await waitFor(() => expect(listMarketRecords).toHaveBeenLastCalledWith(expect.anything(), {kind:"selected", ended:true, after:"history-cursor"}, false, expect.any(AbortSignal)));
  view.rerender(<QueryClientProvider client={client}><SupplyApplicationsPage view="progress"/></QueryClientProvider>);
  await waitFor(() => expect(listMarketRecords).toHaveBeenLastCalledWith({userId:"member-a", organizationId:"org-a"}, {kind:"selected", ended:false, after:undefined}, false, expect.any(AbortSignal)));
});
it("does not query member applications until identity is ready", () => {
  context.isLoading = true;
  page("records");
  expect(listMarketRecords).not.toHaveBeenCalled();
  expect(screen.getByText("正在确认当前身份")).toBeVisible();
});
