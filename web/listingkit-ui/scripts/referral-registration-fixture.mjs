import assert from "node:assert/strict";
import { execFile as execFileCallback, spawn } from "node:child_process";
import { randomBytes } from "node:crypto";
import { readFile, writeFile, mkdir, unlink, rm } from "node:fs/promises";
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
const caddyImage = "caddy:2.11.4-alpine";
const mailImage = "axllent/mailpit:v1.30.4";
const report = {
  schemaVersion: "issue413-referral-registration-fixture-v1",
  status: "NOT_RUN",
  checks: [],
  manualAccessibility: "NOT_RUN",
  startedAt: new Date().toISOString(),
};
let manifest;
let browser;
let outputDirectory;
let caddyName;
let mailName;

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

function safeCode(error) {
  const raw = error instanceof Error ? error.message : String(error);
  return /^[A-Z0-9_:-]{1,160}$/.test(raw) ? raw : "FIXTURE_STEP_FAILED";
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
  } catch {
    throw new Error(`PROCESS_FAILED:${path.basename(command).replace(/[^A-Za-z0-9_.-]/g, "_")}`);
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
  const stdout = await run(process.execPath, [runtimeScript, "start", "--current-application", "--web-dir", uiRoot]);
  const runId = /runId=([0-9a-f-]{36})/.exec(stdout)?.[1];
  ensure(runId, "RUNTIME_ID_MISSING");
  const root = path.join(tmpdir(), "task-processor-issue357", runId);
  manifest = await readJSON(path.join(root, "manifest.json"));
  manifest.directory = root;
  outputDirectory = path.join(root, "referral-registration-evidence");
  await mkdir(outputDirectory, { recursive: true });
  report.runId = runId;
  report.sourceSha = manifest.sourceSha;
  report.webSha = manifest.webSha;
  try {
    await run(process.execPath, [runtimeScript, "stop", "--run", runId]);
  } catch {
    await run(process.execPath, [runtimeScript, "stop", "--run", runId]);
  }
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
  reverse_proxy http://proxy:80 {
    header_up Host localhost:${manifest.ports.issuer}
    header_up X-Forwarded-Proto http
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

async function waitForMessage(mailPort, email) {
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
      const link = /https:\/\/127\.0\.0\.1:\d+\/ui\/v2\/login\/verify\?[^\s<"']+/i.exec(content)?.[0];
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
  if (serious.length) {
    await writeJSON(path.join(outputDirectory, "a11y.json"), serious.map(item => ({ id: item.id, impact: item.impact, targets: item.nodes.map(node => node.target) })));
    const rules = serious.map(item => item.id.toUpperCase().replace(/[^A-Z0-9_]/g, "_")).sort().join("_").slice(0, 120);
    throw new Error(`ACCESSIBILITY_${rules}`);
  }
  return { violations: 0 };
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
  ensure(!extra && ["PREPARED", "CREATED", "CONSUMED"].includes(intentState) && /^[A-Za-z0-9._:-]{1,200}$/.test(subject ?? "") && ["t", "f"].includes(leaseActiveText), "REGISTRATION_INTENT_STATE_INVALID");
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
  return { intentState, leaseActive: leaseActiveText === "t", subjectExists, proofMetadataPresent, metadataCount, matchingMessageCount };
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
      report.referrerSubject = manifest.users.viewer.id;
      return { codeCreated: true };
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
  let admittedIntentID;
  const registrationRequests = { start: 0, resume: 0 };
  page.on("request", request => {
    const requestPath = new URL(request.url()).pathname;
    if (request.method() !== "POST") return;
    if (requestPath === "/api/referral-registration") {
      registrationRequests.start++;
      admissionRequest = { body: request.postData(), key: request.headers()["idempotency-key"] };
    } else if (requestPath === "/api/referral-registration/resume") {
      registrationRequests.resume++;
    }
  });
  await check("registration_ui_desktop_and_automated_accessibility", async () => {
    try {
      await page.goto(`${origins.publicOrigin}/referrals/register?code=${encodeURIComponent(code)}`);
      await page.getByRole("heading", { name: "接受好友邀请" }).waitFor({ state: "visible", timeout: 30_000 });
      await page.screenshot({ path: path.join(outputDirectory, "registration-empty-desktop.png"), fullPage: true });
      await assertNoSeriousA11y(page);
      await page.getByLabel("邮箱").fill(email);
      await page.getByLabel("名字").fill("Referral");
      await page.getByLabel("姓氏").fill("Acceptance");
      const submitButton = page.getByRole("button", { name: "开始注册" });
      await waitForReactHydration(page, submitButton);
      const submitted = page.waitForResponse(response => new URL(response.url()).pathname === "/api/referral-registration" && response.request().method() === "POST", { timeout: 30_000 });
      const resumed = page.waitForResponse(response => new URL(response.url()).pathname === "/api/referral-registration/resume" && response.request().method() === "POST", { timeout: 30_000 });
      await submitButton.click();
      const response = await submitted.catch(() => { throw new Error("REGISTRATION_RESPONSE_MISSING"); });
      if (response.status() !== 200) {
        const payload = await response.json().catch(() => ({}));
        const responseCode = typeof payload.code === "string" ? payload.code.toUpperCase().replace(/[^A-Z0-9_]/g, "_").slice(0, 80) : "UNKNOWN";
        throw new Error(`REGISTRATION_HTTP_${response.status()}_${responseCode}`);
      }
      const admissionPayload = await response.json().catch(() => ({}));
      ensure(typeof admissionPayload.intentID === "string" && /^[A-Za-z0-9._:-]{1,200}$/.test(admissionPayload.intentID), "REGISTRATION_INTENT_MISSING");
      admittedIntentID = admissionPayload.intentID;
      const resumeResponse = await resumed.catch(() => { throw new Error("REGISTRATION_RESUME_RESPONSE_MISSING"); });
      let recoveryResponseStatus;
      if (resumeResponse.status() !== 200) {
        const payload = await resumeResponse.json().catch(() => ({}));
        if (resumeResponse.status() === 503 && payload.code === "referral_outcome_unknown") {
          await page.getByRole("heading", { name: "继续原注册" }).waitFor({ state: "visible", timeout: 15_000 });
          ensure(!(await page.getByRole("heading", { name: "请查看官方验证邮件" }).isVisible().catch(() => false)), "UNKNOWN_SHOWED_MAIL_STATE");
          ensure(!(await page.getByRole("button", { name: /开始注册|重试原请求/ }).isVisible().catch(() => false)), "UNKNOWN_RESTARTED_REGISTRATION");
          await delay(16_000);
          const recovered = page.waitForResponse(candidate => new URL(candidate.url()).pathname === "/api/referral-registration/resume" && candidate.request().method() === "POST", { timeout: 30_000 });
          await page.getByRole("button", { name: "恢复原注册" }).click();
          const recoveryResponse = await recovered.catch(() => { throw new Error("REGISTRATION_RECOVERY_RESPONSE_MISSING"); });
          recoveryResponseStatus = recoveryResponse.status();
          if (recoveryResponseStatus !== 200) {
            const recoveryPayload = await recoveryResponse.json().catch(() => ({}));
            await writeJSON(path.join(outputDirectory, "resume-failure-diagnostic.json"), await readCreationState(admittedIntentID, email, ports.mail, machine.token));
            try {
              const output = await execFile("docker", ["--host", dockerHost, "logs", "--tail", "200", caddyName], { windowsHide: true, timeout: 5_000, maxBuffer: 512 * 1024 });
              await writePrivate(path.join(outputDirectory, "provider-access-diagnostic.log"), `${output.stdout}\n${output.stderr}`.slice(-128 * 1024));
            } catch {}
            const recoveryCode = typeof recoveryPayload.code === "string" ? recoveryPayload.code.toUpperCase().replace(/[^A-Z0-9_]/g, "_").slice(0, 80) : "UNKNOWN";
            throw new Error(`REGISTRATION_RECOVERY_HTTP_${recoveryResponseStatus}_${recoveryCode}`);
          }
        } else {
          const responseCode = typeof payload.code === "string" ? payload.code.toUpperCase().replace(/[^A-Z0-9_]/g, "_").slice(0, 80) : "UNKNOWN";
          throw new Error(`REGISTRATION_RESUME_HTTP_${resumeResponse.status()}_${responseCode}`);
        }
      }
      await page.getByRole("heading", { name: "请查看官方验证邮件" }).waitFor({ state: "visible", timeout: 15_000 })
        .catch(() => { throw new Error("REGISTRATION_MAIL_PENDING_MISSING"); });
      await page.screenshot({ path: path.join(outputDirectory, "registration-mail-pending-desktop.png"), fullPage: true });
      ensure(admissionRequest?.body && /^[A-Za-z0-9_-]{43,128}$/.test(admissionRequest.key), "ADMISSION_REQUEST_NOT_OBSERVED");
      return { viewport: "1440x1000", automated: "PASS", responseStatus: response.status(), resumeResponseStatus: resumeResponse.status(), ...(recoveryResponseStatus ? { recoveryResponseStatus } : {}) };
    } catch (error) {
      await page.screenshot({ path: path.join(outputDirectory, "registration-stage-failure.png"), fullPage: true }).catch(() => {});
      throw error;
    }
  });
  await check("registration_creation_state", async () => {
    ensure(registrationRequests.start === 1 && registrationRequests.resume >= 1 && registrationRequests.resume <= 2, "REGISTRATION_REQUEST_COUNT_INVALID");
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
    return { untrustedDirectStatus: 403, spoofedLoopbackIgnored: true, upstreamSource: "non-loopback Docker gateway" };
  });
  await check("registration_ui_narrow_and_keyboard", async () => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload();
    await page.keyboard.press("Tab");
    ensure(await page.evaluate(() => document.activeElement !== document.body), "KEYBOARD_FOCUS_MISSING");
    await assertNoSeriousA11y(page);
    await page.screenshot({ path: path.join(outputDirectory, "registration-mail-pending-narrow.png"), fullPage: true });
    return { viewport: "390x844", automated: "PASS" };
  });

  const message = await check("official_mail_delivery", () => waitForMessage(ports.mail, email), false);
  const verification = new URL(message.link);
  ensure(verification.origin === origins.providerOrigin, "VERIFICATION_ORIGIN_MISMATCH");
  const subject = verification.searchParams.get("userId");
  ensure(subject, "VERIFICATION_SUBJECT_MISSING");
  report.createdSubject = subject;
  await check("official_email_verification", async () => {
    await page.goto(message.link, { waitUntil: "load" });
    const submit = page.getByTestId("submit-button");
    if (await submit.isVisible().catch(() => false)) await submit.click();
    await until(async () => {
      const user = await provider(`/v2/users/${encodeURIComponent(subject)}`, undefined, machine.token, "GET");
      return user.user?.human?.email?.isVerified === true;
    }, "EMAIL_VERIFIED", 45_000);
    await page.screenshot({ path: path.join(outputDirectory, "official-email-verified.png"), fullPage: true });
    return { sameSubject: true, officialProvider: true };
  });

  await check("official_authenticator_and_generic_oidc", async () => {
    await page.goto(`${origins.publicOrigin}/login?returnTo=${encodeURIComponent("/workbench/account/referrals/complete")}`, { waitUntil: "load" });
    const username = page.getByTestId("username-text-input");
    await username.waitFor({ state: "visible", timeout: 45_000 });
    await username.fill(email);
    await page.getByTestId("submit-button").click();
    const passwordFields = page.locator('input[type="password"]');
    await passwordFields.first().waitFor({ state: "visible", timeout: 20_000 }).catch(() => {});
    const count = await passwordFields.count();
    ensure(count > 0, "BLOCKER_NO_OFFICIAL_AUTHENTICATOR_SETUP");
    const password = `A9!${randomBytes(18).toString("hex")}`;
    for (let index = 0; index < count; index++) await passwordFields.nth(index).fill(password);
    await page.getByTestId("submit-button").click();
    await page.waitForURL(url => url.origin === origins.publicOrigin && url.pathname === "/workbench/account/referrals/complete", { timeout: 45_000 })
      .catch(() => { throw new Error("BLOCKER_GENERIC_OIDC_NOT_AUTHORIZED"); });
    return { subject };
  });

  await check("same_subject_completion", async () => {
    await page.getByRole("button", { name: "完成推广关系" }).click();
    await page.getByText("推广关系已确认").waitFor({ state: "visible", timeout: 30_000 });
    return { subject };
  });
  await check("referrer_real_count", async () => {
    await referrerPage.reload();
    await referrerPage.getByText("已建立关系").waitFor({ state: "visible" });
    ensure((await referrerPage.locator("article").filter({ hasText: "已建立关系" }).locator("strong").textContent()) === "1", "REFERRER_COUNT_NOT_ONE");
    return { count: 1 };
  });
  await context.close();
  await referrerContext.close();
}

async function cleanupOwnedContainer(name) {
  if (!name || !manifest) return;
  const raw = await docker(["container", "inspect", name]).catch(() => "");
  if (!raw) return;
  const inspected = JSON.parse(raw)[0];
  ensure(inspected.Config?.Labels?.[ownerLabel] === manifest.runId, "CLEANUP_OWNERSHIP_MISMATCH");
  await docker(["container", "rm", "-f", inspected.Id]);
}

async function cleanup(machine, bootstrap) {
  if (browser) await browser.close().catch(() => {});
  if (manifest && bootstrap && report.createdSubject) {
    await provider(`/v2/users/${encodeURIComponent(report.createdSubject)}`, undefined, bootstrap, "DELETE").catch(() => {});
  }
  if (manifest && bootstrap && machine?.machineId) {
    await provider(`/v2/users/${encodeURIComponent(machine.machineId)}`, undefined, bootstrap, "DELETE").catch(() => {});
  }
  await cleanupOwnedContainer(caddyName).catch(() => {});
  await cleanupOwnedContainer(mailName).catch(() => {});
  if (manifest) await run(process.execPath, [runtimeScript, "destroy", "--run", manifest.runId]).catch(() => {});
  if (manifest) await cleanupPrivateArtifacts(manifest);
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
  try {
    ensure(process.platform === "win32", "WINDOWS_REQUIRED");
    ensure((await run("git", ["status", "--porcelain"])) === "", "SOURCE_MUST_BE_CLEAN");
    await check("isolated_official_runtime_start", startBaseRuntime);
    bootstrap = (await readFile(path.join(manifest.directory, "bootstrap.pat"), "utf8")).trim();
    const ports = await fixturePorts();
    const caFile = await check("owned_mail_and_tls_proxy_start", () => startOwnedContainers(ports), false);
    machine = await check("org_scoped_provider_credential_and_smtp", () => configureProvider(bootstrap), false);
    const origins = await check("referral_runtime_configuration", () => configureApplications(ports, caFile, machine.token));
    await check("provider_tls_proxy_preflight", () => probeProviderProxy(origins.providerOrigin, caFile, machine.token));
    await check("configured_application_start", () => startConfiguredApplications(ports));
    await browserChain(origins, ports, machine);
    report.status = "PASS";
  } catch (error) {
    report.status = "FAIL";
    report.failure = safeCode(error);
    process.exitCode = 1;
  } finally {
    report.finishedAt = new Date().toISOString();
    if (outputDirectory) await writeJSON(path.join(outputDirectory, "report.json"), report).catch(() => {});
    await cleanup(machine, bootstrap);
    console.log(JSON.stringify({ status: report.status, failure: report.failure, runId: report.runId, checks: report.checks.map(({ name, status, code }) => ({ name, status, ...(code ? { code } : {}) })) }));
  }
}

if (process.argv[2] === "cleanup-artifacts") await cleanupCommand(process.argv[3]);
else await main();
