// Browser transport fault tests against the sole #344 fixture; no response data is fabricated.
import { readFile, writeFile, access, rm } from "node:fs/promises";
import { join, resolve } from "node:path";
import assert from "node:assert/strict";
import { chromium, expect } from "@playwright/test";
const fixture = JSON.parse(await readFile(process.argv[2], "utf8"));
const output = resolve(process.argv[3]);
const skipSwitch = process.argv.includes("--skip-switch");
assert.match(fixture.origin, /^http:\/\/127\.0\.0\.1:\d+$/);
async function observe() {
  await rm(join(fixture.controlDirectory, "observe-done"), { force: true });
  await writeFile(join(fixture.controlDirectory, "observe"), "");
  await expect.poll(async () => { try { await access(join(fixture.controlDirectory, "observe-done")); return true; } catch { return false; } }).toBe(true);
  return JSON.parse(await readFile(join(fixture.controlDirectory, "observations.json"), "utf8"));
}
function deferred() { let resolve; const promise = new Promise((r) => { resolve = r; }); return { promise, resolve }; }
const browser = await chromium.launch();
try {
  const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  await context.addCookies(fixture.sessions.admin);
  const page = await context.newPage(); page.setDefaultTimeout(20000);
  const detail = page.getByRole("region", { name: "标题提案详情" });
  const posted = [];
  page.on("request", (r) => { if (r.method() === "POST" && r.url().includes("/api/product/text-proposals/")) posted.push({ key: r.headers()["idempotency-key"], body: r.postData(), url: r.url() }); });
  async function enter() { await page.goto(`${fixture.origin}/workbench/ai/tasks/pending`); await page.getByRole("button", { name: /查看详情/ }).first().waitFor(); }
  async function openPending() {
    const row = page.getByRole("button", { name: /查看详情/ }).filter({ has: page.getByText("待审核", { exact: true }) }).first();
    await row.click(); await detail.getByRole("button", { name: "接受提案", exact: true }).waitFor();
  }
  async function switchOrg(id) {
    const collection = page.waitForResponse((r) => r.request().method() === "GET" && new URL(r.url()).pathname === "/api/product/text-proposals" && r.request().headers()["x-expected-organization-id"] === id);
    await page.getByRole("combobox", { name: "当前企业", exact: true }).selectOption(id);
    assert.equal((await collection).status(), 200);
    await expect(detail.getByText("选择一条标题提案", { exact: true })).toBeVisible();
  }
  async function holdDecision() {
    const committed = deferred(), release = deferred(), finished = deferred();
    const handler = async (route) => {
      try {
        const response = await route.fetch(); assert.equal(response.status(), 200);
        committed.resolve(); await release.promise;
        // Preserve the actual BFF response; a canceled browser may reject delivery.
        try { await route.fulfill({ response }); } catch { /* caller already canceled */ }
      } finally { finished.resolve(); }
    };
    await page.route("**/api/product/text-proposals/*/decisions", handler);
    return { committed: committed.promise, release: async () => { release.resolve(); await finished.promise; await page.unroute("**/api/product/text-proposals/*/decisions", handler); } };
  }
  const results = [];
  await enter();
  const before = await observe();
  if (!skipSwitch) {
  await openPending();
  await detail.getByRole("button", { name: "编辑标题", exact: true }).click();
  await detail.getByRole("textbox", { name: "编辑标题", exact: true }).fill("仅企业 A 的未保存编辑");
  await switchOrg("300"); await expect(page.getByText("仅企业 A 的未保存编辑", { exact: true })).toHaveCount(0);
  await switchOrg("200"); assert.equal(posted.length, 0);
  results.push("actual organization switch clears selection/editor; return does not restore old selection or POST");

  await openPending(); const late = await holdDecision();
  await detail.getByRole("button", { name: "接受提案", exact: true }).dblclick();
  await late.committed; assert.equal(posted.length, 1);
  assert.equal((await observe()).postCount, before.postCount + 1);
  await switchOrg("300"); await late.release();
  await expect(detail.getByRole("button", { name: "应用到标准商品", exact: true })).toHaveCount(0);
  await switchOrg("200"); assert.equal(posted.length, 1);
  assert.deepEqual((await observe()).versions, before.versions);
  results.push("double click sends one actual POST; switch aborts waiting and late actual response cannot populate new scope; Catalog unchanged by acceptance");
  }

  await openPending(); const canceled = await holdDecision();
  const beforeDouble = posted.length;
  await detail.getByRole("button", { name: "接受提案", exact: true }).dblclick(); await canceled.committed;
  assert.equal(posted.length, beforeDouble + 1);
  const original = posted.at(-1); const count = posted.length;
  await detail.getByRole("button", { name: "停止等待", exact: true }).click();
  await expect(detail.getByText("结果待核实", { exact: true })).toBeVisible();
  await canceled.release(); assert.equal(posted.length, count);
  const verified = page.waitForResponse((r) => r.request().method() === "POST" && r.url().endsWith("/decisions"));
  await detail.getByRole("button", { name: "核实本次操作", exact: true }).click();
  assert.equal((await verified).status(), 200);
  await expect(detail.getByRole("button", { name: "应用到标准商品", exact: true })).toBeVisible();
  assert.deepEqual(posted.at(-1), original);
  assert.deepEqual((await observe()).versions, before.versions);
  results.push("double click sends once; stop waiting after actual commit is unknown; only explicit verification sends same key/body; no new Product version");

  await page.getByRole("button", { name: "刷新记录", exact: true }).click();
  await expect(detail.getByText("选择一条标题提案", { exact: true })).toBeVisible();
  await openPending(); const unmounted = await holdDecision();
  await detail.getByRole("button", { name: "接受提案", exact: true }).click(); await unmounted.committed;
  const beforeLeaving = posted.length;
  await page.getByRole("navigation", { name: "任务状态" }).getByRole("link", { name: "已完成", exact: true }).click();
  await page.waitForURL(`${fixture.origin}/workbench/ai/tasks/completed`);
  await unmounted.release();
  await expect(page.getByRole("region", { name: "标题提案详情" })).toHaveCount(0);
  await page.getByRole("navigation", { name: "任务状态" }).getByRole("link", { name: "待确认", exact: true }).click();
  await page.waitForURL(`${fixture.origin}/workbench/ai/tasks/pending`);
  await expect(detail.getByText("选择一条标题提案", { exact: true })).toBeVisible();
  assert.equal(posted.length, beforeLeaving);
  results.push("navigation unmount cancels waiting; late actual response does not overwrite completed page; re-entry never replays POST");

  // Existing read-only projection uses the same running Go/Catalog database after Apply.
  await page.getByRole("navigation", { name: "任务状态" }).getByRole("link", { name: "已完成", exact: true }).click();
  await page.waitForURL(`${fixture.origin}/workbench/ai/tasks/completed`);
  await page.getByRole("button", { name: /查看详情/ }).first().click();
  await page.getByRole("link", { name: "查看诊断", exact: true }).click();
  await page.getByRole("heading", { name: "未评估范围", exact: true }).waitFor();
  await page.screenshot({ path: join(output, "regression-diagnostic-after-apply-1440.png") });
  const final = await observe(); assert.equal(final.nonTitleUnchanged, true); assert.equal(final.listingUnchanged, true);
  results.push("existing completed -> diagnostic remains readable after title Apply; real fixture verifies original Listing and non-title data unchanged");
  await writeFile(join(output, skipSwitch ? "browser-lifecycle-partial.json" : "browser-lifecycle-evidence.json"), JSON.stringify({ sourceHead: fixture.sourceHead, goHead: fixture.goHead, results, observations: final, notRun: skipSwitch ? ["organization switch and late response across scopes: fixture LiveSwitch audit dependency missing"] : [], faultBoundary: "browser delivery holds the actual BFF response after real Go success; no replacement body" }, null, 2));
  console.log(`PASS: ${results.length} actual browser lifecycle groups`);
} finally { await browser.close(); }
