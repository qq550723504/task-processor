import { cleanup, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

import { ListingKitAppShell } from "@/components/listingkit/shared/listingkit-app-shell";

const navigation = vi.hoisted(() => ({
  pathname: "/listing-kits/sds",
}));

vi.mock("next/navigation", () => ({
  usePathname: () => navigation.pathname,
  useRouter: () => ({ replace: vi.fn() }),
  useSearchParams: () => new URLSearchParams(),
}));

describe("ListingKit product-oriented navigation", () => {
  beforeEach(() => {
    navigation.pathname = "/listing-kits/sds";
  });

  afterEach(() => {
    vi.unstubAllEnvs();
    vi.resetModules();
    cleanup();
  });

  it("groups current product surfaces under product language without changing routes", () => {
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

    expect(screen.getByText("商品 → 平台 → 上架")).toBeInTheDocument();
    expect(screen.getByText("工作")).toBeInTheDocument();
    expect(screen.getByText("管理")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "工作台" })).toHaveAttribute(
      "href",
      "/listing-kits/home",
    );

    const productSection = screen.getByRole("button", { name: "商品" });
    expect(productSection).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByRole("link", { name: "商品中心" })).toHaveAttribute(
      "href",
      "/listing-kits/canonical-products",
    );
    expect(screen.getByRole("link", { name: "导入商品" })).toHaveAttribute(
      "href",
      "/listing-kits/new",
    );
    expect(screen.getByRole("link", { name: "POD" })).toHaveAttribute(
      "href",
      "/listing-kits/sds",
    );
    expect(screen.getByRole("link", { name: "POD" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByRole("link", { name: "款式图库" })).toHaveAttribute(
      "href",
      "/listing-kits/style-gallery",
    );
    expect(screen.getByRole("link", { name: "执行记录" })).toHaveAttribute(
      "href",
      "/listing-kits",
    );
  });

  it("expands the product group when navigation activates a child route", () => {
    navigation.pathname = "/listing-kits/home";
    const { rerender } = render(
      <ListingKitAppShell identity={{ roles: ["listingkit_admin"] }}>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    expect(screen.getByRole("button", { name: "商品" })).toHaveAttribute(
      "aria-expanded",
      "false",
    );
    expect(screen.queryByRole("link", { name: "商品中心" })).not.toBeInTheDocument();

    navigation.pathname = "/listing-kits/canonical-products";
    rerender(
      <ListingKitAppShell identity={{ roles: ["listingkit_admin"] }}>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    expect(screen.getByRole("button", { name: "商品" })).toHaveAttribute(
      "aria-expanded",
      "true",
    );
    expect(screen.getByRole("link", { name: "商品中心" })).toHaveAttribute(
      "aria-current",
      "page",
    );
  });

  it("preserves tenant context while navigation language changes", async () => {
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

    expect(screen.getByText("当前租户")).toBeInTheDocument();
    expect(screen.getAllByText("373211199677923496").length).toBeGreaterThan(0);
    expect(screen.getByRole("combobox", { name: "代管租户 ID" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "切换租户" })).toBeInTheDocument();
  });
});
