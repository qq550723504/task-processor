import { readFileSync } from "node:fs";
import AxeBuilder from "@axe-core/playwright";
import { expect, test, type BrowserContext, type Page } from "@playwright/test";

const manifestPath = process.env.ISSUE324_FIXTURE_MANIFEST;
if (!manifestPath) throw new Error("Start the #323 isolated fixture and set ISSUE324_FIXTURE_MANIFEST to its fixture.json.");
const fixture = JSON.parse(readFileSync(manifestPath, "utf8"));
if (!/^http:\/\/127\.0\.0\.1:\d+$/.test(fixture.origin)) throw new Error("Only the loopback #323 fixture is permitted.");
const pagePath = `${fixture.origin}/workbench/shein-records/${fixture.recordId}/diagnostic`;
const diagnosticPath = `/api/listing/shein-records/${fixture.recordId}/offline-diagnostic`;

test.beforeEach(async ({ context }) => { await context.addCookies(fixture.sessions.owner); });

async function openReport(page: Page) {
  const response = page.waitForResponse((r) => r.url().includes(diagnosticPath));
  await page.goto(pagePath);
  const actual = await response;
  expect(actual.status()).toBe(200);
  expect(actual.request().method()).toBe("GET");
  expect(actual.request().headers().authorization).toBeUndefined();
  expect(actual.request().headers()["x-expected-organization-id"]).toBe("200");
  const payload = await actual.json();
  expect(payload.diagnostic_only).toBe(true);
  expect(payload.action).toBe("publish");
  expect(payload.offline_checks.blockers.length).toBeGreaterThan(0);
  expect(payload.not_evaluated).toContain("submission_gate");
  await expect(page.getByText("发现需要处理的问题")).toBeVisible();
  return payload;
}

test("actual browser → BFF → Go → isolated PG: refresh, digest, action and organization switch", async ({ page }, info) => {
  const requestedOrigins = new Set<string>();
  page.on("request", (request) => requestedOrigins.add(new URL(request.url()).origin));
  await page.setViewportSize({ width: 1440, height: 1000 });
  const first = await openReport(page);
  expect((await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze()).violations).toEqual([]);
  await page.screenshot({ path: info.outputPath("desktop-real-bff.png"), fullPage: true });
  let response = page.waitForResponse((r) => r.url().includes(diagnosticPath));
  await page.getByRole("button", { name: "重新检查", exact: true }).click();
  const refresh = await response;
  expect(new URL(refresh.url()).searchParams.has("expected_digest")).toBe(false);
  expect((await refresh.json()).input.actual_digest).toBe(first.input.actual_digest);
  response = page.waitForResponse((r) => r.url().includes(diagnosticPath));
  await page.getByRole("button", { name: "复核同一内容" }).click();
  expect(new URL((await response).url()).searchParams.get("expected_digest")).toBe(first.input.actual_digest);
  response = page.waitForResponse((r) => r.url().includes(diagnosticPath));
  await page.getByLabel("检查动作").selectOption("save_draft");
  const draft = await response;
  expect(new URL(draft.url()).searchParams.has("expected_digest")).toBe(false);
  expect((await draft.json()).action).toBe("save_draft");
  response = page.waitForResponse((r) => r.url().includes(diagnosticPath));
  await page.getByLabel("当前企业", { exact: true }).selectOption("100");
  expect((await response).status()).toBe(404);
  await expect(page.getByRole("main").getByRole("alert")).toContainText("资料不存在或当前账号不可读取");
  await expect(page.getByText("发现需要处理的问题")).toHaveCount(0);
  response = page.waitForResponse((r) => r.url().includes(diagnosticPath));
  await page.getByLabel("当前企业", { exact: true }).selectOption("200");
  expect((await response).status()).toBe(200);
  await expect(page.getByLabel("检查动作")).toHaveValue("publish");
  await expect(page.getByText("发现需要处理的问题")).toBeVisible();
  expect(requestedOrigins.has(fixture.goOrigin)).toBe(false);
  expect(requestedOrigins.has(fixture.contextOrigin)).toBe(false);
});

test("actual 390px page: keyboard disclosure, no horizontal overflow and accessibility", async ({ page }, info) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await openReport(page);
  const summary = page.getByRole("region", { name: /^问题（/ }).locator("summary").first();
  await summary.focus();
  await expect(summary).toBeFocused();
  await summary.press("Enter");
  await expect(page.getByRole("region", { name: /^问题（/ }).getByText("修正建议").first()).toBeVisible();
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  const audit = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze();
  expect(audit.violations).toEqual([]);
  await page.screenshot({ path: info.outputPath("narrow-real-bff.png"), fullPage: true });
});

test("actual controlled Store-only identity cannot read Listing diagnostics", async ({ page, context }) => {
  await context.addCookies(fixture.sessions.store);
  await page.goto(pagePath);
  await expect(page.getByRole("main").getByRole("alert")).toContainText("没有读取这份资料的权限");
  await expect(page.getByText("发现需要处理的问题")).toHaveCount(0);
});

test("actual Go identity timeout clears prior UI success", async ({ page, context }) => {
  await openReport(page);
  await context.addCookies(fixture.sessions.slow);
  const response = page.waitForResponse((r) => r.url().includes(diagnosticPath));
  await page.getByRole("button", { name: "重新检查", exact: true }).click();
  await expect(page.getByText("发现需要处理的问题")).toHaveCount(0);
  expect((await response).status()).toBe(504);
  await expect(page.getByRole("main").getByRole("alert")).toContainText("检查超时");
});

async function syntheticResponse(context: BrowserContext, payload: unknown, status: number) {
  await context.route(`**${diagnosticPath}?*`, (route) => route.fulfill({ status, contentType: "application/json", body: JSON.stringify(payload) }));
}

for (const [name, status, payload, text] of [
  ["permission", 403, { error: "permission_denied" }, "没有读取这份资料的权限"],
  ["not found", 404, { error: "not_found" }, "资料不存在或当前账号不可读取"],
  ["timeout", 504, { error: "deadline_exceeded" }, "检查超时"],
  ["content change", 409, { error: "stale_input" }, "资料内容或有效性已变化"],
  ["invalid response", 200, { offline_checks: { blockers: null } }, "诊断响应不合法"],
] as const) {
  test(`synthetic browser response only: ${name}`, async ({ page, context }) => {
    await syntheticResponse(context, payload, status);
    await page.goto(pagePath);
    await expect(page.getByRole("main").getByRole("alert")).toContainText(text);
    await expect(page.getByText("发现需要处理的问题")).toHaveCount(0);
  });
}
