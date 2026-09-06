// UI regression against the existing real, task-isolated read-only fixture.
// Usage: node scripts/title-review-ui-regression.mjs <fixture.json> <evidence-dir>
import { readFile, mkdir } from "node:fs/promises";
import { resolve, join } from "node:path";
import assert from "node:assert/strict";
import { chromium, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

const manifest = JSON.parse(await readFile(process.argv[2], "utf8"));
const output = resolve(process.argv[3]);
assert.match(manifest.origin, /^http:\/\/127\.0\.0\.1:\d+$/);
await mkdir(output, { recursive: true });
const browser = await chromium.launch({ headless: true });
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  await context.addCookies(manifest.sessions.owner);
  const page = await context.newPage();
  await page.goto(`${manifest.origin}/workbench/ai/tasks`);
  await page.getByRole("heading", { name: "已完成工作记录", exact: true }).waitFor();
  const states = page.getByRole("navigation", { name: "任务状态" });
  await states.getByRole("link", { name: "已完成", exact: true }).click();
  await page.waitForURL(`${manifest.origin}/workbench/ai/tasks/completed`);
  await expect(states.getByRole("link", { name: "已完成", exact: true })).toHaveAttribute("aria-current", "page");
  await page.getByRole("button", { name: /查看详情/ }).first().click();
  await page.getByRole("link", { name: "查看诊断", exact: true }).waitFor();
  await expect(page.getByRole("heading", { name: "工作记录详情", exact: true })).toBeFocused();
  await page.screenshot({ path: join(output, "regression-completed-1440.png") });
  const issues = (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze()).violations;
  assert.equal(issues.length, 0, JSON.stringify(issues.map((item) => ({ id: item.id, impact: item.impact }))));
  await page.getByRole("link", { name: "查看诊断", exact: true }).click();
  await page.getByRole("heading", { name: /诊断/ }).first().waitFor();
  await page.getByRole("heading", { name: "未评估范围", exact: true }).waitFor();
  await page.screenshot({ path: join(output, "regression-diagnostic-1440.png") });
  await page.goBack();
  await page.getByRole("button", { name: /查看详情/ }).first().waitFor();
  await page.setViewportSize({ width: 390, height: 844 });
  await page.getByRole("button", { name: /查看详情/ }).first().click();
  await page.getByRole("link", { name: "查看诊断", exact: true }).waitFor();
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), true);
  await page.screenshot({ path: join(output, "regression-completed-390.png"), fullPage: true });
  await page.getByRole("button", { name: "返回记录列表" }).click();
  await states.getByRole("link", { name: "待确认", exact: true }).waitFor();
  console.log("PASS: real completed-work entry -> detail -> diagnostic -> back; 1440/390, keyboard focus, axe, no overflow. Product review integration is a separate test.");
} finally { await browser.close(); }
