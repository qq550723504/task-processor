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
      <ListingKitAppShell
        identity={{
          roles: ["listingkit_admin"],
          username: "zone",
          tenantId: "373211199677923496",
          userId: "user-1",
        }}
      >
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
    expect(screen.getByText("管理后台")).toBeInTheDocument();
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
    expect(screen.getByRole("button", { name: "业务运营" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "调度与导入" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "数据字典" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "策略规则" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "账号与系统" })).toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "SHEIN 登录" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "SDS 登录" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "SHEIN 活动报名" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "当前租户订阅" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "提示词" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "租户订阅管理" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "SHEIN 上架" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "任务列表" })).toHaveAttribute("href", "/listing-kits");
    expect(screen.getByRole("link", { name: "POD" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByText("当前页面")).toBeInTheDocument();
    expect(screen.getByText("/listing-kits/sds")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /zone/i })).toBeInTheDocument();
    expect(screen.getByText("workspace content")).toBeInTheDocument();
  });

  it("opens the account menu on the right side", async () => {
    const user = userEvent.setup();

    render(
      <ListingKitAppShell
        identity={{
          roles: ["listingkit_admin", "platform_admin"],
          username: "zone",
          tenantId: "373211199677923496",
          userId: "user-1",
        }}
      >
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    await user.click(screen.getByRole("button", { name: /zone/i }));

    expect(screen.getByText("当前账号")).toBeInTheDocument();
    expect(screen.getByText("当前租户")).toBeInTheDocument();
    expect(screen.getAllByText("373211199677923496").length).toBeGreaterThan(0);
    expect(screen.getByText("listingkit_admin, platform_admin")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "退出登录" })).toHaveAttribute(
      "href",
      "/api/zitadel-auth/logout",
    );
  });

  it("renders second-level menu sections that can expand child links", async () => {
    const user = userEvent.setup();

    render(
      <ListingKitAppShell identity={{ roles: ["listingkit_admin"] }}>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    expect(screen.queryByRole("link", { name: "我的店铺配置" })).not.toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "业务运营" }));

    expect(screen.getByRole("link", { name: "我的店铺配置" })).toHaveAttribute(
      "href",
      "/listing-kits/stores",
    );
    expect(screen.getByRole("link", { name: "SHEIN 活动报名" })).toHaveAttribute(
      "href",
      "/listing-kits/shein-enrollment",
    );
    expect(screen.getByRole("link", { name: "我的上架统计" })).toHaveAttribute(
      "href",
      "/listing-kits/store-statistics",
    );
    expect(screen.getByRole("link", { name: "平台店铺管理" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/stores",
    );
    expect(screen.getByRole("link", { name: "上架统计" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/store-statistics",
    );

    await user.click(screen.getByRole("button", { name: "调度与导入" }));

    expect(screen.getByRole("link", { name: "定时任务配置" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/scheduled-task-configs",
    );
    expect(screen.getByRole("link", { name: "调度事件" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/dispatch-events",
    );
    expect(screen.getByRole("link", { name: "任务导入" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/import-tasks",
    );

    await user.click(screen.getByRole("button", { name: "数据字典" }));

    expect(screen.getByRole("link", { name: "分类" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/categories",
    );
    expect(screen.getByRole("link", { name: "商品数据" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/product-data",
    );
    expect(screen.getByRole("link", { name: "导入映射" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/product-import-mappings",
    );

    await user.click(screen.getByRole("button", { name: "策略规则" }));

    expect(screen.getByRole("link", { name: "筛选规则" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/filter-rules",
    );
    expect(screen.getByRole("link", { name: "运营策略" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/operation-strategies",
    );
    expect(screen.getByRole("link", { name: "利润规则" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/profit-rules",
    );
    expect(screen.getByRole("link", { name: "核价规则" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/pricing-rules",
    );
    expect(screen.getByRole("link", { name: "敏感词" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/sensitive-words",
    );
    expect(screen.getByRole("link", { name: "生成禁用主题" })).toHaveAttribute(
      "href",
      "/listing-kits/admin/generation-topic-policies",
    );

    await user.click(screen.getByRole("button", { name: "账号与系统" }));

    expect(screen.getByRole("link", { name: "SHEIN 登录" })).toHaveAttribute(
      "href",
      "/listing-kits/shein-login",
    );
    expect(screen.getByRole("link", { name: "SDS 登录" })).toHaveAttribute(
      "href",
      "/listing-kits/sds-login",
    );
    expect(screen.getByRole("link", { name: "提示词" })).toHaveAttribute(
      "href",
      "/listing-kits/prompts",
    );
    expect(screen.getByRole("link", { name: "设置" })).toHaveAttribute(
      "href",
      "/listing-kits/settings",
    );
    expect(screen.getByRole("link", { name: "当前租户订阅" })).toHaveAttribute(
      "href",
      "/listing-kits/subscription",
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

    await user.click(screen.getByRole("button", { name: "账号与系统" }));

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
    expect(screen.getByText("管理后台")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "业务运营" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "账号与系统" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "调度与导入" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "数据字典" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "策略规则" })).not.toBeInTheDocument();
  });

  it("keeps privileged menu sections visible for administrators", () => {
    render(
      <ListingKitAppShell identity={{ roles: ["listingkit_admin"] }}>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    expect(screen.getByRole("button", { name: "业务运营" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "调度与导入" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "数据字典" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "策略规则" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "账号与系统" })).toBeInTheDocument();
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

    expect(screen.queryByRole("button", { name: "业务运营" })).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "账号与系统" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "调度与导入" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "数据字典" })).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "策略规则" })).not.toBeInTheDocument();
  });

  it("hides platform store links for tenant users", async () => {
    const user = userEvent.setup();

    render(
      <ListingKitAppShell identity={{ roles: ["listingkit_viewer"] }}>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    await user.click(screen.getByRole("button", { name: "业务运营" }));

    expect(screen.getByRole("link", { name: "我的店铺配置" })).toHaveAttribute(
      "href",
      "/listing-kits/stores",
    );
    expect(screen.getByRole("link", { name: "我的上架统计" })).toHaveAttribute(
      "href",
      "/listing-kits/store-statistics",
    );
    expect(screen.queryByRole("link", { name: "平台店铺管理" })).not.toBeInTheDocument();
    expect(screen.queryByRole("link", { name: "上架统计" })).not.toBeInTheDocument();
  });
});
