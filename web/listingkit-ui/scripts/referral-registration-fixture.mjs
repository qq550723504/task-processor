import assert from "node:assert/strict";
import { execFile as execFileCallback, spawn } from "node:child_process";
import { createHash, randomBytes, randomUUID } from "node:crypto";
import { readFile, writeFile, mkdir, unlink, rm, rename } from "node:fs/promises";
import { request as httpsRequest } from "node:https";
import { createServer } from "node:net";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { setTimeout as delay } from "node:timers/promises";
import { promisify } from "node:util";

const execFile = promisify(execFileCallback);
const uiRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const repo = path.resolve(uiRoot, "../..");
const runtimeScript = path.join(repo, "scripts", "issue357-runtime.mjs");
const dockerHost = "npipe:////./pipe/dockerDesktopLinuxEngine";
const ownerLabel = "com.shuomi.issue413.run";
const runtimeOwnerLabel = "com.shuomi.issue357.run";
const caddyImage = "caddy:2.11.4-alpine";
const mailImage = "axllent/mailpit:v1.30.4";
const report = {
  schemaVersion: "issue413-referral-registration-fixture-v2",
  status: "NOT_RUN",
  checks: [],
  matrix: [],
  manualAccessibility: "NOT_RUN",
  invocationId: randomUUID(),
  injectionPoint: "none",
};
let manifest;
let browser;
let outputDirectory;
let caddyName;
let mailName;
let fixtureAllocatedPorts = {};
const ownedClientNames = [];
let createdSubjectDeleted = false;
let machineDeleted = false;
let runtimeOwnershipUnknown = false;

function ensure(value, code = "ASSERTION_FAILED") {
  assert.ok(value, code);
}

async function check(name, operation, recordEvidence = true) {
  const started = Date.now();
  try {
    const evidence = await operation();
    report.checks.push({ name, status: "PASS", elapsedMs: Date.now() - started, ...(recordEvidence && evidence && typeof evidence === "object" ? evidence : {}) });
    console.log(`PASS ${name}`);
    return evidence;
  } catch (error) {
    report.checks.push({ name, status: "FAIL", elapsedMs: Date.now() - started, code: safeCode(error) });
    throw error;
  }
}

async function matrixCheck(group, name, operation) {
  const started = Date.now();
  try {
    const evidence = await operation();
    report.matrix.push({ group, name, status: "PASS", elapsedMs: Date.now() - started, ...(evidence && typeof evidence === "object" ? evidence : {}) });
    console.log(`PASS ${group}.${name}`);
    return evidence;
  } catch (error) {
    report.matrix.push({ group, name, status: "FAIL", elapsedMs: Date.now() - started, code: safeCode(error) });
    console.error(`FAIL ${group}.${name} ${safeCode(error)}`);
    return undefined;
  }
}

function matrixNotRun(group, name, reason) {
  report.matrix.push({ group, name, status: "NOT_RUN", reason });
}

function safeCode(error) {
  const raw = error instanceof Error ? error.message : String(error);
  return /^[A-Z0-9_:-]{1,160}$/.test(raw) ? raw : "FIXTURE_STEP_FAILED";
}

export async function runCleanupPass({ phase, actions, inspectResiduals, now = () => new Date().toISOString() }) {
  const startedAt = now();
  const results = [];
  for (const action of actions) {
    try {
      await action.run();
      results.push({ name: action.name, status: "PASS" });
    } catch (error) {
      results.push({ name: action.name, status: "FAIL", code: safeCode(error) });
    }
  }
  let residuals;
  try {
    const observed = await inspectResiduals();
    ensure(["containers", "volumes", "networks", "listeners"].every(name => Number.isInteger(observed[name]) && observed[name] >= 0), "INVALID_CLEANUP_OBSERVATION");
    residuals = { status: "PASS", ...observed };
  } catch (error) {
    residuals = { status: "UNKNOWN", code: safeCode(error) };
  }
  const actionFailed = results.some(result => result.status !== "PASS");
  const resourcesRemain = residuals.status === "PASS" && ["containers", "volumes", "networks", "listeners"].some(name => residuals[name] !== 0);
  const status = residuals.status === "UNKNOWN" ? "UNKNOWN" : actionFailed || resourcesRemain ? "FAIL" : "PASS";
  return {
    phase,
    status,
    actions: results,
    residuals,
    ...(resourcesRemain ? { code: "RESOURCES_REMAIN" } : {}),
    startedAt,
    finishedAt: now(),
  };
}

export async function orchestrateFixtureLifecycle({ report, runBusiness, runCleanup, persistReport, emitFailure, now = () => new Date().toISOString() }) {
  report.startedAt ??= now();
  report.business = { status: "NOT_RUN", startedAt: now() };
  try {
    await runBusiness();
    report.business.status = "PASS";
  } catch (error) {
    report.business.status = "FAIL";
    report.business.code = safeCode(error);
  }
  report.business.finishedAt = now();

  const cleanup = {};
  for (const phase of ["initial", "final"]) {
    try {
      cleanup[phase] = await runCleanup(phase);
    } catch (error) {
      cleanup[phase] = {
        phase,
        status: "UNKNOWN",
        code: safeCode(error),
        actions: [],
        residuals: { status: "UNKNOWN", code: safeCode(error) },
        startedAt: now(),
        finishedAt: now(),
      };
    }
  }
  report.cleanup = cleanup;
  const lifecyclePassed = report.business.status === "PASS" && cleanup.initial.status === "PASS" && cleanup.final.status === "PASS";
  report.conclusion = lifecyclePassed ? "PASS" : "FAIL";
  report.status = report.conclusion;
  report.exitCode = lifecyclePassed ? 0 : 1;
  report.finishedAt = now();
  report.evidence = { status: "PASS" };

  try {
    await persistReport(report);
  } catch (error) {
    report.evidence = { status: "FAIL", code: safeCode(error) };
    report.conclusion = "FAIL";
    report.status = "FAIL";
    report.exitCode = 1;
    emitFailure({
      schemaVersion: report.schemaVersion,
      runId: report.runId,
      status: "FAIL",
      stage: "report-write",
      code: report.evidence.code,
    });
  }
  return { report, exitCode: report.exitCode };
}

async function lifecycleTestCommand(name) {
  ensure(process.env.ISSUE413_FIXTURE_TEST_ONLY === "1", "TEST_MODE_REQUIRED");
  const fixture = await import("./fixtures/referral-registration-fixture-fault-runtime.mjs");
  const outcome = await fixture.runCLIFaultScenario(name, {
    orchestrate: orchestrateFixtureLifecycle,
    cleanupPass: runCleanupPass,
    runProcess: run,
    persistAtomic: writeJSONAtomic,
    recordRuntime: recordBaseRuntime,
    runCleanup: phase => cleanup(undefined, undefined, phase),
  });
  process.exitCode = outcome.exitCode;
  console.log(JSON.stringify(outcome));
}

async function run(command, args, options = {}) {
  try {
    const result = await execFile(command, args, {
      cwd: repo,
      windowsHide: true,
      timeout: 360_000,
      maxBuffer: 8 * 1024 * 1024,
      ...options,
    });
    return result.stdout.trim();
  } catch (cause) {
    const error = new Error(`PROCESS_FAILED:${path.basename(command).replace(/[^A-Za-z0-9_-]/g, "_").toUpperCase()}`);
    const privateOutput = `${typeof cause?.stdout === "string" ? cause.stdout : ""}\n${typeof cause?.stderr === "string" ? cause.stderr : ""}`.slice(0, 1024 * 1024);
    Object.defineProperty(error, "privateOutput", { value: privateOutput });
    throw error;
  }
}

function docker(args, options) {
  return run("docker", ["--host", dockerHost, ...args], options);
}

async function runWithInput(command, args, input) {
  await new Promise((resolve, reject) => {
    const child = spawn(command, args, { cwd: repo, windowsHide: true, stdio: ["pipe", "pipe", "pipe"] });
    let outputBytes = 0;
    const drain = chunk => { outputBytes += chunk.length; if (outputBytes > 1024 * 1024) child.kill(); };
    child.stdout.on("data", drain);
    child.stderr.on("data", drain);
    child.once("error", () => reject(new Error("PROCESS_START_FAILED")));
    child.once("close", code => code === 0 ? resolve() : reject(new Error("PROCESS_FAILED:psql")));
    child.stdin.end(input);
  });
}

async function freePort(preferred = 0) {
  const server = createServer();
  await new Promise((resolve, reject) => { server.once("error", reject); server.listen(preferred, "127.0.0.1", resolve); });
  const address = server.address();
  ensure(address && typeof address === "object", "PORT_ALLOCATION_FAILED");
  await new Promise(resolve => server.close(resolve));
  return address.port;
}

async function fixturePorts() {
  const reserved = new Set(Object.values(manifest.ports));
  const allocated = {};
  for (const name of ["next", "provider", "public", "mail"]) {
    let candidate;
    do {
      const preferred = 20_000 + randomBytes(2).readUInt16BE() % 40_000;
      candidate = await freePort(preferred).catch(() => undefined);
    } while (!candidate || reserved.has(candidate));
    reserved.add(candidate);
    allocated[name] = candidate;
  }
  return allocated;
}

async function writePrivate(file, value) {
  await writeFile(file, value, { encoding: "utf8", mode: 0o600 });
}

async function readJSON(file) {
  return JSON.parse(await readFile(file, "utf8"));
}

async function writeJSON(file, value) {
  await writePrivate(file, `${JSON.stringify(value, null, 2)}\n`);
}

export async function writeJSONAtomic(file, value) {
  const temporary = `${file}.${process.pid}.${randomBytes(8).toString("hex")}.tmp`;
  try {
    await writePrivate(temporary, `${JSON.stringify(value, null, 2)}\n`);
    await rename(temporary, file);
  } finally {
    await unlink(temporary).catch(error => { if (error.code !== "ENOENT") throw error; });
  }
}

async function until(operation, code, timeout = 60_000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) {
    try {
      const value = await operation();
      if (value) return value;
    } catch {}
    await delay(250);
  }
  throw new Error(`TIMEOUT:${code}`);
}

async function provider(pathname, body, token, method = "POST") {
  const response = await fetch(`${manifest.origins.issuer}${pathname}`, {
    method,
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
    body: body === undefined ? undefined : JSON.stringify(body),
    redirect: "error",
    signal: AbortSignal.timeout(30_000),
  });
  const bytes = new Uint8Array(await response.arrayBuffer());
  ensure(bytes.byteLength <= 1024 * 1024, "PROVIDER_RESPONSE_TOO_LARGE");
  if (!response.ok) {
    await writeJSON(path.join(outputDirectory, "provider-http-error.json"), {
      pathname,
      status: response.status,
      body: new TextDecoder().decode(bytes.slice(0, 16 * 1024)),
    });
    throw new Error(`PROVIDER_HTTP_${response.status}`);
  }
  return bytes.byteLength ? JSON.parse(new TextDecoder().decode(bytes)) : {};
}

async function startBaseRuntime() {
  report.runtimeDispatched = true;
  let stdout;
  try {
    stdout = await run(process.execPath, [runtimeScript, "start", "--current-application", "--web-dir", uiRoot]);
  } catch (error) {
    await recordBaseRuntime(error.privateOutput ?? "").catch(() => {});
    throw error;
  }
  await recordBaseRuntime(stdout);
  try {
    await run(process.execPath, [runtimeScript, "stop", "--run", report.runId]);
  } catch {
    await run(process.execPath, [runtimeScript, "stop", "--run", report.runId]);
  }
}

export async function recordBaseRuntime(stdout) {
  const runId = /runId=([0-9a-f-]{36})/.exec(stdout)?.[1];
  ensure(runId, "RUNTIME_ID_MISSING");
  const root = path.join(tmpdir(), "task-processor-issue357", runId);
  report.runId = runId;
  outputDirectory = path.join(root, "referral-registration-evidence");
  await mkdir(outputDirectory, { recursive: true });
  let current;
  try { current = await readJSON(path.join(root, "manifest.json")); }
  catch { runtimeOwnershipUnknown = true; throw new Error("RUNTIME_MANIFEST_UNKNOWN"); }
  if (current.runId !== runId) { runtimeOwnershipUnknown = true; throw new Error("RUNTIME_ID_MISMATCH"); }
  manifest = current;
  manifest.directory = root;
  runtimeOwnershipUnknown = false;
  report.sourceSha = manifest.sourceSha;
  report.webSha = manifest.webSha;
}

async function startOwnedContainers(ports) {
  caddyName = `${manifest.project}-referral-caddy`;
  mailName = `${manifest.project}-referral-mailpit`;
  const network = `${manifest.project}-network`;
  await docker(["run", "-d", "--name", mailName, "--label", `${ownerLabel}=${manifest.runId}`, "--network", network,
    "-p", `127.0.0.1:${ports.mail}:8025`, "-e", "MP_MAX_MESSAGES=50", "-e", "MP_MAX_MESSAGE_SIZE=1",
    "-e", "MP_DISABLE_VERSION_CHECK=true", mailImage]);

  const caddyRoot = path.join(manifest.directory, "referral-caddy");
  const caddyData = path.join(caddyRoot, "data");
  await mkdir(caddyData, { recursive: true });
  const config = path.join(caddyRoot, "Caddyfile");
  await writePrivate(config, `{
  admin off
  local_certs
}
:80 {
  reverse_proxy host.docker.internal:${ports.next} {
    header_up -X-Referral-Service-Credential
    header_up -X-Referral-Client-IP
    header_up X-ListingKit-Client-IP {remote_host}
  }
}
https://localhost:443 {
  tls internal
  log {
    output stdout
    format json
  }
  handle /ui/v2/login* {
    reverse_proxy http://zitadel-login:3000 {
      header_up Host localhost:${ports.provider}
      header_up X-Forwarded-Host localhost:${ports.provider}
      header_up X-Forwarded-Proto https
    }
  }
  handle {
    reverse_proxy http://proxy:80 {
      header_up Host localhost:${manifest.ports.issuer}
      header_up X-Forwarded-Proto http
    }
  }
}
https://localhost:444 {
  tls internal
  reverse_proxy host.docker.internal:${ports.next} {
    header_up -X-Referral-Service-Credential
    header_up -X-Referral-Client-IP
    header_up X-ListingKit-Client-IP {remote_host}
  }
}
`);
  await docker(["run", "-d", "--name", caddyName, "--label", `${ownerLabel}=${manifest.runId}`, "--network", network,
    "-p", `127.0.0.1:${manifest.ports.web}:80`, "-p", `127.0.0.1:${ports.provider}:443`, "-p", `127.0.0.1:${ports.public}:444`,
    "--mount", `type=bind,source=${config},target=/etc/caddy/Caddyfile,readonly`, "--mount", `type=bind,source=${caddyData},target=/data`,
    caddyImage, "caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]);
  const caFile = path.join(caddyData, "caddy", "pki", "authorities", "local", "root.crt");
  const leafFile = path.join(caddyData, "caddy", "certificates", "local", "localhost", "localhost.crt");
  await until(async () => {
    const pem = await readFile(caFile, "utf8");
    const leaf = await readFile(leafFile, "utf8");
    return pem.includes("BEGIN CERTIFICATE") && pem.length < 64 * 1024 && leaf.includes("BEGIN CERTIFICATE") && leaf.length < 64 * 1024;
  }, "CADDY_CERTIFICATES", 60_000);
  await until(async () => (await fetch(`http://127.0.0.1:${ports.mail}/api/v1/info`, { signal: AbortSignal.timeout(2_000) })).ok, "MAILPIT", 60_000);
  return caFile;
}

async function configureProvider(bootstrap) {
  const smtp = await provider("/admin/v1/email/smtp", {
    description: `Issue 413 ${manifest.runId}`,
    host: `${mailName}:1025`,
    senderAddress: "referrals@localhost",
    senderName: "ListingKit acceptance",
    replyToAddress: "no-reply@localhost",
    tls: false,
    none: {},
  }, bootstrap);
  ensure(smtp.id, "SMTP_ID_MISSING");
  await provider(`/admin/v1/email/${encodeURIComponent(smtp.id)}/_activate`, {}, bootstrap);

  const machine = await provider("/v2/users/new", {
    organizationId: manifest.organizations.A.id,
    username: `referral-runtime-${manifest.runId.slice(0, 8)}`,
    machine: { name: "Referral registration runtime", description: "Issue 413 isolated acceptance", accessTokenType: "ACCESS_TOKEN_TYPE_BEARER" },
  }, bootstrap);
  ensure(machine.id, "MACHINE_ID_MISSING");
  await provider("/zitadel.internal_permission.v2.InternalPermissionService/CreateAdministrator", {
    userId: machine.id,
    resource: { organizationId: manifest.organizations.A.id },
    roles: ["ORG_USER_MANAGER"],
  }, bootstrap);
  const pat = await provider(`/v2/users/${encodeURIComponent(machine.id)}/pats`, { expirationDate: new Date(Date.now() + 4 * 60 * 60 * 1000).toISOString() }, bootstrap);
  ensure(typeof pat.token === "string" && pat.token.length >= 32, "MACHINE_TOKEN_MISSING");
  await provider(`/v2/users/${encodeURIComponent(manifest.users.viewer.id)}`, undefined, pat.token, "GET");
  return { machineId: machine.id, token: pat.token, smtpId: smtp.id };
}

async function configureDatabase(runtimePassword) {
  const dsnFile = path.join(manifest.directory, "referral-owner.dsn");
  const ownerPassword = (await readJSON(path.join(manifest.directory, "owner-secrets.json"))).commercialDatabase;
  const dsn = `postgres://issue357:${encodeURIComponent(ownerPassword)}@127.0.0.1:${manifest.ports.database}/issue357?sslmode=disable`;
  await writePrivate(dsnFile, `${dsn}\n`);
  await run("go", ["run", "./cmd/referral-schema-init", "-dsn-file", dsnFile, "-timeout", "30s"]);
  const sql = `CREATE ROLE referral_runtime LOGIN PASSWORD '${runtimePassword}';
REVOKE ALL ON SCHEMA public FROM PUBLIC;
GRANT USAGE ON SCHEMA public TO referral_runtime;
REVOKE CREATE,TEMP ON DATABASE issue357 FROM PUBLIC;
GRANT CONNECT ON DATABASE issue357 TO referral_runtime;
GRANT SELECT,INSERT ON public.referral_codes,public.registration_intents,public.referral_relations,public.referral_receipts,public.registration_admission_buckets TO referral_runtime;
GRANT UPDATE(state,ciphertext,lease_until) ON public.registration_intents TO referral_runtime;
GRANT UPDATE,DELETE ON public.registration_admission_buckets TO referral_runtime;
`;
  await runWithInput("docker", ["--host", dockerHost, "exec", "-i", `${manifest.project}-commercial-db`, "psql", "-v", "ON_ERROR_STOP=1", "-U", "issue357", "-d", "issue357"], sql);
}

async function configureApplications(ports, caFile, providerCredential) {
  const applications = await readJSON(path.join(manifest.directory, "applications.json"));
  const bootstrap = (await readFile(path.join(manifest.directory, "bootstrap.pat"), "utf8")).trim();
  ensure(typeof applications.ProjectID === "string" && applications.ProjectID.length > 0, "PROJECT_ID_MISSING");
  ensure(typeof applications.OIDCAppID === "string" && applications.OIDCAppID.length > 0, "OIDC_APP_ID_MISSING");
  const publicOrigin = `https://localhost:${ports.public}`;
  const providerOrigin = `https://localhost:${ports.provider}`;
  await provider("/zitadel.application.v2.ApplicationService/UpdateApplication", {
    applicationId: applications.OIDCAppID,
    projectId: applications.ProjectID,
    oidcConfiguration: {
      redirectUris: [`${publicOrigin}/api/auth/callback/zitadel`],
      postLogoutRedirectUris: [publicOrigin],
    },
  }, bootstrap).catch(() => { throw new Error("OIDC_APPLICATION_UPDATE_FAILED"); });

  const secretPaths = Object.fromEntries(["provider", "service", "lookup", "proof", "encryption"].map(name => [name, path.join(manifest.directory, `referral-${name}.secret`)]));
  await writePrivate(secretPaths.provider, `${providerCredential}\n`);
  for (const name of ["service", "lookup", "proof", "encryption"]) await writePrivate(secretPaths[name], `${randomBytes(32).toString("hex")}\n`);
  const runtimePassword = randomBytes(24).toString("hex");
  await configureDatabase(runtimePassword);

  const current = await readJSON(path.join(manifest.directory, "current-application.json"));
  current.referrals = {
    enabled: true,
    issuer: current.identity.issuerURL,
    instanceID: manifest.instanceId,
    signupOrganizationID: manifest.organizations.A.id,
    providerOrigin,
    officialLoginOrigin: providerOrigin,
    publicAppOrigin: publicOrigin,
    credentialFile: secretPaths.provider,
    serviceCredentialFile: secretPaths.service,
    lookupKeyFile: secretPaths.lookup,
    keyID: "issue413-v1",
    proofKeyFiles: { "issue413-v1": secretPaths.proof },
    encryptionKeyFiles: { "issue413-v1": secretPaths.encryption },
    providerCAFile: caFile,
    referralDatabase: { host: "127.0.0.1", port: manifest.ports.database, user: "referral_runtime", password: runtimePassword, database: "issue357", maxConnections: 4 },
  };
  await writeJSON(path.join(manifest.directory, "current-application.json"), current);

  const services = await readJSON(path.join(manifest.directory, "services.json"));
  services.webPort = ports.next;
  Object.assign(services.nextEnvironment, {
    AUTH_URL: publicOrigin,
    LISTINGKIT_PUBLIC_BASE_URL: publicOrigin,
    ZITADEL_REDIRECT_URI: `${publicOrigin}/api/auth/callback/zitadel`,
    ZITADEL_POST_LOGOUT_REDIRECT_URI: publicOrigin,
    LISTINGKIT_REFERRAL_SERVICE_CREDENTIAL_FILE: secretPaths.service,
    NODE_EXTRA_CA_CERTS: caFile,
  });
  await writeJSON(path.join(manifest.directory, "services.json"), services);
  return { publicOrigin, providerOrigin };
}

async function startConfiguredApplications(ports) {
  for (const name of ["stop-go", "stop-next", "stop-services", "go-ready.json", "next-ready.json", "services-stopped.json"]) {
    await unlink(path.join(manifest.directory, name)).catch(error => { if (error.code !== "ENOENT") throw error; });
  }
  const child = spawn(process.execPath, [path.join(repo, "scripts", "issue357", "serve.mjs"), manifest.directory], {
    cwd: manifest.directory,
    detached: true,
    windowsHide: true,
    stdio: "ignore",
  });
  await new Promise((resolve, reject) => { child.once("spawn", resolve); child.once("error", () => reject(new Error("SUPERVISOR_START_FAILED"))); });
  child.unref();
  await until(async () => {
    await readJSON(path.join(manifest.directory, "go-ready.json"));
    await readJSON(path.join(manifest.directory, "next-ready.json"));
    return true;
  }, "CONFIGURED_APPLICATION_START", 300_000);
  const go = await fetch(`${manifest.origins.go}/api/v1/account/profile`, { signal: AbortSignal.timeout(10_000) });
  ensure(go.status === 401, "GO_HEALTH_FAILED");
  const providersResponse = await fetch(`http://127.0.0.1:${ports.next}/api/auth/providers`, { signal: AbortSignal.timeout(90_000) });
  ensure(providersResponse.ok, "NEXT_HEALTH_FAILED");
  const providers = await providersResponse.json();
  ensure(providers.zitadel, "NEXT_PROVIDER_MISSING");
  return { ready: true };
}

async function restartConfiguredApplications(ports) {
  await writePrivate(path.join(manifest.directory, "stop-services"), "stop\n");
  await until(async () => {
    await readJSON(path.join(manifest.directory, "services-stopped.json"));
    return true;
  }, "CONFIGURED_APPLICATION_STOP", 90_000);
  await startConfiguredApplications(ports);
  return { processMemoryLost: true, persistentDatabaseRetained: true };
}

async function probeProviderProxy(providerOrigin, caFile, machineToken) {
  const ca = await readFile(caFile);
  const subject = manifest.users.viewer.id;
  const target = new URL(`/v2/users/${encodeURIComponent(subject)}`, providerOrigin);
  const result = await new Promise((resolve, reject) => {
    const request = httpsRequest(target, {
      method: "GET",
      ca,
      rejectUnauthorized: true,
      headers: { Authorization: `Bearer ${machineToken}`, "Content-Type": "application/json" },
      timeout: 10_000,
    }, response => {
      const chunks = [];
      let size = 0;
      response.on("data", chunk => {
        size += chunk.length;
        if (size > 1024 * 1024) request.destroy(new Error("PROVIDER_PROXY_RESPONSE_TOO_LARGE"));
        else chunks.push(chunk);
      });
      response.on("end", () => resolve({ status: response.statusCode, body: Buffer.concat(chunks) }));
    });
    request.once("timeout", () => request.destroy(new Error("PROVIDER_PROXY_TIMEOUT")));
    request.once("error", reject);
    request.end();
  }).catch(async error => {
    await writePrivate(path.join(outputDirectory, "provider-proxy-error.log"), `${error instanceof Error ? error.stack ?? error.message : String(error)}\n`);
    try {
      const output = await execFile("docker", ["--host", dockerHost, "logs", "--tail", "200", caddyName], { windowsHide: true, timeout: 5_000, maxBuffer: 512 * 1024 });
      await writePrivate(path.join(outputDirectory, "provider-proxy-caddy.log"), `${output.stdout}\n${output.stderr}`.slice(-128 * 1024));
    } catch {}
    const code = typeof error?.code === "string" ? error.code.toUpperCase().replace(/[^A-Z0-9_]/g, "_").slice(0, 100) : "REQUEST_FAILED";
    throw new Error(`PROVIDER_PROXY_${code}`);
  });
  ensure(result.status === 200, `PROVIDER_PROXY_HTTP_${result.status ?? "UNKNOWN"}`);
  const payload = JSON.parse(result.body.toString("utf8"));
  ensure(payload.user?.userId === subject, "PROVIDER_PROXY_SUBJECT_MISMATCH");
  return { tlsVerified: true, providerReadStatus: result.status };
}

async function login(page, origin, credential, target) {
  await page.goto(`${origin}${target}`, { waitUntil: "load" }).catch(() => { throw new Error("PUBLIC_PROXY_PAGE_LOAD_FAILED"); });
  const username = page.getByTestId("username-text-input");
  await username.waitFor({ state: "visible", timeout: 45_000 }).catch(() => { throw new Error("OFFICIAL_USERNAME_PAGE_MISSING"); });
  await username.fill(credential.username);
  await page.getByTestId("submit-button").click();
  const password = page.getByTestId("password-text-input");
  await password.waitFor({ state: "visible", timeout: 30_000 }).catch(() => { throw new Error("OFFICIAL_PASSWORD_PAGE_MISSING"); });
  await password.fill(credential.password);
  await page.getByTestId("submit-button").click();
  await page.waitForURL(url => url.origin === origin && url.pathname === target, { timeout: 45_000 })
    .catch(() => { throw new Error("OIDC_CALLBACK_DID_NOT_RETURN"); });
}

async function waitForMessage(mailPort, email, providerOrigin) {
  let latest = { messages: [] };
  let matching;
  let detail;
  try {
    return await until(async () => {
      latest = await (await fetch(`http://127.0.0.1:${mailPort}/api/v1/messages`, { signal: AbortSignal.timeout(2_000) })).json();
      matching = latest.messages?.find(message => message.To?.some(recipient => recipient.Address === email));
      if (!matching?.ID) return null;
      detail = await (await fetch(`http://127.0.0.1:${mailPort}/api/v1/message/${encodeURIComponent(matching.ID)}`, { signal: AbortSignal.timeout(2_000) })).json();
      const content = `${detail.Text ?? ""}\n${detail.HTML ?? ""}`.replaceAll("&amp;", "&");
      const link = (/https:\/\/localhost:\d+\/ui\/v2\/login\/verify\?[^\s<"']+/gi.exec(content) ?? []).find(candidate => {
        const verification = new URL(candidate);
        return verification.origin === providerOrigin && verification.pathname === "/ui/v2/login/verify" && verification.searchParams.has("code") && verification.searchParams.has("userId") && verification.searchParams.has("organization");
      });
      return link ? { link, id: matching.ID } : null;
    }, "OFFICIAL_VERIFICATION_MAIL", 60_000);
  } catch {
    let mailpitLog = "";
    let providerLog = "";
    try {
      const output = await execFile("docker", ["--host", dockerHost, "logs", "--tail", "100", mailName], { windowsHide: true, timeout: 5_000, maxBuffer: 256 * 1024 });
      mailpitLog = `${output.stdout}\n${output.stderr}`.slice(-64 * 1024);
    } catch {}
    try {
      const output = await execFile("docker", ["--host", dockerHost, "logs", "--tail", "200", `${manifest.project}-zitadel-api`], { windowsHide: true, timeout: 5_000, maxBuffer: 512 * 1024 });
      providerLog = `${output.stdout}\n${output.stderr}`.slice(-128 * 1024);
    } catch {}
    await writeJSON(path.join(outputDirectory, "mailbox-diagnostic.json"), {
      messageCount: Array.isArray(latest.messages) ? latest.messages.length : 0,
      matchingRecipient: Boolean(matching?.ID),
      matchingBodyPresent: Boolean(detail && (detail.Text || detail.HTML)),
    });
    await writePrivate(path.join(outputDirectory, "mailpit-diagnostic.log"), mailpitLog);
    await writePrivate(path.join(outputDirectory, "provider-diagnostic.log"), providerLog);
    if (matching?.ID) throw new Error("OFFICIAL_VERIFICATION_LINK_MISSING");
    if (Array.isArray(latest.messages) && latest.messages.length > 0) throw new Error("OFFICIAL_MAIL_RECIPIENT_MISMATCH");
    throw new Error("OFFICIAL_VERIFICATION_MAIL_MISSING");
  }
}

async function assertNoSeriousA11y(page) {
  const { default: AxeBuilder } = await import("@axe-core/playwright");
  const results = await new AxeBuilder({ page }).analyze();
  const serious = results.violations.filter(item => ["serious", "critical"].includes(item.impact));
  const safeResults = {
    violations: results.violations.map(item => ({ id: item.id, impact: item.impact, nodeCount: item.nodes.length })),
    incomplete: results.incomplete.map(item => ({ id: item.id, impact: item.impact, nodeCount: item.nodes.length })),
  };
  await writeJSON(path.join(outputDirectory, `axe-${report.axeRuns = (report.axeRuns ?? 0) + 1}.json`), safeResults);
  if (serious.length) {
    const rules = serious.map(item => item.id.toUpperCase().replace(/[^A-Z0-9_]/g, "_")).sort().join("_").slice(0, 120);
    throw new Error(`ACCESSIBILITY_${rules}`);
  }
  return { violations: results.violations.length, incomplete: results.incomplete.length };
}

async function waitForReactHydration(page, locator) {
  const element = await locator.elementHandle();
  ensure(element, "HYDRATION_TARGET_MISSING");
  await page.waitForFunction(node => Object.keys(node).some(key => key.startsWith("__reactProps$")), element, { timeout: 30_000 })
    .catch(() => { throw new Error("REACT_HYDRATION_TIMEOUT"); });
}

async function readCreationState(intentID, email, mailPort, machineToken) {
  ensure(/^[A-Za-z0-9._:-]{1,200}$/.test(intentID ?? ""), "REGISTRATION_INTENT_MISSING");
  const row = await run("docker", ["--host", dockerHost, "exec", `${manifest.project}-commercial-db`, "psql", "-At", "-U", "issue357", "-d", "issue357", "-c", `SELECT state || '|' || subject || '|' || (lease_until > clock_timestamp())::text FROM public.registration_intents WHERE id='${intentID}'`]);
  const [intentState, subject, leaseActiveText, extra] = row.split("|");
  ensure(!extra && ["PREPARED", "CREATED", "CONSUMED"].includes(intentState) && /^[A-Za-z0-9._:-]{1,200}$/.test(subject ?? "") && ["true", "false"].includes(leaseActiveText), "REGISTRATION_INTENT_STATE_INVALID");
  let subjectExists = false;
  let proofMetadataPresent = false;
  let metadataCount = 0;
  try {
    const user = await provider(`/v2/users/${encodeURIComponent(subject)}`, undefined, machineToken, "GET");
    subjectExists = user.user?.userId === subject;
    if (subjectExists) {
      const metadata = await provider(`/v2/users/${encodeURIComponent(subject)}/metadata/search`, { pagination: { limit: 100 } }, machineToken);
      const entries = Array.isArray(metadata.metadata) ? metadata.metadata : [];
      metadataCount = entries.length;
      proofMetadataPresent = entries.some(entry => entry?.key === "referral-registration-proof" && typeof entry.value === "string" && entry.value.length > 0);
    }
  } catch {}
  const mailbox = await (await fetch(`http://127.0.0.1:${mailPort}/api/v1/messages`, { signal: AbortSignal.timeout(2_000) })).json();
  const matchingMessageCount = mailbox.messages?.filter(message => message.To?.some(recipient => recipient.Address === email)).length ?? 0;
  return { intentState, leaseActive: leaseActiveText === "true", subjectExists, proofMetadataPresent, metadataCount, matchingMessageCount };
}

async function referralTablesDigest() {
  const sql = `SELECT md5(concat_ws('|',
    (SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text)::text,'[]') FROM public.referral_codes t),
    (SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text)::text,'[]') FROM public.registration_intents t),
    (SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text)::text,'[]') FROM public.referral_relations t),
    (SELECT COALESCE(jsonb_agg(to_jsonb(t) ORDER BY to_jsonb(t)::text)::text,'[]') FROM public.referral_receipts t)))`;
  const digest = await run("docker", ["--host", dockerHost, "exec", `${manifest.project}-commercial-db`, "psql", "-At", "-U", "issue357", "-d", "issue357", "-c", sql]);
  ensure(/^[a-f0-9]{32}$/.test(digest), "REFERRAL_TABLE_DIGEST_INVALID");
  return digest;
}

async function browserChain(origins, ports, machine) {
  const { chromium } = await import("@playwright/test");
  browser = await chromium.launch({ headless: true });
  const referrer = await readJSON(path.join(manifest.directory, "viewer.credentials.json"));
  const referrerContext = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN", ignoreHTTPSErrors: true });
  const referrerPage = await referrerContext.newPage();
  await check("referrer_generic_oidc_and_code", async () => {
    try {
      await login(referrerPage, origins.publicOrigin, referrer, "/workbench/account/referrals");
      const create = referrerPage.getByRole("button", { name: "创建推广码" });
      await create.waitFor({ state: "visible", timeout: 30_000 }).catch(() => { throw new Error("REFERRAL_CODE_ACTION_MISSING"); });
      const created = referrerPage.waitForResponse(response => new URL(response.url()).pathname === "/api/account/referrals" && response.request().method() === "POST", { timeout: 30_000 });
      await create.click();
      const response = await created.catch(() => { throw new Error("REFERRAL_CREATE_RESPONSE_MISSING"); });
      if (response.status() !== 200) {
        const payload = await response.json().catch(() => ({}));
        const code = typeof payload.code === "string" ? payload.code.toUpperCase().replace(/[^A-Z0-9_]/g, "_").slice(0, 80) : "UNKNOWN";
        throw new Error(`REFERRAL_CREATE_HTTP_${response.status()}_${code}`);
      }
      const codeElement = referrerPage.locator("code");
      await codeElement.waitFor({ state: "visible", timeout: 30_000 }).catch(() => { throw new Error("REFERRAL_CODE_RESULT_MISSING"); });
      const value = await codeElement.textContent();
      ensure(value && value.length <= 200, "REFERRAL_CODE_MISSING");
      const axe = await assertNoSeriousA11y(referrerPage);
      await referrerPage.screenshot({ path: path.join(outputDirectory, "referrals-overview-desktop.png"), fullPage: true });
      report.referrerSubject = manifest.users.viewer.id;
      return { codeCreated: true, viewport: "1440x1000", ...axe };
    } catch (error) {
      await referrerPage.screenshot({ path: path.join(outputDirectory, "referrer-stage-failure.png"), fullPage: true }).catch(() => {});
      throw error;
    }
  });
  const code = (await referrerPage.locator("code").textContent()).trim();

  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN", ignoreHTTPSErrors: true,
    extraHTTPHeaders: { "X-Forwarded-For": "127.0.0.1", "X-ListingKit-Client-IP": "127.0.0.1" } });
  const page = await context.newPage();
  const email = `referral.${manifest.runId.slice(0, 8)}@example.test`;
  let admissionRequest;
  let admissionPayload;
  let admittedIntentID;
  const admissionAttempts = [];
  const registrationRequests = { start: 0, resume: 0 };
  const resumeStatuses = [];
  page.on("request", request => {
    const requestPath = new URL(request.url()).pathname;
    if (request.method() !== "POST") return;
    if (requestPath === "/api/referral-registration") {
      registrationRequests.start++;
      admissionRequest = { body: request.postData(), key: request.headers()["idempotency-key"] };
      admissionAttempts.push(admissionRequest);
    } else if (requestPath === "/api/referral-registration/resume") {
      registrationRequests.resume++;
    }
  });
  page.on("response", response => {
    if (response.request().method() === "POST" && new URL(response.url()).pathname === "/api/referral-registration/resume") resumeStatuses.push(response.status());
  });
  await check("registration_ui_desktop_and_automated_accessibility", async () => {
    try {
      await page.goto(`${origins.publicOrigin}/referrals/register?code=${encodeURIComponent(code)}`);
      await page.getByRole("heading", { name: "接受好友邀请" }).waitFor({ state: "visible", timeout: 30_000 });
      await page.screenshot({ path: path.join(outputDirectory, "registration-empty-desktop.png"), fullPage: true });
      const desktopAxe = await assertNoSeriousA11y(page);
      await page.getByLabel("邮箱").fill(email);
      await page.getByLabel("名字").fill("Referral");
      await page.getByLabel("姓氏").fill("Acceptance");
      const submitButton = page.getByRole("button", { name: "开始注册" });
      await waitForReactHydration(page, submitButton);
      let lostAdmission;
      const responseLost = new Promise((resolve, reject) => {
        page.route("**/api/referral-registration", async route => {
          try {
            const upstream = await route.fetch();
            lostAdmission = { status: upstream.status(), payload: await upstream.json() };
            await route.abort("connectionreset");
            resolve();
          } catch { reject(new Error("ADMISSION_RESPONSE_LOSS_INJECTION_FAILED")); }
        }, { times: 1 }).catch(reject);
      });
      await submitButton.click();
      await responseLost;
      ensure(lostAdmission?.status === 200, "ADMISSION_LOST_RESPONSE_NOT_COMMITTED");
      await page.getByText("暂时无法确认注册结果").waitFor({ state: "visible", timeout: 15_000 });
      ensure(await page.getByLabel("邮箱").isDisabled() || await page.getByLabel("邮箱").getAttribute("readonly") !== null, "ADMISSION_INPUT_NOT_LOCKED");
      const submitted = page.waitForResponse(response => new URL(response.url()).pathname === "/api/referral-registration" && response.request().method() === "POST", { timeout: 30_000 });
      await page.getByRole("button", { name: "重试原请求" }).click();
      const response = await submitted.catch(() => { throw new Error("REGISTRATION_RESPONSE_MISSING"); });
      if (response.status() !== 200) {
        const payload = await response.json().catch(() => ({}));
        const responseCode = typeof payload.code === "string" ? payload.code.toUpperCase().replace(/[^A-Z0-9_]/g, "_").slice(0, 80) : "UNKNOWN";
        throw new Error(`REGISTRATION_HTTP_${response.status()}_${responseCode}`);
      }
      admissionPayload = await response.json().catch(() => ({}));
      ensure(typeof admissionPayload.intentID === "string" && /^[A-Za-z0-9._:-]{1,200}$/.test(admissionPayload.intentID), "REGISTRATION_INTENT_MISSING");
      ensure(JSON.stringify(admissionPayload) === JSON.stringify(lostAdmission.payload), "ADMISSION_REPLAY_CHANGED_RECEIPT");
      ensure(admissionAttempts.length === 2 && admissionAttempts[0].key === admissionAttempts[1].key && admissionAttempts[0].body === admissionAttempts[1].body, "ADMISSION_RETRY_CHANGED_REQUEST");
      admittedIntentID = admissionPayload.intentID;
      let recoveryResponseStatus;
      const outcome = await Promise.race([
        page.getByRole("heading", { name: "请查看官方验证邮件" }).waitFor({ state: "visible", timeout: 45_000 }).then(() => "created"),
        page.getByRole("heading", { name: "继续原注册" }).waitFor({ state: "visible", timeout: 45_000 }).then(() => "recovery"),
      ]);
      if (outcome === "recovery") {
        await delay(16_000);
        const recovered = page.waitForResponse(candidate => new URL(candidate.url()).pathname === "/api/referral-registration/resume" && candidate.request().method() === "POST", { timeout: 30_000 });
        await page.getByRole("button", { name: "恢复原注册" }).click();
        recoveryResponseStatus = (await recovered).status();
        ensure(recoveryResponseStatus === 200, `REGISTRATION_RECOVERY_HTTP_${recoveryResponseStatus}`);
      }
      await page.getByRole("heading", { name: "请查看官方验证邮件" }).waitFor({ state: "visible", timeout: 30_000 })
        .catch(() => { throw new Error("REGISTRATION_MAIL_PENDING_MISSING"); });
      ensure(resumeStatuses.includes(200), "REGISTRATION_RESUME_SUCCESS_NOT_OBSERVED");
      await page.screenshot({ path: path.join(outputDirectory, "registration-mail-pending-desktop.png"), fullPage: true });
      ensure(admissionRequest?.body && /^[A-Za-z0-9_-]{43,128}$/.test(admissionRequest.key), "ADMISSION_REQUEST_NOT_OBSERVED");
      report.matrix.push({ group: "A", name: "admission_response_loss_same_request_and_receipt", status: "PASS", originalIntentPreserved: true, originalSubjectPreserved: true, originalSecretPreserved: true });
      report.matrix.push({ group: "F", name: "registration_desktop_axe", status: "PASS", axe: desktopAxe });
      return { viewport: "1440x1000", automated: "PASS", ...desktopAxe, lostResponseStatus: lostAdmission.status, responseStatus: response.status(), resumeStatuses, ...(recoveryResponseStatus ? { recoveryResponseStatus } : {}) };
    } catch (error) {
      await page.screenshot({ path: path.join(outputDirectory, "registration-stage-failure.png"), fullPage: true }).catch(() => {});
      throw error;
    }
  });
  await check("registration_creation_state", async () => {
    ensure(registrationRequests.start === 2 && registrationRequests.resume >= 1 && registrationRequests.resume <= 3, "REGISTRATION_REQUEST_COUNT_INVALID");
    const state = await readCreationState(admittedIntentID, email, ports.mail, machine.token);
    ensure(state.intentState === "CREATED" && state.subjectExists && state.proofMetadataPresent, "REGISTRATION_INTENT_NOT_CREATED");
    return { startRequestCount: registrationRequests.start, resumeRequestCount: registrationRequests.resume, ...state };
  });
  await check("trusted_proxy_overwrite_and_idempotent_replay", async () => {
    const direct = await fetch(`http://127.0.0.1:${ports.next}/api/referral-registration`, {
      method: "POST", headers: { Origin: origins.publicOrigin, "Sec-Fetch-Site": "same-origin", "Content-Type": "application/json", "Idempotency-Key": admissionRequest.key }, body: admissionRequest.body,
    });
    ensure(direct.status === 403, "DIRECT_NEXT_DID_NOT_FAIL_CLOSED");
    const replay = await context.request.post(`${origins.publicOrigin}/api/referral-registration`, {
      data: JSON.parse(admissionRequest.body),
      headers: { Origin: origins.publicOrigin, "Sec-Fetch-Site": "same-origin", "Idempotency-Key": admissionRequest.key },
      maxRedirects: 0,
    });
    if (replay.status() !== 200) {
      const payload = await replay.json().catch(() => ({}));
      const responseCode = typeof payload.code === "string" ? payload.code.toUpperCase().replace(/[^A-Z0-9_]/g, "_").slice(0, 80) : "UNKNOWN";
      throw new Error(`IDEMPOTENT_REPLAY_HTTP_${replay.status()}_${responseCode}`);
    }
    const replayPayload = await replay.json();
    ensure(JSON.stringify(replayPayload) === JSON.stringify(admissionPayload), "IDEMPOTENT_REPLAY_CHANGED_RECEIPT");
    const changed = await context.request.post(`${origins.publicOrigin}/api/referral-registration`, {
      data: { ...JSON.parse(admissionRequest.body), givenName: "Changed" },
      headers: { Origin: origins.publicOrigin, "Sec-Fetch-Site": "same-origin", "Idempotency-Key": admissionRequest.key },
      maxRedirects: 0,
    });
    ensure(changed.status() === 409, "IDEMPOTENCY_PAYLOAD_CONFLICT_NOT_REJECTED");
    report.matrix.push({ group: "A", name: "same_key_original_receipt_and_different_payload_conflict", status: "PASS", replayStatus: 200, conflictStatus: 409 });
    return { untrustedDirectStatus: 403, spoofedLoopbackIgnored: true, upstreamSource: "non-loopback Docker gateway" };
  });
  await check("registration_ui_narrow_and_keyboard", async () => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload();
    await page.getByRole("heading", { name: "继续原注册" }).waitFor({ state: "visible", timeout: 20_000 });
    const recovered = page.waitForResponse(response => new URL(response.url()).pathname === "/api/referral-registration/resume" && response.request().method() === "POST", { timeout: 30_000 });
    await page.getByRole("button", { name: "恢复原注册" }).click();
    ensure((await recovered).status() === 200, "RELOAD_RECOVERY_FAILED");
    await page.getByRole("heading", { name: "请查看官方验证邮件" }).waitFor({ state: "visible", timeout: 20_000 });
    await page.keyboard.press("Tab");
    ensure(await page.evaluate(() => document.activeElement !== document.body), "KEYBOARD_FOCUS_MISSING");
    const axe = await assertNoSeriousA11y(page);
    await page.screenshot({ path: path.join(outputDirectory, "registration-mail-pending-narrow.png"), fullPage: true });
    report.matrix.push({ group: "A", name: "reload_with_recovery_fragment_resumes_original", status: "PASS" });
    report.matrix.push({ group: "F", name: "registration_narrow_keyboard_axe", status: "PASS", axe });
    return { viewport: "390x844", automated: "PASS", ...axe };
  });
  await matrixCheck("A", "fresh_browser_without_recovery_cannot_switch_identity", async () => {
    const fresh = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", ignoreHTTPSErrors: true });
    const freshPage = await fresh.newPage();
    let writes = 0;
    freshPage.on("request", request => { if (request.method() === "POST" && new URL(request.url()).pathname.startsWith("/api/referral-registration")) writes++; });
    await freshPage.goto(`${origins.publicOrigin}/referrals/register?code=${encodeURIComponent(code)}`);
    await freshPage.getByRole("button", { name: "开始注册" }).waitFor({ state: "visible" });
    await freshPage.reload();
    await freshPage.getByRole("button", { name: "开始注册" }).waitFor({ state: "visible" });
    ensure(writes === 0, "FRESH_BROWSER_SILENTLY_RECOVERED");
    await fresh.close();
    return { writes: 0, newIdentityCreated: false };
  });
  await matrixCheck("B", "existing_account_cannot_be_bound_to_new_intent", async () => {
    const key = randomBytes(32).toString("hex");
    const start = await context.request.post(`${origins.publicOrigin}/api/referral-registration`, {
      data: { code, email: referrer.username, givenName: "Existing", familyName: "Account" },
      headers: { Origin: origins.publicOrigin, "Sec-Fetch-Site": "same-origin", "Idempotency-Key": key },
    });
    ensure(start.status() === 200, "EXISTING_ACCOUNT_ADMISSION_FAILED");
    const admission = await start.json();
    ensure(admission.intentID && admission.resumeSecret, "EXISTING_ACCOUNT_ADMISSION_INVALID");
    const resume = await context.request.post(`${origins.publicOrigin}/api/referral-registration/resume`, {
      data: { intentID: admission.intentID, resumeSecret: admission.resumeSecret },
      headers: { Origin: origins.publicOrigin, "Sec-Fetch-Site": "same-origin" },
    });
    ensure(resume.status() !== 200, "EXISTING_ACCOUNT_WAS_BOUND");
    const state = await readCreationState(admission.intentID, referrer.username, ports.mail, machine.token);
    ensure(!state.subjectExists && state.intentState !== "CONSUMED", "EXISTING_ACCOUNT_CHANGED_SUBJECT");
    return { rejectedStatus: resume.status(), fixedSubjectNotCreated: true, existingSubjectUnchanged: true };
  });

  const message = await check("official_mail_delivery", () => waitForMessage(ports.mail, email, origins.providerOrigin), false);
  const verification = new URL(message.link);
  ensure(verification.origin === origins.providerOrigin, "VERIFICATION_ORIGIN_MISMATCH");
  const subject = verification.searchParams.get("userId");
  ensure(subject, "VERIFICATION_SUBJECT_MISSING");
  report.createdSubject = subject;
  await matrixCheck("B", "new_browser_invalid_verification_does_not_verify", async () => {
    const invalidContext = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", ignoreHTTPSErrors: true });
    const invalidPage = await invalidContext.newPage();
    const invalid = new URL(message.link);
    invalid.searchParams.set("code", "invalid-verification-check");
    await invalidPage.goto(invalid.href, { waitUntil: "load" });
    const input = invalidPage.getByTestId("code-text-input");
    await input.waitFor({ state: "visible", timeout: 30_000 });
    await input.fill("invalid-verification-check");
    await invalidPage.getByTestId("submit-button").click();
    await delay(1_000);
    const current = await provider(`/v2/users/${encodeURIComponent(subject)}`, undefined, machine.token, "GET");
    ensure(current.user?.human?.email?.isVerified !== true, "INVALID_CHECK_VERIFIED_EMAIL");
    await invalidContext.close();
    return { separateBrowser: true, verified: false };
  });
  await check("official_email_verification", async () => {
    await page.goto(message.link, { waitUntil: "load" });
    const code = verification.searchParams.get("code");
    ensure(code, "VERIFICATION_CODE_MISSING");
    const codeInput = page.getByTestId("code-text-input");
    await codeInput.waitFor({ state: "visible", timeout: 30_000 }).catch(() => { throw new Error("OFFICIAL_VERIFICATION_INPUT_MISSING"); });
    await codeInput.fill(code);
    ensure(await codeInput.inputValue() === code, "OFFICIAL_VERIFICATION_CODE_MISMATCH");
    const submit = page.getByTestId("submit-button");
    await submit.waitFor({ state: "visible", timeout: 30_000 }).catch(() => { throw new Error("OFFICIAL_VERIFICATION_ACTION_MISSING"); });
    const submitElement = await submit.elementHandle();
    ensure(submitElement, "OFFICIAL_VERIFICATION_ACTION_MISSING");
    await page.waitForFunction(button => !button.disabled, submitElement, { timeout: 30_000 })
      .catch(() => { throw new Error("OFFICIAL_VERIFICATION_ACTION_DISABLED"); });
    await submit.click();
    try {
      await until(async () => {
        const user = await provider(`/v2/users/${encodeURIComponent(subject)}`, undefined, machine.token, "GET");
        return user.user?.human?.email?.isVerified === true;
      }, "EMAIL_VERIFIED", 45_000);
    } catch (error) {
      await writeJSON(path.join(outputDirectory, "verification-page-diagnostic.json"), {
        pathname: new URL(page.url()).pathname,
        errorVisible: await page.getByTestId("error").isVisible().catch(() => false),
        submitDisabled: await submit.isDisabled().catch(() => true),
        codeLength: code.length,
        codeShapeValid: /^[A-Za-z0-9_-]{1,64}$/.test(code),
        subjectMatchesIntent: subject === verification.searchParams.get("userId"),
        organizationMatchesSignup: verification.searchParams.get("organization") === manifest.organizations.A.id,
      });
      for (const [container, file] of [[`${manifest.project}-zitadel-login`, "verification-login-diagnostic.log"], [`${manifest.project}-zitadel-api`, "verification-provider-diagnostic.log"]]) {
        try {
          const output = await execFile("docker", ["--host", dockerHost, "logs", "--tail", "300", container], { windowsHide: true, timeout: 5_000, maxBuffer: 1024 * 1024 });
          await writePrivate(path.join(outputDirectory, file), `${output.stdout}\n${output.stderr}`.slice(-256 * 1024));
        } catch {}
      }
      throw error;
    }
    await page.screenshot({ path: path.join(outputDirectory, "official-email-verified.png"), fullPage: true });
    return { sameSubject: true, officialProvider: true };
  });

  await matrixCheck("B", "verified_without_authenticator_cannot_complete", async () => {
    const probe = await context.newPage();
    await probe.goto(`${origins.publicOrigin}/workbench/account/referrals/complete`, { waitUntil: "load" });
    await delay(1_000);
    const authorized = new URL(probe.url()).origin === origins.publicOrigin && await probe.getByRole("button", { name: "完成推广关系" }).isVisible().catch(() => false);
    await probe.close();
    ensure(!authorized, "VERIFIED_WITHOUT_AUTHENTICATOR_AUTHORIZED");
    return { authorized: false };
  });

  let registeredPassword;
  await check("official_authenticator_setup", async () => {
    await page.waitForURL(url => url.origin === origins.providerOrigin && url.pathname === "/ui/v2/login/authenticator/set", { timeout: 45_000 })
      .catch(() => { throw new Error("BLOCKER_NO_OFFICIAL_AUTHENTICATOR_SETUP"); });
    const passwordChoice = page.getByRole("link", { name: "Password" });
    await passwordChoice.waitFor({ state: "visible", timeout: 20_000 }).catch(() => { throw new Error("BLOCKER_NO_OFFICIAL_PASSWORD_CHOICE"); });
    await passwordChoice.click();
    await page.waitForURL(url => url.origin === origins.providerOrigin && url.pathname === "/ui/v2/login/password/set", { timeout: 30_000 })
      .catch(() => { throw new Error("BLOCKER_OFFICIAL_PASSWORD_SETUP_MISSING"); });
    const password = page.getByTestId("password-set-text-input");
    const confirmation = page.getByTestId("password-set-confirm-text-input");
    await password.waitFor({ state: "visible", timeout: 20_000 }).catch(() => { throw new Error("BLOCKER_OFFICIAL_PASSWORD_SETUP_MISSING"); });
    registeredPassword = `A9!${randomBytes(18).toString("hex")}`;
    await password.fill(registeredPassword);
    await confirmation.fill(registeredPassword);
    const submit = page.getByTestId("submit-button");
    const submitElement = await submit.elementHandle();
    ensure(submitElement, "BLOCKER_OFFICIAL_PASSWORD_ACTION_MISSING");
    await page.waitForFunction(button => !button.disabled, submitElement, { timeout: 30_000 })
      .catch(() => { throw new Error("BLOCKER_OFFICIAL_PASSWORD_ACTION_DISABLED"); });
    await submit.click();
    await page.waitForURL(url => url.pathname !== "/ui/v2/login/password/set", { timeout: 45_000 })
      .catch(() => { throw new Error("BLOCKER_OFFICIAL_PASSWORD_NOT_SET"); });
    return { subject, method: "password" };
  });

  await check("generic_oidc_authjs_login", async () => {
    ensure(typeof registeredPassword === "string" && registeredPassword.length >= 20, "OFFICIAL_PASSWORD_NOT_RETAINED");
    await context.clearCookies();
    await page.goto(`${origins.publicOrigin}/login?returnTo=${encodeURIComponent("/workbench/account/referrals/complete")}`, { waitUntil: "load" });
    const username = page.getByTestId("username-text-input");
    await username.waitFor({ state: "visible", timeout: 45_000 }).catch(() => { throw new Error("OFFICIAL_NEW_USERNAME_PAGE_MISSING"); });
    await username.fill(email);
    await page.getByTestId("submit-button").click();
    const password = page.getByTestId("password-text-input");
    await password.waitFor({ state: "visible", timeout: 30_000 }).catch(() => { throw new Error("OFFICIAL_NEW_PASSWORD_PAGE_MISSING"); });
    await password.fill(registeredPassword);
    await page.getByTestId("submit-button").click();
    await page.waitForURL(url => url.origin === origins.publicOrigin && url.pathname === "/workbench/account/referrals/complete", { timeout: 45_000 })
      .catch(() => { throw new Error("BLOCKER_GENERIC_OIDC_NOT_AUTHORIZED"); });
    return { subject };
  });

  await matrixCheck("B", "another_subject_cannot_claim_intent", async () => {
    const response = await referrerContext.request.post(`${origins.publicOrigin}/api/account/referrals/complete`, {
      headers: { Origin: origins.publicOrigin, "Sec-Fetch-Site": "same-origin", "X-Expected-User-ID": manifest.users.viewer.id },
    });
    ensure(response.status() === 404, "OTHER_SUBJECT_CLAIM_NOT_REJECTED");
    return { rejectedStatus: 404 };
  });

  let completionReceipt;
  await check("same_subject_completion", async () => {
    const button = page.getByRole("button", { name: "完成推广关系" });
    await button.waitFor({ state: "visible", timeout: 30_000 }).catch(() => { throw new Error("COMPLETION_ACTION_MISSING"); });
    await waitForReactHydration(page, button);
    let completionRequestCount = 0;
    const refreshStatuses = [];
    const observe = response => {
      const url = new URL(response.url());
      if (url.pathname === "/api/account/referrals/complete" && response.request().method() === "POST") completionRequestCount++;
      if (url.pathname === "/api/account/referrals" && response.request().method() === "GET") refreshStatuses.push(response.status());
    };
    page.on("response", observe);
    try {
      let lostReceipt;
      const completionLost = new Promise((resolve, reject) => {
        page.route("**/api/account/referrals/complete", async route => {
          try {
            const upstream = await route.fetch();
            lostReceipt = { status: upstream.status(), payload: await upstream.json() };
            await route.abort("connectionreset");
            resolve();
          } catch { reject(new Error("COMPLETION_RESPONSE_LOSS_INJECTION_FAILED")); }
        }, { times: 1 }).catch(reject);
      });
      await button.click();
      await completionLost;
      ensure(lostReceipt?.status === 200, "COMPLETION_LOST_RESPONSE_NOT_COMMITTED");
      await page.getByText("推广服务暂不可用").waitFor({ state: "visible", timeout: 15_000 });
      await restartConfiguredApplications(ports);
      await page.reload({ waitUntil: "load" });
      await button.waitFor({ state: "visible", timeout: 45_000 });
      await waitForReactHydration(page, button);
      await page.route("**/api/account/referrals", route => route.fulfill({
        status: 503,
        contentType: "application/json",
        body: JSON.stringify({ code: "DEPENDENCY_UNAVAILABLE", message: "Referral request could not be completed", requestId: "", fieldErrors: [] }),
      }), { times: 1 });
      for (let attempt = 1; attempt <= 2; attempt++) {
        const completed = page.waitForResponse(response => new URL(response.url()).pathname === "/api/account/referrals/complete" && response.request().method() === "POST", { timeout: 30_000 }).catch(() => null);
        await button.click();
        const response = await completed;
        ensure(response, "COMPLETION_RESPONSE_MISSING");
        if (response.status() !== 200) {
          const payload = await response.json().catch(() => ({}));
          const responseCode = typeof payload.code === "string" ? payload.code.toUpperCase().replace(/[^A-Z0-9_]/g, "_").slice(0, 80) : "UNKNOWN";
          throw new Error(`COMPLETION_HTTP_${response.status()}_${responseCode}`);
        }
        completionReceipt = await response.json();
        ensure(JSON.stringify(completionReceipt) === JSON.stringify(lostReceipt.payload), "COMPLETION_REPLAY_CHANGED_RECEIPT");
        const confirmed = await page.getByText("推广关系已确认").waitFor({ state: "visible", timeout: 5_000 }).then(() => true).catch(() => false);
        if (confirmed) break;
        if (attempt === 1) {
          await button.waitFor({ state: "visible", timeout: 20_000 }).catch(() => { throw new Error("COMPLETION_RECOVERY_ACTION_MISSING"); });
          const recoveryElement = await button.elementHandle();
          ensure(recoveryElement, "COMPLETION_RECOVERY_ACTION_MISSING");
          await page.waitForFunction(element => !element.disabled, recoveryElement, { timeout: 20_000 })
            .catch(() => { throw new Error("COMPLETION_RECOVERY_ACTION_DISABLED"); });
        }
      }
      await page.getByText("推广关系已确认").waitFor({ state: "visible", timeout: 30_000 });
      await page.getByText("推广汇总暂不可用").waitFor({ state: "visible", timeout: 15_000 });
      ensure(refreshStatuses.includes(503), "COMPLETION_PROJECTION_FAILURE_NOT_OBSERVED");
      await page.screenshot({ path: path.join(outputDirectory, "completion-receipt-with-projection-failure.png"), fullPage: true });
      report.matrix.push({ group: "C", name: "completion_response_loss_restart_receipt_replay", status: "PASS", realApplicationRestart: true, originalReceiptPreserved: true });
      report.matrix.push({ group: "C", name: "projection_failure_preserves_completion_receipt", status: "PASS", projectionStatus: 503 });
      await page.reload({ waitUntil: "load" });
      await button.waitFor({ state: "visible", timeout: 30_000 });
      const durableReplay = page.waitForResponse(response => new URL(response.url()).pathname === "/api/account/referrals/complete" && response.request().method() === "POST", { timeout: 30_000 });
      await button.click();
      ensure((await durableReplay).status() === 200, "DURABLE_RECEIPT_REPLAY_FAILED");
      await page.getByText("推广关系已确认").waitFor({ state: "visible", timeout: 30_000 });
      const desktopAxe = await assertNoSeriousA11y(page);
      await page.screenshot({ path: path.join(outputDirectory, "referrals-complete-desktop.png"), fullPage: true });
      await page.setViewportSize({ width: 390, height: 844 });
      await page.keyboard.press("Tab");
      ensure(await page.evaluate(() => document.activeElement !== document.body), "COMPLETION_KEYBOARD_FOCUS_MISSING");
      const narrowAxe = await assertNoSeriousA11y(page);
      await page.screenshot({ path: path.join(outputDirectory, "referrals-complete-narrow.png"), fullPage: true });
      report.matrix.push({ group: "F", name: "completion_desktop_narrow_keyboard_axe", status: "PASS", desktop: desktopAxe, narrow: narrowAxe });
    } catch (error) {
      const knownErrors = ["登录已失效", "登录身份已变化", "官方邮箱或认证方式尚未完成", "注册确认期限已结束", "推广关系存在冲突", "暂时无法确认操作结果", "推广服务尚未配置", "推广服务暂不可用", "推广请求超时", "推广请求未完成"];
      let displayedError = "none";
      for (const candidate of knownErrors) {
        if (await page.getByText(candidate, { exact: true }).isVisible().catch(() => false)) {
          displayedError = candidate;
          break;
        }
      }
      await writeJSON(path.join(outputDirectory, "completion-page-diagnostic.json"), {
        pathname: new URL(page.url()).pathname,
        loadingVisible: await page.getByText("正在读取推广事实").isVisible().catch(() => false),
        identityErrorVisible: await page.getByText("登录身份已变化").isVisible().catch(() => false),
        completionButtonVisible: await button.isVisible().catch(() => false),
        displayedError,
        completionRequestCount,
        refreshStatuses,
      });
      throw error;
    } finally {
      page.off("response", observe);
    }
    return { subject, completionRequestCount, refreshStatuses, realApplicationRestart: true };
  });
  await matrixCheck("C", "concurrent_completion_replays_one_durable_receipt", async () => {
    const request = () => context.request.post(`${origins.publicOrigin}/api/account/referrals/complete`, {
      headers: { Origin: origins.publicOrigin, "Sec-Fetch-Site": "same-origin", "X-Expected-User-ID": subject },
    });
    const responses = await Promise.all([request(), request(), request(), request()]);
    ensure(responses.every(response => response.status() === 200), "CONCURRENT_COMPLETION_FAILED");
    const receipts = await Promise.all(responses.map(response => response.json()));
    ensure(receipts.every(receipt => JSON.stringify(receipt) === JSON.stringify(completionReceipt)), "CONCURRENT_COMPLETION_CHANGED_RECEIPT");
    const facts = await run("docker", ["--host", dockerHost, "exec", `${manifest.project}-commercial-db`, "psql", "-At", "-U", "issue357", "-d", "issue357", "-c", "SELECT (SELECT count(*) FROM public.referral_relations) || '|' || (SELECT count(*) FROM public.referral_receipts) || '|' || (SELECT count(*) FROM public.registration_intents WHERE state='CONSUMED' AND ciphertext IS NULL)"]);
    ensure(facts === "1|1|1", "CONCURRENT_COMPLETION_FACT_COUNT_INVALID");
    return { requests: 4, relations: 1, receipts: 1, consumedIntents: 1 };
  });
  await check("referrer_real_count", async () => {
    await referrerPage.reload();
    await referrerPage.getByText("已建立关系").waitFor({ state: "visible" });
    ensure((await referrerPage.locator("article").filter({ hasText: "已建立关系" }).locator("strong").textContent()) === "1", "REFERRER_COUNT_NOT_ONE");
    return { count: 1 };
  });
  await matrixCheck("D", "authenticated_get_is_pure_on_referral_tables", async () => {
    const before = await referralTablesDigest();
    const response = await referrerContext.request.get(`${origins.publicOrigin}/api/account/referrals`, {
      headers: { "X-Expected-User-ID": manifest.users.viewer.id },
    });
    ensure(response.status() === 200, "SELF_READ_FAILED");
    ensure(await referralTablesDigest() === before, "SELF_READ_MUTATED_REFERRAL_TABLES");
    return { status: 200, referralTablesChanged: false };
  });
  await matrixCheck("D", "admin_reads_only_own_personal_projection", async () => {
    const admin = await readJSON(path.join(manifest.directory, "admin.credentials.json"));
    const adminContext = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN", ignoreHTTPSErrors: true });
    const adminPage = await adminContext.newPage();
    await login(adminPage, origins.publicOrigin, admin, "/workbench/account/referrals");
    await adminPage.getByText("已建立关系").waitFor({ state: "visible", timeout: 30_000 });
    ensure((await adminPage.locator("article").filter({ hasText: "已建立关系" }).locator("strong").textContent()) === "0", "ADMIN_READ_OTHER_RELATION");
    await adminContext.close();
    return { targetRelationVisible: false, ownCount: 0 };
  });
  await matrixCheck("D", "no_enterprise_user_can_read_own_empty_projection", async () => {
    const credential = await readJSON(path.join(manifest.directory, "no-org.credentials.json"));
    const isolated = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", ignoreHTTPSErrors: true });
    const isolatedPage = await isolated.newPage();
    await login(isolatedPage, origins.publicOrigin, credential, "/workbench/account/referrals");
    await isolatedPage.getByText("已建立关系").waitFor({ state: "visible", timeout: 30_000 });
    ensure((await isolatedPage.locator("article").filter({ hasText: "已建立关系" }).locator("strong").textContent()) === "0", "NO_ENTERPRISE_SELF_READ_INVALID");
    await isolated.close();
    return { ownCount: 0 };
  });
  await matrixCheck("E", "bff_and_go_reject_untrusted_credentials_and_csrf", async () => {
    const csrf = await context.request.post(`${origins.publicOrigin}/api/referral-registration`, { data: JSON.parse(admissionRequest.body), headers: { "Idempotency-Key": admissionRequest.key } });
    const goURL = `http://127.0.0.1:${ports.go}/api/v1/referral-registration/intents`;
    const direct = headers => fetch(goURL, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": admissionRequest.key, ...headers }, body: admissionRequest.body });
    const [missing, wrong, userToken] = await Promise.all([direct({}), direct({ "X-Referral-Service-Credential": "0".repeat(64) }), direct({ "X-Referral-Service-Credential": machine.token })]);
    ensure(csrf.status() === 403 && [missing, wrong, userToken].every(response => response.status === 401 || response.status === 403), "SERVICE_BOUNDARY_DID_NOT_FAIL_CLOSED");
    return { csrfStatus: 403, directCredentialFailures: [missing.status, wrong.status, userToken.status] };
  });
  await matrixCheck("E", "real_source_ips_and_cross_process_rate_limit", async () => {
    const network = `${manifest.project}-network`;
    for (const suffix of ["a", "b"]) {
      const name = `${manifest.project}-referral-source-${suffix}`;
      ownedClientNames.push(name);
      await docker(["run", "-d", "--name", name, "--label", `${ownerLabel}=${manifest.runId}`, "--network", network, caddyImage, "sh", "-c", "sleep 600"]);
    }
    const status = async (name, sequence) => {
      const body = JSON.stringify({ code, email: `rate.${sequence}.${manifest.runId.slice(0, 8)}@example.test`, givenName: "Rate", familyName: "Limit" });
      const key = createHash("sha256").update(`${manifest.runId}:${sequence}`).digest("hex");
      const shell = `wget -S -O /dev/null --header='Origin: ${origins.publicOrigin}' --header='Sec-Fetch-Site: same-origin' --header='Content-Type: application/json' --header='Idempotency-Key: ${key}' --post-data='${body}' http://${caddyName}/api/referral-registration 2>&1 | awk '/  HTTP\\// {s=$2} END {print s}'`;
      return Number(await docker(["exec", name, "sh", "-c", shell]));
    };
    if (new Date().getUTCSeconds() > 45) await delay((61 - new Date().getUTCSeconds()) * 1_000);
    const first = [];
    for (let index = 1; index <= 5; index++) first.push(await status(ownedClientNames[0], index));
    ensure(first.every(value => value === 200), "SOURCE_RATE_LIMIT_PRECONDITION_FAILED");
    await restartConfiguredApplications(ports);
    const limited = await status(ownedClientNames[0], 6);
    const independent = await status(ownedClientNames[1], 7);
    ensure(limited === 429 && independent === 200, "CROSS_PROCESS_SOURCE_RATE_LIMIT_FAILED");
    return { firstSourceStatuses: first, afterRestartSameSource: limited, secondSource: independent };
  });
  await matrixCheck("E", "missing_dependency_disables_invite_entry", async () => {
    const secret = path.join(manifest.directory, "referral-service.secret");
    const disabled = `${secret}.disabled`;
    await rename(secret, disabled);
    try {
      await restartConfiguredApplications(ports);
      await referrerPage.reload({ waitUntil: "load" });
      await referrerPage.getByText("注册入口暂不可用").waitFor({ state: "visible", timeout: 30_000 });
      ensure(!(await referrerPage.getByRole("link", { name: "打开邀请链接" }).isVisible().catch(() => false)), "INVITE_LINK_ENABLED_WITHOUT_DEPENDENCY");
    } finally {
      await rename(disabled, secret).catch(() => {});
      await restartConfiguredApplications(ports);
    }
    return { inviteEnabled: false };
  });
  await referrerPage.setViewportSize({ width: 390, height: 844 });
  await referrerPage.keyboard.press("Tab");
  ensure(await referrerPage.evaluate(() => document.activeElement !== document.body), "OVERVIEW_KEYBOARD_FOCUS_MISSING");
  const overviewAxe = await assertNoSeriousA11y(referrerPage);
  await referrerPage.screenshot({ path: path.join(outputDirectory, "referrals-overview-narrow.png"), fullPage: true });
  report.matrix.push({ group: "F", name: "overview_desktop_narrow_keyboard_axe", status: "PASS", narrow: overviewAxe });
  await matrixCheck("D", "logout_removes_personal_projection", async () => {
    await referrerPage.goto(`${origins.publicOrigin}/api/zitadel-auth/logout`, { waitUntil: "load" });
    await referrerPage.goto(`${origins.publicOrigin}/workbench/account/referrals`, { waitUntil: "load" });
    await referrerPage.getByTestId("username-text-input").waitFor({ state: "visible", timeout: 45_000 });
    ensure(!(await referrerPage.getByText("已建立关系").isVisible().catch(() => false)), "LOGOUT_LEFT_REFERRAL_DATA_VISIBLE");
    return { referralDataVisible: false };
  });
  matrixNotRun("D", "enterprise_removed_switching_expiry_late_response", "CURRENT_RUN_HAS_NO_SAFE_ENTERPRISE_OR_TOKEN_TIME_CONTROL");
  matrixNotRun("E", "cancel_deadline_zero_late_dispatch", "CURRENT_PRODUCT_HAS_NO_TASK_OWNED_DISPATCH_OBSERVER");
  matrixNotRun("F", "screen_reader", "REAL_SCREEN_READER_NOT_EXECUTED");
  report.manualAccessibility = "NOT_RUN";
  if (report.matrix.some(item => item.status === "FAIL" || (item.status === "NOT_RUN" && item.group !== "F"))) throw new Error("MATRIX_MUST_INCOMPLETE");
  await context.close();
  await referrerContext.close();
}

async function cleanupOwnedContainer(name) {
  if (!name || !manifest) return;
  const inspected = await inspectDockerResource("container", name);
  if (!inspected) return;
  ensure(inspected.Config?.Labels?.[ownerLabel] === manifest.runId, "CLEANUP_OWNERSHIP_MISMATCH");
  await docker(["container", "rm", "-f", inspected.Id]);
}

async function cleanupBaseResource(record, owner) {
  const resource = await inspectDockerResource(record.kind, record.name);
  if (!resource) return;
  const labels = record.kind === "container" ? resource.Config?.Labels : resource.Labels;
  const id = resource.Id ?? resource.ID ?? resource.Name;
  ensure(labels?.[runtimeOwnerLabel] === owner.runId && id === record.id, "CLEANUP_OWNERSHIP_MISMATCH");
  await docker([record.kind, "rm", ...(record.kind === "container" ? ["-f"] : []), record.id]);
}

export function ownedResourceCleanupActions(records, cleanupResource) {
  return Object.values(records ?? {}).map(record => ({ name: `base-${record.kind}-${record.name}`, run: () => cleanupResource(record) }));
}

async function cleanup(machine, bootstrap, phase) {
  if (manifest) {
    try {
      const current = await readJSON(path.join(manifest.directory, "manifest.json"));
      ensure(current.runId === manifest.runId, "RUNTIME_ID_MISMATCH");
      manifest = { ...current, directory: manifest.directory };
    } catch { runtimeOwnershipUnknown = true; }
  }
  const actions = [
    { name: "browser", run: async () => { if (browser) { await browser.close(); browser = null; } } },
    { name: "created-subject", run: async () => {
      if (manifest && bootstrap && report.createdSubject && !createdSubjectDeleted) {
        await provider(`/v2/users/${encodeURIComponent(report.createdSubject)}`, undefined, bootstrap, "DELETE");
        createdSubjectDeleted = true;
      }
    } },
    { name: "machine-subject", run: async () => {
      if (manifest && bootstrap && machine?.machineId && !machineDeleted) {
        await provider(`/v2/users/${encodeURIComponent(machine.machineId)}`, undefined, bootstrap, "DELETE");
        machineDeleted = true;
      }
    } },
    { name: "caddy", run: async () => cleanupOwnedContainer(caddyName) },
    { name: "mailpit", run: async () => cleanupOwnedContainer(mailName) },
    ...ownedClientNames.map(name => ({ name: `source-${name}`, run: async () => cleanupOwnedContainer(name) })),
    { name: "base-runtime", run: async () => {
      if (runtimeOwnershipUnknown || report.runtimeDispatched && !manifest) throw new Error("RUNTIME_OWNERSHIP_UNKNOWN");
      if (!manifest) return;
      const current = await readJSON(path.join(manifest.directory, "manifest.json"));
      ensure(current.runId === manifest.runId, "RUNTIME_ID_MISMATCH");
      if (current.status !== "destroyed") await run(process.execPath, [runtimeScript, "destroy", "--run", manifest.runId]);
    } },
    { name: "refresh-base-inventory", run: async () => {
      if (runtimeOwnershipUnknown || report.runtimeDispatched && !manifest) throw new Error("RUNTIME_OWNERSHIP_UNKNOWN");
      if (!manifest) return;
      const latest = await readJSON(path.join(manifest.directory, "manifest.json"));
      ensure(latest.runId === manifest.runId, "RUNTIME_ID_MISMATCH");
      manifest = { ...latest, directory: manifest.directory };
      const insertion = actions.findIndex(action => action.name === "private-artifacts");
      actions.splice(insertion, 0, ...ownedResourceCleanupActions(manifest.resources, record => cleanupBaseResource(record, manifest)));
    } },
    { name: "private-artifacts", run: async () => { if (manifest) await cleanupPrivateArtifacts(manifest); } },
  ];
  return runCleanupPass({ phase, actions, inspectResiduals });
}

async function inspectDockerResource(kind, name) {
  try {
    return JSON.parse(await docker([kind, "inspect", name]))[0];
  } catch (error) {
    await docker(["info", "--format", "{{.ServerVersion}}"]);
    if (/No such|not found/i.test(error.privateOutput ?? "")) return null;
    throw new Error("QUERY_FAILED:DOCKER");
  }
}

async function inspectResiduals() {
  if (runtimeOwnershipUnknown || report.runtimeDispatched && !manifest) throw new Error("RUNTIME_OWNERSHIP_UNKNOWN");
  if (!manifest) return { containers: 0, volumes: 0, networks: 0, listeners: 0 };
  const latest = await readJSON(path.join(manifest.directory, "manifest.json"));
  ensure(latest.runId === manifest.runId, "RUNTIME_ID_MISMATCH");
  manifest = { ...latest, directory: manifest.directory };
  let containers = 0;
  let volumes = 0;
  let networks = 0;
  for (const name of [caddyName, mailName].filter(Boolean)) {
    const resource = await inspectDockerResource("container", name);
    if (!resource) continue;
    ensure(resource.Config?.Labels?.[ownerLabel] === manifest.runId, "CLEANUP_OWNERSHIP_MISMATCH");
    containers++;
  }
  for (const record of Object.values(manifest.resources ?? {})) {
    const resource = await inspectDockerResource(record.kind, record.name);
    if (!resource) continue;
    const labels = record.kind === "container" ? resource.Config?.Labels : resource.Labels;
    const id = resource.Id ?? resource.ID ?? resource.Name;
    ensure(labels?.[runtimeOwnerLabel] === manifest.runId && id === record.id, "CLEANUP_OWNERSHIP_MISMATCH");
    if (record.kind === "container") containers++;
    else if (record.kind === "volume") volumes++;
    else if (record.kind === "network") networks++;
  }
  let listeners = 0;
  const ports = [...Object.values(manifest.ports ?? {}), ...Object.values(fixtureAllocatedPorts)];
  for (const port of new Set(ports)) {
    const available = await freePort(port).then(() => true).catch(error => {
      if (error?.code === "EADDRINUSE") return false;
      throw new Error("QUERY_FAILED:PORT");
    });
    if (!available) listeners++;
  }
  return { containers, volumes, networks, listeners };
}

async function cleanupPrivateArtifacts(owner) {
  const expected = path.resolve(tmpdir(), "task-processor-issue357", owner.runId);
  ensure(path.resolve(owner.directory) === expected, "CLEANUP_PATH_INVALID");
  for (const name of ["referral-owner.dsn", "referral-provider.secret", "referral-service.secret", "referral-lookup.secret", "referral-proof.secret", "referral-encryption.secret"]) {
    await unlink(path.join(expected, name)).catch(error => { if (error.code !== "ENOENT") throw error; });
  }
  const caddyRoot = path.resolve(expected, "referral-caddy");
  ensure(path.dirname(caddyRoot) === expected, "CLEANUP_PATH_INVALID");
  await rm(caddyRoot, { recursive: true, force: true });
}

async function cleanupCommand(runId) {
  ensure(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(runId ?? ""), "INVALID_RUN_ID");
  const directory = path.resolve(tmpdir(), "task-processor-issue357", runId);
  const owner = await readJSON(path.join(directory, "manifest.json"));
  ensure(owner.runId === runId, "RUN_OWNERSHIP_MISMATCH");
  owner.directory = directory;
  await cleanupPrivateArtifacts(owner);
  console.log("PRIVATE_ARTIFACTS_REMOVED");
}

async function main() {
  let machine;
  let bootstrap;
  report.runnerSha256 = createHash("sha256").update(await readFile(fileURLToPath(import.meta.url))).digest("hex");
  const outcome = await orchestrateFixtureLifecycle({
    report,
    runBusiness: async () => {
      ensure(process.platform === "win32", "WINDOWS_REQUIRED");
      ensure((await run("git", ["status", "--porcelain"])) === "", "SOURCE_MUST_BE_CLEAN");
      await check("isolated_official_runtime_start", startBaseRuntime);
      bootstrap = (await readFile(path.join(manifest.directory, "bootstrap.pat"), "utf8")).trim();
      const ports = await fixturePorts();
      fixtureAllocatedPorts = ports;
      const caFile = await check("owned_mail_and_tls_proxy_start", () => startOwnedContainers(ports), false);
      machine = await check("org_scoped_provider_credential_and_smtp", () => configureProvider(bootstrap), false);
      const origins = await check("referral_runtime_configuration", () => configureApplications(ports, caFile, machine.token));
      await check("provider_tls_proxy_preflight", () => probeProviderProxy(origins.providerOrigin, caFile, machine.token));
      await check("configured_application_start", () => startConfiguredApplications(ports));
      await browserChain(origins, ports, machine);
    },
    runCleanup: phase => cleanup(machine, bootstrap, phase),
    persistReport: async value => {
      ensure(outputDirectory, "REPORT_DIRECTORY_UNAVAILABLE");
      await writeJSONAtomic(path.join(outputDirectory, "report.json"), value);
    },
    emitFailure: value => console.error(JSON.stringify(value)),
  });
  process.exitCode = outcome.exitCode;
  console.log(JSON.stringify({
    status: report.status,
    runId: report.runId,
    invocationId: report.invocationId,
    business: report.business,
    cleanup: report.cleanup,
    evidence: report.evidence,
    checks: report.checks.map(({ name, status, code }) => ({ name, status, ...(code ? { code } : {}) })),
  }));
}

const invokedAsScript = process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url);
if (invokedAsScript) {
  if (process.argv[2] === "lifecycle-test") await lifecycleTestCommand(process.argv[3]);
  else if (process.argv[2] === "cleanup-artifacts") await cleanupCommand(process.argv[3]);
  else await main();
}
