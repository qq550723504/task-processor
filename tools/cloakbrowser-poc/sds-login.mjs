import fs from "node:fs/promises";
import path from "node:path";
import { setTimeout as sleep } from "node:timers/promises";
import { launchPersistentContext } from "cloakbrowser";

const merchantName = process.env.TASK_PROCESSOR_SDS_MERCHANT_NAME?.trim() || "xuweixia";
const username = process.env.TASK_PROCESSOR_SDS_USERNAME?.trim() || "13480610496";
const password = process.env.TASK_PROCESSOR_SDS_PASSWORD?.trim() || "Zone5571886$$$";
const proxy = process.env.TASK_PROCESSOR_SDS_PROXY?.trim() || "";
const headless = process.env.TASK_PROCESSOR_SDS_HEADLESS === "true";
const profileDir = path.resolve(process.cwd(), "../../.local/tmp/cloakbrowser/sds-profile");
const artifactDir = path.resolve(process.cwd(), "../../.local/tmp/cloakbrowser/sds-artifacts");
const loginURL = "https://www.sdsdiy.com/user/login?redirect=%2Fadmin%2Fmaterial";
const targetURL = "https://www.sdsdiy.com/admin/material";

async function ensureDir(dir) {
  await fs.mkdir(dir, { recursive: true });
}

async function writeJson(filePath, payload) {
  await fs.writeFile(filePath, `${JSON.stringify(payload, null, 2)}\n`, "utf8");
}

async function visibleText(page) {
  try {
    const raw = await page.locator("body").textContent();
    return String(raw || "").replace(/\s+/g, " ").trim().slice(0, 3000);
  } catch {
    return "";
  }
}

async function firstVisible(page, selectors) {
  for (const selector of selectors) {
    const loc = page.locator(selector).first();
    try {
      if (await loc.isVisible()) {
        return loc;
      }
    } catch {}
  }
  throw new Error(`visible element not found: ${selectors.join(" | ")}`);
}

async function typeSlow(loc, value) {
  await loc.click();
  try {
    await loc.press("Control+A");
    await loc.press("Backspace");
  } catch {}
  await loc.type(value, { delay: 85 });
  try {
    await loc.press("Tab");
  } catch {}
}

async function readState(page) {
  return page.evaluate(() => {
    const readNum = (key) => {
      const raw = window.localStorage.getItem(key);
      if (!raw) return 0;
      const num = Number(raw);
      return Number.isFinite(num) ? num : 0;
    };
    const text = document.body?.innerText ?? "";
    const hasVerifyInput =
      Boolean(document.querySelector("#verifyCode")) ||
      Boolean(document.querySelector('input[placeholder*="验证码"]')) ||
      Boolean(document.querySelector('input[autocomplete="one-time-code"]'));
    return {
      href: window.location.href,
      token: window.localStorage.getItem("token") || "",
      outToken: window.localStorage.getItem("outToken") || "",
      merchantId: readNum("merchant_id"),
      userId: readNum("userid"),
      hasVerifyInput,
      bodyText: String(text).replace(/\s+/g, " ").trim().slice(0, 3000)
    };
  });
}

async function captureArtifacts(page, name, extra = {}) {
  const stamp = new Date().toISOString().replace(/[:.]/g, "-");
  const dir = path.join(artifactDir, `${stamp}-${name}`);
  await ensureDir(dir);
  await page.screenshot({ path: path.join(dir, "page.png"), fullPage: true });
  await fs.writeFile(path.join(dir, "page.html"), await page.content(), "utf8");
  await writeJson(path.join(dir, "state.json"), {
    ...(await readState(page)),
    ...(extra || {})
  });
  return dir;
}

async function main() {
  await ensureDir(artifactDir);
  const ctx = await launchPersistentContext({
    userDataDir: profileDir,
    headless,
    proxy: proxy || undefined,
    geoip: Boolean(proxy),
    locale: "zh-CN",
    timezone: "Asia/Shanghai",
    viewport: { width: 1440, height: 960 }
  });
  const page = ctx.pages()[0] || await ctx.newPage();
  try {
    await page.goto(loginURL, { waitUntil: "domcontentloaded", timeout: 90000 });
    await sleep(3000);

    const merchantInput = await firstVisible(page, [
      'input[placeholder*="商户"]',
      'input[name="merchant_name"]',
      'input[id*="merchant"]',
      'input[autocomplete="organization"]'
    ]);
    const usernameInput = await firstVisible(page, [
      'input[placeholder*="手机"]',
      'input[placeholder*="账号"]',
      'input[placeholder*="用户名"]',
      'input[name="username"]',
      'input[name="account"]',
      'input[type="text"]',
      'input[type="tel"]'
    ]);
    const passwordInput = await firstVisible(page, [
      'input[type="password"]',
      'input[placeholder*="密码"]',
      'input[name="password"]'
    ]);

    await typeSlow(merchantInput, merchantName);
    await sleep(500);
    await typeSlow(usernameInput, username);
    await sleep(500);
    await typeSlow(passwordInput, password);
    await sleep(2500);

    const loginButton = await firstVisible(page, [
      'button:has-text("登录")',
      'button[type="submit"]',
      'div[role="button"]:has-text("登录")',
      'span:has-text("登录")'
    ]);
    await loginButton.click({ timeout: 5000 });

    const startedAt = Date.now();
    let lastState = await readState(page);
    while (Date.now() - startedAt < 90000) {
      await sleep(2000);
      lastState = await readState(page);
      if (lastState.token && lastState.merchantId > 0 && lastState.href && !lastState.href.includes("/user/login")) {
        const artifact = await captureArtifacts(page, "success", { targetURL, lastState });
        console.log(JSON.stringify({ success: true, artifact, state: lastState }, null, 2));
        return;
      }
      if (lastState.hasVerifyInput) {
        const artifact = await captureArtifacts(page, "verify-required", { targetURL, lastState });
        console.log(JSON.stringify({ success: false, waiting_for_verify_code: true, artifact, state: lastState }, null, 2));
        return;
      }
    }

    const artifact = await captureArtifacts(page, "timeout", { targetURL, lastState, bodyText: await visibleText(page) });
    console.log(JSON.stringify({ success: false, waiting_for_verify_code: false, artifact, state: lastState }, null, 2));
  } finally {
    await ctx.close();
  }
}

main().catch((error) => {
  console.error(error);
  process.exitCode = 1;
});
