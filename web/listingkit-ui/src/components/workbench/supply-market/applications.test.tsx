import {cleanup, render, screen, waitFor, within} from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {QueryClient, QueryClientProvider} from "@tanstack/react-query";
import {afterEach, beforeEach, expect, it, vi} from "vitest";
import {listMarketRecords,readApplicationOverview} from "@/lib/api/supply-market";
import {SupplyApplicationsPage} from "./applications";

const context = vi.hoisted(() => ({user:{id:"member-a"}, effectiveOrganization:{id:"org-a"}, isLoading:false, isSwitching:false, error:null, blockingError:null}));
vi.mock("@/components/providers/workbench-context-provider", () => ({useWorkbenchContext:() => context}));
vi.mock("@/lib/api/supply-market", async importOriginal => ({...await importOriginal<typeof import("@/lib/api/supply-market")>(), listMarketRecords:vi.fn(),readApplicationOverview:vi.fn()}));
afterEach(cleanup);
beforeEach(() => {
  context.user.id = "member-a";
  context.isLoading = false;
  context.isSwitching = false;
  context.effectiveOrganization.id = "org-a";
  vi.mocked(listMarketRecords).mockReset().mockResolvedValue({items:[], total:0});
  vi.mocked(readApplicationOverview).mockReset().mockResolvedValue({eligibleProducts:54,reviewing:27,supplementRequired:4,approved:7,products:[{itemId:"12752596-6056-4316-9f2f-380c97df9675",title:"当前优化商品",thumbnailUrl:"https://images.example.org/real.png",groupName:"真实分组",source:"own"}]});
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
it("keeps platform type/state filters available", async () => {
  context.user.id = "specialist";
  page(undefined, true);
  await screen.findByText("暂无申请记录");
  await userEvent.setup().selectOptions(screen.getByRole("combobox", {name:"申请类型"}), "connection");
  await userEvent.setup().click(screen.getByRole("button", {name:"已结束"}));
  await waitFor(() => expect(listMarketRecords).toHaveBeenLastCalledWith({userId:"specialist", organizationId:""}, {kind:"connection", ended:true, after:undefined}, true, expect.any(AbortSignal)));
});
it("replaces the generic member list with the real selected overview and existing entry points", async () => {
  page();
  await screen.findByText("当前优化商品");
  expect(screen.getByRole("heading", {name:"优选申请", level:1})).toBeVisible();
  expect(screen.getByText("54")).toBeVisible();expect(screen.getByText("27")).toBeVisible();expect(screen.getByText("4")).toBeVisible();expect(screen.getByText("7")).toBeVisible();
  expect(screen.getByText("真实分组")).toBeVisible();expect(screen.getByRole("img",{name:"当前优化商品"})).toHaveAttribute("src","https://images.example.org/real.png");
  expect(screen.getByRole("link",{name:"选择商品发起申请"})).toHaveAttribute("href","/workbench/supply/applications/new");expect(screen.getByRole("link",{name:"查看申请记录"})).toHaveAttribute("href","/workbench/supply/applications/records");
  expect(screen.getByRole("heading",{name:"申请条件"})).toBeVisible();expect(screen.getByRole("heading",{name:"申请流程"})).toBeVisible();expect(screen.queryByRole("combobox",{name:"申请类型"})).not.toBeInTheDocument();
  expect(readApplicationOverview).toHaveBeenCalledWith({userId:"member-a",organizationId:"org-a"},expect.any(AbortSignal));expect(listMarketRecords).not.toHaveBeenCalled();
});
it("shows genuine zero only after a complete empty overview",async()=>{vi.mocked(readApplicationOverview).mockResolvedValue({eligibleProducts:0,reviewing:0,supplementRequired:0,approved:0,products:[]});page();await screen.findByText("暂无可申请商品");expect(screen.getAllByText("0")).toHaveLength(4)});
it("does not replace an overview read failure with zero or an empty preview",async()=>{vi.mocked(readApplicationOverview).mockRejectedValue(new Error("unavailable"));page();await screen.findByRole("button",{name:"重新读取"});expect(screen.queryByText("0")).not.toBeInTheDocument();expect(screen.queryByText("暂无可申请商品")).not.toBeInTheDocument()});
it("hides previously loaded facts when refreshing the overview fails", async () => {
  const client = new QueryClient({defaultOptions:{queries:{retry:false, gcTime:0}}});
  render(<QueryClientProvider client={client}><SupplyApplicationsPage/></QueryClientProvider>);
  await screen.findByText("当前优化商品");
  vi.mocked(readApplicationOverview).mockRejectedValue(new Error("unavailable"));
  await client.invalidateQueries({queryKey:["supply-market", "member-a", "org-a", "application-overview"]});
  await screen.findByRole("button", {name:"重新读取"});
  expect(screen.queryByText("54")).not.toBeInTheDocument();
  expect(screen.queryByText("当前优化商品")).not.toBeInTheDocument();
  expect(screen.queryByText("0")).not.toBeInTheDocument();
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
it("does not query the overview while confirming identity", () => {
  context.isLoading = true;
  page();
  expect(readApplicationOverview).not.toHaveBeenCalled();
  expect(screen.getByText("正在确认当前身份")).toBeVisible();
});
it("removes old overview facts and cancels the read when the enterprise changes", async () => {
  const client = new QueryClient({defaultOptions:{queries:{retry:false, gcTime:0}}});
  const view = render(<QueryClientProvider client={client}><SupplyApplicationsPage/></QueryClientProvider>);
  await screen.findByText("当前优化商品");
  const previousSignal = vi.mocked(readApplicationOverview).mock.calls[0]![1]!;
  vi.mocked(readApplicationOverview).mockImplementation((_scope, signal) => new Promise((_resolve, reject) => {
    signal?.addEventListener("abort", () => reject(new DOMException("Aborted", "AbortError")));
  }));
  context.effectiveOrganization.id = "org-b";
  view.rerender(<QueryClientProvider client={client}><SupplyApplicationsPage/></QueryClientProvider>);
  await waitFor(() => expect(readApplicationOverview).toHaveBeenLastCalledWith({userId:"member-a", organizationId:"org-b"}, expect.any(AbortSignal)));
  expect(screen.queryByText("当前优化商品")).not.toBeInTheDocument();
  expect(screen.queryByText("54")).not.toBeInTheDocument();
  // Only an in-flight read needs cancellation; a completed read remains scoped.
  expect(previousSignal).toBeInstanceOf(AbortSignal);
  const pendingSignal = vi.mocked(readApplicationOverview).mock.calls[1]![1]!;
  context.isSwitching = true;
  view.rerender(<QueryClientProvider client={client}><SupplyApplicationsPage/></QueryClientProvider>);
  expect(screen.getByText("正在确认当前身份")).toBeVisible();
  await waitFor(() => expect(pendingSignal.aborted).toBe(true));
});
