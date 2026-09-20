// Task-owned integration harness: real Next/Auth.js/BFF/client/current Go app/PG.
// External identity HTTP and encrypted session issuance are synthetic, not IAM acceptance.
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { createWriteStream, readFileSync } from "node:fs";
import { access, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, resolve, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createRequire } from "node:module";
import { createServer as netServer } from "node:net";
import { createServer as httpServer } from "node:http";
import assert from "node:assert/strict";
import { encode } from "@auth/core/jwt";
import ts from "typescript";

const web = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repo = resolve(web, "../..");
const host = "127.0.4.12"; // A task-specific loopback host avoids other Account fixtures' cookie jars.
const dir = await mkdtemp(join(tmpdir(), "issue412-audit-"));
const children = []; let go, next, port, bootstrap, bootstrapPort;
const pause = ms => new Promise(resolvePause => setTimeout(resolvePause, ms));
function run(command, args, options = {}) {
  return new Promise((resolveRun, reject) => {
    const child = spawn(command, args, { cwd: repo, windowsHide: true, ...options }); let output = "";
    child.stdout?.on("data", chunk => output += chunk); child.stderr?.on("data", chunk => output += chunk);
    child.on("error", reject); child.on("exit", code => code === 0 ? resolveRun(output.trim()) : reject(new Error(`${command} exit ${code}: ${output.slice(-2000)}`)));
  });
}
function start(command, args, name, options = {}) {
  const log = createWriteStream(join(dir, name));
  const child = spawn(command, args, { cwd: repo, windowsHide: true, stdio: ["ignore", "pipe", "pipe"], ...options });
  child.stdout.pipe(log); child.stderr.pipe(log); child.on("exit", () => log.end()); children.push(child); return child;
}
async function until(check, label, timeout = 180000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) {
    try { const value = await check(); if (value) return value; } catch { /* startup */ }
    if (children.some(child => child.exitCode !== null)) throw new Error(`Fixture exited during ${label}; ${dir}`);
    await pause(250);
  }
  throw new Error(`Timeout ${label}; ${dir}`);
}
async function probe(preferred = 0) {
  const server = netServer(); await new Promise((resolveListen, reject) => { server.once("error", reject); server.listen(preferred, host, resolveListen); });
  const result = server.address().port; await new Promise(resolveClose => server.close(resolveClose)); return result;
}
function loadClient(file) {
  const exports = {}, require = createRequire(file);
  const code = ts.transpileModule(readFileSync(file, "utf8"), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2022 } }).outputText;
  new Function("require", "exports", code)(name => name.startsWith(".") ? loadClient(resolve(dirname(file), `${name}.ts`)) : require(name), exports);
  return exports;
}
let closing;
async function cleanup() {
  return closing ??= (async () => {
    if (bootstrap) await new Promise(resolveClose => bootstrap.close(resolveClose));
    if (next && next.exitCode === null) {
      if (process.platform === "win32") await run("taskkill", ["/PID", String(next.pid), "/T", "/F"]);
      else next.kill("SIGTERM");
      await until(async () => next.exitCode !== null, "Next stop", 10000);
    }
    if (go && go.exitCode === null) {
      await writeFile(join(dir, "stop-go"), "stop");
      const end = Date.now() + 20000; while (go.exitCode === null && Date.now() < end) await pause(100);
      assert.equal(go.exitCode, 0, "Go fixture must verify zero writes and finish with PASS");
    }
    if (port) await probe(port); if (bootstrapPort) await probe(bootstrapPort);
    await writeFile(join(dir, "cleanup.json"), JSON.stringify({ goExit: go?.exitCode, nextExit: next?.exitCode, portsReleased: true }));
  })();
}
process.once("SIGINT", () => { void cleanup().then(() => process.exit(0)); });
process.once("SIGTERM", () => { void cleanup().then(() => process.exit(0)); });
try {
  const binary = join(dir, process.platform === "win32" ? "audit.test.exe" : "audit.test");
  console.log(`Audit fixture building; logs ${dir}`);
  await run("go", ["test", "-tags=integration", "-c", "-o", binary, "./internal/app/httpapi"]);
  go = start(binary, ["-test.run=^TestAccountAuditBrowserFixture$", "-test.v", "-test.timeout=30m"], "go.log", { env: { ...process.env, ISSUE412_FIXTURE_DIR: dir } });
  const seed = await until(async () => JSON.parse(await readFile(join(dir, "go.json"), "utf8")), "Go current app / task PG");
  port = await probe(); const origin = `http://${host}:${port}`, secret = randomBytes(48).toString("base64url");
  next = start(process.execPath, [join(web, "node_modules/next/dist/bin/next"), "dev", "--hostname", host, "--port", String(port)], "next.log", { cwd: web,
    env: { ...process.env, NODE_ENV: "development", AUTH_SECRET: secret, AUTH_URL: origin, LISTINGKIT_PUBLIC_BASE_URL: origin, ZITADEL_ISSUER_URL: seed.issuerURL, ZITADEL_CLIENT_ID: "fixture-client", ZITADEL_CLIENT_SECRET: "", LISTINGKIT_SERVICE_API_BASE: `${seed.goOrigin}/api/v1` } });
  const sessions = {};
  for (const user of seed.tokens) sessions[user] = await encode({ secret, salt: "authjs.session-token", maxAge: 1800, token: { sub: user, name: `Fixture ${user}`, accessToken: user, expiresAt: Math.floor(Date.now() / 1000) + 1800, identityVersion: 3, identity: { tenantId: "A", userId: user, roles: [], userType: "zitadel" } } });
  await until(async () => (await fetch(`${origin}/api/auth/session`)).ok, "Next Auth.js");
  const sourceHead = await run("git", ["rev-parse", "HEAD"]);
  bootstrap = httpServer((request, response) => {
    const match = /^\/session\/(u1|expired|grant-down|no-org)\/(B|C|A)$/.exec(request.url ?? "");
    if (request.method !== "GET" || !match) { response.writeHead(404); response.end(); return; }
    response.writeHead(303, { "Cache-Control": "no-store", "Set-Cookie": [`authjs.session-token=${sessions[match[1]]}; Path=/; HttpOnly; SameSite=Lax`, `shuomi_effective_organization=${match[2]}; Path=/; HttpOnly; SameSite=Lax`], Location: `${origin}/workbench/account/organization/audit` }); response.end();
  });
  await new Promise(resolveListen => bootstrap.listen(0, host, resolveListen)); bootstrapPort = bootstrap.address().port;
  const manifest = { origin, ...seed, sourceHead, controlDirectory: dir, bootstrapOrigin: `http://${host}:${bootstrapPort}`, externalBoundary: "synthetic external identity HTTP and encrypted session issuance; actual Next/BFF/Go/current source/PG" };
  await writeFile(join(dir, "fixture.json"), JSON.stringify(manifest));
  console.log(JSON.stringify({ manifest: join(dir, "fixture.json"), origin, bootstrapOrigin: manifest.bootstrapOrigin }));
  const { getAccountAudit } = loadClient(join(web, "src/lib/api/account-audit.ts"));
  const nativeFetch = globalThis.fetch; let user = "u1", selected = "B";
  globalThis.fetch = (input, init = {}) => nativeFetch(new URL(String(input), origin), { ...init, headers: { ...Object.fromEntries(new Headers(init.headers)), cookie: `authjs.session-token=${sessions[user]}; shuomi_effective_organization=${selected}` } });
  const checks = []; const options = () => ({ expectedUserId: user, expectedOrganizationId: selected });
  async function check(name, action) { await action(); checks.push(name); console.log(`PASS ${name}`); }
  try {
    await check("actual client -> Auth.js BFF -> current Go app -> owner PG history", async () => {
      const first = await getAccountAudit(options()); assert.equal(first.items.length, 20); assert.equal(first.effectiveOrganizationId, "B");
      const last = await getAccountAudit({ ...options(), cursor: first.nextCursor }); assert.equal(last.items.length, 5); assert.equal(last.nextCursor, null);
      assert.equal(new Set([...first.items, ...last.items].map(item => `${item.objectReference}:${item.relation.version}`)).size, 25);
      assert.ok(first.items.every(item => item.actor === "u1" && item.operation === "register" && item.result === "succeeded"));
    });
    await check("empty authorized C differs from unavailable", async () => { selected = "C"; assert.equal((await getAccountAudit(options())).items.length, 0); selected = "B"; });
    await check("cross-org A denied", async () => { selected = "A"; await assert.rejects(getAccountAudit(options()), { status: 403 }); selected = "B"; });
    await check("expired identity denied", async () => { user = "expired"; await assert.rejects(getAccountAudit(options()), { code: "AUTHENTICATION_REQUIRED" }); user = "u1"; });
    await check("revocation is fresh on every page", async () => { await nativeFetch(`${seed.controlOrigin}/revoke`, { method: "POST" }); await assert.rejects(getAccountAudit(options()), { status: 403 }); await nativeFetch(`${seed.controlOrigin}/restore`, { method: "POST" }); });
    await check("identity dependency outage fails closed", async () => { await nativeFetch(`${seed.controlOrigin}/unavailable`, { method: "POST" }); await assert.rejects(getAccountAudit(options()), { code: "DEPENDENCY_UNAVAILABLE" }); await nativeFetch(`${seed.controlOrigin}/restore`, { method: "POST" }); });
    await check("restored live grants recover without stale empty audit", async () => { assert.equal((await getAccountAudit(options())).items.length, 20); });
  } finally { globalThis.fetch = nativeFetch; }
  await writeFile(join(dir, "integration.json"), JSON.stringify({ sourceHead, checks, fixtureOnlyIdentity: true }));
  if (process.argv.includes("--serve")) {
    console.log(`READY ${manifest.bootstrapOrigin}/session/u1/B`);
    const end = Date.now() + 25 * 60000;
    while (Date.now() < end) { try { await access(join(dir, "stop-fixture")); break; } catch { /* serving */ } await pause(500); }
  }
} finally { await cleanup(); }
