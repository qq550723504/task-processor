import { cleanup, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, describe, expect, it, vi } from "vitest";

import { ListingKitAppShell } from "@/components/listingkit/shared/listingkit-app-shell";

vi.mock("next/navigation", () => ({
  usePathname: () => "/listing-kits/sds",
}));

describe("ListingKitAppShell", () => {
  afterEach(() => {
    vi.unstubAllEnvs();
    vi.resetModules();
    cleanup();
  });

  it("renders the main ListingKit workflow navigation", () => {
    render(
      <ListingKitAppShell identity={{ roles: ["listingkit_admin"] }}>
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
    expect(screen.getByRole("button", { name: "店铺运营" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "数据配置" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "规则策略" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "订阅计费" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "系统配置" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "SHEIN 登录" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "当前租户订阅" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "提示词" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "租户订阅管理" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "SHEIN 上架" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "任务列表" })).toHaveAttribute("href", "/listing-kits");
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

  it("renders second-level menu sections that can expand child links", async () => {
    const user = userEvent.setup();

    render(
      <ListingKitAppShell identity={{ roles: ["listingkit_admin"] }}>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    expect(screen.queryByRole("link", { name: "店铺" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "店铺运营" }));

    expect(screen.getByRole("link", { name: "店铺" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/stores",
    );
    expect(screen.getByRole("link", { name: "上架统计" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/store-statistics",
    );

    await user.click(screen.getByRole("button", { name: "规则策略" }));

    expect(screen.getByRole("link", { name: "运营策略" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/operation-strategies",
    );
    expect(screen.getByRole("link", { name: "SHEIN 登录" })).toHaveAttribute(
      "href",
      "/listing-kits/shein-login",
    );

    await user.click(screen.getByRole("button", { name: "系统配置" }));

    expect(screen.getByRole("link", { name: "提示词" })).toHaveAttribute(
      "href",
      "/listing-kits/prompts",
    );
    expect(screen.getByRole("link", { name: "设置" })).toHaveAttribute(
      "href",
      "/listing-kits/settings",
    );
    expect(screen.queryByRole("link", { name: "用户管理" })).not.toBeInTheDocument();
  });

  it("renders the ZITADEL console link only when explicitly configured", async () => {
    vi.stubEnv(
      "NEXT_PUBLIC_ZITADEL_CONSOLE_URL",
      "https://auth.example.com/ui/console",
    );
    const { ListingKitAppShell: ConfiguredShell } = await import(
      "@/components/listingkit/shared/listingkit-app-shell"
    );

    const user = userEvent.setup();
    render(
      <ConfiguredShell identity={{ roles: ["listingkit_admin"] }}>
        <div>workspace content</div>
      </ConfiguredShell>,
    );

    await user.click(screen.getByRole("button", { name: "系统配置" }));

    expect(screen.getByRole("link", { name: "用户管理" })).toHaveAttribute(
      "href",
      "https://auth.example.com/ui/console",
    );
    expect(screen.getByRole("link", { name: "用户管理" })).toHaveAttribute(
      "target",
      "_blank",
    );
  });

  it("filters privileged menu sections by ZITADEL roles", () => {
    render(
      <ListingKitAppShell identity={{ roles: ["listingkit_viewer"] }}>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    expect(screen.getByRole("link", { name: "POD" })).toBeInTheDocument();
    expect(screen.getByText("运营管理")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "店铺运营" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "数据配置" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "规则策略" })).toBeInTheDocument();
    expect(screen.getByText("系统")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "系统配置" })).toBeInTheDocument();
  });

  it("keeps privileged menu sections visible for administrators", () => {
    render(
      <ListingKitAppShell identity={{ roles: ["listingkit_admin"] }}>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    expect(screen.getByRole("button", { name: "店铺运营" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "系统配置" })).toBeInTheDocument();
  });

  it("renders main content in the sidebar inset layout", () => {
    render(
      <ListingKitAppShell identity={{ roles: ["listingkit_admin"] }}>
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

  it("collapses and expands the desktop sidebar navigation", async () => {
    const user = userEvent.setup();

    render(
      <ListingKitAppShell identity={{ roles: ["listingkit_admin"] }}>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    const sidebar = screen.getByLabelText("ListingKit");
    const toggle = within(sidebar).getByRole("button", { name: "折叠菜单" });

    expect(sidebar).toHaveAttribute("data-state", "expanded");

    await user.click(toggle);

    expect(sidebar).toHaveAttribute("data-state", "collapsed");
    expect(screen.getByRole("link", { name: "POD" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(toggle).toHaveAccessibleName("展开菜单");

    await user.click(toggle);

    expect(sidebar).toHaveAttribute("data-state", "expanded");
    expect(toggle).toHaveAccessibleName("折叠菜单");
  });

  it("keeps all menu sections visible before identity is loaded", () => {
    render(
      <ListingKitAppShell>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    expect(screen.getByText("运营管理")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "店铺运营" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "数据配置" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "规则策略" })).toBeInTheDocument();
    expect(screen.getByText("系统")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "系统配置" })).toBeInTheDocument();
  });
});
