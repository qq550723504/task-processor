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
    expect(screen.getByText("源信息 -> Canonical Product -> 平台资料")).toBeInTheDocument();
    expect(screen.getByRole("navigation", { name: "ListingKit 主导航" })).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "首页" })).toHaveAttribute("href", "/");
    expect(screen.getByRole("link", { name: "新建任务" })).toHaveAttribute("href", "/listing-kits/new");
    expect(screen.getByRole("link", { name: "SDS 源" })).toHaveAttribute("href", "/listing-kits/sds");
    expect(screen.getByRole("link", { name: "款式图库" })).toHaveAttribute(
      "href",
      "/listing-kits/style-gallery",
    );
    expect(screen.getByRole("link", { name: "Canonical Products" })).toHaveAttribute(
      "href",
      "/listing-kits/canonical-products",
    );
    expect(screen.queryByRole("link", { name: "SHEIN 上架" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "任务列表" })).toHaveAttribute("href", "/listing-kits");
    expect(screen.getByRole("link", { name: "SDS 源" })).toHaveAttribute(
      "aria-current",
      "page",
    );
    expect(screen.getByText("当前页面")).toBeInTheDocument();
    expect(screen.getByText("/listing-kits/sds")).toBeInTheDocument();
    expect(screen.getByText("workspace content")).toBeInTheDocument();
  });
});
