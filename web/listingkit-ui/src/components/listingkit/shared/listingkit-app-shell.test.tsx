import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ListingKitAppShell } from "@/components/listingkit/shared/listingkit-app-shell";

vi.mock("next/navigation", () => ({
  usePathname: () => "/listing-kits/shein",
}));

describe("ListingKitAppShell", () => {
  it("renders a shared product header with home link and current path", () => {
    render(
      <ListingKitAppShell>
        <div>workspace content</div>
      </ListingKitAppShell>,
    );

    expect(screen.getByText("ListingKit")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "返回首页" })).toHaveAttribute(
      "href",
      "/",
    );
    expect(screen.getByText("当前页面")).toBeInTheDocument();
    expect(screen.getByText("/listing-kits/shein")).toBeInTheDocument();
    expect(screen.getByText("workspace content")).toBeInTheDocument();
  });
});
