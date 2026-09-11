import assert from "node:assert/strict";
import { execFile as execFileCallback } from "node:child_process";
import { readFile, realpath, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { promisify } from "node:util";

const execFile = promisify(execFileCallback);
const ui = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const repo = path.resolve(ui, "../..");
const [expectedSha] = process.argv.slice(2);
const runtime = path.join(repo, "scripts/issue357-runtime.mjs");
let runId;
let runDirectory;
let browser;

async function command(executable, args, options = {}) {
  const result = await execFile(executable, args, { cwd: repo, windowsHide: true, maxBuffer: 8 * 1024 * 1024, ...options });
  return result.stdout.trim();
}

async function control(action, ...args) {
  return command(process.execPath, [runtime, action, "--run", runId, ...args]);
}

async function json(file) {
  const raw = await readFile(file);
  assert.ok(raw.length < 262144);
  return JSON.parse(raw.toString("utf8"));
}

async function login(manifest, user) {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  const page = await context.newPage();
  const credentialsPath = await realpath(manifest.users[user].credentialFile);
  assert.equal(path.dirname(credentialsPath), await realpath(runDirectory));
  const credentials = await json(credentialsPath);
  let officialLogin = false;
  let codeFlow = false;
  page.on("request", request => {
    const url = new URL(request.url());
    if (url.origin === manifest.origins.issuer && url.pathname.startsWith("/ui/v2/login")) officialLogin = true;
    if (url.origin === manifest.origins.issuer && url.pathname === "/oauth/v2/authorize") codeFlow = url.searchParams.get("response_type") === "code" && url.searchParams.get("code_challenge_method") === "S256";
  });
  await page.goto(`${manifest.origins.web}/workbench/account/profile`, { waitUntil: "load" });
  const username = page.getByTestId("username-text-input");
  await username.waitFor({ state: "visible", timeout: 45000 });
  await username.fill(credentials.username);
  await page.getByTestId("submit-button").click();
  const password = page.getByTestId("password-text-input");
  await password.waitFor({ state: "visible", timeout: 30000 });
  await password.fill(credentials.password);
  await page.getByTestId("submit-button").click();
  await page.waitForURL(url => url.origin === manifest.origins.web && url.pathname === "/workbench/account/profile", { timeout: 45000 });
  assert.ok(officialLogin && codeFlow, "official Login V2 authorization-code PKCE flow was not observed");
  const sessionResponse = await context.request.get(`${manifest.origins.web}/api/auth/session`);
  const session = await sessionResponse.json();
  assert.equal(session.identity?.userId, manifest.users[user].id);
  const cookies = (await context.cookies(manifest.origins.web)).filter(cookie => /(?:authjs|next-auth)\.session-token/.test(cookie.name));
  assert.ok(cookies.length >= 1);
  await context.close();
  return { subject: manifest.users[user].id, cookie: cookies.map(cookie => `${cookie.name}=${cookie.value}`).join("; ") };
}

async function runClient(manifest, sessions, phase) {
  const chainManifest = path.join(runDirectory, "issue390-chain.json");
  const statePath = path.join(runDirectory, "issue390-state.json");
  const evidencePath = path.join(runDirectory, "issue390-evidence.json");
  await writeFile(chainManifest, JSON.stringify({ origin: manifest.origins.web, goOrigin: manifest.origins.go, phase, statePath, evidencePath, organizations: { B: manifest.organizations.B.id, C: manifest.organizations.C.id }, sessions }), { mode: 0o600 });
  const pnpm = process.platform === "win32" ? ["cmd.exe", ["/c", "pnpm.cmd", "exec", "vitest", "run", "--config", "e2e/issue390-current-application.config.ts"]] : ["pnpm", ["exec", "vitest", "run", "--config", "e2e/issue390-current-application.config.ts"]];
  await command(pnpm[0], pnpm[1], { cwd: ui, env: { ...process.env, CURRENT_APPLICATION_CHAIN_MANIFEST: chainManifest } });
  return { statePath, evidencePath };
}

try {
  assert.match(expectedSha ?? "", /^[a-f0-9]{40}$/);
  assert.equal(await command("git", ["rev-parse", "HEAD"]), expectedSha);
  assert.equal(await command("git", ["status", "--porcelain"]), "");
  const started = await command(process.execPath, [runtime, "start", "--current-application"]);
  runId = started.match(/runId=([0-9a-f-]{36})/)?.[1];
  assert.ok(runId, "owned run identifier missing");
  runDirectory = path.join(tmpdir(), "task-processor-issue357", runId);
  const manifest = await json(path.join(runDirectory, "manifest.json"));
  assert.equal(manifest.runtimeMode, "current-application");
  const services = await json(path.join(runDirectory, "services.json"));
  assert.equal(path.basename(services.binary), "current-application.exe");
  assert.deepEqual(services.goArgs.slice(0, 1), ["-config"]);
  const { chromium } = await import("@playwright/test");
  browser = await chromium.launch({ headless: true });
  const sessions = { admin: await login(manifest, "admin"), viewer: await login(manifest, "viewer") };
  const adminHeaders = { Cookie: `${sessions.admin.cookie}; shuomi_effective_organization=${manifest.organizations.B.id}`, Origin: manifest.origins.web, "Sec-Fetch-Site": "same-origin", "X-Expected-User-ID": manifest.users.admin.id, "X-Expected-Organization-ID": manifest.organizations.B.id };
  assert.equal((await fetch(`${manifest.origins.web}/api/account/profile`, { headers: adminHeaders })).status, 200);
  assert.equal((await fetch(`${manifest.origins.web}/api/workbench/commercial/overview`, { headers: adminHeaders })).status, 200);
  assert.equal((await fetch(`${manifest.origins.go}/api/v1/workbench/source-accounts`)).status, 401);
  const paths = await runClient(manifest, sessions, "create");
  await control("revoke", "--user", "admin", "--org", "B");
  try { await runClient(manifest, sessions, "revoked"); }
  finally { await control("restore", "--user", "admin", "--org", "B"); }

  const stopped = JSON.parse(await control("stop"));
  assert.equal(stopped.passed, true);
  assert.equal(stopped.resourcesRetained, true);
  assert.deepEqual({ resources: stopped.factsRetained.resources, operations: stopped.factsRetained.operations }, { resources: 3, operations: 6 });

  await control("start");
  await runClient(manifest, sessions, "verify");
  await control("restart");
  await runClient(manifest, sessions, "verify");
  await control("stop");
  await control("source-permission-revoke");
  await assert.rejects(control("start"));
  assert.equal((await json(path.join(runDirectory, "manifest.json"))).status, "start-failed");
  await control("source-permission-restore");
  await control("start");
  await runClient(manifest, sessions, "verify");
  await control("stop");
  await control("provider-stop");
  await assert.rejects(control("start"));
  assert.equal((await json(path.join(runDirectory, "manifest.json"))).status, "start-failed");
  await control("provider-start");
  await control("start");
  await runClient(manifest, sessions, "verify");
  await control("provider-stop");
  try {
    const unavailable = await fetch(`${manifest.origins.web}/api/account/profile`, { headers: adminHeaders });
    assert.ok([401, 502, 503, 504].includes(unavailable.status));
  } finally {
    await control("provider-start");
    await control("check");
  }
  const evidence = await json(paths.evidencePath);
  assert.equal(evidence.passed, true);
  console.log(`PASS RUN-1 normal binary + official Login V2/Auth.js + SA2 client/BFF + SA1 + retained restart facts; run=${runId}`);
} finally {
  if (browser) await browser.close().catch(() => {});
  if (runId) {
    try {
      const destroyed = JSON.parse(await control("destroy"));
      assert.equal(destroyed.resourcesReleased, true);
      await assert.rejects(readFile(path.join(runDirectory, "issue390-chain.json")), error => error.code === "ENOENT");
      console.log(`DESTROYED owned run ${runId}; evidence retained at ${runDirectory}`);
    } catch {
      console.error(`DESTROY_INCOMPLETE run=${runId}`);
      process.exitCode = 1;
    }
  }
}
