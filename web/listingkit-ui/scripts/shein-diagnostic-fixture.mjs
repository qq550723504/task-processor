// Standalone, loopback-only acceptance fixture. Never imported by application code.
import { spawn } from "node:child_process";
import { randomBytes, randomUUID } from "node:crypto";
import { createWriteStream } from "node:fs";
import { mkdtemp, readFile, writeFile, access } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, resolve, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "node:net";
import { request as httpRequest } from "node:http";
import assert from "node:assert/strict";
import { encode } from "@auth/core/jwt";

const sourceWeb = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repo = resolve(sourceWeb, "../..");
const webArg = process.argv.indexOf("--web-dir");
const web = webArg < 0 ? sourceWeb : resolve(process.argv[webArg + 1]);
const serve = process.argv.includes("--serve");
const dir = await mkdtemp(join(tmpdir(), "issue323-"));
const container = `issue323-${randomUUID()}`;
let containerId, go, next;
const children = [];
function run(command, args, options = {}) {
  return new Promise((resolveRun, reject) => {
    const p = spawn(command, args, { cwd: repo, windowsHide: true, ...options });
    let stdout = "", stderr = "";
    p.stdout?.on("data", (s) => { stdout += s; });
    p.stderr?.on("data", (s) => { stderr += s; });
    p.on("error", reject);
    p.on("exit", (code) => code === 0 ? resolveRun(stdout.trim()) : reject(new Error(`${command} exited ${code}: ${stderr.slice(-2000)}`)));
  });
}
function start(command, args, log, options = {}) {
  const output = createWriteStream(join(dir, log));
  const p = spawn(command, args, { cwd: repo, windowsHide: true, stdio: ["ignore", "pipe", "pipe"], ...options });
  p.stdout.pipe(output); p.stderr.pipe(output);
  p.on("error", (error) => output.write(String(error)));
  p.on("exit", () => output.end());
  children.push(p);
  return p;
}
async function until(check, description, budget = 120000) {
  const end = Date.now() + budget;
  while (Date.now() < end) {
    try { const result = await check(); if (result) return result; } catch { /* still starting */ }
    if (children.some((p) => p.exitCode !== null)) throw new Error(`Fixture process exited while waiting for ${description}; logs: ${dir}`);
    await new Promise((r) => setTimeout(r, 250));
  }
  throw new Error(`Timeout waiting for ${description}; logs: ${dir}`);
}
async function freePort() {
  const server = createServer();
  await new Promise((r) => server.listen(0, "127.0.0.1", r));
  const port = server.address().port;
  await new Promise((r) => server.close(r));
  return port;
}
let closing;
function cleanup() {
  return closing ??= (async () => {
    next?.kill();
    if (go && go.exitCode === null) {
      await writeFile(join(dir, "stop"), "stop");
      await Promise.race([new Promise((r) => go.once("exit", r)), new Promise((r) => setTimeout(r, 10000))]);
      if (go.exitCode === null) go.kill();
    }
    if (containerId) await run("docker", ["stop", containerId]);
    if (go && go.exitCode !== 0) throw new Error(`Go fixture did not pass read-only/cleanup assertions; inspect ${join(dir, "go.log")}`);
  })();
}
process.once("SIGINT", () => { cleanup().then(() => process.exit(0), (e) => { console.error(e.message); process.exit(1); }); });
process.once("SIGTERM", () => { cleanup().then(() => process.exit(0), (e) => { console.error(e.message); process.exit(1); }); });

try {
  await access(join(web, "node_modules/next/dist/bin/next"));
  const binary = join(dir, process.platform === "win32" ? "fixture.test.exe" : "fixture.test");
  console.log("Building test-only Go fixture...");
  await run("go", ["test", "-c", "-o", binary, "./internal/app/httpapi"]);
  containerId = await run("docker", ["run", "--rm", "-d", "--name", container, "-e", "POSTGRES_HOST_AUTH_METHOD=trust", "-e", "POSTGRES_DB=issue323_fixture", "-p", "127.0.0.1::5432", "postgres:17.2-alpine"]);
  const mapping = await run("docker", ["port", containerId, "5432/tcp"]);
  assert.match(mapping, /^127\.0\.0\.1:\d+$/);
  const pgPort = mapping.split(":")[1];
  await until(async () => { await run("docker", ["exec", containerId, "pg_isready", "-U", "postgres", "-d", "issue323_fixture"]); return true; }, "isolated PostgreSQL");
  go = start(binary, ["-test.run=^TestSheinDiagnosticBrowserFixture$", "-test.v", "-test.timeout=31m"], "go.log", {
    cwd: join(repo, "internal/app/httpapi"),
    env: { ...process.env, ISSUE323_FIXTURE_DIR: dir, ISSUE323_FIXTURE_DSN: `host=127.0.0.1 port=${pgPort} user=postgres dbname=issue323_fixture sslmode=disable` },
  });
  const seed = await until(async () => JSON.parse(await readFile(join(dir, "go.json"), "utf8")), "Catalog Publisher -> POST seed");
  const port = await freePort();
  const origin = `http://127.0.0.1:${port}`;
  const secret = randomBytes(48).toString("base64url");
  next = start(process.execPath, [join(web, "node_modules/next/dist/bin/next"), "dev", "--hostname", "127.0.0.1", "--port", String(port)], "next.log", {
    cwd: web,
    env: { ...process.env, NODE_ENV: "development", AUTH_SECRET: secret, AUTH_URL: origin, ZITADEL_ISSUER_URL: "http://127.0.0.1:1/fixture-external-identity", ZITADEL_CLIENT_ID: "fixture-client", ZITADEL_CLIENT_SECRET: "", SHEIN_RECORDS_API_ORIGIN: seed.goOrigin, LISTINGKIT_SERVICE_API_BASE: `${seed.contextOrigin}/api/v1` },
  });
  const sessions = {};
  for (const [name, accessToken] of Object.entries(seed.tokens)) {
    const role = { owner: "listingkit_operator", other: "listingkit_operator", admin: "listingkit_admin", readonly: "admin", store: "store_viewer", revoked: "listingkit_admin", slow: "listingkit_operator", unavailable: "listingkit_operator" }[name];
    // Controlled external session issuance; actual Auth.js decryption, callbacks,
    // server token helper and page/API route handlers all run unchanged.
    const value = await encode({ secret, salt: "authjs.session-token", maxAge: 1800, token: { sub: name, name: `Fixture ${name}`, accessToken, expiresAt: Math.floor(Date.now() / 1000) + 1800, identityVersion: 3, identity: { tenantId: "200", userId: name, roles: [role], userType: "zitadel" } } });
    sessions[name] = [{ name: "authjs.session-token", value, url: origin, httpOnly: true, sameSite: "Lax" }, { name: "shuomi_effective_organization", value: "200", url: origin, httpOnly: true, sameSite: "Lax" }];
  }
  const manifest = { origin, goOrigin: seed.goOrigin, contextOrigin: seed.contextOrigin, recordId: seed.recordId, recordCount: seed.recordCount, otherRecordCount: seed.otherRecordCount, readonlyRecordId: seed.readonlyRecordId, sessions, controlDirectory: dir, containerId, sessionBoundary: "synthetic Auth.js issuance and external Go verifier/grants only; no real ZITADEL login" };
  await writeFile(join(dir, "fixture.json"), JSON.stringify(manifest, null, 2), { mode: 0o600 });
  await until(async () => (await fetch(`${origin}/api/auth/session`)).ok, "actual Next server");
  console.log(JSON.stringify({ manifest: join(dir, "fixture.json"), origin, recordId: seed.recordId }));
  if (serve) {
    console.log("Ready. Create stop-fixture in the control directory or send SIGINT for verified cleanup. Maximum lifetime: 30 minutes.");
    const expires = Date.now() + 29 * 60 * 1000;
    while (Date.now() < expires) {
      try { await access(join(dir, "stop-fixture")); break; } catch { /* continue serving */ }
      if (go.exitCode !== null || next.exitCode !== null) throw new Error(`Fixture process exited; inspect ${dir}`);
      await new Promise((r) => setTimeout(r, 500));
    }
  } else {
    const cookie = (name, org = "200") => sessions[name].map((c) => `${c.name}=${c.name === "shuomi_effective_organization" ? org : c.value}`).join("; ");
    const path = `/api/listing/shein-records/${seed.recordId}/offline-diagnostic`;
    const get = (name, query = "action=publish", org = "200", expected = org) => fetch(`${origin}${path}?${query}`, { headers: { cookie: cookie(name, org), "X-Expected-Organization-ID": expected, Authorization: "Bearer forged", "X-User-ID": "admin", "X-Requested-Organization-ID": "100" } });
    const list = (name, query = "limit=20", org = "200", expected = org) => fetch(`${origin}/api/listing/shein-records?${query}`, { headers: { cookie: cookie(name, org), "X-Expected-Organization-ID": expected, Authorization: "Bearer forged", "X-User-ID": "admin", "X-Requested-Organization-ID": "100" } });
    const report = [];
    async function check(name, response, status, code) {
      assert.equal(response.status, status, name);
      assert.match(response.headers.get("cache-control"), /no-store/, name);
      const body = await response.json();
      if (code) assert.equal(body.error ?? body.code, code, name);
      report.push({ name, status, code: body.error ?? body.code ?? "diagnostic" });
      return body;
    }
    const ownerFirst = await check("owner list page 1 actual BFF -> Go -> PG", await list("owner"), 200);
    assert.equal(ownerFirst.items.length, 20);
    assert.equal(typeof ownerFirst.next_cursor, "string");
    assert.equal(ownerFirst.items[0].snapshot_version, "1");
    const ownerSecond = await check("owner list page 2", await list("owner", `limit=20&cursor=${encodeURIComponent(ownerFirst.next_cursor)}`), 200);
    assert.equal(ownerSecond.items.length, seed.recordCount - 20);
    assert.equal(ownerSecond.next_cursor, null);
    assert.equal(new Set([...ownerFirst.items, ...ownerSecond.items].map((item) => item.record_id)).size, seed.recordCount);
    const adminList = await check("organization admin list", await list("admin", "limit=100"), 200);
    assert.equal(adminList.items.length, seed.recordCount + seed.otherRecordCount + 1);
    const otherList = await check("operator owner scope", await list("other", "limit=100"), 200);
    assert.equal(otherList.items.length, seed.otherRecordCount);
    const readonlyList = await check("read permission without write lists", await list("readonly", "limit=100"), 200);
    assert.equal(readonlyList.items.length, adminList.items.length);
    const emptyOrganization = await check("authorized organization with no records", await list("owner", "limit=20", "100"), 200);
    assert.deepEqual(emptyOrganization, { items: [], next_cursor: null });
    await check("cursor cannot cross organization", await list("owner", `limit=20&cursor=${encodeURIComponent(ownerFirst.next_cursor)}`, "100"), 400, "invalid_request");
    await check("store read cannot list", await list("store"), 403, "PERMISSION_DENIED");
    await check("revoked grant cannot list", await list("revoked"), 403, "ORGANIZATION_ACCESS_REVOKED");
    await check("invalid list limit", await list("owner", "limit=020"), 400, "invalid_request");
    await check("duplicate list query", await list("owner", "limit=20&limit=20"), 400, "invalid_request");
    await check("list organization assertion mismatch", await list("owner", "limit=20", "200", "100"), 409, "ORGANIZATION_CONTEXT_CHANGED");
    await check("list dependency outage", await list("unavailable"), 503, "DEPENDENCY_UNAVAILABLE");
    const discoveredPath = `/api/listing/shein-records/${ownerFirst.items[0].record_id}/offline-diagnostic`;
    const first = await check("list-discovered record diagnostic", await fetch(`${origin}${discoveredPath}?action=publish`, { headers: { cookie: cookie("owner"), "X-Expected-Organization-ID": "200" } }), 200);
    assert.equal(first.diagnostic_only, true);
    assert.equal(first.external_freshness.status, "not_evaluated");
    assert.ok(first.not_evaluated.includes("submission_gate"));
    assert.ok(Array.isArray(first.offline_checks.blockers));
    assert.equal(first.action, "publish");
    await check("save_draft", await get("owner", "action=save_draft"), 200);
    await check("matching digest", await get("owner", `action=publish&expected_digest=${encodeURIComponent(first.input.actual_digest)}`), 200);
    await check("expected mismatch", await get("owner", `action=publish&expected_digest=sha256:${"0".repeat(64)}`), 409, "stale_input");
    await check("non-owner", await get("other"), 404, "not_found");
    await check("cross organization", await get("owner", "action=publish", "100"), 404, "not_found");
    await check("admin", await get("admin"), 200);
    await check("read permission without write", await fetch(`${origin}/api/listing/shein-records/${seed.readonlyRecordId}/offline-diagnostic?action=publish`, { headers: { cookie: cookie("readonly"), "X-Expected-Organization-ID": "200" } }), 200);
    await check("store read is insufficient", await get("store"), 403, "PERMISSION_DENIED");
    await check("revoked grant", await get("revoked"), 403, "ORGANIZATION_ACCESS_REVOKED");
    await check("organization assertion mismatch", await get("owner", "action=publish", "200", "100"), 409, "ORGANIZATION_CONTEXT_CHANGED");
    await check("missing action", await get("owner", ""), 400, "unsupported_action");
    await check("duplicate action", await get("owner", "action=publish&action=publish"), 400, "invalid_request");
    await check("external authorization outage", await get("unavailable"), 503, "DEPENDENCY_UNAVAILABLE");
    await check("real Go deadline survives BFF", await get("slow"), 504, "DEADLINE_EXCEEDED");
    await check("missing session ignores bearer", await fetch(`${origin}${path}?action=publish`, { headers: { Authorization: "Bearer forged" } }), 401, "AUTHENTICATION_REQUIRED");
    for (const method of ["POST", "PUT", "PATCH", "DELETE", "OPTIONS"]) await check(`reject ${method}`, await fetch(`${origin}${path}`, { method }), 405, "INVALID_REQUEST");
    const head = await fetch(`${origin}${path}`, { method: "HEAD" });
    assert.equal(head.status, 405); assert.match(head.headers.get("cache-control"), /no-store/);
    const bodyResponse = await new Promise((resolveResponse, reject) => {
      const r = httpRequest(`${origin}${path}?action=publish`, { method: "GET", headers: { cookie: cookie("owner"), "X-Expected-Organization-ID": "200", "Content-Length": "1" } }, (response) => {
        const chunks = []; response.on("data", (chunk) => chunks.push(chunk));
        response.on("end", () => resolveResponse(new Response(Buffer.concat(chunks), { status: response.statusCode, headers: response.headers })));
      });
      r.on("error", reject); r.end("x");
    });
    await check("actual GET body rejected", bodyResponse, 400, "invalid_request");
    const context = await fetch(`${origin}/api/workbench/context`, { headers: { cookie: cookie("owner") } });
    assert.equal(context.status, 200); assert.equal((await context.json()).organizations.length, 2);
    const switched = await fetch(`${origin}/api/workbench/context/effective-organization`, { method: "PUT", headers: { cookie: cookie("owner"), "Content-Type": "application/json" }, body: JSON.stringify({ organizationId: "100" }) });
    assert.equal(switched.status, 200, "real OrganizationSwitcher requires its audit dependency");
    assert.equal((await switched.json()).effectiveOrganizationId, "100");
    assert.match(switched.headers.get("set-cookie"), /shuomi_effective_organization=100/);
    report.push({ name: "actual context switch and selection cookie", status: 200 });
    const publicSession = await fetch(`${origin}/api/auth/session`, { headers: { cookie: cookie("owner") } });
    assert.ok(!(await publicSession.text()).includes(seed.tokens.owner));
    await writeFile(join(dir, "restart"), "restart");
    await until(async () => { await access(join(dir, "restarted")); return true; }, "repository/application rebuild");
    const again = await check("after app/repository rebuild", await get("owner"), 200);
    assert.equal(again.input.actual_digest, first.input.actual_digest);
    const listAgain = await check("collection after app/repository rebuild", await list("owner"), 200);
    assert.equal(listAgain.items.length, 20);
    console.log("Running related Go regressions against the isolated PostgreSQL...");
    const regression = await run("go", ["test", "./internal/app/httpapi", "./internal/listing/record", "./internal/marketplace/shein/validator", "./internal/marketplace/validator", "-count=1"], { env: { ...process.env, ISSUE319_TEST_DSN: `host=127.0.0.1 port=${pgPort} user=postgres dbname=issue323_fixture sslmode=disable` } });
    await writeFile(join(dir, "go-regression.log"), regression);
    await writeFile(join(dir, "evidence.json"), JSON.stringify(report, null, 2));
    console.log(`PASS: ${report.length} cross-process assertions; evidence ${join(dir, "evidence.json")}`);
  }
} finally {
  await cleanup();
  console.log(`Fixture stopped; own container removed. Logs retained at ${dir}`);
}
