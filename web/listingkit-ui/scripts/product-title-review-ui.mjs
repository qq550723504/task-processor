// Browser acceptance only. The sole #344 fixture owns servers, data and fault controls.
// node scripts/product-title-review-ui.mjs <#344 fixture.json> <evidence-directory>
import { readFile, writeFile, mkdir, access, rm } from "node:fs/promises";
import { resolve, join } from "node:path";
import assert from "node:assert/strict";
import { chromium, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";

const fixture = JSON.parse(await readFile(process.argv[2], "utf8"));
const output = resolve(process.argv[3]);
assert.match(fixture.origin, /^http:\/\/127\.0\.0\.1:\d+$/);
await mkdir(output, { recursive: true });
const results = [];
async function control(name) {
  await rm(join(fixture.controlDirectory, `${name}-done`), { force: true });
  await writeFile(join(fixture.controlDirectory, name), "");
  await expect.poll(async () => { try { await access(join(fixture.controlDirectory, `${name}-done`)); return true; } catch { return false; } }).toBe(true);
}
async function observe() { await control("observe"); return JSON.parse(await readFile(join(fixture.controlDirectory, "observations.json"), "utf8")); }
const browser = await chromium.launch({ headless: true });
const context = await browser.newContext({ viewport: { width: 1440, height: 900 } });
await context.addCookies(fixture.sessions.admin);
const page = await context.newPage();
page.setDefaultTimeout(20000);
const posts = [];
page.on("request", (request) => { if (request.method() === "POST" && request.url().includes("/api/product/text-proposals/")) posts.push({ url: request.url(), body: request.postData(), key: request.headers()["idempotency-key"] }); });
const detail = page.getByRole("region", { name: "标题提案详情" });
const rows = () => page.getByRole("button", { name: /查看详情/ });
async function changeList(click) {
  const response = page.waitForResponse((r) => r.request().method() === "GET" && new URL(r.url()).pathname === "/api/product/text-proposals");
  await click(); assert.equal((await response).status(), 200);
  await expect(page.getByText("正在读取标题提案", { exact: true })).toHaveCount(0);
  await expect(rows()).toHaveCount((await (await response).json()).items.length);
}
async function select(product) {
  await changeList(() => page.getByRole("button", { name: "刷新记录", exact: true }).click());
  for (let index = 0; index < 3; index++) {
    const row = rows().filter({ has: page.getByText(product, { exact: true }) }).first();
    if (await row.count()) {
      const response = page.waitForResponse((r) => r.request().method() === "GET" && /\/api\/product\/text-proposals\/[^/?]+$/.test(r.url()));
      await row.click(); const data = await (await response).json();
      await expect(detail.getByText("标题修改", { exact: true })).toBeVisible();
      await expect(page.getByRole("heading", { name: "标题提案详情", exact: true })).toBeFocused();
      return data;
    }
    await changeList(() => page.getByRole("button", { name: "下一页", exact: true }).click());
  }
  throw new Error(`No discoverable proposal for ${product}`);
}
async function action(name, endpoint = "decisions") {
  const response = page.waitForResponse((r) => r.request().method() === "POST" && r.url().endsWith(`/${endpoint}`));
  await detail.getByRole("button", { name, exact: true }).click();
  const result = await response; assert.equal(result.status(), 200, name); return result.json();
}
async function apply() {
  await detail.getByRole("button", { name: "应用到标准商品", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("button", { name: "返回审核", exact: true })).toBeFocused();
  return action("确认应用", "apply");
}
async function axe(label) {
  const violations = (await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]).analyze()).violations;
  assert.deepEqual(violations.map((v) => ({ id: v.id, impact: v.impact, nodes: v.nodes.map((n) => n.target) })), [], label);
  results.push(`${label}: axe zero violations`);
}
async function shot(name, fullPage = false) { await page.screenshot({ path: join(output, name), fullPage }); }
try {
  await page.goto(`${fixture.origin}/workbench/ai/tasks`);
  await page.getByRole("heading", { name: "已完成工作记录", exact: true }).waitFor();
  const nav = page.getByRole("navigation", { name: "任务状态" });
  await nav.getByRole("link", { name: "待确认", exact: true }).click();
  await page.waitForURL(`${fixture.origin}/workbench/ai/tasks/pending`);
  await expect(nav.getByRole("link", { name: "待确认", exact: true })).toHaveAttribute("aria-current", "page");
  await rows().first().waitFor();
  const first = await select("product");
  await page.evaluate(() => window.scrollTo(0, 0));
  await shot("pending-1440.png"); await axe("pending desktop");
  const baseline = await observe();
  const accepted = await action("接受提案");
  assert.equal(accepted.state, "accepted");
  assert.deepEqual((await observe()).versions, baseline.versions);
  const count = posts.length;
  await detail.getByRole("button", { name: "应用到标准商品", exact: true }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("button", { name: "返回审核", exact: true })).toBeFocused();
  await page.keyboard.press("Tab");
  await expect(dialog.getByRole("button", { name: "确认应用", exact: true })).toBeFocused();
  await page.keyboard.press("Tab");
  // Native Chromium may visit browser chrome between the dialog endpoints.
  // No background app control receives focus; the next Tab re-enters the modal.
  if (await page.evaluate(() => document.activeElement === document.body)) await page.keyboard.press("Tab");
  await expect(dialog.getByRole("button", { name: "返回审核", exact: true })).toBeFocused();
  await shot("confirm-1440.png"); await axe("native confirmation");
  await page.keyboard.press("Escape");
  await expect(dialog).toHaveCount(0); assert.equal(posts.length, count);
  await expect(detail.getByRole("button", { name: "应用到标准商品", exact: true })).toBeFocused();
  const applied = await apply();
  await expect(detail.getByText("已生成新版本", { exact: true })).toBeVisible();
  assert.equal(applied.apply_receipt.product_version, "2");
  const persisted = await observe(); assert.equal(persisted.titles["200/product"], applied.after);
  assert.equal(persisted.versions["200/product"], 2);
  await shot("receipt-1440.png", true);
  results.push("real entry/GET/evidence, accept does not mutate Catalog, separate Apply creates one durable version and receipt; native modal keyboard cycle/Escape restore");

  const rejected = await select("product");
  await action("拒绝提案");
  await expect(detail.getByText("已拒绝", { exact: true })).toBeVisible();
  await expect(detail.getByRole("button", { name: "应用到标准商品", exact: true })).toHaveCount(0);
  assert.deepEqual((await observe()).versions, persisted.versions);
  const stale = await select("product"); await action("接受提案");
  await detail.getByRole("button", { name: "应用到标准商品", exact: true }).click();
  const conflictResponse = page.waitForResponse((r) => r.request().method() === "POST" && r.url().endsWith("/apply"));
  await detail.getByRole("button", { name: "确认应用", exact: true }).click();
  assert.equal((await conflictResponse).status(), 409);
  await expect(detail.getByText("资料已变化，请重新读取", { exact: true })).toBeVisible();
  assert.deepEqual((await observe()).versions, persisted.versions);
  results.push("reject is terminal without Catalog write; stale base Apply conflicts without overwrite");

  await select("edit-product"); await action("接受提案");
  await detail.getByRole("button", { name: "编辑标题", exact: true }).click();
  await expect(detail.getByRole("button", { name: "应用到标准商品", exact: true })).toHaveCount(0);
  const longTitle = "人工复核的不锈钢保温杯 · 轻便耐用款式 · " .repeat(18).trim();
  await detail.getByRole("textbox", { name: "编辑标题", exact: true }).fill(longTitle);
  const edited = await action("保存为待审核提案"); assert.equal(edited.state, "pending");
  assert.equal(edited.revision, "3"); assert.equal(edited.after, longTitle);
  await page.setViewportSize({ width: 390, height: 844 });
  await expect(detail.getByRole("button", { name: "接受提案", exact: true })).toBeVisible();
  assert.equal(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth), true);
  await shot("pending-long-title-390.png", true); await axe("pending narrow long title");
  await action("接受提案");
  await detail.getByRole("button", { name: "应用到标准商品", exact: true }).click();
  await shot("confirm-390.png"); await axe("confirmation narrow");
  const editApplied = await action("确认应用", "apply");
  assert.equal(editApplied.apply_receipt.revision, "4");
  assert.equal((await observe()).titles["200/edit-product"], longTitle);
  results.push("accepted edit clears approval, long-title revision 3 is pending, fresh acceptance revision 4 and explicit Apply persist edited title; 390 no horizontal overflow");

  await page.setViewportSize({ width: 1440, height: 900 });
  await select("lost-product"); await action("接受提案");
  await control("lose-response");
  await detail.getByRole("button", { name: "应用到标准商品", exact: true }).click();
  const loss = page.waitForResponse((r) => r.request().method() === "POST" && r.url().endsWith("/apply"));
  await detail.getByRole("button", { name: "确认应用", exact: true }).click();
  assert.equal((await loss).status(), 502);
  await expect(detail.getByText("结果待核实", { exact: true })).toBeVisible();
  const lostIntent = posts.at(-1); const postCount = posts.length;
  const lostPersisted = await observe(); assert.equal(lostPersisted.versions["200/lost-product"], 2);
  await detail.getByRole("button", { name: "重新读取当前状态", exact: true }).click();
  await expect(detail.getByText("已生成新版本", { exact: true })).toBeVisible();
  await expect(detail.getByText("结果待核实", { exact: true })).toBeVisible();
  assert.equal(posts.length, postCount);
  await shot("unknown-receipt-1440.png", true);
  await control("restart");
  await action("核实本次操作", "apply");
  await expect(detail.getByText("结果待核实", { exact: true })).toHaveCount(0);
  await expect(detail.getByText("已生成新版本", { exact: true })).toBeVisible();
  assert.deepEqual(posts.at(-1), lostIntent);
  const confirmed = await observe(); assert.deepEqual(confirmed.versions, lostPersisted.versions);
  assert.equal(confirmed.nonTitleUnchanged, true); assert.equal(confirmed.listingUnchanged, true);
  await page.reload(); await expect(detail.getByText("已生成新版本", { exact: true })).toBeVisible();
  assert.equal(posts.length, postCount + 1);
  results.push("actual commit/TCP response loss stays unknown even with receipt; reconstructed Go + explicit exact-key replay yields same version; refresh does not POST; non-title and original Listing unchanged");

  await select("revoked-product"); await action("接受提案"); await control("revoke");
  await detail.getByRole("button", { name: "应用到标准商品", exact: true }).click();
  const revokedResponse = page.waitForResponse((r) => r.request().method() === "POST" && r.url().endsWith("/apply"));
  await detail.getByRole("button", { name: "确认应用", exact: true }).click();
  assert.equal((await revokedResponse).status(), 403);
  await expect(page.getByText("当前企业访问已撤销", { exact: true })).toBeVisible();
  await expect(page.getByRole("region", { name: "标题提案详情" })).toHaveCount(0);
  assert.equal((await observe()).versions["200/revoked-product"], 1);
  await shot("revoked-1440.png"); await control("restore");
  results.push("actual LiveWrite revocation rejects Apply, clears sensitive projection, leaves Catalog unchanged");
  await writeFile(join(output, "browser-evidence.json"), JSON.stringify({ sourceHead: fixture.sourceHead, goHead: fixture.goHead, results, observations: await observe(), exercised: { applied: first.proposal_id, rejected: rejected.proposal_id, stale: stale.proposal_id }, externalSubstitutes: ["session/token issuance", "grant provider", "CandidateGenerator"], notRun: ["production", "real IAM", "paid model"] }, null, 2));
  console.log(`PASS: ${results.length} browser acceptance groups; actual BFF/Go/PG; evidence ${output}`);
} catch (error) { await shot("browser-failure.png", true); throw error; }
finally { await browser.close(); }
