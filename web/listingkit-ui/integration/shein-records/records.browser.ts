import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";

const manifestPath = process.env.ISSUE328_FIXTURE_MANIFEST;
if (!manifestPath) throw new Error("Start the isolated #327 fixture and set ISSUE328_FIXTURE_MANIFEST.");
const fixture = JSON.parse(readFileSync(manifestPath, "utf8"));
if (!/^http:\/\/127\.0\.0\.1:\d+$/.test(fixture.origin)) throw new Error("Only the loopback fixture is permitted.");

test.beforeEach(async ({ context }) => { await context.addCookies(fixture.sessions.owner); });

for (const width of [1440, 390]) {
  test(`withdrawn entry stays unavailable with the backend configured at ${width}px`, async ({ page }) => {
    const collectionRequests: string[] = [];
    page.on("request", (request) => {
      if (new URL(request.url()).pathname === "/api/listing/shein-records") collectionRequests.push(request.url());
    });
    await page.setViewportSize({ width, height: 900 });
    await page.goto(`${fixture.origin}/workbench`);
    await expect(page.getByRole("heading", { name: "工作台", exact: true })).toBeVisible();
    await expect(page.getByText("SHEIN 本地资料", { exact: true })).toHaveCount(0);
    await expect(page.getByRole("link", { name: "查看本地资料" })).toHaveCount(0);
    await page.goto(`${fixture.origin}/workbench/shein-records`);
    await expect(page.getByText("404", { exact: true })).toBeVisible();
    await expect(page.getByRole("link", { name: "查看诊断", exact: true })).toHaveCount(0);
    expect(collectionRequests).toEqual([]);
  });
}
