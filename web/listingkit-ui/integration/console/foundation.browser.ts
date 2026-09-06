import { readFileSync } from "node:fs";
import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

const path = process.env.ISSUE331_FIXTURE_MANIFEST;
if (!path) throw new Error("Start the isolated diagnostic fixture and supply its current manifest.");
const fixture = JSON.parse(readFileSync(path, "utf8"));
if (!/^http:\/\/127\.0\.0\.1:\d+$/.test(fixture.origin)) throw new Error("Loopback fixture required");

// Explicit UI contract fixture only. Store service is NOT provided by the Go diagnostic fixture.
const store = { id: "11111111-1111-4111-8111-111111111111", name: "隔离视觉验证店铺", platform: "shein", region: "US", externalStoreId: "ISOLATED-VISUAL-ONLY", lifecycleStatus: "active", connectionStatus: "disconnected", version: 1, createdAt: "2026-09-06T00:00:00Z", updatedAt: "2026-09-06T00:00:00Z" };
const list = { items: [store], quota: { used: 1, reserved: 0, limit: 5, allowed: true, reason: "" }, pagination: { page: 1, pageSize: 20, total: 1 } };
async function accessible(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  expect((await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze()).violations).toEqual([]);
}
test.beforeEach(async ({ context }) => { await context.addCookies(fixture.sessions.owner); });

for (const width of [1440, 390]) {
  test(`single Shell: overview → Chat → Store, ${width}px`, async ({ page }, info) => {
    await page.setViewportSize({ width, height: width === 1440 ? 900 : 844 });
    const businessRequests: string[] = [];
    page.on("request", (r) => { if (r.url().includes("/api/") && !r.url().includes("/context") && !r.url().includes("/auth")) businessRequests.push(r.url()); });
    await page.goto(`${fixture.origin}/workbench`);
    await expect(page.getByRole("heading", { name: "经营全局，一屏掌握" })).toBeVisible();
    await accessible(page);
    await page.mouse.move(0, 0);
    await page.screenshot({ path: info.outputPath(`overview-${width}.png`), fullPage: width === 390 });
    expect(businessRequests).toEqual([]);
    if (width === 390) await page.getByRole("button", { name: "打开工作台导航" }).click();
    let nav = page.getByRole("navigation", { name: width === 390 ? "移动工作台导航" : "工作台导航", exact: true });
    await nav.getByRole("button", { name: "展开AI工作台", exact: true }).click();
    await nav.getByRole("link", { name: "硕米Chat", exact: true }).click();
    await expect(page.getByRole("heading", { name: "硕米Chat", exact: true })).toBeVisible();
    await expect(page.getByRole("status")).toContainText("暂未启用");
    await accessible(page);
    await page.mouse.move(0, 0);
    await page.screenshot({ path: info.outputPath(`chat-${width}.png`), fullPage: width === 390 });
    expect(businessRequests).toEqual([]);
    await page.route((url) => url.pathname === "/api/workbench/stores", (route) => route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(list) }));
    if (width === 390) await page.getByRole("button", { name: "打开工作台导航" }).click();
    nav = page.getByRole("navigation", { name: width === 390 ? "移动工作台导航" : "工作台导航", exact: true });
    await nav.getByRole("button", { name: "展开店铺中心", exact: true }).click();
    await nav.getByRole("link", { name: "我的店铺", exact: true }).click();
    await expect(page.getByRole("heading", { name: store.name })).toBeVisible();
    await expect(page.getByRole("main")).toHaveCount(1);
    await accessible(page);
    await page.mouse.move(0, 0);
    await page.screenshot({ path: info.outputPath(`stores-${width}.png`), fullPage: width === 390 });
    if (width === 390) {
      const trigger = page.getByRole("button", { name: "打开工作台导航" });
      await trigger.click();
      await page.keyboard.press("Escape");
      await expect(page.getByRole("navigation", { name: "移动工作台导航" })).toHaveCount(0, { timeout: 1000 });
      await expect(trigger).toBeFocused();
      await trigger.click();
      const link = page.getByRole("navigation", { name: "移动工作台导航" }).getByRole("link", { name: "我的店铺", exact: true });
      await link.focus(); await link.press("Escape");
      await expect(trigger).toBeFocused();
      await expect(page.getByRole("navigation", { name: "移动工作台导航" })).toHaveCount(0);
    }
  });
}

test("light theme uses the same structure and real preference toggle", async ({ page }, info) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  await page.goto(`${fixture.origin}/workbench`);
  await page.getByRole("switch", { name: "浅色模式" }).click();
  await expect(page.locator("html")).toHaveClass(/light/);
  await accessible(page);
  await page.screenshot({ path: info.outputPath("overview-light-1440.png") });
  await page.reload();
  await expect(page.getByRole("switch", { name: "浅色模式" })).toBeChecked();
});

test("existing diagnostic uses actual BFF/Go/PG and keeps organization isolation", async ({ page }, info) => {
  await page.setViewportSize({ width: 1440, height: 900 });
  const diagnostic = () => page.waitForResponse((r) => r.url().includes("offline-diagnostic"));
  let response = diagnostic();
  // Internal diagnostic deep link remains permitted for #331 template regression, not #328 discovery.
  await page.goto(`${fixture.origin}/workbench/shein-records/${fixture.recordId}/diagnostic`);
  expect((await response).status()).toBe(200);
  await expect(page.getByText("发现需要处理的问题")).toBeVisible();
  await accessible(page);
  await page.screenshot({ path: info.outputPath("diagnostic-1440.png") });
  response = diagnostic();
  await page.getByLabel("当前企业", { exact: true }).selectOption("100");
  expect((await response).status()).toBe(404);
  await expect(page.getByRole("main").getByRole("alert")).toContainText("资料不存在或当前账号不可读取");
  await expect(page.getByText("发现需要处理的问题")).toHaveCount(0);
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(page.getByLabel("企业代管状态")).toBeVisible();
  await accessible(page);
  await page.screenshot({ path: info.outputPath("delegated-error-390.png") });
  response = diagnostic();
  await page.getByLabel("当前企业", { exact: true }).selectOption("200");
  expect((await response).status()).toBe(200);
  await expect(page.getByText("发现需要处理的问题")).toBeVisible();
  await page.setViewportSize({ width: 390, height: 844 });
  await accessible(page);
  await page.screenshot({ path: info.outputPath("diagnostic-390.png") });
});

test("third-level navigation and browser history keep selection and mobile state consistent", async ({ page }) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${fixture.origin}/workbench/ai/chat/recent`);
  const trigger = page.getByRole("button", { name: "打开工作台导航" });
  await trigger.click();
  const nav = page.getByRole("navigation", { name: "移动工作台导航" });
  await expect(nav.getByRole("link", { name: "最近会话", exact: true })).toHaveAttribute("aria-current", "page");
  await expect(nav.getByRole("button", { name: "收起硕米Chat" })).toHaveAttribute("aria-expanded", "true");
  await nav.getByRole("link", { name: "收藏会话", exact: true }).click();
  await expect(page.getByRole("heading", { name: "收藏会话", exact: true })).toBeVisible();
  await trigger.click();
  await page.goBack();
  await expect(page.getByRole("heading", { name: "最近会话", exact: true })).toBeVisible();
  await expect(nav).toHaveCount(0);
  await expect(page.getByRole("navigation", { name: "面包屑" })).toContainText("AI工作台 / 硕米Chat / 最近会话");
});

test("delayed actual diagnostic response cannot repopulate a different enterprise", async ({ page }) => {
  let release!: () => void;
  const barrier = new Promise<void>((resolve) => { release = resolve; });
  let received!: () => void;
  const upstreamReceived = new Promise<void>((resolve) => { received = resolve; });
  let delivered!: () => void;
  const responseReleased = new Promise<void>((resolve) => { delivered = resolve; });
  // Delay only the first real BFF response; never replace its payload with invented data.
  let first = true;
  await page.route("**/offline-diagnostic?*", async (route) => {
    if (!first) { await route.continue(); return; }
    first = false;
    const actual = await route.fetch();
    expect(actual.status()).toBe(200);
    received();
    await barrier;
    try { await route.fulfill({ response: actual }); } finally { delivered(); }
  });
  await page.goto(`${fixture.origin}/workbench/shein-records/${fixture.recordId}/diagnostic`);
  await upstreamReceived;
  try {
    await page.getByLabel("当前企业", { exact: true }).selectOption("100");
    await expect(page.getByRole("main").getByRole("alert")).toContainText("资料不存在或当前账号不可读取");
  } finally { release(); }
  await responseReleased;
  await expect(page.getByText("发现需要处理的问题")).toHaveCount(0);
  await page.getByLabel("当前企业", { exact: true }).selectOption("200");
  await expect(page.getByText("发现需要处理的问题")).toBeVisible();
});
