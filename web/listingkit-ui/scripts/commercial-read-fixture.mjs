// Test-only lifecycle/session pattern shared with the existing SHEIN fixture.
// The Go harness owns all domain seeding, PostgreSQL and zero-write assertions.
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { createWriteStream } from "node:fs";
import { mkdtemp, readFile, writeFile, access } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, resolve, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "node:net";
import assert from "node:assert/strict";
import { encode } from "@auth/core/jwt";

const sourceWeb = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repo = resolve(sourceWeb, "../..");
const webIndex = process.argv.indexOf("--web-dir");
const web = webIndex < 0 ? sourceWeb : resolve(process.argv[webIndex + 1]);
const serve = process.argv.includes("--serve");
const dir = await mkdtemp(join(tmpdir(), "issue347-browser-"));
// This budget includes compilation, PostgreSQL and Next prewarming. It ends
// before the Go fixture's 30-minute safety deadline, regardless of warmup time.
const expiresAt = Date.now() + 29 * 60000;
const children = [];
let go, next, port;
const sleep = ms => new Promise(resolveSleep => setTimeout(resolveSleep, ms));
const exited = child => child.exitCode !== null || child.signalCode !== null;
function run(command, args, options = {}) {
  return new Promise((resolveRun, reject) => {
    const child = spawn(command, args, { cwd: repo, windowsHide: true, ...options });
    let output = "";
    child.stdout?.on("data", data => { output += data; });
    child.stderr?.on("data", data => { output += data; });
    child.on("error", reject);
    child.on("exit", code => code === 0 ? resolveRun(output.trim()) : reject(new Error(`${command} exited ${code}; inspect ${dir}`)));
  });
}
function start(command, args, log, options) {
  const output = createWriteStream(join(dir, log), { mode: 0o600 });
  const child = spawn(command, args, { cwd: repo, windowsHide: true, stdio: ["ignore", "pipe", "pipe"], ...options });
  child.stdout.pipe(output); child.stderr.pipe(output);
  child.on("error", () => output.end("Fixture process failed to start"));
  child.on("exit", () => output.end());
  children.push(child);
  return child;
}
async function until(check, description, budget = 120000) {
  const end = Date.now() + budget;
  while (Date.now() < end) {
    try { const result = await check(); if (result) return result; } catch { /* starting */ }
    if (children.some(exited)) throw new Error(`Fixture exited during ${description}; inspect ${dir}`);
    await sleep(250);
  }
  throw new Error(`Timeout during ${description}; inspect ${dir}`);
}
async function freePort() {
  const server = createServer();
  await new Promise(resolveListen => server.listen(0, "127.0.0.1", resolveListen));
  const selected = server.address().port;
  await new Promise(resolveClose => server.close(resolveClose));
  return selected;
}
let closing;
function cleanup() {
  return closing ??= (async () => {
    const errors = [];
    try {
      if (next && !exited(next)) {
        if (process.platform === "win32") await run("taskkill", ["/PID", String(next.pid), "/T", "/F"]);
        else next.kill("SIGTERM");
        const end = Date.now() + 10000;
        while (!exited(next) && Date.now() < end) await sleep(100);
        assert.equal(exited(next), true, "Next child did not exit");
      }
      if (port) {
        const probe = createServer();
        await new Promise((resolveProbe, reject) => { probe.once("error", reject); probe.listen(port, "127.0.0.1", resolveProbe); });
        await new Promise(resolveClose => probe.close(resolveClose));
      }
    } catch (error) { errors.push(error); }
    try {
      if (go && !exited(go)) {
        await writeFile(join(dir, "stop"), "stop", { mode: 0o600 });
        const end = Date.now() + 30000;
        while (!exited(go) && Date.now() < end) await sleep(100);
      }
      if (go) assert.equal(go.exitCode, 0, `Go zero-write/cleanup did not pass; inspect ${dir}`);
    } catch (error) { errors.push(error); }
    await writeFile(join(dir, "cleanup.json"), JSON.stringify({ nextPid: next?.pid, port, goExit: go?.exitCode, passed: errors.length === 0 }), { mode: 0o600 });
    if (errors.length) throw new AggregateError(errors, `Fixture cleanup failed; inspect ${dir}`);
  })();
}
for (const signal of ["SIGINT", "SIGTERM"]) process.once(signal, () => { cleanup().then(() => process.exit(0), () => process.exit(1)); });

try {
  await access(join(web, "node_modules/next/dist/bin/next"));
  const binary = join(dir, process.platform === "win32" ? "fixture.test.exe" : "fixture.test");
  console.log(`Building isolated commercial fixture; evidence directory ${dir}`);
  await run("go", ["test", "-race", "-tags", "integration", "-c", "-o", binary, "./internal/app/httpapi"]);
  go = start(binary, ["-test.run=^TestCommercialHTTPPostgresBFFClientZeroWrites$", "-test.v", "-test.timeout=31m"], "go.log", {
    cwd: join(repo, "internal/app/httpapi"), env: { ...process.env, ISSUE347_BROWSER_FIXTURE_DIR: dir },
  });
  const seed = await until(async () => JSON.parse(await readFile(join(dir, "go.json"), "utf8")), "isolated PostgreSQL and real Go owner");
  port = await freePort();
  const origin = `http://127.0.0.1:${port}`, secret = randomBytes(48).toString("base64url");
  next = start(process.execPath, [join(web, "node_modules/next/dist/bin/next"), "dev", "--hostname", "127.0.0.1", "--port", String(port)], "next.log", {
    cwd: web, env: { ...process.env, NODE_ENV: "development", AUTH_SECRET: secret, AUTH_URL: origin, LISTINGKIT_PUBLIC_BASE_URL: origin,
      ZITADEL_ISSUER_URL: "http://127.0.0.1:1/fixture-external-identity", ZITADEL_CLIENT_ID: "fixture-client", ZITADEL_CLIENT_SECRET: "",
      COMMERCIAL_API_ORIGIN: seed.goOrigin, LISTINGKIT_SERVICE_API_BASE: `${seed.contextOrigin}/api/v1` },
  });
  const sessions = {};
  for (const [subject, accessToken] of Object.entries(seed.tokens)) {
    const role = subject === "viewer" ? "listingkit_viewer" : "listingkit_operator";
    const value = await encode({ secret, salt: "authjs.session-token", maxAge: 1800, token: { sub: subject, name: `Fixture ${subject}`, accessToken,
      expiresAt: Math.floor(Date.now() / 1000) + 1800, identityVersion: 3, identity: { tenantId: "home-A", userId: subject, roles: [role], userType: "zitadel" } } });
    sessions[subject] = [{ name: "authjs.session-token", value, url: origin, httpOnly: true, sameSite: "Lax" }, { name: "shuomi_effective_organization", value: "org-B", url: origin, httpOnly: true, sameSite: "Lax" }];
  }
  const manifest = { origin, goOrigin: seed.goOrigin, contextOrigin: seed.contextOrigin, sessions, controlDirectory: dir, webDirectory: web, expiresAt,
    sourceHead: await run("git", ["rev-parse", "HEAD"]), webHead: await run("git", ["rev-parse", "HEAD"], { cwd: web }),
    scenarioURL: `${seed.goOrigin}/fixture/scenario`, controlToken: seed.tokens.owner,
    scenarios: ["normal", "revoked", "role-downgraded", "provider-unavailable", "suspended", "signed-out", "database-unavailable", "slow"],
    organizations: ["org-B", "org-C", "org-custom", "org-empty", "org-expired", "org-disabled", "org-future", "org-viewer"],
    sessionBoundary: "synthetic external issuance/grants only; actual Auth.js/context resolver/Casbin/commercial owner/PostgreSQL/BFF" };
  await writeFile(join(dir, "fixture.json"), JSON.stringify(manifest, null, 2), { mode: 0o600 });
  await until(async () => (await fetch(`${origin}/api/auth/session`)).ok, "actual Next server");
  const cookie = (subject = "owner", org = "org-B") => sessions[subject].map(c => `${c.name}=${c.name === "shuomi_effective_organization" ? org : c.value}`).join("; ");
  const call = (path, { subject = "owner", org = "org-B", ...init } = {}) => fetch(`${origin}${path}`, { ...init, headers: { Cookie: cookie(subject, org), "X-Expected-Organization-ID": org, Origin: origin, "Content-Type": "application/json", ...init.headers } });
  const scenario = async value => assert.equal((await fetch(manifest.scenarioURL, { method: "POST", headers: { "Content-Type": "application/json", "X-Fixture-Control": manifest.controlToken }, body: JSON.stringify({ scenario: value }) })).status, 204);
  const endpoint = "/api/workbench/commercial/overview";
  const response = await call(endpoint); assert.equal(response.status, 200);
  assert.equal((await response.json()).organization_id, "org-B");
  const context = await call("/api/workbench/context"); assert.equal(context.status, 200);
  assert.equal((await context.json()).homeOrganizationId, "home-A");
  const switched = await call("/api/workbench/context/effective-organization", { method: "PUT", body: JSON.stringify({ organizationId: "org-C" }) });
  assert.equal(switched.status, 200); assert.match(switched.headers.get("set-cookie"), /org-C/);
  const other = await call(endpoint, { org: "org-C" }); assert.equal(other.status, 200); assert.equal((await other.json()).usage[0].committed, "2");
  assert.equal((await call(endpoint, { subject: "viewer" })).status, 403);
  assert.equal((await fetch(`${origin}${endpoint}`)).status, 401);
  for (const [value, status] of [["revoked", 403], ["role-downgraded", 403], ["suspended", 403], ["signed-out", 401], ["provider-unavailable", 503], ["database-unavailable", 503]]) {
    await scenario(value); assert.equal((await call(endpoint)).status, status); await scenario("normal");
    assert.equal((await call(endpoint)).status, 200);
  }
  const publicSession = await call("/api/auth/session"); assert.equal(JSON.stringify(await publicSession.json()).includes(seed.tokens.owner), false);
  await scenario("slow");
  await assert.rejects(call(endpoint, { signal: AbortSignal.timeout(100) }));
  await scenario("normal");
  assert.equal((await call(endpoint)).status, 200);
  await writeFile(join(dir, "evidence.json"), JSON.stringify({ passed: true, actual: ["Auth.js encrypted session", "Next server/BFF", "context list/switch", "Go resolver/Casbin/commercial owner", "PostgreSQL"], notRun: ["real IAM/payment/customer data", "UI visual/lifecycle acceptance"] }), { mode: 0o600 });
  console.log(JSON.stringify({ manifest: join(dir, "fixture.json"), origin, smoke: "PASS" }));
  if (serve) {
    console.log("Ready for browser. Create stop-fixture in controlDirectory; maximum 29 minutes. Session values are only in the private manifest.");
    while (Date.now() < expiresAt) {
      try { await access(join(dir, "stop-fixture")); break; } catch { /* serving */ }
      if (children.some(exited)) throw new Error(`Fixture exited; inspect ${dir}`);
      await sleep(500);
    }
  }
} finally { await cleanup(); }
