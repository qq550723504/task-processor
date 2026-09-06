import { readFileSync } from "node:fs";
import AxeBuilder from "@axe-core/playwright";
import { expect, test, type Page } from "@playwright/test";

const manifestPath = process.env.ISSUE328_FIXTURE_MANIFEST;
if (!manifestPath) throw new Error("Start the isolated #327 fixture and set ISSUE328_FIXTURE_MANIFEST.");
const fixture = JSON.parse(readFileSync(manifestPath, "utf8"));
if (!/^http:\/\/127\.0\.0\.1:\d+$/.test(fixture.origin)) throw new Error("Only the loopback fixture is permitted.");
const collectionPath = "/api/listing/shein-records";
const isCollection = (url: string) => new URL(url).pathname === collectionPath;
const nextCollection = (page: Page) => page.waitForResponse((r) => isCollection(r.url()));

test.beforeEach(async ({ context }) => { await context.addCookies(fixture.sessions.owner); });

for (const width of [1440, 390]) {
  test(`actual workbench → list → diagnostic → list at ${width}px`, async ({ page }, info) => {
    const requests: string[] = [];
    page.on("request", (request) => requests.push(request.url()));
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
    await page.goto(`${fixture.origin}/workbench`);
    const entry = page.getByRole("link", { name: "查看本地资料" });
    await expect(entry).toBeVisible();
    await entry.focus();
    await expect(entry).toBeFocused();
    expect((await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze()).violations).toEqual([]);
    await page.screenshot({ path: info.outputPath(`workbench-${width}.png`), fullPage: true });
    let response = nextCollection(page);
    await entry.press("Enter");
    const firstResponse = await response;
    expect(firstResponse.status()).toBe(200);
    expect(firstResponse.request().headers().authorization).toBeUndefined();
    expect(firstResponse.request().headers()["x-expected-organization-id"]).toBe("200");
    const first = await firstResponse.json();
    expect(first.items).toHaveLength(20);
    expect(first.next_cursor).toBeTruthy();
    await expect(page.getByRole("link", { name: "查看诊断", exact: true })).toHaveCount(20);
    expect(requests.some((url) => url.includes("offline-diagnostic"))).toBe(false);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    expect((await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze()).violations).toEqual([]);
    await page.screenshot({ path: info.outputPath(`list-${width}.png`), fullPage: true });
    response = nextCollection(page);
    await page.getByRole("button", { name: "下一页", exact: true }).click();
    const second = await (await response).json();
    expect(second.items.length).toBeGreaterThan(0);
    expect(second.items.every((item: { record_id: string }) => !first.items.some((old: { record_id: string }) => old.record_id === item.record_id))).toBe(true);
    response = nextCollection(page);
    await page.getByRole("button", { name: "上一页", exact: true }).click();
    expect((await (await response).json()).items).toEqual(first.items);
    response = nextCollection(page);
    await page.getByRole("button", { name: "刷新列表" }).click();
    expect(new URL((await response).url()).searchParams.has("cursor")).toBe(false);
    const diagnostic = page.waitForResponse((r) => r.url().includes("offline-diagnostic"));
    // The navigation ID comes only from the actual collection response/DOM.
    await page.locator(`a[href="/workbench/shein-records/${first.items[0].record_id}/diagnostic"]`).click();
    expect((await diagnostic).status()).toBe(200);
    await expect(page.getByRole("heading", { name: "SHEIN 资料诊断" })).toBeVisible();
    response = nextCollection(page);
    await page.getByRole("link", { name: "返回本地资料列表" }).click();
    expect((await response).status()).toBe(200);
    await expect(page.getByRole("heading", { name: "SHEIN 本地资料", exact: true })).toBeVisible();
    response = nextCollection(page);
    await page.getByLabel("当前企业", { exact: true }).selectOption("100");
    const empty = await response;
    expect(empty.status()).toBe(200);
    expect((await empty.json()).items).toEqual([]);
    await expect(page.getByText("当前范围内暂无本地资料")).toBeVisible();
    await expect(page.getByRole("link", { name: "查看诊断", exact: true })).toHaveCount(0);
    response = nextCollection(page);
    await page.getByLabel("当前企业", { exact: true }).selectOption("200");
    expect(new URL((await response).url()).searchParams.has("cursor")).toBe(false);
    await expect(page.getByRole("link", { name: "查看诊断", exact: true })).toHaveCount(20);
    expect(requests.some((url) => url.startsWith(fixture.goOrigin) || url.startsWith(fixture.contextOrigin))).toBe(false);
  });
}

test("actual Store-only identity enters from workbench and sees permission failure", async ({ page, context }) => {
  await context.addCookies(fixture.sessions.store);
  await page.goto(`${fixture.origin}/workbench`);
  const response = nextCollection(page);
  await page.getByRole("link", { name: "查看本地资料" }).click();
  expect((await response).status()).toBe(403);
  await expect(page.getByRole("main").getByRole("alert")).toContainText("没有读取本地资料的权限");
  await expect(page.getByRole("link", { name: "查看诊断", exact: true })).toHaveCount(0);
});

for (const [code, status, message] of [
  ["DEPENDENCY_UNAVAILABLE", 502, "资料服务暂不可用"],
  ["DEADLINE_EXCEEDED", 504, "读取资料超时"],
  ["INVALID_UPSTREAM_RESPONSE", 200, "资料列表响应不合法"],
] as const) {
  test(`synthetic collection failure through actual browser entry: ${code}`, async ({ page }) => {
    await page.route((url) => isCollection(url.toString()), (route) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(status === 200 ? { items: null } : { code, message: "synthetic only", requestId: "synthetic", fieldErrors: [] }) }));
    await page.goto(`${fixture.origin}/workbench`);
    await page.getByRole("link", { name: "查看本地资料" }).click();
    await expect(page.getByRole("main").getByRole("alert")).toContainText(message);
    await expect(page.getByText("当前范围内暂无本地资料")).toHaveCount(0);
    await expect(page.getByRole("link", { name: "查看诊断", exact: true })).toHaveCount(0);
  });
}
