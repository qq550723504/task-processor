// Task-only launcher following the existing SHEIN fixture lifecycle/session pattern.
import { spawn } from "node:child_process";
import { randomBytes, randomUUID } from "node:crypto";
import { createWriteStream } from "node:fs";
import { mkdtemp, readFile, writeFile, access, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, resolve, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer } from "node:net";
import assert from "node:assert/strict";
import { encode } from "@auth/core/jwt";

const sourceWeb = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const sourceRepo = resolve(sourceWeb, "../..");
const goArg = process.argv.indexOf("--go-repo");
const repo = goArg < 0 ? sourceRepo : resolve(process.argv[goArg + 1]);
const webArg = process.argv.indexOf("--web-dir");
const web = webArg < 0 ? sourceWeb : resolve(process.argv[webArg + 1]);
const serve = process.argv.includes("--serve");
const dir = await mkdtemp(join(tmpdir(), "issue344-"));
let containerId, go, next, port;
const children = [];
function run(command, args, options = {}) {
  return new Promise((resolveRun, reject) => {
    const p = spawn(command, args, { cwd: repo, windowsHide: true, ...options });
    let stdout = "", stderr = "";
    p.stdout?.on("data", (s) => { stdout += s; }); p.stderr?.on("data", (s) => { stderr += s; });
    p.on("error", reject);
    p.on("exit", (code) => code === 0 ? resolveRun(stdout.trim()) : reject(new Error(`${command} exited ${code}: ${stderr.slice(-1500)}`)));
  });
}
function start(command, args, log, options = {}) {
  const output = createWriteStream(join(dir, log));
  const p = spawn(command, args, { cwd: repo, windowsHide: true, stdio: ["ignore", "pipe", "pipe"], ...options });
  p.stdout.pipe(output); p.stderr.pipe(output);
  p.on("error", (e) => output.write(String(e))); p.on("exit", () => output.end());
  children.push(p); return p;
}
async function until(check, description, budget = 120000) {
  const end = Date.now() + budget;
  while (Date.now() < end) {
    try { const result = await check(); if (result) return result; } catch { /* still starting */ }
    if (children.some((p) => p.exitCode !== null)) throw new Error(`Fixture exited during ${description}; logs ${dir}`);
    await new Promise((r) => setTimeout(r, 250));
  }
  throw new Error(`Timeout during ${description}; logs ${dir}`);
}
async function freePort() {
  const server = createServer(); await new Promise((r) => server.listen(0, "127.0.0.1", r));
  const port = server.address().port; await new Promise((r) => server.close(r)); return port;
}
let closing;
function cleanup() {
  return closing ??= (async () => {
    const errors = [];
    try {
      if (next && next.exitCode === null) {
        const exited = new Promise((r) => next.once("exit", r));
        // Next dev forks a server. Stop only our spawned PID tree on Windows.
        if (process.platform === "win32") await run("taskkill", ["/PID", String(next.pid), "/T", "/F"]);
        else next.kill("SIGTERM");
        await Promise.race([exited, new Promise((_, reject) => setTimeout(() => reject(new Error("Next did not exit")), 10000))]);
      }
      if (port) {
        const probe = createServer();
        await new Promise((resolveProbe, reject) => { probe.once("error", reject); probe.listen(port, "127.0.0.1", resolveProbe); });
        await new Promise((r) => probe.close(r));
      }
    } catch (error) { errors.push(error); }
    if (go && go.exitCode === null) {
      await writeFile(join(dir, "stop"), "stop");
      await Promise.race([new Promise((r) => go.once("exit", r)), new Promise((r) => setTimeout(r, 10000))]);
      if (go.exitCode === null) go.kill();
    }
    try { if (containerId) await run("docker", ["stop", containerId]); } catch (error) { errors.push(error); }
    if (go && go.exitCode !== 0) errors.push(new Error(`Go fixture assertions/cleanup did not pass; ${join(dir, "go.log")}`));
    if (errors.length) throw new AggregateError(errors, "Fixture cleanup failed");
    await writeFile(join(dir, "cleanup.json"), JSON.stringify({ nextPid: next?.pid, port, portReleased: true, goExit: go?.exitCode, containerStopped: !!containerId }));
  })();
}
process.once("SIGINT", () => { cleanup().then(() => process.exit(0), (e) => { console.error(e.message); process.exit(1); }); });
process.once("SIGTERM", () => { cleanup().then(() => process.exit(0), (e) => { console.error(e.message); process.exit(1); }); });

try {
  await access(join(web, "node_modules/next/dist/bin/next"));
  const binary = join(dir, process.platform === "win32" ? "fixture.test.exe" : "fixture.test");
  console.log(`Building Product Review test fixture; evidence directory ${dir}`);
  await run("go", ["test", "-c", "-o", binary, "./internal/app/httpapi"]);
  containerId = await run("docker", ["run", "--rm", "-d", "--name", `issue344-${randomUUID()}`, "-e", "POSTGRES_HOST_AUTH_METHOD=trust", "-e", "POSTGRES_DB=issue344_fixture", "-p", "127.0.0.1::5432", "postgres:17.2-alpine"]);
  const mapping = await run("docker", ["port", containerId, "5432/tcp"]); assert.match(mapping, /^127\.0\.0\.1:\d+$/);
  const pgPort = mapping.split(":")[1];
  await until(async () => { await run("docker", ["exec", containerId, "pg_isready", "-U", "postgres", "-d", "issue344_fixture"]); return true; }, "isolated PostgreSQL");
  const dsn = `host=127.0.0.1 port=${pgPort} user=postgres dbname=issue344_fixture sslmode=disable`;
  go = start(binary, ["-test.run=^TestProductTitleReviewBrowserFixture$", "-test.v", "-test.timeout=31m"], "go.log", {
    cwd: join(repo, "internal/app/httpapi"), env: { ...process.env, ISSUE344_FIXTURE_DIR: dir, ISSUE344_FIXTURE_DSN: dsn },
  });
  const seed = await until(async () => JSON.parse(await readFile(join(dir, "go.json"), "utf8")), "real Publisher/Proposer/POST seed");
  port = await freePort(); const origin = `http://127.0.0.1:${port}`; const secret = randomBytes(48).toString("base64url");
  next = start(process.execPath, [join(web, "node_modules/next/dist/bin/next"), "dev", "--hostname", "127.0.0.1", "--port", String(port)], "next.log", {
    cwd: web, env: { ...process.env, NODE_ENV: "development", AUTH_SECRET: secret, AUTH_URL: origin, LISTINGKIT_PUBLIC_BASE_URL: origin,
      ZITADEL_ISSUER_URL: "http://127.0.0.1:1/fixture-external-identity", ZITADEL_CLIENT_ID: "fixture-client", ZITADEL_CLIENT_SECRET: "",
      PRODUCT_REVIEW_API_ORIGIN: seed.goOrigin, SHEIN_RECORDS_API_ORIGIN: seed.goOrigin, LISTINGKIT_SERVICE_API_BASE: `${seed.contextOrigin}/api/v1` },
  });
  const sessions = {};
  for (const [name, accessToken] of Object.entries(seed.tokens)) {
    const role = name === "admin" || name === "revoked" ? "listingkit_admin" : name === "readonly" ? "admin" : "listingkit_operator";
    const value = await encode({ secret, salt: "authjs.session-token", maxAge: 1800, token: { sub: name, name: `Fixture ${name}`, accessToken,
      expiresAt: Math.floor(Date.now() / 1000) + 1800, identityVersion: 3, identity: { tenantId: "200", userId: name, roles: [role], userType: "zitadel" } } });
    sessions[name] = [{ name: "authjs.session-token", value, url: origin, httpOnly: true, sameSite: "Lax" }, { name: "shuomi_effective_organization", value: "200", url: origin, httpOnly: true, sameSite: "Lax" }];
  }
  const head = await run("git", ["rev-parse", "HEAD"], { cwd: sourceRepo });
  const goHead = await run("git", ["rev-parse", "HEAD"]);
  const manifest = { origin, goOrigin: seed.goOrigin, contextOrigin: seed.contextOrigin, proposals: seed.proposals, ownerCount: seed.ownerCount, recordId: seed.recordId,
    sessions, controlDirectory: dir, containerId, webDirectory: web, sourceHead: head, goHead, sessionBoundary: "synthetic external issuance/grants; actual Auth.js/Go middleware/Review/UoW/Catalog/PG" };
  await writeFile(join(dir, "fixture.json"), JSON.stringify(manifest, null, 2), { mode: 0o600 });
  await until(async () => (await fetch(`${origin}/api/auth/session`)).ok, "actual Next server");
  console.log(JSON.stringify({ manifest: join(dir, "fixture.json"), origin }));
  if (serve) {
    console.log("Ready for UI. Create stop-fixture in controlDirectory or SIGINT; maximum 29 minutes.");
    const expires = Date.now() + 29 * 60000;
    while (Date.now() < expires) {
      try { await access(join(dir, "stop-fixture")); break; } catch { /* continue */ }
      if (children.some((p) => p.exitCode !== null)) throw new Error(`Fixture exited; inspect ${dir}`);
      await new Promise((r) => setTimeout(r, 500));
    }
  } else {
    const report = [];
    const base = "/api/product/text-proposals";
    const cookie = (name, org) => sessions[name].map((c) => `${c.name}=${c.name === "shuomi_effective_organization" ? org : c.value}`).join("; ");
    const call = (path, { name = "owner", org = "200", method = "GET", body, key = randomUUID(), headers = {} } = {}) => fetch(`${origin}${path}`, { method, headers: {
      Cookie: cookie(name, org), "X-Expected-Organization-ID": org, Origin: origin, "Sec-Fetch-Site": "same-origin", Accept: "application/json",
      Authorization: "Bearer forged", "X-Requested-Organization-ID": "100", "X-User-Roles": "listingkit_admin",
      ...(body !== undefined ? { "Content-Type": "application/json", "Idempotency-Key": key } : {}), ...headers,
    }, ...(body !== undefined ? { body: typeof body === "string" ? body : JSON.stringify(body) } : {}) });
    async function check(name, response, status, code) {
      assert.equal(response.status, status, name); assert.match(response.headers.get("cache-control"), /no-store/, name);
      assert.equal(response.headers.get("x-content-type-options"), "nosniff", name);
      const body = await response.json(); if (code) assert.equal(body.code ?? body.error, code, name);
      report.push({ name, status, code: body.code ?? body.error ?? "review-v1" }); return body;
    }
    async function control(name) {
      await rm(join(dir, `${name}-done`), { force: true }); await writeFile(join(dir, name), "");
      await until(async () => { await access(join(dir, `${name}-done`)); return true; }, name, 15000);
    }
    async function observe() { await control("observe"); return JSON.parse(await readFile(join(dir, "observations.json"), "utf8")); }
    const decisions = (id, action, revision, extra = {}) => call(`${base}/${id}/decisions`, { method: "POST", name: "admin", body: { action, expected_revision: revision }, ...extra });
    const apply = (id, revision, extra = {}) => call(`${base}/${id}/apply`, { method: "POST", name: "admin", body: { expected_revision: revision }, ...extra });
    const first = await check("actual collection first page", await call(`${base}?view=actionable`), 200);
    const second = await check("actual collection second page", await call(`${base}?view=actionable&cursor=${encodeURIComponent(first.next_cursor)}`), 200);
    const items = [...first.items, ...second.items]; assert.equal(items.length, seed.ownerCount); assert.equal(new Set(items.map((i) => i.proposal_id)).size, seed.ownerCount);
    assert.equal(first.schema_version, 1); assert.equal(first.coverage, "product-title-proposals-only"); assert.equal(second.next_cursor, null);
    const id = items.find((i) => i.product_key === "product").proposal_id;
    const initial = await check("detail discovered from actual collection", await call(`${base}/${id}`), 200);
    assert.equal(initial.revision, "1"); assert.equal(initial.input.base_version, "1"); assert.ok(initial.evidence.length > 0);
    await check("unaccepted cannot apply", await apply(id, "1"), 409, "operation_conflict");
    await check("operator cannot accept own proposal", await decisions(id, "accept", "1", { name: "owner" }), 403, "permission_denied");
    const beforeAccept = await observe();
    const accepted = await check("admin accepts explicit proposal", await decisions(id, "accept", "1"), 200);
    assert.equal(accepted.revision, "2"); assert.equal(accepted.state, "accepted");
    const afterAccept = await observe(); assert.deepEqual(afterAccept.versions, beforeAccept.versions); assert.equal(afterAccept.postCount, beforeAccept.postCount + 1);
    const applyKey = randomUUID();
    const applied = await check("separate explicit Apply", await apply(id, "2", { key: applyKey }), 200);
    assert.equal(applied.state, "applied"); assert.equal(applied.apply_receipt.product_version, "2");
    const beforeReplay = await observe(); assert.equal(beforeReplay.titles["200/product"], applied.after);
    const replay = await check("same key exact receipt replay", await apply(id, "2", { key: applyKey }), 200); assert.deepEqual(replay, applied);
    const afterReplay = await observe(); assert.deepEqual(afterReplay.versions, beforeReplay.versions); assert.deepEqual(afterReplay.titles, beforeReplay.titles);
    await check("same key changed revision conflicts", await apply(id, "1", { key: applyKey }), 409, "operation_conflict");
    const refreshed = await check("applied leaves actionable collection", await call(`${base}?view=actionable&limit=100`), 200); assert.ok(!refreshed.items.some((i) => i.proposal_id === id));
    const rejectedId = refreshed.items.find((i) => i.product_key === "product").proposal_id;
    await check("reject separate proposal", await decisions(rejectedId, "reject", "1"), 200);
    await check("rejected cannot Apply", await apply(rejectedId, "2"), 409, "operation_conflict");
    const staleId = refreshed.items.find((i) => i.product_key === "product" && i.proposal_id !== rejectedId).proposal_id;
    await check("accept old Product base", await decisions(staleId, "accept", "1"), 200);
    await check("stale Product cannot Apply", await apply(staleId, "2"), 409, "stale_product_version");
    const editId = seed.proposals["edit-product"];
    await check("accept before edit", await decisions(editId, "accept", "1"), 200);
    const edited = await check("owner edit clears acceptance", await decisions(editId, "edit", "2", { name: "owner", body: { action: "edit", expected_revision: "2", title: "Human reviewed title" } }), 200);
    assert.equal(edited.state, "pending"); assert.equal(edited.revision, "3");
    await check("edited proposal requires fresh acceptance", await apply(editId, "3"), 409, "operation_conflict");
    await check("accept edited revision", await decisions(editId, "accept", "3"), 200);
    await check("apply edited accepted revision", await apply(editId, "4"), 200);
    assert.equal((await observe()).titles["200/edit-product"], "Human reviewed title");
    const lostId = seed.proposals["lost-product"]; await check("accept lost-response target", await decisions(lostId, "accept", "1"), 200);
    const beforeLoss = await observe(); await control("lose-response"); const lostKey = randomUUID();
    const lost = await check("real Go commit then TCP response loss", await apply(lostId, "2", { key: lostKey }), 502, "RESULT_UNVERIFIED"); assert.equal(lost.outcome, "unknown");
    const afterLoss = await observe(); assert.equal(afterLoss.postCount, beforeLoss.postCount + 1); assert.equal(afterLoss.versions["200/lost-product"], 2);
    await control("restart");
    const confirmed = await check("explicit same-key confirmation after reconstruction", await apply(lostId, "2", { key: lostKey }), 200); assert.equal(confirmed.apply_receipt.product_version, "2");
    const afterConfirmation = await observe(); assert.deepEqual(afterConfirmation.versions, afterLoss.versions); assert.deepEqual(afterConfirmation.titles, afterLoss.titles); assert.equal(afterConfirmation.titles["200/lost-product"], confirmed.after);
    const reread = await check("reconstructed durable receipt read", await call(`${base}/${lostId}`, { name: "admin" }), 200); assert.deepEqual(reread.apply_receipt, confirmed.apply_receipt);
    const revokeId = seed.proposals["revoked-product"]; await check("accept before grant revocation", await decisions(revokeId, "accept", "1"), 200);
    await control("revoke");
    await check("cached read does not grant future LiveWrite", await call(`${base}/${revokeId}`, { name: "admin" }), 200);
    const denial = await apply(revokeId, "2"); assert.match(denial.headers.get("set-cookie"), /shuomi_effective_organization=;/);
    await check("LiveWrite revoked admin cannot Apply", denial, 403, "ORGANIZATION_ACCESS_REVOKED"); await control("restore");
    await check("nonowner detail hidden", await call(`${base}/${id}`, { name: "other" }), 404, "not_found");
    await check("cross-org detail hidden", await call(`${base}/${id}`, { org: "300", name: "admin" }), 404, "not_found");
    await check("org mismatch rejected before forwarding", await call(`${base}/${id}`, { headers: { "X-Expected-Organization-ID": "300" } }), 409, "ORGANIZATION_CONTEXT_CHANGED");
    await check("missing session", await call(`${base}/${id}`, { headers: { Cookie: "shuomi_effective_organization=200" } }), 401, "AUTHENTICATION_REQUIRED");
    const other = await check("other owner scoped list", await call(`${base}?view=actionable`, { name: "other" }), 200); assert.equal(other.items.length, 1);
    const org300 = await check("second actual organization collection", await call(`${base}?view=actionable`, { org: "300" }), 200); assert.equal(org300.items.length, 1);
    await check("Store read does not imply Product read", await call(`${base}?view=actionable`, { name: "store" }), 403, "PERMISSION_DENIED");
    await check("grant dependency does not become empty collection", await call(`${base}?view=actionable`, { name: "unavailable" }), 503, "DEPENDENCY_UNAVAILABLE");
    await check("Go request deadline", await call(`${base}?view=actionable`, { name: "slow" }), 504, "DEADLINE_EXCEEDED");
    const beforeNegative = await observe();
    for (const action of ["decisions", "apply"]) {
      await check(`${action} cross-site blocked`, await call(`${base}/${revokeId}/${action}`, { method: "POST", body: { expected_revision: "2" }, headers: { Origin: "https://evil.invalid", "Sec-Fetch-Site": "cross-site", "X-Forwarded-Host": "evil.invalid" } }), 403, "PERMISSION_DENIED");
      await check(`${action} duplicate JSON blocked`, await call(`${base}/${revokeId}/${action}`, { method: "POST", body: '{"expected_revision":"2","expected_revision":"2"}' }), 400, "INVALID_REQUEST");
    }
    await check("generation BFF remains unavailable", await call(base, { method: "POST", body: { product_key: "product", base_version: "1" } }), 405, "INVALID_REQUEST");
    await check("oversized actual request body", await call(`${base}/${revokeId}/apply`, { method: "POST", body: " ".repeat(32769) }), 413, "INPUT_TOO_LARGE");
    const afterNegative = await observe(); assert.equal(afterNegative.postCount, beforeNegative.postCount);
    await check("existing completed-work projection", await call("/api/workbench/completed-work?source=listing-local-preparation"), 200);
    await check("existing Listing diagnostic", await call(`/api/listing/shein-records/${seed.recordId}/offline-diagnostic?action=publish`), 200);
    const observations = await observe(); assert.equal(observations.nonTitleUnchanged, true); assert.equal(observations.listingUnchanged, true);
    await writeFile(join(dir, "evidence.json"), JSON.stringify({ sourceHead: head, goHead, assertions: report.length, report, observations, externalSubstitutes: ["session/token issuance", "grant provider", "CandidateGenerator"], actual: ["Next Auth.js/BFF", "Go middleware/resolver/Review/UoW", "Sourcing/Catalog Publisher", "PostgreSQL"], notRun: ["real IAM", "production", "paid model"] }, null, 2));
    console.log(`PASS ${report.length} actual BFF -> Go -> isolated PostgreSQL assertions; ${join(dir, "evidence.json")}`);
  }
} finally {
  await cleanup(); console.log(`Owned processes/database cleaned; evidence retained at ${dir}`);
}
