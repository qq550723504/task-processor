import { readFileSync } from "node:fs";
import AxeBuilder from "@axe-core/playwright";
import { expect, test } from "@playwright/test";

const manifest = process.env.ISSUE331_FIXTURE_MANIFEST;
if (!manifest) throw new Error("Supply this run's isolated fixture manifest");
const fixture = JSON.parse(readFileSync(manifest, "utf8"));
if (!/^http:\/\/127\.0\.0\.1:\d+$/.test(fixture.origin)) throw new Error("Loopback required");

for (const state of ["loading", "failure", "no-organization"] as const) {
  test(`FV1 context HTTP fixture: ${state} retains Console tokens`, async ({ page, context }, info) => {
    await context.addCookies(fixture.sessions.owner);
    await page.setViewportSize({ width: state === "no-organization" ? 390 : 1440, height: 900 });
    let release!: () => void;
    const pending = new Promise<void>((resolve) => { release = resolve; });
    await page.route("**/api/workbench/context", async (route) => {
      if (state === "loading") { await pending; await route.abort(); return; }
      await route.fulfill({ status: state === "failure" ? 503 : 200, contentType: "application/json", body: JSON.stringify(state === "failure" ? { code: "DEPENDENCY_UNAVAILABLE", message: "controlled fault", requestId: "fv1", fieldErrors: [] } : { user: { id: "owner" }, homeOrganizationId: "200", effectiveOrganizationId: null, selectionRequired: false, organizations: [] }) });
    });
    try {
      await page.goto(`${fixture.origin}/workbench${state === "no-organization" ? "/no-organization" : ""}`);
      if (state === "loading") await expect(page.getByText("正在加载工作台...")).toBeVisible();
      if (state === "failure") await expect(page.getByRole("button", { name: "重新加载" })).toBeVisible();
      if (state === "no-organization") await expect(page.getByRole("heading", { name: "暂无可用企业" })).toBeVisible();
      await expect(page.locator("html")).toHaveClass(/dark/);
      await page.screenshot({ path: info.outputPath(`fv1-${state}.png`), fullPage: true });
      const main = page.getByRole("main");
      expect(await main.evaluate((element) => getComputedStyle(element).getPropertyValue("--background").trim())).toBe("#07111e");
      if (state === "failure") expect(await main.locator("section").evaluate((element) => getComputedStyle(element).borderTopColor)).toBe("rgb(32, 60, 86)");
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      expect((await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze()).violations).toEqual([]);
      // Existing ToastProvider is mounted but currently has no public consumer/API.
      await expect(page.locator(".pointer-events-none.fixed [role=status]")).toHaveCount(0);
    } finally { release(); }
  });
}

test("FV1 actual Go revocation removes the report and retains Console tokens", async ({ page, context }, info) => {
  await context.addCookies(fixture.sessions.owner);
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto(`${fixture.origin}/workbench/shein-records/${fixture.recordId}/diagnostic`);
  await expect(page.getByText("发现需要处理的问题")).toBeVisible();
  await context.addCookies(fixture.sessions.revoked);
  const revoked = page.waitForResponse((response) => response.url().includes("offline-diagnostic"));
  await page.getByRole("button", { name: "重新检查", exact: true }).click();
  expect((await revoked).status()).toBe(403);
  await expect(page.getByText("发现需要处理的问题")).toHaveCount(0);
  await expect(page.getByRole("main").getByRole("alert")).toContainText("撤销");
  expect(await page.getByRole("main").evaluate((element) => getComputedStyle(element).getPropertyValue("--background").trim())).toBe("#07111e");
  await page.screenshot({ path: info.outputPath("fv1-revoked-390.png"), fullPage: true });
  expect((await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze()).violations).toEqual([]);
});
