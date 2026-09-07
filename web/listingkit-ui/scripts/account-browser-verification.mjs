// Usage: node scripts/account-browser-verification.mjs <private fixture.json> [output directory]
// The normal scenarios use the actual account client, Auth.js/BFF and Go readers.
import { chromium, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
import { readFile, mkdir, writeFile } from "node:fs/promises";
import { resolve, join } from "node:path";
import { execFileSync } from "node:child_process";
import { createHash } from "node:crypto";

const fixture = JSON.parse(await readFile(process.argv[2], "utf8"));
const output = resolve(process.argv[3] ?? "../../docs/engineering/evidence/issue348/browser");
await mkdir(output, { recursive: true });
const browser = await chromium.launch({ headless: true });
const results = [], screenshots = [];
const report = { sourceHead: execFileSync("git", ["rev-parse", "HEAD"], { encoding: "utf8" }).trim(), dependencyFixtureHead: fixture.sourceHead, externalBoundary: fixture.externalBoundary, results, screenshots };
const files = execFileSync("git", ["ls-files", "--cached", "--others", "--exclude-standard", "src", "public/console/account"], { encoding: "utf8" }).trim().split(/\r?\n/);
const sourceHash = createHash("sha256");
for (const file of [...new Set(files)].sort()) sourceHash.update(`${file}\0`).update((await readFile(file, "utf8")).replaceAll("\r\n", "\n"));
report.sourceTreeSHA256 = sourceHash.digest("hex"); report.sourceFileCount = new Set(files).size;
async function open(user, path, width = 1440, theme = "light", selection = true) {
  const context = await browser.newContext({ viewport: { width, height: 900 }, colorScheme: theme });
  await context.addCookies(fixture.sessions[user].filter(cookie => selection || cookie.name !== "shuomi_effective_organization"));
  await context.addInitScript(theme => localStorage.setItem("listingkit-theme", theme), theme);
  const page = await context.newPage();
  await page.goto(`${fixture.origin}/workbench/account/${path}`);
  return { context, page };
}
async function snapshot(page, name) {
  await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.evaluate(() => window.scrollTo(0, 0));
  // Hide only the Next development-tools overlay in the exported image, not product UI.
  await page.screenshot({ path: join(output, `${name}.png`), fullPage: true, style: "nextjs-portal { visibility: hidden; }" }); screenshots.push(`${name}.png`);
}
async function check(name, operation) { await operation(); results.push(name); console.log(`PASS ${name}`); }
try {
  for (const pageKind of ["profile", "organization"]) for (const theme of ["light", "dark"]) for (const width of [1440, 390]) {
    await check(`${pageKind} ${theme} ${width}: actual client/BFF/Go, no overflow, axe, keyboard`, async () => {
      const { page, context } = await open("u1", pageKind, width, theme);
      try {
        await expect(page.getByRole("heading", { name: pageKind === "profile" ? "Fixture u1" : "Enterprise B", exact: true })).toBeVisible();
        await expect(page.getByRole("switch", { name: "浅色模式" })).toHaveAttribute("aria-checked", String(theme === "light"));
        await expect(page.getByText("归属企业（Home）：A", { exact: true })).toBeVisible();
        if (pageKind === "profile") await expect(page.getByText(/@PHONE.INVALID/i)).toHaveCount(0);
        else await expect(page.getByText("当前有效企业：B", { exact: true })).toBeVisible();
        const axe = await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa"]).analyze();
        expect(axe.violations).toEqual([]);
        if (width === 390) {
          const trigger = page.getByRole("button", { name: "打开工作台导航" }); await trigger.focus(); await page.keyboard.press("Enter");
          await expect(page.getByRole("navigation", { name: "移动工作台导航" })).toBeVisible(); await page.keyboard.press("Escape"); await expect(trigger).toBeFocused();
        }
        const provenance = page.getByText("资料来源与读取时间", { exact: true }); await provenance.focus(); await page.keyboard.press("Enter");
        await expect(page.getByText(/读取时间：/)).toBeVisible(); await page.keyboard.press("Enter");
        await page.keyboard.press("Tab"); await expect(page.getByRole("button", { name: "刷新资料", exact: true })).toBeFocused();
        await page.keyboard.press("Shift+Tab"); await expect(provenance).toBeFocused();
        await page.getByRole("switch", { name: "浅色模式" }).focus(); await page.keyboard.press("Tab");
        const accountMenu = page.locator(".console-user > summary"); await expect(accountMenu).toBeFocused(); await page.keyboard.press("Enter");
        await page.keyboard.press("Tab"); await expect(page.locator(".console-user").getByRole("link", { name: "账户资料", exact: true })).toBeFocused();
        await page.keyboard.press("Tab"); await expect(page.getByRole("link", { name: "退出登录", exact: true })).toBeFocused();
        await page.keyboard.press("Shift+Tab"); await expect(page.locator(".console-user").getByRole("link", { name: "账户资料", exact: true })).toBeFocused();
        await page.keyboard.press("Shift+Tab"); await page.keyboard.press("Enter");
        await snapshot(page, `${pageKind}-${theme}-${width}`);
      } finally { await context.close(); }
    });
  }
  for (const [user, selection] of [["no-org", false], ["u1", false], ["grant-down", false]]) await check(`profile independent of enterprise: ${user}, selection=${selection}`, async () => {
    const { page, context } = await open(user, "profile", 390, "light", selection);
    try { await expect(page.getByRole("heading", { name: `Fixture ${user}`, exact: true })).toBeVisible(); expect(page.url()).toContain("/account/profile"); await snapshot(page, `profile-${user}-no-selection`); } finally { await context.close(); }
  });
  for (const [user, text] of [["down", "资料服务暂不可用"], ["mismatch", "资料响应无效"], ["expired", "登录状态已失效，请重新登录"]]) await check(`actual provider error: ${user}`, async () => {
    const { page, context } = await open(user, "profile");
    try { await expect(page.getByRole("alert").filter({ hasText: text })).toBeVisible(); await expect(page.getByText(/provider-private|@PHONE.INVALID/)).toHaveCount(0); } finally { await context.close(); }
  });
  await check("actual enterprise switch clears B and reads C", async () => {
    const { page, context } = await open("u1", "organization");
    try { await expect(page.getByRole("heading", { name: "Enterprise B", exact: true })).toBeVisible(); await page.getByRole("combobox", { name: "当前企业" }).selectOption("C"); await expect(page.getByRole("heading", { name: "Enterprise C", exact: true })).toBeVisible(); await expect(page.getByRole("heading", { name: "Enterprise B", exact: true })).toHaveCount(0); await snapshot(page, "organization-switched-C"); } finally { await context.close(); }
  });
  await check("actual identity changes reauthorize and clear old profile", async () => {
    const { page, context } = await open("u1", "organization");
    try { await expect(page.getByRole("heading", { name: "Enterprise B", exact: true })).toBeVisible(); await page.getByRole("navigation", { name: "工作台导航" }).getByRole("link", { name: "账户资料", exact: true }).click(); await expect(page.getByRole("heading", { name: "Fixture u1", exact: true })).toBeVisible(); await context.addCookies(fixture.sessions.u2); await page.getByRole("button", { name: "刷新资料" }).click(); await expect(page.getByRole("alert").filter({ hasText: "登录身份已变化" })).toBeVisible(); await expect(page.getByText("Fixture u1", { exact: true })).toHaveCount(0); } finally { await context.close(); }
  });
  await check("actual slow request is cancelled when leaving profile", async () => {
    const { page, context } = await open("slow", "profile");
    try { await expect(page.getByText("正在读取资料", { exact: true })).toBeVisible(); await page.getByRole("navigation", { name: "工作台导航" }).getByRole("link", { name: "企业空间", exact: true }).click(); await expect(page.getByRole("heading", { name: "Enterprise B", exact: true })).toBeVisible(); await expect(page.getByText("资料读取超时", { exact: true })).toHaveCount(0); } finally { await context.close(); }
  });
  await check("actual pending profile logout aborts the request and clears the Auth.js session", async () => {
    const { page, context } = await open("slow", "profile");
    try {
      await expect(page.getByText("正在读取资料", { exact: true })).toBeVisible();
      const failed = page.waitForEvent("requestfailed", { predicate: request => new URL(request.url()).pathname === "/api/account/profile" });
      await page.locator(".console-user > summary").click(); await page.getByRole("link", { name: "退出登录", exact: true }).click();
      await failed;
      await expect.poll(async () => (await context.cookies()).filter(cookie => cookie.name === "authjs.session-token").length).toBe(0);
      await expect(page.getByText("资料读取超时", { exact: true })).toHaveCount(0);
    } finally { await context.close(); }
  });
  await check("supplemental synthetic long-name layout at 390; account response stub only", async () => {
    const { page, context } = await open("u1", "organization", 390);
    try {
      await expect(page.getByRole("heading", { name: "Enterprise B", exact: true })).toBeVisible();
      const longName = "跨境企业很长的显示名称".repeat(14);
      await page.route("**/api/account/organization", async route => {
        const response = await route.fetch(); const value = await response.json();
        await route.fulfill({ response, json: { ...value, name: longName, roles: ["role_" + "x".repeat(120)] } });
      });
      await page.getByRole("button", { name: "刷新资料" }).click(); await expect(page.getByRole("heading", { name: longName, exact: true })).toBeVisible();
      await snapshot(page, "supplemental-synthetic-long-name-390");
    } finally { await context.close(); }
  });
  await check("actual grant revoke after supported cache expiry clears enterprise data", async () => {
    const { page, context } = await open("u1", "organization");
    try { await expect(page.getByRole("heading", { name: "Enterprise B", exact: true })).toBeVisible(); await fetch(`${fixture.controlOrigin}/revoke`, { method: "POST" }); await fetch(`${fixture.controlOrigin}/expire`, { method: "POST" }); await page.getByRole("button", { name: "刷新资料" }).click(); await expect(page.getByRole("alert").filter({ hasText: "企业访问已撤销" })).toBeVisible(); await expect(page.getByRole("heading", { name: "Enterprise B", exact: true })).toHaveCount(0); } finally { await fetch(`${fixture.controlOrigin}/restore`, { method: "POST" }); await fetch(`${fixture.controlOrigin}/expire`, { method: "POST" }); await context.close(); }
  });
  await check("unauthenticated server navigation never requests a profile", async () => {
    const context = await browser.newContext(); const page = await context.newPage(); let reads = 0;
    page.on("request", request => { if (new URL(request.url()).pathname === "/api/account/profile") reads++; });
    try { await page.goto(`${fixture.origin}/workbench/account/profile`); await expect(page).toHaveURL(/\/login\?/); expect(reads).toBe(0); } finally { await context.close(); }
  });
  for (const path of ["account/organization", "ai/tasks", "stores", "plans/options"]) await check(`actual no-membership enterprise gate: ${path}`, async () => {
    const context = await browser.newContext(); await context.addCookies(fixture.sessions["no-org"].filter(cookie => cookie.name === "authjs.session-token"));
    const page = await context.newPage();
    try { await page.goto(`${fixture.origin}/workbench/${path}`); await expect(page).toHaveURL(/\/workbench\/no-organization$/); } finally { await context.close(); }
  });
  report.status = "PASS";
} catch (error) { report.status = "FAIL"; report.failure = error.message; throw error; }
finally { await writeFile(join(output, "report.json"), JSON.stringify(report, null, 2)); await browser.close(); }
