// Read-only UI acceptance against #347's real, isolated browser fixture.
// Usage: node scripts/commercial-ui-acceptance.mjs <private fixture.json> <output directory>
// Never print/copy the manifest, cookies, control token, headers or response bodies.
import assert from "node:assert/strict";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { resolve, join } from "node:path";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";
import { chromium, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

const manifest = JSON.parse(await readFile(process.argv[2], "utf8"));
const output = resolve(process.argv[3]);
assert.equal(new URL(manifest.origin).hostname, "127.0.0.1");
assert.ok(Date.now() < manifest.expiresAt, "Fixture has expired");
await mkdir(output, { recursive: true });
const web = resolve(manifest.webDirectory);
const head = execFileSync("git", ["rev-parse", "HEAD"], { cwd: web, encoding: "utf8" }).trim();
const sourceFiles = ["src/components/workbench/commercial/commercial-page.tsx", "src/components/workbench/commercial/commercial-views.tsx", "src/components/workbench/commercial/commercial.module.css", "src/lib/api/commercial.ts", "src/components/workbench/workspace-app-shell.tsx", "src/lib/workbench/console-navigation.ts"];
const sourceHashes = Object.fromEntries(await Promise.all(sourceFiles.map(async file => [file, createHash("sha256").update(await readFile(join(web, file))).digest("hex")])));
const report = { head, fixtureSourceHead: manifest.sourceHead, fixtureWebHead: manifest.webHead, sourceHashes, startedAt: new Date().toISOString(), checks: [], screenshots: [], accessibility: [], requests: [], notRun: ["real IAM, payment, customer data, production", "OpenMeter", "unmerged #345/#348 page combination"], passed: false };
const browser = await chromium.launch();
const context = await browser.newContext({ viewport: { width: 1440, height: 900 }, colorScheme: "dark" });
await context.addCookies(manifest.sessions.owner);
const page = await context.newPage();
const endpoint = "/api/workbench/commercial/overview";
page.on("response", response => {
  const url = new URL(response.url());
  if (url.origin === manifest.origin && url.pathname.startsWith("/api/workbench/")) report.requests.push({ path: url.pathname, method: response.request().method(), status: response.status() });
});
const region = name => page.getByRole("region", { name, exact: true });
const ready = () => expect(region("实际订阅")).toBeVisible();
const usage = () => region("已记录用量概要").locator("article").filter({ has: page.getByRole("heading", { name: "资料生成作业", exact: true }) });
async function scenario(value) {
  assert.ok(manifest.scenarios.includes(value));
  const response = await fetch(manifest.scenarioURL, { method: "POST", headers: { "Content-Type": "application/json", "X-Fixture-Control": manifest.controlToken }, body: JSON.stringify({ scenario: value }) });
  assert.equal(response.status, 204);
}
async function go(kind = "entitlements") {
  await page.goto(`${manifest.origin}/workbench/plans/${kind}`);
  await expect(region(kind === "options" ? "方案说明" : "实际订阅")).toBeVisible();
}
async function switchOrg(org) {
  await page.getByRole("combobox", { name: "当前企业" }).selectOption(org);
  await expect(page.getByRole("combobox", { name: "当前企业" })).toHaveValue(org);
  await ready();
  await expect(page.locator("#console-main")).toContainText(`（${org}）`);
}
async function capture(name, fullPage = false) {
  await page.screenshot({ path: join(output, `${name}.png`), fullPage });
  report.screenshots.push(`${name}.png`);
}
async function visual(kind, width, theme) {
  await page.setViewportSize({ width, height: width === 390 ? 844 : 900 });
  await go(kind);
  const toggle = page.getByRole("switch", { name: "浅色模式" });
  if ((await toggle.getAttribute("aria-checked") === "true") !== (theme === "light")) await toggle.click();
  await expect(page.locator("html")).toHaveClass(new RegExp(theme));
  const dimensions = await page.evaluate(() => ({ scroll: document.documentElement.scrollWidth, client: document.documentElement.clientWidth }));
  assert.ok(dimensions.scroll <= dimensions.client, `Horizontal overflow: ${JSON.stringify(dimensions)}`);
  await capture(`${kind}-${width}-${theme}`);
  if (width === 390) await capture(`${kind}-${width}-${theme}-full`, true);
  const result = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  report.accessibility.push({ kind, width, theme, violations: result.violations.map(v => ({ id: v.id, impact: v.impact, nodes: v.nodes.map(n => n.target) })) });
  assert.deepEqual(result.violations.map(v => v.id), [], `axe ${kind}/${width}/${theme}`);
  report.checks.push(`${kind}/${width}/${theme}: no overflow; axe WCAG A/AA zero violations`);
}
try {
  await scenario("normal");
  // Test reset only: replace a previous run's conservative denied grant cache
  // using the same actual live-authorized GET, without reading its payload.
  assert.equal((await context.request.get(`${manifest.origin}${endpoint}`, { headers: { "X-Expected-Organization-ID": "org-B" } })).status(), 200);
  for (const kind of ["options", "entitlements"]) for (const width of [1440, 390]) for (const theme of ["dark", "light"]) await visual(kind, width, theme);
  await page.setViewportSize({ width: 1440, height: 900 });
  await go();
  await expect(usage()).toContainText("1 作业次");
  await expect(region("已记录用量概要")).toContainText("9007199254740993 字节");
  await expect(region("已记录用量概要")).toContainText("-1 字节");
  await expect(region("已记录用量概要")).toContainText("0 作业次");
  await expect(region("已记录用量概要")).toContainText("未知（作业次）");
  await expect(region("已授予权益")).toContainText("不限额（作业次）");
  await switchOrg("org-C"); await expect(usage()).toContainText("2 作业次");
  await switchOrg("org-B"); await expect(usage()).toContainText("1 作业次");
  report.checks.push("actual home-A/effective org-B -> org-C -> org-B; distinct persisted usage, exact int64, signed byte delta, known zero, unknown with units, unlimited grants");
  for (const [org, label] of [["org-empty", "无订阅"], ["org-expired", "已过期"], ["org-disabled", "已停用"], ["org-future", "尚未开始"], ["org-custom", "Existing paid contract"]]) {
    await switchOrg(org); await expect(region("实际订阅")).toContainText(label);
    if (org === "org-empty") await expect(region("已授予权益")).toContainText("暂无已授予权益");
    await capture(org);
  }
  report.checks.push("actual empty/expired/disabled/future/custom subscription and missing grants");
  await switchOrg("org-B");
  for (const [mode, status] of [["revoked", 403], ["role-downgraded", 403], ["suspended", 403], ["signed-out", 401], ["provider-unavailable", 503], ["database-unavailable", 503]]) {
    await scenario(mode);
    const denied = page.waitForResponse(r => new URL(r.url()).pathname === endpoint);
    await page.getByRole("button", { name: "刷新数据" }).click();
    assert.equal((await denied).status(), status);
    await expect(region("实际订阅")).toHaveCount(0);
    await expect(page.locator("#console-main")).not.toContainText("9007199254740993");
    await capture(mode);
    await scenario("normal");
    // The BFF clears the effective-org cookie after revoked access. Re-select
    // through the actual switch UI, which performs live authorization again.
    await switchOrg("org-C"); await switchOrg("org-B");
  }
  report.checks.push("refresh reauthorizes: revoked, downgraded role, suspended, signed-out, provider/database failure discard old success");
  await scenario("slow");
  const started = page.waitForRequest(r => new URL(r.url()).pathname === endpoint);
  await page.getByRole("button", { name: "刷新数据" }).click(); await started;
  await expect(region("实际订阅")).toHaveCount(0);
  await page.getByRole("combobox", { name: "当前企业" }).selectOption("org-C");
  await scenario("normal"); await ready(); await expect(usage()).toContainText("2 作业次");
  // Wait past the fixture's 2s response delay to catch a stale response/error.
  await page.waitForTimeout(2300);
  await expect(usage()).toContainText("2 作业次");
  await expect(page.locator("#console-main")).not.toContainText("9007199254740993");
  report.checks.push("in-flight slow org-B refresh then org-C; old success/error absent after delay");
  await switchOrg("org-B");
  await page.getByRole("button", { name: "刷新数据" }).focus();
  const focus = await page.getByRole("button", { name: "刷新数据" }).evaluate(el => ({ focused: el === document.activeElement, outline: getComputedStyle(el).outlineStyle, shadow: getComputedStyle(el).boxShadow }));
  assert.equal(focus.focused, true);
  await page.keyboard.press("Tab"); await page.keyboard.press("Shift+Tab");
  await expect(page.getByRole("button", { name: "刷新数据" })).toBeFocused();
  const refreshed = page.waitForResponse(r => new URL(r.url()).pathname === endpoint);
  await page.keyboard.press("Enter"); assert.equal((await refreshed).status(), 200); await ready();
  await page.setViewportSize({ width: 390, height: 844 });
  const menu = page.getByRole("button", { name: "打开工作台导航" });
  await menu.focus(); await page.keyboard.press("Enter");
  await expect(page.getByRole("navigation", { name: "移动工作台导航" })).toBeVisible();
  await page.keyboard.press("Escape"); await expect(menu).toBeFocused();
  report.checks.push("keyboard refresh via Enter, Tab/Shift+Tab reachability; mobile menu Enter/Escape focus return");
  await capture("entitlements-keyboard-390");
  // Layout-only stress probe, explicitly separate from real contract acceptance.
  await region("实际订阅").locator("h3").evaluate(el => { el.textContent = "超长企业定制套餐名称".repeat(20) + "A".repeat(150); });
  assert.ok(await page.evaluate(() => document.documentElement.scrollWidth <= document.documentElement.clientWidth));
  await capture("entitlements-390-long-name-layout-probe", true);
  report.checks.push("390 long unbroken name DOM-only layout probe; no contract/persistence modification");
  await context.addCookies(manifest.sessions.viewer); await page.reload();
  await expect(region("实际订阅")).toHaveCount(0);
  await expect(page.getByText("无查看权限", { exact: true })).toBeVisible();
  await capture("viewer-subject-denied");
  await context.addCookies(manifest.sessions.owner); await go();
  await page.locator(".console-user summary").click();
  await page.getByRole("link", { name: "退出登录", exact: true }).click();
  await expect(page).not.toHaveURL(/\/workbench\//);
  await page.goto(`${manifest.origin}/workbench/plans/entitlements`);
  await expect(region("实际订阅")).toHaveCount(0);
  assert.equal((await context.request.get(`${manifest.origin}${endpoint}`, { headers: { "X-Expected-Organization-ID": "org-B" } })).status(), 401);
  const session = await context.request.get(`${manifest.origin}/api/auth/session`);
  assert.equal((await session.json())?.user, undefined);
  report.checks.push("viewer subject denied; actual Auth.js logout clears session; commercial BFF returns 401 and no previous UI data");
  report.notRun.push("external sign-in after logout: fixture issuer is deliberately unreachable; actual login redirects to Auth.js configuration error");
  assert.ok(report.requests.filter(r => r.path === endpoint).every(r => r.method === "GET"));
  assert.ok(report.requests.every(r => r.method === "GET" || (r.path === "/api/workbench/context/effective-organization" && r.method === "PUT")));
  report.checks.push("all commercial requests GET; only context selection PUT among workbench writes");
  report.passed = true;
} finally {
  await scenario("normal");
  await context.close(); await browser.close();
  report.finishedAt = new Date().toISOString();
  await writeFile(join(output, "browser-report.json"), JSON.stringify(report, null, 2));
  console.log(JSON.stringify({ passed: report.passed, head, checks: report.checks.length, report: join(output, "browser-report.json") }));
}
