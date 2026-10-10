import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ecoRequest } from "@/lib/api/ecoservices";
import { ConsoleNavigation } from "./console-navigation";
import { findConsoleRoute } from "@/lib/workbench/console-navigation";

const query=vi.hoisted(()=>({value:""}));
it("advertises each Store observation entry only with ready runtime and its read permission", () => {
  const view=render(<ConsoleNavigation pathname="/workbench/store-orders" ariaLabel="主导航" storeProductsAvailable={false} storeOrdersAvailable={false}/>);
  expect(screen.getByRole("link",{name:"订单履约"})).toHaveAttribute("title","订单履约：业务暂未启用");
  view.rerender(<ConsoleNavigation pathname="/workbench/store-orders" ariaLabel="主导航" storeProductsAvailable={false} storeOrdersAvailable/>);
  expect(screen.getByRole("link",{name:"订单履约"})).toHaveAttribute("title","订单履约");
  expect(screen.getByRole("link",{name:"店铺商品"})).toHaveAttribute("title","店铺商品：业务暂未启用");
  view.rerender(<ConsoleNavigation pathname="/workbench/store-orders" ariaLabel="主导航" storeProductsAvailable storeOrdersAvailable/>);
  expect(screen.getByRole("link",{name:"店铺商品"})).toHaveAttribute("title","店铺商品");
});
vi.mock("next/navigation",()=>({useSearchParams:()=>new URLSearchParams(query.value)}));
afterEach(()=>{cleanup();query.value="";});

vi.mock("@/lib/api/ecoservices", async (importOriginal) => ({ ...await importOriginal<typeof import("@/lib/api/ecoservices")>(), ecoRequest: vi.fn() }));

it("discovers platform review through the original global backend gate without enterprise grants", async () => {
  vi.mocked(ecoRequest).mockResolvedValue({ applications: [], total: "0" });
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const view = render(<QueryClientProvider client={client}><ConsoleNavigation pathname="/workbench/services/review" ariaLabel="主导航" ecoservicesAvailable userId="platform-user" /></QueryClientProvider>);
  expect(await screen.findByRole("link", { name: "平台审核" })).toHaveAttribute("href", "/workbench/services/review");
  expect(screen.getByRole("link", { name: "平台审核" })).toHaveAttribute("aria-current", "page");
  expect(ecoRequest).toHaveBeenCalledWith({ userId: "platform-user", organizationId: "" }, "applications?page=1&pageSize=20", expect.anything(), expect.anything(), true);
  vi.mocked(ecoRequest).mockRejectedValue(new Error("ECOSERVICES_FORBIDDEN"));
  view.rerender(<QueryClientProvider client={client}><ConsoleNavigation pathname="/workbench/services/review" ariaLabel="主导航" ecoservicesAvailable userId="enterprise-admin-only" /></QueryClientProvider>);
  expect(screen.queryByRole("link", { name: "平台审核" })).not.toBeInTheDocument();
  view.rerender(<QueryClientProvider client={client}><ConsoleNavigation pathname="/workbench/services/review" ariaLabel="主导航" ecoservicesAvailable={false} userId="platform-user" /></QueryClientProvider>);
  expect(screen.queryByRole("link", { name: "平台审核" })).not.toBeInTheDocument();
});


it("only advertises Chat and BusinessTask when the current application mounts AI Workbench", () => {
  const view = render(<ConsoleNavigation pathname="/workbench/ai/chat" ariaLabel="主导航" aiWorkbenchAvailable={false} />);
  expect(screen.queryByRole("link", { name: "硕米Chat" })).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "任务中心" })).not.toBeInTheDocument();
  view.rerender(<ConsoleNavigation pathname="/workbench/ai/chat" ariaLabel="主导航" aiWorkbenchAvailable />);
  expect(screen.getByRole("link", { name: "硕米Chat" })).toBeVisible();
  expect(screen.getByRole("link", { name: "任务中心" })).toBeVisible();
});

it("keeps independently available Review and history pages discoverable without AI Workbench", async () => {
  render(<ConsoleNavigation pathname="/workbench" ariaLabel="主导航" aiWorkbenchAvailable={false} productReviewAvailable sheinRecordsAvailable />);
  await userEvent.click(screen.getByRole("button", { name: "展开AI工作台" }));
  expect(screen.queryByRole("link", { name: "硕米Chat" })).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "任务中心" })).toHaveAttribute("href", "/workbench/ai/tasks");
  await userEvent.click(screen.getByRole("button", { name: "展开任务中心" }));
  expect(screen.getByRole("link", { name: "待确认" })).toBeVisible();
  expect(screen.getByRole("link", { name: "已完成" })).toBeVisible();
  expect(screen.queryByRole("link", { name: "进行中" })).not.toBeInTheDocument();
  expect(screen.queryByRole("link", { name: "异常任务" })).not.toBeInTheDocument();
  expect(screen.getByRole("link", { name: "标题审核" })).toHaveAttribute("href", "/workbench/ai/tasks/pending/other");
  expect(screen.getByRole("link", { name: "历史工作记录" })).toHaveAttribute("href", "/workbench/ai/tasks/completed/history");
});

it("shows only the configured Task Center branch without AI Workbench", async () => {
  for (const item of [
    { productReviewAvailable: true, sheinRecordsAvailable: false, visible: "待确认", hidden: "已完成" },
    { productReviewAvailable: false, sheinRecordsAvailable: true, visible: "已完成", hidden: "待确认" },
  ]) {
    const view = render(<ConsoleNavigation pathname="/workbench" ariaLabel="主导航" aiWorkbenchAvailable={false} productReviewAvailable={item.productReviewAvailable} sheinRecordsAvailable={item.sheinRecordsAvailable} />);
    await userEvent.click(screen.getByRole("button", { name: "展开AI工作台" }));
    await userEvent.click(screen.getByRole("button", { name: "展开任务中心" }));
    expect(screen.getByRole("link", { name: item.visible })).toBeVisible();
    expect(screen.queryByRole("link", { name: item.hidden })).not.toBeInTheDocument();
    view.unmount();
  }
});

it("hides the acquisition entry unless the serving deployment enables it", async () => {
  const view = render(<ConsoleNavigation pathname="/workbench" ariaLabel="主导航" productAcquisitionAvailable={false} />);
  expect(screen.queryByRole("link", { name: "1688采集" })).not.toBeInTheDocument();
  view.rerender(<ConsoleNavigation pathname="/workbench" ariaLabel="主导航" productAcquisitionAvailable />);
  await userEvent.click(screen.getByRole("button", { name: "展开供应市场" }));
  expect(screen.getByRole("link", { name: "1688采集" })).toBeVisible();
});

it("reveals the selected account page after navigation from another module", async () => {
  const view = render(<ConsoleNavigation pathname="/workbench/stores" ariaLabel="主导航" />);
  view.rerender(<ConsoleNavigation pathname="/workbench/account/organization/members" ariaLabel="主导航" />);
  expect(screen.getByRole("link", { name: "成员与权限" })).toHaveAttribute("aria-current", "page");
  await userEvent.click(screen.getByRole("button", { name: "收起企业空间" }));
  expect(screen.queryByRole("link", { name: "成员与权限" })).not.toBeInTheDocument();
  view.rerender(<ConsoleNavigation pathname="/workbench/account/organization/audit" ariaLabel="主导航" />);
  expect(screen.getByRole("link", { name: "操作记录" })).toHaveAttribute("aria-current", "page");
  view.rerender(<ConsoleNavigation pathname="/workbench/account/organization/members" ariaLabel="主导航" />);
  expect(screen.getByRole("link", { name: "成员与权限" })).toHaveAttribute("aria-current", "page");
});

it("keeps the plans module as a connected five-page tree", () => {
  const pages = [
    ["/workbench/plans", "套餐与权益"],
    ["/workbench/plans/options", "套餐方案"],
    ["/workbench/plans/entitlements", "我的权益"],
    ["/workbench/plans/usage", "用量明细"],
    ["/workbench/plans/top-up", "充值中心"],
    ["/workbench/plans/orders", "账单与订单"],
  ] as const;

  for (const [pathname, label] of pages) {
    const route = findConsoleRoute(pathname);
    expect(route?.node.label).toBe(label);
    expect(route?.node.availability).toBe("connected");
    expect(route?.trail.map((item) => item.label)).toEqual(
      pathname === "/workbench/plans"
        ? ["套餐与权益"]
        : ["套餐与权益", label],
    );
  }
});

it("expands the plans module and selects a child route", () => {
  const view = render(<ConsoleNavigation pathname="/workbench/plans/usage" ariaLabel="主导航" />);
  expect(screen.getByRole("button", { name: "收起套餐与权益" })).toBeVisible();
  expect(screen.getByRole("link", { name: "用量明细" })).toHaveAttribute("aria-current", "page");
  expect(screen.getByRole("link", { name: "充值中心" })).toBeVisible();
  view.rerender(<ConsoleNavigation pathname="/workbench/plans/orders" ariaLabel="主导航" />);
  expect(screen.getByRole("link", { name: "账单与订单" })).toHaveAttribute("aria-current", "page");
});

it("retains three-level route trails outside the plans module", () => {
  const route = findConsoleRoute("/workbench/account/profile/settings");
  expect(route?.trail.map((item) => item.label)).toEqual([
    "我的账户",
    "账户资料",
    "账户设置",
  ]);
});

it("only opens supply stage navigation when the serving runtime enables it",async()=>{
 const view=render(<ConsoleNavigation pathname="/workbench/supply/mine/ready" ariaLabel="主导航" supplyChainAvailable={false}/>);
 expect(screen.queryByRole("link",{name:"已适配"})).not.toBeInTheDocument();
 view.rerender(<ConsoleNavigation pathname="/workbench/supply/mine/ready" ariaLabel="主导航" supplyChainAvailable/>);
 expect(screen.getByRole("link",{name:"已适配"})).toHaveAttribute("href","/workbench/supply/mine/ready");expect(screen.getByRole("link",{name:"已适配"})).toHaveAttribute("aria-current","page");
 expect(screen.getByRole("link",{name:"待补全"})).toHaveAttribute("href","/workbench/supply/mine/missing");
});

it("retains only valid batch and store selection across supply stage links",()=>{
 query.value="preparation=11111111-1111-4111-8111-111111111111&store=22222222-2222-4222-8222-222222222222&operation=discard";
 const view=render(<ConsoleNavigation pathname="/workbench/supply/mine/waiting" ariaLabel="主导航" supplyChainAvailable/>);
 expect(screen.getByRole("link",{name:"待审核"})).toHaveAttribute("href","/workbench/supply/mine/review?preparation=11111111-1111-4111-8111-111111111111&store=22222222-2222-4222-8222-222222222222");
 expect(screen.getByRole("link",{name:"待审核"})).toHaveAttribute("title","待审核");
 query.value="preparation=bad&store=bad";view.rerender(<ConsoleNavigation pathname="/workbench/supply/mine/waiting" ariaLabel="主导航" supplyChainAvailable/>);
 expect(screen.getByRole("link",{name:"待审核"})).toHaveAttribute("href","/workbench/supply/mine/review");
});
