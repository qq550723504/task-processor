// Isolated external-provider substitute; real Auth.js, Next BFF, Go auth/readers,
// and the production TypeScript client. No database or real IAM credentials.
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { createWriteStream, readFileSync } from "node:fs";
import { access, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, resolve, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { createServer } from "node:net";
import assert from "node:assert/strict";
import { encode } from "@auth/core/jwt";
import ts from "typescript";

const sourceWeb = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repo = resolve(sourceWeb, "../..");
const webArg = process.argv.indexOf("--web-dir");
const web = webArg < 0 ? sourceWeb : resolve(process.argv[webArg + 1]);
const serve = process.argv.includes("--serve");
const dir = await mkdtemp(join(tmpdir(), "issue346-"));
const children = [];
let go, next, port, closing;
function run(command, args, options = {}) {
  return new Promise((resolveRun, reject) => {
    const p = spawn(command, args, { cwd: repo, windowsHide: true, ...options });
    let output = ""; p.stdout?.on("data", d => output += d); p.stderr?.on("data", d => output += d);
    p.on("error", reject); p.on("exit", code => code === 0 ? resolveRun(output.trim()) : reject(new Error(`${command} failed (${code}): ${output.slice(-1800)}`)));
  });
}
function start(command, args, log, options = {}) {
  const output = createWriteStream(join(dir, log));
  const p = spawn(command, args, { cwd: repo, windowsHide: true, stdio: ["ignore", "pipe", "pipe"], ...options });
  p.stdout.pipe(output); p.stderr.pipe(output); p.on("exit", () => output.end());
  children.push(p); return p;
}
const pause = ms => new Promise(r => setTimeout(r, ms));
async function until(check, label, timeout = 120000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) {
    try { const value = await check(); if (value) return value; } catch { /* startup */ }
    if (children.some(p => p.exitCode !== null)) throw new Error(`Fixture exited during ${label}; logs: ${dir}`);
    await pause(250);
  }
  throw new Error(`Timeout: ${label}; logs: ${dir}`);
}
async function portProbe(preferred = 0) {
  const s = createServer(); await new Promise((r, reject) => { s.once("error", reject); s.listen(preferred, "127.0.0.1", r); });
  const result = s.address().port; await new Promise(r => s.close(r)); return result;
}
async function cleanup() {
  return closing ??= (async () => {
    if (next && next.exitCode === null) {
      if (process.platform === "win32") await run("taskkill", ["/PID", String(next.pid), "/T", "/F"]);
      else next.kill("SIGTERM");
    }
    if (go && go.exitCode === null) {
      await writeFile(join(dir, "stop-go"), "stop");
      const end = Date.now() + 10000; while (go.exitCode === null && Date.now() < end) await pause(100);
      assert.equal(go.exitCode, 0, "Go fixture should finish with PASS");
    }
    if (port) { await pause(200); await portProbe(port); }
    await writeFile(join(dir, "cleanup.json"), JSON.stringify({ goExit: go?.exitCode, portReleased: true }));
  })();
}
process.once("SIGINT", () => { void cleanup().then(() => process.exit(0)); });
process.once("SIGTERM", () => { void cleanup().then(() => process.exit(0)); });

// Compile the actual client and strict JSON reader without substituting their
// logic. Only fetch's browser origin/cookie transport is supplied by this harness.
function loadClient(file) {
  const exports = {};
  const require = createRequire(file);
  const code = ts.transpileModule(readFileSync(file, "utf8"), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
  new Function("require", "exports", code)(name => name.startsWith(".") ? loadClient(resolve(dirname(file), `${name}.ts`)) : require(name), exports);
  return exports;
}

try {
  const binary = join(dir, process.platform === "win32" ? "account.test.exe" : "account.test");
  console.log(`Building isolated account fixture; logs ${dir}`);
  await run("go", ["test", "-c", "-o", binary, "./internal/app/httpapi"]);
  go = start(binary, ["-test.run=^TestAccountBrowserFixture$", "-test.v", "-test.timeout=30m"], "go.log", { env: { ...process.env, ISSUE346_FIXTURE_DIR: dir } });
  const seed = await until(async () => JSON.parse(await readFile(join(dir, "go.json"), "utf8")), "real Go auth/readers");
  port = await portProbe(); const origin = `http://127.0.0.1:${port}`; const secret = randomBytes(48).toString("base64url");
  next = start(process.execPath, [join(web, "node_modules/next/dist/bin/next"), "dev", "--hostname", "127.0.0.1", "--port", String(port)], "next.log", {
    cwd: web, env: { ...process.env, NODE_ENV: "development", AUTH_SECRET: secret, AUTH_URL: origin, LISTINGKIT_PUBLIC_BASE_URL: origin,
      ZITADEL_ISSUER_URL: seed.providerOrigin, ZITADEL_CLIENT_ID: "fixture-client", ZITADEL_CLIENT_SECRET: "", LISTINGKIT_SERVICE_API_BASE: `${seed.goOrigin}/api/v1` },
  });
  const sessions = {};
  for (const user of seed.tokens) {
    const value = await encode({ secret, salt: "authjs.session-token", maxAge: 1800, token: { sub: user, name: `Fixture ${user}`, accessToken: user, expiresAt: Math.floor(Date.now() / 1000) + 1800, identityVersion: 3, identity: { tenantId: "A", userId: user, roles: [], userType: "zitadel" } } });
    sessions[user] = [{ name: "authjs.session-token", value, url: origin, httpOnly: true, sameSite: "Lax" }, { name: "shuomi_effective_organization", value: "B", url: origin, httpOnly: true, sameSite: "Lax" }];
  }
  const sourceHead = await run("git", ["rev-parse", "HEAD"]);
  const manifest = { origin, ...seed, sessions, controlDirectory: dir, sourceHead, webDirectory: web, externalBoundary: "synthetic Auth.js session issuance and external OIDC/UserInfo/Authorization HTTP only; real BFF/client/Go verifier/grants/cache/switch/audit" };
  await writeFile(join(dir, "fixture.json"), JSON.stringify(manifest), { mode: 0o600 });
  await until(async () => (await fetch(`${origin}/api/auth/session`)).ok, "Next Auth.js");
  console.log(JSON.stringify({ manifest: join(dir, "fixture.json"), origin }));
  if (serve) {
    console.log("UI fixture ready. Write stop-fixture to controlDirectory to stop; maximum 27 minutes.");
    const end = Date.now() + 27 * 60000;
    while (Date.now() < end) { try { await access(join(dir, "stop-fixture")); break; } catch { /* running */ } await pause(500); }
  } else {
    const { getAccountProfile, getAccountOrganization } = loadClient(join(sourceWeb, "src/lib/api/account.ts"));
    const actualFetch = globalThis.fetch;
    let user = "u1", selected = "B";
    const cookie = () => `authjs.session-token=${sessions[user][0].value}${selected ? `; shuomi_effective_organization=${selected}` : ""}`;
    globalThis.fetch = (input, init = {}) => actualFetch(new URL(String(input), origin), { ...init, headers: { ...Object.fromEntries(new Headers(init.headers)), cookie: cookie() } });
    const results = [];
    async function check(name, operation) { await operation(); results.push(name); console.log(`PASS ${name}`); }
    const org = () => getAccountOrganization({ expectedUserId: user, expectedOrganizationId: selected });
    try {
      await check("real client → Next serverAuth → Go introspection → userinfo", async () => { const p = await getAccountProfile({ expectedUserId: user }); assert.equal(p.userId, "u1"); assert.equal(p.homeOrganizationId, "A"); assert.equal(p.email, null); assert.equal(p.emailVerified, null); });
      await check("Home A differs from Effective B", async () => { const o = await org(); assert.equal(o.homeOrganizationId, "A"); assert.equal(o.effectiveOrganizationId, "B"); assert.deepEqual(o.roles, ["listingkit_viewer"]); });
      await check("real live switch and audit chain", async () => { const r = await actualFetch(`${origin}/api/workbench/context/effective-organization`, { method: "PUT", headers: { cookie: cookie(), "Content-Type": "application/json" }, body: JSON.stringify({ organizationId: "C" }) }); assert.equal(r.status, 200, await r.text()); selected = "C"; assert.equal((await org()).effectiveOrganizationId, "C"); });
      await check("wrong expected user fails closed", async () => { await assert.rejects(getAccountProfile({ expectedUserId: "u2" }), { code: "IDENTITY_CONTEXT_CHANGED" }); });
      await check("stale browser org assertion fails closed", async () => { await assert.rejects(getAccountOrganization({ expectedUserId: user, expectedOrganizationId: "B" }), { code: "ORGANIZATION_CONTEXT_CHANGED" }); });
      await check("userinfo subject mismatch rejected", async () => { user = "mismatch"; await assert.rejects(getAccountProfile({ expectedUserId: user }), { code: "INVALID_UPSTREAM_RESPONSE" }); });
      await check("profile independent of no membership and no selection", async () => { user = "no-org"; selected = ""; assert.equal((await getAccountProfile({ expectedUserId: user })).userId, user); });
      await check("profile independent of authorization dependency outage", async () => { user = "grant-down"; assert.equal((await getAccountProfile({ expectedUserId: user })).userId, user); });
      await check("provider failure is not absent profile", async () => { user = "down"; await assert.rejects(getAccountProfile({ expectedUserId: user }), { status: 503, code: "DEPENDENCY_UNAVAILABLE" }); });
      await check("token revocation is not cached authorization", async () => { user = "expired"; await assert.rejects(getAccountProfile({ expectedUserId: user }), { status: 401, code: "AUTHENTICATION_REQUIRED" }); });
      await check("grant revoke cache window then expiry", async () => { user = "u1"; selected = "B"; await org(); await actualFetch(`${seed.controlOrigin}/revoke`, { method: "POST" }); await org(); await actualFetch(`${seed.controlOrigin}/expire`, { method: "POST" }); await assert.rejects(org(), { code: "ORGANIZATION_ACCESS_REVOKED" }); });
      await check("cancelled actual slow read cannot return profile", async () => { user = "slow"; const controller = new AbortController(); const pending = getAccountProfile({ expectedUserId: user, signal: controller.signal }); setTimeout(() => controller.abort(), 200); await assert.rejects(pending, { code: "DEADLINE_EXCEEDED" }); });
    } finally { globalThis.fetch = actualFetch; }
    await writeFile(join(dir, "report.json"), JSON.stringify({ sourceHead, results, NOT_RUN: ["real IAM", "real login issuance", "production", "UI rendering (owned by #348)"], externalBoundary: manifest.externalBoundary }, null, 2));
    console.log(`PASS ${results.length} actual client/BFF/Go groups; report ${join(dir, "report.json")}`);
  }
} finally { await cleanup(); }
