import { readFileSync } from "node:fs";
import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const manifest = process.env.ISSUE328_FIXTURE_MANIFEST;
if (!manifest) throw new Error("Start the task-isolated launcher and supply its current manifest.");
const fixture = JSON.parse(readFileSync(manifest, "utf8"));
if (!/^http:\/\/127\.0\.0\.1:\d+$/.test(fixture.origin)) throw new Error("Loopback fixture required");

// R328-C1 prepared component slice: projection import is not wired yet.
// This is explicitly unavailable-page acceptance, NOT real #340 acceptance.
for (const width of [1440, 390]) {
  test(`existing navigation reaches the unavailable task center at ${width}px`, async ({ page, context }, info) => {
    await page.setViewportSize({ width, height: 900 });
    await context.addCookies(fixture.sessions.owner);
    const projectionRequests: string[] = [];
    page.on("request", (request) => { if (new URL(request.url()).pathname === "/api/workbench/completed-work") projectionRequests.push(request.url()); });
    await page.goto(`${fixture.origin}/workbench`);
    await expect(page.getByRole("heading", { name: "经营全局，一屏掌握" })).toBeVisible();
    if (width === 390) await page.getByRole("button", { name: "打开工作台导航" }).click();
    const nav = page.getByRole("navigation", { name: width === 390 ? "移动工作台导航" : "工作台导航", exact: true });
    await nav.getByRole("button", { name: "展开AI工作台", exact: true }).click();
    await nav.getByRole("link", { name: "任务中心", exact: true }).click();
    await expect(page.getByRole("heading", { name: "任务中心", exact: true })).toBeVisible();
    await page.getByRole("navigation", { name: "任务状态" }).getByRole("link", { name: "已完成", exact: true }).click();
    await expect(page).toHaveURL(/\/workbench\/ai\/tasks\/completed$/);
    await expect(page.getByText("暂未启用", { exact: true })).toBeVisible();
    expect(projectionRequests).toEqual([]);
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    expect((await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze()).violations).toEqual([]);
    await page.screenshot({ path: info.outputPath(`unavailable-${width}.png`), fullPage: width === 390 });
    if (width === 390) {
      const trigger = page.getByRole("button", { name: "打开工作台导航" });
      await trigger.click(); await page.keyboard.press("Escape");
      await expect(trigger).toBeFocused();
      await expect(page.getByRole("navigation", { name: "移动工作台导航" })).toHaveCount(0);
    }
  });
}
