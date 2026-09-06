import { readFileSync } from "node:fs";
import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page, type Response } from "@playwright/test";
import { parseCompletedWorkList } from "../../src/lib/api/completed-work";

const manifest = process.env.ISSUE328_FIXTURE_MANIFEST;
if (!manifest) throw new Error("Start --serve --completed-work and supply its current manifest.");
const fixture = JSON.parse(readFileSync(manifest, "utf8"));
if (!/^http:\/\/127\.0\.0\.1:\d+$/.test(fixture.origin)) throw new Error("Loopback fixture required");
const projection = (r: Response) => new URL(r.url()).pathname === "/api/workbench/completed-work";
async function payload(response: Response) {
  expect(response.status()).toBe(200);
  expect(response.request().headers().authorization).toBeUndefined();
  const data = parseCompletedWorkList(await response.json());
  expect(data).not.toBeNull();
  return data!;
}
async function openCompleted(page: Page, width = 1440) {
  await page.setViewportSize({ width, height: 900 });
  await page.goto(`${fixture.origin}/workbench`);
  await expect(page.getByRole("heading", { name: "经营全局，一屏掌握" })).toBeVisible();
  if (width === 390) await page.getByRole("button", { name: "打开工作台导航" }).click();
  const nav = page.getByRole("navigation", { name: width === 390 ? "移动工作台导航" : "工作台导航", exact: true });
  await nav.getByRole("button", { name: "展开AI工作台", exact: true }).click();
  const initial = page.waitForResponse(projection);
  await nav.getByRole("link", { name: "任务中心", exact: true }).click();
  await initial;
  const completed = page.waitForResponse(projection);
  await page.getByRole("navigation", { name: "任务状态" }).getByRole("link", { name: "已完成", exact: true }).click();
  await expect(page).toHaveURL(/\/workbench\/ai\/tasks\/completed$/);
  return completed;
}
async function accessible(page: Page) {
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
  expect((await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze()).violations).toEqual([]);
}
const rows = (page: Page) => page.getByRole("list", { name: "已完成工作记录" }).getByRole("button");
test.beforeEach(async ({ context }) => { await context.addCookies(fixture.sessions.owner); });

for (const width of [1440, 390]) {
  test(`actual projection → selection → reauthorized diagnostic → return, ${width}px`, async ({ page }, info) => {
    const data = await payload(await openCompleted(page, width));
    expect(data.items).toHaveLength(20); expect(data.next_cursor).toBeTruthy();
    await expect(rows(page)).toHaveCount(20);
    const first = rows(page).first(); await first.focus(); await first.press("Enter");
    await expect(page.getByRole("heading", { name: "工作记录详情" })).toBeFocused();
    const result = page.getByRole("link", { name: "查看诊断" });
    await expect(result).toHaveAttribute("href", data.items[0].result.href);
    await accessible(page);
    if (width === 1440) await page.evaluate(() => scrollTo(0,0));
    await page.screenshot({ path: info.outputPath(`completed-${width}.png`), fullPage: width === 390 });
    if (width === 1440) await page.getByRole("region", { name: "工作记录详情" }).screenshot({ path: info.outputPath("result-detail-1440.png") });
    const diagnostic = page.waitForResponse((r) => new URL(r.url()).pathname === `/api/listing/shein-records/${data.items[0].source_record_id}/offline-diagnostic`);
    await result.click();
    const report = await diagnostic; expect(report.status()).toBe(200);
    expect(report.request().headers()["x-expected-organization-id"]).toBe("200");
    expect((await report.json()).diagnostic_only).toBe(true);
    await expect(page.getByText("发现需要处理的问题")).toBeVisible();
    const returned = page.waitForResponse(projection);
    await page.getByRole("link", { name: "返回任务中心" }).click();
    await payload(await returned);
    await expect(page).toHaveURL(/\/workbench\/ai\/tasks\/completed$/);
    await expect(page.getByText("选择一条工作记录")).toBeVisible();
    await expect(page.getByRole("link", { name: "查看诊断" })).toHaveCount(0);
    await page.goBack();
    await expect(page.getByText("发现需要处理的问题")).toBeVisible();
    const forward = page.waitForResponse(projection);
    await page.goForward(); await payload(await forward);
    await expect(page.getByRole("navigation", { name: "任务状态" }).getByRole("link", { name: "已完成", exact: true })).toHaveAttribute("aria-current", "page");
    if(width===390){const trigger=page.getByRole("button",{name:"打开工作台导航"});await trigger.click();await page.keyboard.press("Escape");await expect(trigger).toBeFocused();}
  });
}

test("actual bounded pagination, refresh, organization 200 → 100 → 200 and light theme", async ({ page }, info) => {
  const first = await payload(await openCompleted(page));
  let response = page.waitForResponse(projection);
  await page.getByRole("button", { name: "下一页" }).click();
  const second = await payload(await response); expect(second.items).toHaveLength(fixture.recordCount - 20);
  expect(second.items.every((item) => !first.items.some((old) => old.source_record_id === item.source_record_id))).toBe(true);
  await expect(page.getByText("第 2 页")).toBeFocused();
  response = page.waitForResponse(projection); await page.getByRole("button", { name: "刷新记录" }).click();
  expect((await payload(await response)).items).toEqual(first.items);
  await rows(page).first().click();
  response = page.waitForResponse(projection); await page.getByLabel("当前企业", { exact: true }).selectOption("100");
  expect((await payload(await response)).items).toEqual([]);
  await expect(page.getByText("当前授权范围内暂无本地资料准备记录")).toBeVisible();
  await expect(page.getByRole("link", { name: "查看诊断" })).toHaveCount(0);
  await accessible(page); await page.screenshot({ path: info.outputPath("empty-1440.png") });
  response = page.waitForResponse(projection); await page.getByLabel("当前企业", { exact: true }).selectOption("200");
  expect((await payload(await response)).items).toEqual(first.items);
  await expect(page.getByText("选择一条工作记录")).toBeVisible();
  await page.getByRole("switch", { name: "浅色模式" }).click(); await accessible(page);
  await page.screenshot({ path: info.outputPath("completed-light-1440.png") });
});

test("actual late HTTP body cannot reintroduce organization 200 into 100", async ({ page }) => {
  await payload(await openCompleted(page));
  let release!: () => void;
  const gate = new Promise<void>((resolve) => { release = resolve; });
  let captured!: () => void;
  const caught = new Promise<void>((resolve) => { captured = resolve; });
  await page.route("**/api/workbench/completed-work?*", async (route) => {
    if (route.request().headers()["x-expected-organization-id"] !== "200") return route.continue();
    const response = await route.fetch(); captured(); await gate;
    await route.fulfill({ response }).catch(() => { /* the old request is cancelled */ });
  });
  try {
    await page.getByRole("button", { name: "刷新记录" }).click(); await caught;
    const changed = page.waitForResponse(projection);
    await page.getByLabel("当前企业", { exact: true }).selectOption("100");
    expect((await payload(await changed)).items).toEqual([]);
    release();
    await expect(page.getByText("当前授权范围内暂无本地资料准备记录")).toBeVisible();
    await expect(page.getByRole("link", { name: "查看诊断" })).toHaveCount(0);
    await expect(rows(page)).toHaveCount(0);
  } finally { release(); }
});

for (const [identity, code, status] of [["revoked", "撤销", 403], ["store", "没有读取工作记录的权限", 403], ["slow", "超时", 504], ["unavailable", "暂不可用", 503]] as const) {
  test(`actual Go ${identity} after a valid result hides selection`, async ({ page, context }, info) => {
    await payload(await openCompleted(page)); await rows(page).first().click();
    await context.addCookies(fixture.sessions[identity]);
    const response = page.waitForResponse(projection); await page.getByRole("button", { name: "刷新记录" }).click();
    expect((await response).status()).toBe(status);
    await expect(page.getByRole("main").getByRole("alert")).toContainText(code);
    await expect(page.getByRole("link", { name: "查看诊断" })).toHaveCount(0);
    await page.screenshot({ path: info.outputPath(`actual-${identity}-1440.png`) });
  });
}

test("actual selected result reauthorizes at diagnostic after Listing permission is removed", async ({ page, context }) => {
  const data = await payload(await openCompleted(page)); await rows(page).first().click();
  await context.addCookies(fixture.sessions.store);
  const diagnostic = page.waitForResponse((r) => new URL(r.url()).pathname === `/api/listing/shein-records/${data.items[0].source_record_id}/offline-diagnostic`);
  await page.getByRole("link", { name: "查看诊断" }).click(); expect((await diagnostic).status()).toBe(403);
  await expect(page.getByRole("main").getByRole("alert")).toContainText("没有读取这份资料的权限");
  await expect(page.getByText("发现需要处理的问题")).toHaveCount(0);
});
