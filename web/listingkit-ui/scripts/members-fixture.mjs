// Task-only browser harness. Real Next/Auth.js/BFF/current Go application and
// three isolated PostgreSQL roles; external ZITADEL HTTP is explicitly synthetic.
import { spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { createWriteStream } from "node:fs";
import { access, mkdtemp, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, resolve, join } from "node:path";
import { fileURLToPath } from "node:url";
import { createServer, request as proxyRequest } from "node:http";
import { connect } from "node:net";
import { encode } from "@auth/core/jwt";

const web = resolve(dirname(fileURLToPath(import.meta.url)), ".."); const repo = resolve(web, "../..");
const directory = await mkdtemp(join(tmpdir(), "issue410-browser-")); const children = []; let go, next, outer, closing;
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
function run(command, args, options = {}) { return new Promise((resolveRun, reject) => { const child = spawn(command, args, { cwd: repo, windowsHide: true, ...options }); let output = ""; child.stdout?.on("data", data => output += data); child.stderr?.on("data", data => output += data); child.on("error", reject); child.on("exit", code => code === 0 ? resolveRun(output.trim()) : reject(new Error(`${command} failed: ${output.slice(-1800)}`))); }); }
function start(command, args, logfile, options = {}) { const out = createWriteStream(join(directory, logfile)); const child = spawn(command, args, { cwd: repo, windowsHide: true, stdio: ["ignore", "pipe", "pipe"], ...options }); child.stdout.pipe(out); child.stderr.pipe(out); child.on("exit", () => out.end()); children.push(child); return child; }
async function until(check, label) { const end = Date.now() + 120000; while (Date.now() < end) { try { const value = await check(); if (value) return value; } catch {} if (children.some(child => child.exitCode !== null)) throw new Error(`Fixture exited during ${label}; ${directory}`); await pause(250); } throw new Error(`Timeout ${label}; ${directory}`); }
async function port() { const server = createServer(); await new Promise(resolve => server.listen(0, "127.0.0.1", resolve)); const value = server.address().port; await new Promise(resolve => server.close(resolve)); return value; }
async function cleanup() { return closing ??= (async () => { if (outer) { outer.closeAllConnections(); await new Promise(resolve => outer.close(resolve)); } if (next && next.exitCode === null) { if (process.platform === "win32") await run("taskkill", ["/PID", String(next.pid), "/T", "/F"]); else next.kill("SIGTERM"); } if (go && go.exitCode === null) { await writeFile(join(directory, "stop-go"), "stop"); const end = Date.now() + 30000; while (go.exitCode === null && Date.now() < end) await pause(100); if (go.exitCode !== 0) throw new Error("Go fixture did not cleanly stop"); } await writeFile(join(directory, "cleanup.json"), JSON.stringify({ goExit: go?.exitCode, nextStopped: next?.exitCode !== null })); })(); }
process.once("SIGINT", () => void cleanup().then(() => process.exit(0))); process.once("SIGTERM", () => void cleanup().then(() => process.exit(0)));
try {
  console.log(`MEMBERS_FIXTURE_DIRECTORY=${directory}`);
  const binary = join(directory, process.platform === "win32" ? "members.test.exe" : "members.test");
  await run("go", ["test", "-tags=integration", "-c", "-o", binary, "./internal/app/httpapi"]);
  go = start(binary, ["-test.run=^TestMembershipBrowserFixture$", "-test.v", "-test.timeout=27m"], "go.log", { env: { ...process.env, ISSUE410_BROWSER_FIXTURE_DIR: directory } });
  const seed = await until(async () => JSON.parse(await readFile(join(directory, "go.json"), "utf8")), "Go/PG");
  const innerPort = await port(); const outerPort = await port(); const origin = `http://127.0.0.1:${outerPort}`; const secret = randomBytes(48).toString("base64url");
  const sessions = {};
  for (const actor of seed.tokens) sessions[actor] = await encode({ secret, salt: "authjs.session-token", maxAge: 1800, token: { sub: actor, name: actor, accessToken: actor, expiresAt: Math.floor(Date.now() / 1000) + 1800, identityVersion: 3, identity: { tenantId: "A", userId: actor, roles: [], userType: "zitadel" } } });
  // A task-only same-origin login bridge installs synthetic test sessions through
  // normal HTTP cookies. No browser cookie-store introspection or production route.
  outer = createServer((request, response) => {
    const url = new URL(request.url, origin);
    if (url.pathname.startsWith("/__fixture/login/")) { const actor = url.pathname.split("/").at(-1); if (!sessions[actor]) { response.writeHead(404).end(); return; } response.writeHead(302, { "Set-Cookie": [`authjs.session-token=${sessions[actor]}; Path=/; HttpOnly; SameSite=Lax`, "shuomi_effective_organization=B; Path=/; HttpOnly; SameSite=Lax"], Location: "/workbench/account/organization/members", "Cache-Control": "no-store" }); response.end(); return; }
    const upstream = proxyRequest({ host: "127.0.0.1", port: innerPort, path: request.url, method: request.method, headers: { ...request.headers, host: `127.0.0.1:${outerPort}` } }, incoming => { response.writeHead(incoming.statusCode, incoming.headers); incoming.pipe(response); }); upstream.on("error", () => { if (!response.headersSent) response.writeHead(502); response.end(); }); request.pipe(upstream);
  });
  // Next development streams require the same-origin HMR upgrade as well as HTTP.
  const upgradeSockets = new Set();
  outer.on("upgrade", (request, socket, head) => {
    const upstream = connect(innerPort, "127.0.0.1", () => {
      upstream.write(`${request.method} ${request.url} HTTP/${request.httpVersion}\r\n${Object.entries(request.headers).map(([key, value]) => `${key}: ${value}`).join("\r\n")}\r\n\r\n`);
      if (head.length) upstream.write(head);
      socket.pipe(upstream).pipe(socket);
    });
    upgradeSockets.add(socket);
    socket.on("close", () => { upgradeSockets.delete(socket); upstream.destroy(); });
    upstream.on("error", () => socket.destroy());
    socket.on("error", () => upstream.destroy());
  });
  const closeHTTP = outer.close.bind(outer);
  outer.close = callback => { for (const socket of upgradeSockets) socket.destroy(); return closeHTTP(callback); };
  await new Promise(resolve => outer.listen(outerPort, "127.0.0.1", resolve));
  next = start(process.execPath, [join(web, "node_modules/next/dist/bin/next"), "dev", "--hostname", "127.0.0.1", "--port", String(innerPort)], "next.log", { cwd: web, env: { ...process.env, NODE_ENV: "development", AUTH_SECRET: secret, AUTH_URL: origin, LISTINGKIT_PUBLIC_BASE_URL: origin, ZITADEL_ISSUER_URL: seed.issuerURL, ZITADEL_CLIENT_ID: "membership-fixture", ZITADEL_CLIENT_SECRET: "", LISTINGKIT_SERVICE_API_BASE: `${seed.goOrigin}/api/v1` } });
  await until(async () => (await fetch(`${origin}/api/auth/session`)).ok, "Next/Auth.js");
  await writeFile(join(directory, "fixture.json"), JSON.stringify({ origin, ...seed, sourceHead: await run("git", ["rev-parse", "HEAD"]), dirtyWorktree: true, boundary: "synthetic external ZITADEL and session issuance; production Auth.js/BFF/current Go/PG" }), { mode: 0o600 });
  console.log(JSON.stringify({ origin, manifest: join(directory, "fixture.json"), loginAdmin: `${origin}/__fixture/login/admin`, loginViewer: `${origin}/__fixture/login/viewer` }));
  const end = Date.now() + 22 * 60000; while (Date.now() < end) { try { await access(join(directory, "stop-fixture")); break; } catch {} await pause(500); }
} finally { await cleanup(); }
