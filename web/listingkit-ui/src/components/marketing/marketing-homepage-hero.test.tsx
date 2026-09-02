import { act, render, screen, within } from "@testing-library/react";
import { useLayoutEffect, useRef } from "react";
import { renderToString } from "react-dom/server";

import { HeroSystemVisual } from "@/components/marketing/hero-system-visual";
import { MarketingHomepage } from "@/components/marketing/marketing-homepage";

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("MarketingHomepage hero", () => {
  it("presents the AI commerce operating-system promise and routes entry to the workbench", () => {
    render(<MarketingHomepage />);

    const hero = document.getElementById("home");
    expect(hero).not.toBeNull();
    expect(within(hero!).getByRole("heading", { level: 1 })).toHaveTextContent(
      "让智能，成为电商经营的默认能力",
    );
    expect(
      within(hero!).getByRole("link", { name: /进入硕米 OS/ }),
    ).toHaveAttribute("href", "/login?returnTo=%2Fworkbench");
    expect(
      within(hero!).getByRole("link", { name: "查看系统架构" }),
    ).toHaveAttribute("href", "#architecture");
  });

  it("uses the focused product navigation and sends the header entry to the workbench", () => {
    render(<MarketingHomepage />);

    const nav = screen.getByRole("navigation", { name: "官网导航" });
    expect(within(nav).getByRole("link", { name: "产品架构" })).toHaveAttribute(
      "href",
      "#architecture",
    );
    expect(within(nav).getByRole("link", { name: "能力中心" })).toHaveAttribute(
      "href",
      "#agents",
    );
    expect(within(nav).getByRole("link", { name: "场景方案" })).toHaveAttribute(
      "href",
      "#solutions",
    );
    expect(within(nav).queryByRole("link", { name: "开发者" })).not.toBeInTheDocument();
    expect(screen.getByRole("link", { name: "进入系统" })).toHaveAttribute(
      "href",
      "/login?returnTo=%2Fworkbench",
    );
  });

  it("describes the six real commerce-platform capabilities without a device-edge claim", () => {
    render(<MarketingHomepage />);

    const architecture = screen.getByRole("group", {
      name: "硕米 AI 电商能力架构",
    });
    const capabilities = within(architecture).getByRole("list");
    expect(within(capabilities).getAllByRole("listitem")).toHaveLength(6);
    for (const label of [
      "模型与调用治理",
      "智能体运行时",
      "商品智能",
      "电商工具",
      "上架执行面",
      "平台连接器",
    ]) {
      expect(within(capabilities).getByText(label)).toBeInTheDocument();
    }
    expect(within(architecture).queryByText("设备与边缘")).not.toBeInTheDocument();
    expect(architecture).toHaveAttribute(
      "data-motion-sequence",
      "boot-reveal-active-pulse",
    );
  });

  it("keeps the capability status text readable at its rendered size", () => {
    render(<MarketingHomepage />);

    expect(
      getComputedStyle(
        screen.getByText("Agent · 商品 · 工具 · 平台执行能力已连接"),
      ).color,
    ).toBe("rgb(157, 181, 216)");
  });

  it("allows capability labels to wrap inside narrow cards", () => {
    render(<MarketingHomepage />);

    expect(
      getComputedStyle(screen.getByText("模型与调用治理")).whiteSpace,
    ).toBe("normal");
  });
});

it("keeps the architecture visible in server-rendered markup", () => {
  const markup = renderToString(<HeroSystemVisual />);

  expect(markup).toContain("模型与调用治理");
  expect(markup).not.toMatch(/opacity:0(?:;|\"|})/);
});

it("enters the boot state without interpolating from the active state", async () => {
  const frames: FrameRequestCallback[] = [];
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
    frames.push(callback);
    return frames.length;
  });
  vi.stubGlobal("cancelAnimationFrame", vi.fn());

  render(<HeroSystemVisual />);

  await act(async () => {
    await new Promise((resolve) => window.setTimeout(resolve, 30));
  });

  const architecture = screen.getByRole("group", {
    name: "硕米 AI 电商能力架构",
  });
  const core = architecture.querySelector('[class*="coreAnchor"]');
  expect(core).toHaveStyle({
    opacity: "0",
  });

  act(() => frames.shift()?.(0));
});

it("applies the boot state before the first hydrated paint", () => {
  const frames: FrameRequestCallback[] = [];
  const layoutSnapshots: string[] = [];
  vi.stubGlobal("requestAnimationFrame", (callback: FrameRequestCallback) => {
    frames.push(callback);
    return frames.length;
  });
  vi.stubGlobal("cancelAnimationFrame", vi.fn());

  function LayoutProbe() {
    const rootRef = useRef<HTMLDivElement>(null);

    useLayoutEffect(() => {
      const core = rootRef.current?.querySelector('[class*="coreAnchor"]');
      layoutSnapshots.push(core?.getAttribute("style") ?? "");
    }, []);

    return (
      <div ref={rootRef}>
        <HeroSystemVisual />
      </div>
    );
  }

  render(<LayoutProbe />);

  expect(layoutSnapshots[0]).toContain("opacity: 0");
});
