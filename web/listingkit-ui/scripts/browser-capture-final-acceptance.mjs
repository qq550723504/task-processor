import assert from "node:assert/strict";
import { spawn } from "node:child_process";
import { randomBytes, randomUUID } from "node:crypto";
import { mkdtemp, readFile, writeFile, unlink, lstat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, join, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { encode } from "@auth/core/jwt";
import { childEnvironment } from "../../../scripts/issue357/contract.mjs";
import { run, port, until, privateDirectory, processIdentity, sameProcess, json } from "../../../scripts/issue357/io.mjs";

// Reuse the existing task-only Auth.js session boundary; no provider bootstrap,
// real credentials, production switch, shared database or IAM management.
const web = resolve(dirname(fileURLToPath(import.meta.url)), "..");
const repo = resolve(web, "../..");
const task = "01a09116-9490-7c42-a117-b818192a10a8";
const runId = randomUUID();
const dir = await mkdtemp(join(tmpdir(), "issue399-browser-"));
await privateDirectory(dir);
const expectedSha = process.argv[2];
const children = [];
let container, containerName, pgPort, webPort, stage = "source", passed = false;
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
const privateJSON = async file => JSON.parse(await readFile(join(dir, file), "utf8"));

async function start(binary, args, env, cwd) {
  const child = spawn(binary, args, { cwd, env: { ...childEnvironment(), ...env }, windowsHide: true, stdio: "ignore" });
  await new Promise((resolve, reject) => { child.once("spawn", resolve); child.once("error", reject); });
  const record = await processIdentity(child.pid);
  children.push({ child, record });
  await json(join(dir, "processes.json"), children.map(({ record }) => record));
  return child;
}
async function stopChildren() {
  await writeFile(join(dir, "stop-next"), "stop");
  await writeFile(join(dir, "stop-go"), "stop");
  for (const { child, record } of [...children].reverse()) {
    const end = Date.now() + 20_000;
    while (child.exitCode === null && child.signalCode === null && Date.now() < end) await pause(100);
    if (await sameProcess(record)) await run("taskkill.exe", ["/PID", String(record.pid), "/T", "/F"]);
    assert.equal(await sameProcess(record), false, "OWNED_PROCESS_NOT_RELEASED");
  }
}
async function ownedPG() {
  const info = JSON.parse(await run("docker", ["inspect", container]))[0];
  assert.equal(info.Id, container); assert.equal(info.Name, `/${containerName}`);
  assert.equal(info.Config.Labels["com.shuomi.task"], task);
  assert.equal(info.Config.Labels["com.shuomi.issue399.run"], runId);
  return info;
}

console.log(`Browser isolated acceptance run=${runId} evidence=${dir}`);
try {
  assert.match(expectedSha ?? "", /^[a-f0-9]{40}$/);
  assert.equal(await run("git", ["rev-parse", "HEAD"], { cwd: repo }), expectedSha);
  assert.equal(await run("git", ["status", "--porcelain"], { cwd: repo }), "", "DIRTY_SOURCE");
  for (const parent of [repo, web]) for (const name of [".env", ".env.local", ".env.development", ".env.development.local"]) {
    await assert.rejects(lstat(join(parent, name)), error => error.code === "ENOENT");
  }
  stage = "compile";
  const binary = join(dir, "browser-capture.test.exe");
  await run("go", ["test", "-c", "-o", binary, "./internal/app/httpapi"], { cwd: repo });
  stage = "owned-postgres";
  const password = randomBytes(32).toString("hex");
  containerName = `issue399-browser-${runId}`;
  assert.equal(await run("docker", ["ps", "-aq", "--filter", `name=^/${containerName}$`]), "");
  container = await run("docker", ["run", "-d", "--name", containerName, "--label", `com.shuomi.task=${task}`, "--label", `com.shuomi.issue399.run=${runId}`, "-e", `POSTGRES_PASSWORD=${password}`, "-e", "POSTGRES_USER=issue398_owner", "-e", "POSTGRES_DB=postgres", "-p", "127.0.0.1::5432", "postgres:16-alpine"]);
  await ownedPG();
  const mapping = await run("docker", ["port", container, "5432/tcp"]); assert.match(mapping, /^127\.0\.0\.1:\d+$/); pgPort = Number(mapping.split(":")[1]);
  await until(async () => { await run("docker", ["exec", container, "pg_isready", "-U", "issue398_owner", "-d", "postgres"]); return true; }, "OWNED_PG", 30_000);
  stage = "go-fixture";
  const go = await start(binary, ["-test.run=^TestBrowserCaptureWebFixture$", "-test.timeout=13m"], { ISSUE399_FIXTURE_DIR: dir, ISSUE398_TEST_DSN: `host=127.0.0.1 port=${pgPort} user=issue398_owner password=${password} dbname=postgres sslmode=disable` }, join(repo, "internal/app/httpapi"));
  const goInfo = await until(() => privateJSON("go.json"), "GO_BROWSER", 45_000);
  assert.equal(new URL(goInfo.goOrigin).hostname, "127.0.0.1");
  stage = "next-auth";
  webPort = await port(); const origin = `http://localhost:${webPort}`;
  const secret = randomBytes(48).toString("base64url");
  await json(join(dir, "services.json"), { uiDirectory: web, webPort });
  const next = await start(process.execPath, [join(repo, "scripts/issue357/next.mjs"), dir], { NODE_ENV: "development", NEXT_TELEMETRY_DISABLED: "1", AUTH_SECRET: secret, AUTH_URL: origin, AUTH_TRUST_HOST: "true", LISTINGKIT_PUBLIC_BASE_URL: origin, ZITADEL_ISSUER_URL: "http://127.0.0.1:1/fixture-external-identity", ZITADEL_CLIENT_ID: "fixture-client", LISTINGKIT_SERVICE_API_BASE: `${goInfo.goOrigin}/api/v1` }, web);
  await until(async () => { assert.equal(next.exitCode, null); assert.equal(go.exitCode, null); return (await fetch(`${origin}/api/auth/session`, { signal: AbortSignal.timeout(5000) })).ok; }, "NEXT_AUTH", 90_000);
  const sessions = {};
  for (const actor of ["operator", "operator2", "viewer"]) {
    const value = await encode({ secret, salt: "authjs.session-token", maxAge: 1800, token: { sub: actor, name: `Fixture ${actor}`, accessToken: actor, expiresAt: Math.floor(Date.now()/1000)+1800, identityVersion: 3, identity: { tenantId: "A", userId: actor, roles: [actor === "viewer" ? "listingkit_viewer" : "listingkit_operator"], userType: "zitadel" } } });
    sessions[actor] = { subject: actor, cookie: `authjs.session-token=${value}` };
  }
  const manifest = join(dir, "fixture.json");
  await json(manifest, { origin, ...goInfo, sourceHead: expectedSha, evidencePath: join(dir, "evidence.json"), sessions });
  stage = "actual-chain";
  const command = process.platform === "win32" ? ["cmd.exe", ["/c", "pnpm.cmd", "exec", "vitest", "run", "--config", "e2e/issue399-browser.config.ts"]] : ["pnpm", ["exec", "vitest", "run", "--config", "e2e/issue399-browser.config.ts"]];
  await run(command[0], command[1], { cwd: web, env: { ...childEnvironment(), BROWSER_CAPTURE_FIXTURE_MANIFEST: manifest } });
  const evidence = await privateJSON("evidence.json"); assert.equal(evidence.passed, true); assert.equal(evidence.sourceHead, expectedSha);
  passed = true;
} catch {
  // Never expose child diagnostics, cookies, keys or captured payloads.
  console.error(`BROWSER_ACCEPTANCE_FAILED stage=${stage} run=${runId}`);
  process.exitCode = 1;
} finally {
  let released = false;
  try {
    await stopChildren();
    if (container) { await ownedPG(); await run("docker", ["stop", "--time", "10", container]); await ownedPG(); await run("docker", ["rm", "-v", container]); }
    if (webPort) assert.equal(await port(webPort), webPort);
    if (pgPort) assert.equal(await port(pgPort), pgPort);
    released = true;
  } catch { console.error(`BROWSER_CLEANUP_INCOMPLETE run=${runId}`); process.exitCode = 1; }
  for (const name of ["fixture.json", "go.json", "services.json"]) await unlink(join(dir, name)).catch(error => { if (error.code !== "ENOENT") throw error; });
  await json(join(dir, "cleanup.json"), { runId, sourceHead: expectedSha, resourcesReleased: released, secretsRemoved: true });
  console.log(`Browser cleanup released=${released} run=${runId}`);
}
if (passed && !process.exitCode) console.log(`PASS actual Browser client/Next/Auth.js/Go/task-PG; controlled external identity; source=${expectedSha}`);
