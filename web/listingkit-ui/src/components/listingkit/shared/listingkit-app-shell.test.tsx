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
    expect(screen.getByRole("navigation", { name: "ListingKit 主导航" })).toBeInTheDocument();
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

  it("places the navigation and main content on the same layout rail", () => {
    render(
      <ListingKitAppShell>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    const headerRail = screen.getByRole("banner").parentElement;
    const mainRail = screen.getByRole("main");

    expect(headerRail).toHaveClass("max-w-[1600px]");
    expect(headerRail).toHaveClass("px-4");
    expect(headerRail).toHaveClass("sm:px-6");
    expect(headerRail).toHaveClass("lg:px-8");
    expect(mainRail).toHaveClass("max-w-[1600px]");
    expect(mainRail).toHaveClass("px-4");
    expect(mainRail).toHaveClass("sm:px-6");
    expect(mainRail).toHaveClass("lg:px-8");
  });
});
