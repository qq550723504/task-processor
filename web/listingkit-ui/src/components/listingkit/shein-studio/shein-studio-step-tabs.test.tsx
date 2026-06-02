import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SheinStudioStepTabs } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";

const replaceBrowserHistory = vi.fn();

vi.mock("next/navigation", () => ({
  usePathname: () => "/listing-kits/sds/new",
}));

vi.mock("@/lib/utils/live-search-params", () => ({
  useLiveSearchParams: () => new URLSearchParams("step=select"),
}));

vi.mock("@/lib/utils/browser-history", () => ({
  replaceBrowserHistory: (href: string) => replaceBrowserHistory(href),
}));

describe("SheinStudioStepTabs", () => {
  it("uses progressive responsive grids in compact layout", () => {
    const { container } = render(
      <SheinStudioStepTabs
        activeStep="select"
        hasSelection
        layout="compact"
      />,
    );

    const nav = container.querySelector("nav") as HTMLElement | null;
    expect(nav).not.toBeNull();
    expect(nav?.className).not.toContain("md:grid-cols-4");
  });

  it("keeps locked steps inert until a selection exists", () => {
    render(
      <SheinStudioStepTabs
        activeStep="select"
        hasSelection={false}
        layout="studio"
      />,
    );

    const lockedStep = screen.getByRole("button", { name: /2. 生成图片/ });
    fireEvent.click(lockedStep);
    expect(replaceBrowserHistory).not.toHaveBeenCalled();
  });
});
