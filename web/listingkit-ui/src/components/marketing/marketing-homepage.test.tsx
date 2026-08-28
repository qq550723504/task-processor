import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { readFileSync } from "node:fs";
import { afterEach, vi } from "vitest";

import { MarketingHomepage } from "@/components/marketing/marketing-homepage";

afterEach(() => vi.unstubAllGlobals());

describe("MarketingHomepage", () => {
  it("presents the public site as 硕米智能引擎", () => {
    render(<MarketingHomepage />);

    const banner = screen.getByRole("banner");
    expect(within(banner).getByRole("link", { name: "硕米智能引擎首页" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { level: 1 })).toHaveTextContent("硕米智能引擎");
    expect(screen.queryByText("ListingKit", { exact: true })).not.toBeInTheDocument();
  });

  it("connects the navigation and primary calls to action to the right destinations", () => {
    render(<MarketingHomepage />);

    const nav = screen.getByRole("navigation", { name: "官网导航" });
    const expectedLinks = [
      ["首页", "#home"],
      ["电商智能体", "#agents"],
      ["供应链与数据", "#supply-chain"],
      ["解决方案", "#solutions"],
      ["服务生态", "#services"],
      ["应用实践", "#practices"],
      ["价格与服务", "#pricing"],
    ];

    for (const [name, href] of expectedLinks) {
      expect(within(nav).getByRole("link", { name })).toHaveAttribute("href", href);
    }

    expect(screen.getByRole("link", { name: "进入硕米" })).toHaveAttribute(
      "href",
      "/login?returnTo=%2Flisting-kits%2Fhome",
    );
    const hero = document.getElementById("home");
    expect(hero).not.toBeNull();
    expect(within(hero!).getByRole("link", { name: /了解平台能力/ })).toHaveAttribute("href", "#agents");
    expect(within(hero!).getByRole("link", { name: /查看解决方案/ })).toHaveAttribute("href", "#solutions");
  });

  it("renders the Figma hero motion layers as decorative content", () => {
    render(<MarketingHomepage />);

    const hero = document.getElementById("home");
    expect(hero).not.toBeNull();
    const decorativeMotion = hero!.querySelector('[aria-hidden="true"]');
    expect(decorativeMotion).not.toBeNull();

    for (const nodeId of ["356:303", "356:304", "356:306", "356:308", "356:310", "356:311", "356:312"]) {
      const layer = hero!.querySelector(`[data-node-id="${nodeId}"]`);
      expect(layer).not.toBeNull();
      expect(decorativeMotion).toContainElement(layer as HTMLElement);
    }
  });

  it("brings the wide-screen hero copy closer to the commerce network without changing narrower breakpoints", () => {
    const styles = readFileSync("src/components/marketing/marketing-homepage.module.css", "utf8");

    expect(styles).toMatch(
      /@media \(min-width: 1200px\) \{[\s\S]*?\.heroContent \{ left: 120px; width: 570px; \}/,
    );
    expect(styles).toMatch(/@media \(max-width: 1180px\) \{[\s\S]*?\.heroContent \{ left: 48px; \}/);
    expect(styles).toMatch(/@media \(max-width: 980px\) \{[\s\S]*?\.nav \{ display: none; \}/);
  });

  it("includes the complete long-form Figma story", () => {
    render(<MarketingHomepage />);

    expect(screen.getByText("电商正在进入AI时代")).toBeInTheDocument();
    expect(screen.getByText("每一个电商岗位，都可以拥有一位 AI 员工")).toBeInTheDocument();
    expect(screen.getByText("连接商品、货盘与工厂，让好产品快速进入市场")).toBeInTheDocument();
    expect(screen.getByText("不同的业务起点，同一套 AI 增长能力")).toBeInTheDocument();
    expect(screen.getByText("价格与服务")).toBeInTheDocument();
    expect(screen.queryByText("让每个人，都拥有一支智能电商团队")).not.toBeInTheDocument();
    expect(document.getElementById("contact")).toBeNull();
  });

  it("opens and closes the Figma contact panel without restoring a duplicate page section", async () => {
    const user = userEvent.setup();
    render(<MarketingHomepage />);

    expect(screen.queryByRole("link", { name: "联系我们" })).not.toBeInTheDocument();
    expect(screen.queryAllByRole("link", { name: /咨询方案/ })).toHaveLength(0);

    await user.click(screen.getByRole("button", { name: "联系硕米" }));
    expect(screen.getByRole("dialog", { name: "联系我们" })).toBeInTheDocument();
    expect(screen.getByLabelText("电话号码")).toBeInTheDocument();

    await user.click(screen.getByRole("button", { name: "关闭联系浮层" }));
    expect(screen.queryByRole("dialog", { name: "联系我们" })).not.toBeInTheDocument();
  });

  it("defines a visible focus ring around active contact fields", () => {
    const styles = readFileSync("src/components/marketing/marketing-homepage.module.css", "utf8");

    expect(styles).toMatch(
      /\.contactForm label:focus-within \{[^}]*outline: 2px solid #8abbff;[^}]*box-shadow:/,
    );
  });

  it("keeps contact-panel focus in the dialog and ignores a request that finishes after closing", async () => {
    const user = userEvent.setup();
    let resolveRequest!: (response: Response) => void;
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>((resolve) => { resolveRequest = resolve; })));
    render(<MarketingHomepage />);

    const launcher = screen.getByRole("button", { name: "联系硕米" });
    await user.click(launcher);
    const closeButton = screen.getByRole("button", { name: "关闭联系浮层" });
    expect(closeButton).toHaveFocus();
    expect(screen.getByRole("img", { name: "微信扫码咨询客服" })).toBeInTheDocument();

    await user.keyboard("{Shift>}{Tab}{/Shift}");
    expect(within(screen.getByRole("dialog", { name: "联系我们" })).getByRole("link", { name: "用户协议" })).toHaveFocus();
    await user.keyboard("{Tab}");
    expect(closeButton).toHaveFocus();

    await user.type(screen.getByLabelText("电话号码"), "13800138000");
    await user.click(screen.getByRole("button", { name: "提交联系信息" }));
    await user.click(closeButton);
    expect(launcher).toHaveFocus();

    resolveRequest(new Response(null, { status: 200 }));
    await Promise.resolve();
    await Promise.resolve();

    await user.click(launcher);
    expect(screen.queryByRole("status")).not.toBeInTheDocument();
    await user.keyboard("{Escape}");
    expect(screen.queryByRole("dialog", { name: "联系我们" })).not.toBeInTheDocument();
    expect(launcher).toHaveFocus();
  });

  it("links the contact consent and footer labels to the published policy pages", async () => {
    const user = userEvent.setup();
    render(<MarketingHomepage />);

    await user.click(screen.getByRole("button", { name: "联系硕米" }));
    const dialog = screen.getByRole("dialog", { name: "联系我们" });
    expect(within(dialog).getByRole("link", { name: "隐私政策" })).toHaveAttribute("href", "/privacy-policy");
    expect(within(dialog).getByRole("link", { name: "用户协议" })).toHaveAttribute("href", "/user-agreement");
    expect(screen.getAllByRole("link", { name: "算力计费说明" }).at(-1)).toHaveAttribute("href", "/ai-compute-billing");
    expect(screen.getAllByRole("link", { name: "服务协议" }).at(-1)).toHaveAttribute("href", "/service-agreement");
  });

  it("returns focus to the contact dialog when a pending submission disables the focused control", async () => {
    const user = userEvent.setup();
    vi.stubGlobal("fetch", vi.fn(() => new Promise<Response>(() => {})));
    render(<MarketingHomepage />);

    await user.click(screen.getByRole("button", { name: "联系硕米" }));
    const closeButton = screen.getByRole("button", { name: "关闭联系浮层" });
    const launcher = screen.getByRole("button", { name: "联系硕米" });
    await user.type(screen.getByLabelText("电话号码"), "13800138000");
    await user.click(screen.getByRole("button", { name: "提交联系信息" }));

    launcher.focus();
    await user.keyboard("{Tab}");
    expect(closeButton).toHaveFocus();
  });

  it("keeps the contact overlay usable on short mobile viewports", () => {
    const styles = readFileSync("src/components/marketing/marketing-homepage.module.css", "utf8");

    expect(styles).toMatch(/\.contactOverlay \{[^}]*overflow-y: auto;/);
    expect(styles).toMatch(/\.contactPanel \{[^}]*max-height: calc\(100dvh - 40px\);[^}]*overflow-y: auto;/);
  });

  it("uses a mobile-specific team layout instead of shrinking the desktop diagram into overlapping nodes", () => {
    const styles = readFileSync("src/components/marketing/marketing-homepage.module.css", "utf8");

    expect(styles).toMatch(
      /@media \(max-width: 620px\) \{[\s\S]*?\.teamStage \{[^}]*?width: 100%;[^}]*?transform: none;[^}]*?\}[\s\S]*?\.teamNodeOne \{ top: 20px; left: 0; \}/,
    );
  });

  it("switches the role solution panel when a user selects a different role", async () => {
    const user = userEvent.setup();
    render(<MarketingHomepage />);
    const roleTabs = within(screen.getByRole("tablist", { name: "角色解决方案" }));

    await user.click(roleTabs.getByRole("tab", { name: /工厂与供应商/ }));

    expect(roleTabs.getByRole("tab", { name: /工厂与供应商/ })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tabpanel", { name: /工厂与供应商/ })).toHaveTextContent(
      "把产品和生产能力转化为全球销售机会",
    );
    expect(screen.getByRole("link", { name: /查看供应商解决方案/ })).toBeInTheDocument();
  });

  it("uses roving focus and standard keyboard selection for role tabs", async () => {
    const user = userEvent.setup();
    render(<MarketingHomepage />);
    const roleTabs = within(screen.getByRole("tablist", { name: "角色解决方案" }));
    const seller = roleTabs.getByRole("tab", { name: /电商卖家/ });

    seller.focus();
    await user.keyboard("{ArrowRight}");
    const supplier = roleTabs.getByRole("tab", { name: /工厂与供应商/ });
    expect(supplier).toHaveFocus();
    expect(supplier).toHaveAttribute("aria-selected", "true");

    await user.keyboard("{Home}");
    const starter = roleTabs.getByRole("tab", { name: /电商创业者/ });
    expect(starter).toHaveFocus();
    expect(starter).toHaveAttribute("aria-selected", "true");

    await user.keyboard("{End}");
    const opc = roleTabs.getByRole("tab", { name: /OPC电商社区/ });
    expect(opc).toHaveFocus();
    expect(opc).toHaveAttribute("aria-selected", "true");
  });

  it("switches the practice case when a user selects a different scenario", async () => {
    const user = userEvent.setup();
    render(<MarketingHomepage />);
    const practiceTabs = within(screen.getByRole("tablist", { name: "实践场景" }));

    await user.click(practiceTabs.getByRole("tab", { name: /跨境电商卖家/ }));

    expect(practiceTabs.getByRole("tab", { name: /跨境电商卖家/ })).toHaveAttribute("aria-selected", "true");
    expect(screen.getByRole("tabpanel", { name: /跨境电商卖家/ })).toHaveTextContent(
      "AI诊断店铺，找到被忽略的增长机会",
    );
  });

  it("uses roving focus and standard keyboard selection for practice tabs", async () => {
    const user = userEvent.setup();
    render(<MarketingHomepage />);
    const practiceTabs = within(screen.getByRole("tablist", { name: "实践场景" }));
    const opc = practiceTabs.getByRole("tab", { name: /OPC电商社区/ });

    opc.focus();
    await user.keyboard("{ArrowRight}");
    const starter = practiceTabs.getByRole("tab", { name: /新手创业者/ });
    expect(starter).toHaveFocus();
    expect(starter).toHaveAttribute("aria-selected", "true");
  });

});
