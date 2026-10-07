import type { ReactNode } from "react";

// Motion is checked in the browser; component tests exercise content and interaction.
vi.mock("@/components/marketing/marketing-motion", () => ({
  MotionLayer: ({ children, className, nodeId }: { children: ReactNode; className?: string; nodeId: string }) => <div className={className} data-node-id={nodeId}>{children}</div>,
}));
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { renderToString } from "react-dom/server";
import { MarketingHero } from "./marketing-hero";

describe("MarketingHero", () => {
  it("keeps all capability labels and the brand promise in server-rendered markup", () => {
    const markup = renderToString(<MarketingHero loginHref="/login?returnTo=%2Fworkbench" />);
    for (const label of ["硕米智能引擎", "新一代AI电商", "智能操作系统", "AI智能体", "全球商品数据", "供应链货盘", "生态服务"]) {
      expect(markup).toContain(label);
    }
  });

  it("lets mobile visitors navigate to every existing homepage section", async () => {
    const user = userEvent.setup();
    render(<MarketingHero loginHref="/login?returnTo=%2Fworkbench" />);
    await user.click(screen.getByLabelText("展开官网导航"));
    const nav = screen.getByLabelText("移动端官网导航", { selector: "nav" });
    expect(within(nav).getAllByRole("link", { hidden: true })).toHaveLength(7);
    const pricing = within(nav).getByText("价格与服务");
    expect(pricing).toHaveAttribute("href", "#pricing");
    await user.click(pricing);
    expect(nav.closest("details")).not.toHaveAttribute("open");
  });
});
