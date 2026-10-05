import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, expect, it } from "vitest";
import { ConsoleNavigation } from "./console-navigation";
import { findConsoleRoute } from "@/lib/workbench/console-navigation";

afterEach(cleanup);

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
