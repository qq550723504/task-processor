import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ListingKitAppShell } from "@/components/listingkit/shared/listingkit-app-shell";

vi.mock("next/navigation", () => ({
  usePathname: () => "/listing-kits/sds",
}));

describe("ListingKitAppShell", () => {
  it("renders the main ListingKit workflow navigation", () => {
    render(
      <ListingKitAppShell>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    expect(screen.getByText("ListingKit")).toBeInTheDocument();
    expect(screen.getByText("源信息 -> 标准商品 -> 平台资料")).toBeInTheDocument();
    const sidebarNav = screen.getByRole("navigation", {
      name: "ListingKit 侧边栏导航",
    });

    expect(sidebarNav).toBeInTheDocument();
    expect(sidebarNav.closest("[data-slot='sidebar']")).toBeInTheDocument();
    expect(screen.getByText("主流程")).toBeInTheDocument();
    expect(screen.getByText("运营管理")).toBeInTheDocument();
    expect(screen.getByText("系统")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "首页" })).toHaveAttribute("href", "/");
    expect(screen.getByRole("link", { name: "新建任务" })).toHaveAttribute("href", "/listing-kits/new");
    expect(screen.getByRole("link", { name: "POD" })).toHaveAttribute("href", "/listing-kits/sds");
    expect(screen.getByRole("link", { name: "款式图库" })).toHaveAttribute(
      "href",
      "/listing-kits/style-gallery",
    );
    expect(screen.getByRole("link", { name: "标准商品" })).toHaveAttribute(
      "href",
      "/listing-kits/canonical-products",
    );
    expect(screen.getByRole("link", { name: "SHEIN 登录" })).toHaveAttribute(
      "href",
      "/listing-kits/shein-login",
    );
    expect(screen.getByRole("link", { name: "订阅" })).toHaveAttribute(
      "href",
      "/listing-kits/subscription",
    );
    expect(screen.getByRole("link", { name: "提示词" })).toHaveAttribute(
      "href",
      "/listing-kits/prompts",
    );
    expect(screen.getByRole("link", { name: "平台订阅" })).toHaveAttribute(
      "href",
      "/listing-kits/platform/subscriptions",
    );
    expect(screen.queryByRole("link", { name: "SHEIN 上架" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "任务列表" })).toHaveAttribute("href", "/listing-kits");
    expect(screen.getByRole("link", { name: "设置" })).toHaveAttribute(
      "href",
      "/listing-kits/settings",
    );
    expect(screen.getByRole("link", { name: "用户管理" })).toHaveAttribute(
      "href",
      "http://localhost:8080/ui/console",
    );
    expect(screen.getByRole("link", { name: "用户管理" })).toHaveAttribute(
      "target",
      "_blank",
    );
    expect(screen.getByRole("link", { name: "店铺" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/stores",
    );
    expect(screen.getByRole("link", { name: "运营策略" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/operation-strategies",
    );
    expect(screen.getByRole("link", { name: "退出登录" })).toHaveAttribute(
      "href",
      "/api/zitadel-auth/logout",
    );
    expect(screen.getByRole("link", { name: "POD" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByText("当前页面")).toBeInTheDocument();
    expect(screen.getByText("/listing-kits/sds")).toBeInTheDocument();
    expect(screen.getByText("workspace content")).toBeInTheDocument();
  });

  it("renders main content in the sidebar inset layout", () => {
    render(
      <ListingKitAppShell>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    const mainRail = screen.getByRole("main");

    expect(screen.queryByRole("banner")).not.toBeInTheDocument();
    expect(mainRail.closest("[data-slot='sidebar-inset']")).toBeInTheDocument();
    expect(mainRail).toHaveClass("max-w-[1600px]");
    expect(mainRail).toHaveClass("px-4");
    expect(mainRail).toHaveClass("sm:px-6");
    expect(mainRail).toHaveClass("lg:px-8");
  });
});
