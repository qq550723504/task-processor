import { execFileSync, execFile as execFileCallback } from "node:child_process";
import { readFile, realpath, mkdir, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { setTimeout as delay } from "node:timers/promises";
import { promisify } from "node:util";
import { validateBrowserHandoff, publicBrowserOrigins, browserExpectedHeaders, assertBrowserDiagnosticsDisabled, classifyLateResponseDelivery, classifyRevocationRead, classifyUnavailableLogin, classifyUnavailableProviderTarget, isFinalApplicationLanding, retryOwnerHealth } from "./real-provider-browser-contract.mjs";
import { browserSignalOwnershipOptions, createOwnerMutationGuard, createRunFinalizer, ownerControlExecOptions, platformSignalMatrix } from "./real-provider-browser-lifecycle.mjs";

// No default server, inherited Playwright config, authentication fixtures, traces,
// HAR, video, retries or raw exception output. Only #357 starts/stops the runtime.
const uiRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const [manifestPath, runtimeSha, outputArgument, runtimeCheckout, cleanupFlag] = process.argv.slice(2);
const execFile = promisify(execFileCallback);
const report = { suite: "LOCAL_REAL_PROVIDER_ACCEPTANCE", status: "NOT_RUN", checks: [], screenshots: [], requests: [] };
const profilePath = "/workbench/account/profile";
const organizationPath = "/workbench/account/organization";
const entitlementPath = "/workbench/plans/entitlements";
const commercialPath = "/api/workbench/commercial/overview";
const selectionCookie = "shuomi_effective_organization";
let manifest, output, browser, activeCase = "handoff";
let controlsReady = false;
let finalizer;
let controlQueue = Promise.resolve();
let ownedStopPromise;
const finalizationFailures = [];
const interruptController = new AbortController();
const pageObservations = new WeakMap();
const ownerMutationStates = new Map();
const ownerMutations = createOwnerMutationGuard({
  interrupted: () => finalizer?.interrupted === true,
  onState: state => ownerMutationStates.set(state.key, state),
});
const ensure = condition => { if (!condition) throw new Error("assertion_failed"); };
const pause = milliseconds => delay(milliseconds, undefined, { signal: interruptController.signal });

async function check(name, operation) {
  finalizer?.throwIfInterrupted();
  activeCase = name;
  const started = Date.now();
  try {
    await operation();
    report.checks.push({ name, status: "PASS", elapsedMs: Date.now() - started });
    console.log(`PASS ${name}`);
  } catch {
    report.checks.push({ name, status: "FAIL", elapsedMs: Date.now() - started });
    // Playwright error messages/call logs may contain a filled password or code.
    throw new Error("case_failed");
  }
}

async function json(response) {
  const bytes = await response.body();
  ensure(bytes.length <= 65536);
  return JSON.parse(bytes.toString("utf8"));
}
function urlOf(raw) { return new URL(raw, manifest.origins.web); }
async function api(context, pathname, user, organization) {
  const headers = browserExpectedHeaders(manifest, user, organization);
  // This request client shares only cookies naturally issued in this same context.
  const response = await context.request.get(`${manifest.origins.web}${pathname}`, { headers, maxRedirects: 0, timeout: 20000 });
  const body = await json(response);
  return { status: response.status(), body };
}
async function apiStatus(context, pathname, user, organization) {
  const response = await context.request.get(`${manifest.origins.web}${pathname}`, { headers: browserExpectedHeaders(manifest, user, organization), maxRedirects: 0, timeout: 20000 });
  return response.status();
}
function hasToken(value) {
  if (!value || typeof value !== "object") return false;
  return Object.entries(value).some(([key, item]) => /^(?:access_?token|refresh_?token|id_?token)$/i.test(key) || hasToken(item));
}
async function session(context, user) {
  const result = await api(context, "/api/auth/session");
  ensure(result.status === 200 && !hasToken(result.body));
  ensure(result.body.identity?.userId === manifest.users[user].id && !result.body.error);
  ensure(result.body.identity?.tenantId === manifest.organizations.A.id);
  ensure(Number.isFinite(result.body.expiresAt));
  return result.body;
}
async function textContains(page, value) {
  await page.getByText(value, { exact: false }).first().waitFor({ state: "visible", timeout: 30000 });
}
async function screenshot(page, name) {
  ensure(urlOf(page.url()).origin === manifest.origins.web);
  ensure(await page.locator('input[type="password"]').count() === 0);
  await page.screenshot({ path: path.join(output, `${name}.png`), fullPage: true });
  report.screenshots.push(`${name}.png`);
}
async function credentials(user) {
  const source = await realpath(manifest.users[user].credentialFile);
  const root = await realpath(path.dirname(manifestPath));
  const relative = path.relative(root, source);
  ensure(relative && !relative.startsWith("..") && !path.isAbsolute(relative));
  const raw = await readFile(source);
  ensure(raw.length < 8192);
  const value = JSON.parse(raw.toString("utf8"));
  ensure(typeof value.username === "string" && value.username.length > 0);
  ensure(typeof value.password === "string" && value.password.length > 0);
  return value;
}

function observe(page) {
  const previous = pageObservations.get(page);
  if (previous) { previous.clear(); return previous; }
  const seen = new Set();
  pageObservations.set(page, seen);
  const endpoints = new Set(["/login", "/api/zitadel-auth/login", "/api/auth/callback/zitadel", "/api/zitadel-auth/logout", "/oauth/v2/authorize", "/oidc/v1/end_session", "/ui/v2/login/loginname", "/ui/v2/login/password", "/api/account/profile", "/api/account/organization", commercialPath, "/api/workbench/context", "/api/workbench/context/effective-organization"]);
  page.on("request", request => {
    const url = new URL(request.url());
    if (url.origin === manifest.origins.issuer && url.pathname.startsWith("/ui/v2/login")) seen.add("official-login-v2");
    if (url.origin === manifest.origins.issuer && url.pathname === "/oauth/v2/authorize") {
      report.protocol = { codeFlow: url.searchParams.get("response_type") === "code", pkceS256: url.searchParams.get("code_challenge_method") === "S256", statePresent: url.searchParams.has("state"), noncePresent: url.searchParams.has("nonce") };
    }
    if (endpoints.has(url.pathname) && [manifest.origins.web, manifest.origins.issuer].includes(url.origin)) seen.add(url.pathname);
  });
  page.on("response", response => {
    const url = new URL(response.url());
    if (endpoints.has(url.pathname) && [manifest.origins.web, manifest.origins.issuer].includes(url.origin)) {
      report.requests.push({ service: url.origin === manifest.origins.web ? "next" : "provider", path: url.pathname, method: response.request().method(), status: response.status() });
    }
  });
  return seen;
}

async function login(page, context, user, target = profilePath, bare = false) {
  const seen = observe(page);
  const value = await credentials(user);
  report.loginStage = "official_page_load";
  await page.goto(`${manifest.origins.web}${bare ? `/login?returnTo=${encodeURIComponent(target)}` : target}`, { waitUntil: "load" });
  const username = page.getByTestId("username-text-input");
  await username.waitFor({ state: "visible", timeout: 45000 });
  ensure(urlOf(page.url()).origin === manifest.origins.issuer && seen.has("official-login-v2"));
  if (!report.screenshots.includes("official-login-empty.png")) {
    ensure(await username.inputValue() === "" && await page.locator('input[type="password"]').count() === 0);
    await page.screenshot({ path: path.join(output, "official-login-empty.png"), fullPage: true });
    report.screenshots.push("official-login-empty.png");
  }
  // Selectors belong to the pinned official Login V2; confirm visible form first.
  report.loginStage = "username_input";
  await username.fill(value.username);
  report.loginStage = "username_submit";
  await page.getByTestId("submit-button").click();
  report.loginStage = "password_form";
  const password = page.getByTestId("password-text-input");
  await password.waitFor({ state: "visible", timeout: 30000 });
  ensure(urlOf(page.url()).origin === manifest.origins.issuer);
  report.loginStage = "password_input";
  await password.fill(value.password);
  report.loginStage = "password_submit";
  await page.getByTestId("submit-button").click();
  report.loginStage = "application_callback";
  await page.waitForURL(url => url.origin === manifest.origins.web && url.pathname === target, { timeout: 45000 });
  ensure(seen.has("/login") && seen.has("/api/zitadel-auth/login") && seen.has("/api/auth/callback/zitadel"));
  ensure(report.protocol?.codeFlow && report.protocol?.pkceS256);
  await session(context, user);
  report.loginStage = "public_session_verified";
  return seen;
}

async function select(page, context, key) {
  const organization = manifest.organizations[key];
  const switched = page.waitForResponse(response => new URL(response.url()).pathname === "/api/workbench/context/effective-organization" && response.request().method() === "PUT");
  await page.getByRole("combobox", { name: "当前企业" }).selectOption(organization.id);
  const response = await switched;
  ensure(response.status() === 200 && (await response.json()).effectiveOrganizationId === organization.id);
  await page.waitForFunction(id => document.querySelector('select')?.value === id && !document.querySelector('select')?.disabled, organization.id);
  const result = await api(context, "/api/workbench/context");
  ensure(result.status === 200 && result.body.effectiveOrganizationId === organization.id);
  ensure(result.body.homeOrganizationId === manifest.organizations.A.id);
}

async function logout(page, context, user) {
  const seen = observe(page);
  const landed = page.waitForResponse(response => isFinalApplicationLanding({ url: response.url(), status: response.status() }, manifest.origins.web) && response.request().method() === "GET", { timeout: 45000 });
  await page.locator("summary").filter({ hasText: "我的账户" }).click();
  await page.getByRole("link", { name: "退出登录", exact: true }).click();
  await landed;
  await page.waitForURL(url => url.origin === manifest.origins.web && url.pathname === "/", { timeout: 45000 });
  ensure(seen.has("/api/zitadel-auth/logout") && seen.has("/oidc/v1/end_session"));
  // Must be before any context/API call can hide an incomplete logout cleanup.
  const cookies = await context.cookies(manifest.origins.web);
  ensure(!cookies.some(cookie => cookie.name === selectionCookie || /(?:authjs|next-auth)\.session-token/.test(cookie.name)));
  const current = await api(context, "/api/auth/session");
  ensure(current.status === 200 && !current.body?.identity && !current.body?.user && !hasToken(current.body));
  for (const [route, organization] of [["/api/account/profile", undefined], ["/api/account/organization", "B"], [commercialPath, "B"]]) {
    ensure((await api(context, route, user, organization)).status === 401);
  }
  await screenshot(page, `logout-${user}`);
}

async function holdRealResponse(page, pathname) {
  let release, received, finished;
  let inFlight = false;
  let delivered = false, originalRequest, cancellationError;
  const failed = request => {
    if (request === originalRequest) cancellationError = request.failure()?.errorText;
  };
  page.on("requestfailed", failed);
  const gate = new Promise(resolve => { release = resolve; });
  const started = new Promise(resolve => { received = resolve; });
  const done = new Promise(resolve => { finished = resolve; });
  const target = `${manifest.origins.web}${pathname}`;
  const handler = async route => {
    inFlight = true;
    originalRequest = route.request();
    try {
      // Every byte/status/header comes from the actual BFF. Only delivery waits.
      const response = await route.fetch({ maxRedirects: 0, timeout: 20000 });
      ensure(response.status() === 200);
      received(true);
      await gate;
      await route.fulfill({ response });
      delivered = true;
    } catch { received(false); }
    finally { finished(); }
  };
  await page.route(target, handler, { times: 1 });
  return {
    ready: async () => ensure(await Promise.race([started, delay(22000, false, { ref: false })])),
    release: async () => {
      release();
      try {
        await page.unroute(target, handler);
        if (inFlight) {
          await done;
          const outcome = classifyLateResponseDelivery(delivered, cancellationError);
          (report.lateResponses ??= []).push({ path: pathname, outcome });
        }
      } finally { page.off("requestfailed", failed); }
    },
  };
}

async function lateSwitches(page, context) {
  for (const [name, pagePath, apiPath, button] of [["account", organizationPath, "/api/account/organization", "刷新资料"], ["commercial", entitlementPath, commercialPath, "刷新数据"]]) {
    await check(`M5_${name}_late_switch`, async () => {
      await select(page, context, "B");
      await page.goto(`${manifest.origins.web}${pagePath}`);
      await textContains(page, name === "account" ? `当前有效企业：${manifest.organizations.B.id}` : "实际订阅");
      const held = await holdRealResponse(page, apiPath);
      try {
        await page.getByRole("button", { name: button, exact: true }).click();
        await held.ready();
        await select(page, context, "C");
        await textContains(page, name === "account" ? `当前有效企业：${manifest.organizations.C.id}` : "实际订阅");
      } finally { await held.release(); }
      const content = await page.locator("#console-main").innerText();
      ensure(content.includes(manifest.organizations.C.id) && !content.includes(manifest.organizations.B.id));
      const current = await api(context, apiPath, "admin", "C");
      ensure(current.status === 200 && (current.body.effectiveOrganizationId ?? current.body.organization_id) === manifest.organizations.C.id);
    });
  }
}

async function realRefresh(context) {
  await check("M10_real_token_lifecycle", async () => {
    const initial = await session(context, "admin");
    const started = Date.now();
    // The #357 contract uses actual 120s tokens. Never modify exp or the clock.
    ensure(initial.expiresAt * 1000 - started <= 150000);
    let refreshed = false;
    while (Date.now() - started < 150000) {
      await pause(3000);
      finalizer?.throwIfInterrupted();
      const current = await session(context, "admin");
      if (current.expiresAt > initial.expiresAt) { refreshed = true; break; }
    }
    ensure(refreshed);
    ensure((await api(context, "/api/account/profile", "admin")).status === 200);
    report.tokenLifecycle = { status: "PASS", observedRenewal: true, elapsedMs: Date.now() - started, clock: "real" };
  });
}

async function control(command, user, organization) {
  finalizer?.throwIfInterrupted();
  return queuedControl(command, user, organization);
}

async function queuedControl(command, user, organization) {
  ensure(controlsReady);
  const operation = async () => {
    const args = [path.join(runtimeCheckout, "scripts/issue357-runtime.mjs"), command, "--run", manifest.runId];
    if (user) args.push("--user", user, "--org", organization);
    await execFile(process.execPath, args, { cwd: runtimeCheckout, ...ownerControlExecOptions() });
    (report.controls ??= []).push({ command, user, organization, at: new Date().toISOString(), status: "PASS" });
  };
  const pending = controlQueue.then(operation, operation);
  controlQueue = pending.catch(() => {});
  return pending;
}

async function ownerEvidence(kind) {
  const raw = await readFile(path.join(path.dirname(manifestPath), `${kind}.json`));
  ensure(raw.length < 262144);
  const value = JSON.parse(raw.toString("utf8"));
  ensure(value.schemaVersion === `issue357-${kind}-v1` && value.runId === manifest.runId && value.sourceSha === runtimeSha && value.webSha === report.sourceSha);
  const zero = value.zeroWrite;
  ensure(zero?.passed === true && typeof zero.before === "string" && /^[a-f0-9]{64}$/.test(zero.before) && zero.before === zero.after);
  const evidence = { runId: value.runId, sourceSha: value.sourceSha, webSha: value.webSha, zeroWrite: { passed: true, before: zero.before, after: zero.after } };
  if (kind === "check") ensure(value.healthPassed === true && value.realBrowser === "NOT_RUN_BY_CHECK");
  else {
    ensure(value.passed === true && value.resourcesReleased === true && value.portsReleased === true && value.applications?.passed === true && value.applications.goExit === 0 && value.applications.nextExit === 0);
    evidence.cleanup = { passed: true, resourcesReleased: true, portsReleased: true, applications: { passed: true, goExit: 0, nextExit: 0 } };
  }
  return evidence;
}

async function controlCases() {
  const contexts = [];
  try {
    for (let index = 0; index < 3; index++) {
      const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "en-US" });
      contexts.push(context);
      const page = await context.newPage();
      await login(page, context, "admin");
      await select(page, context, "B");
      // Leave Console to avoid background context reads clearing the selection.
      // API clients continue with their own naturally issued browser cookies.
      await page.goto(manifest.origins.web);
    }
    const [cached, live, afterWindow] = contexts;
    await check("M9_revocation", async () => {
      ensure((await api(cached, "/api/account/organization", "admin", "B")).status === 200);
      ensure((await api(live, commercialPath, "admin", "B")).status === 200);
      await ownerMutations.run({
        key: "grant-admin-B",
        recoveryCommand: `node scripts/issue357-runtime.mjs restore --run ${manifest.runId} --user admin --org B`,
        mutate: () => control("revoke", "admin", "B"),
        restore: () => queuedControl("restore", "admin", "B"),
      }, async () => {
        const confirmedAt = Date.now();
        const commercial = await api(live, commercialPath, "admin", "B");
        ensure(commercial.status === 403 && ["ORGANIZATION_ACCESS_REVOKED", "ORGANIZATION_ACCESS_DENIED"].includes(commercial.body.code));
        while (Date.now() - confirmedAt < 60000) {
          const requestStartedAt = Date.now();
          const organization = await api(cached, "/api/account/organization", "admin", "B");
          if (classifyRevocationRead({ status: organization.status, requestStartedAt, confirmedAt }) === "denied") {
            ensure(["ORGANIZATION_ACCESS_REVOKED", "ORGANIZATION_ACCESS_DENIED"].includes(organization.body.code));
            break;
          }
          await pause(2000);
          finalizer?.throwIfInterrupted();
        }
        // A separate normal-login context retains its original selection even
        // when the earlier denial clears the probing context's cookie.
        await pause(Math.max(0, confirmedAt + 60001 - Date.now()));
        finalizer?.throwIfInterrupted();
        const afterWindowStartedAt = Date.now();
        const expiredCache = await api(afterWindow, "/api/account/organization", "admin", "B");
        ensure(afterWindowStartedAt > confirmedAt + 60000 && expiredCache.status === 403 && ["ORGANIZATION_ACCESS_REVOKED", "ORGANIZATION_ACCESS_DENIED"].includes(expiredCache.body.code));
        report.revocation = { liveCommercial: "DENIED", cachedAccount: "DENIED", afterCacheWindow: "DENIED", elapsedMs: Date.now() - confirmedAt, policyMaxAgeSeconds: 60 };
        ensure((await api(cached, "/api/account/profile", "admin")).status === 200);
      });
      const restored = await live.request.put(`${manifest.origins.web}/api/workbench/context/effective-organization`, { data: { organizationId: manifest.organizations.B.id }, maxRedirects: 0 });
      ensure(restored.status() === 200 && (await api(live, commercialPath, "admin", "B")).status === 200);
    });
    await check("M8_provider_failure", async () => {
      await ownerMutations.run({
        key: "provider",
        recoveryCommand: `node scripts/issue357-runtime.mjs provider-start --run ${manifest.runId}`,
        mutate: () => control("provider-stop"),
        restore: async () => {
          await queuedControl("provider-start");
          await retryOwnerHealth(() => queuedControl("check"));
        },
      }, async () => {
        ensure([401, 502, 503, 504].includes(await apiStatus(cached, "/api/account/profile", "admin")));
        const empty = await browser.newContext();
        try {
          const response = await empty.request.get(`${manifest.origins.web}/api/zitadel-auth/login`, { timeout: 30000, maxRedirects: 0 });
          const outcome = classifyUnavailableLogin({ status: response.status(), location: response.headers().location, issuer: manifest.origins.issuer, web: manifest.origins.web });
          if (outcome.endsWith("-redirect")) {
            let unavailableStatus;
            try {
              const unavailable = await empty.request.get(new URL(response.headers().location, manifest.origins.web).toString(), { timeout: 10000, maxRedirects: 0 });
              unavailableStatus = unavailable.status();
            } catch { /* connection refusal is the expected unavailable-provider boundary */ }
            classifyUnavailableProviderTarget(unavailableStatus);
          }
          const result = await api(empty, "/api/auth/session");
          ensure(!result.body?.identity && !result.body?.user && !hasToken(result.body));
        } finally { await empty.close(); }
      });
      await ownerEvidence("check");
    });
  } finally { for (const context of contexts) await context.close(); }
}

async function entries() {
  const context = await browser.newContext();
  try {
    for (const method of ["otp", "password"]) {
      await check(`M7_${method}_503`, async () => {
        const response = await context.request.get(`${manifest.origins.web}/api/zitadel-auth/login?method=${method}`, { maxRedirects: 0 });
        ensure(response.status() === 503 && (await json(response)).error === "login_capability_unavailable");
      });
    }
    for (const [name, returnTo] of [["external", "https://invalid.example/workbench"], ["network_path", "//invalid.example/workbench"], ["backslash", "/\\invalid.example/workbench"], ["encoding", "/workbench/%"], ["unsupported", "/api/auth/session"]]) {
      await check(`M7_returnTo_${name}`, async () => {
        const response = await context.request.get(`${manifest.origins.web}/login?returnTo=${encodeURIComponent(returnTo)}`, { maxRedirects: 0 });
        ensure([302, 303, 307].includes(response.status()));
        const target = urlOf(response.headers().location);
        ensure(target.origin === manifest.origins.web && target.pathname === "/api/zitadel-auth/login" && target.searchParams.get("returnTo") === "/");
      });
    }
    for (const [name, query] of [["bare", ""], ["unknown", "method=unknown&"], ["repeated", "method=otp&method=password&"]]) {
      await check(`M7_${name}_generic`, async () => {
        const response = await context.request.get(`${manifest.origins.web}/login?${query}returnTo=${encodeURIComponent(profilePath)}`, { maxRedirects: 0 });
        const target = urlOf(response.headers().location);
        ensure(target.pathname === "/api/zitadel-auth/login" && target.searchParams.get("returnTo") === profilePath && !target.searchParams.has("method"));
        const authorization = await context.request.get(target.toString(), { maxRedirects: 0 });
        ensure([302, 303, 307].includes(authorization.status()));
        const provider = new URL(authorization.headers().location);
        ensure(provider.origin === manifest.origins.issuer && provider.pathname === "/oauth/v2/authorize");
      });
    }
  } finally { await context.close(); }
}

async function core() {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "en-US" });
  const page = await context.newPage();
  page.setDefaultTimeout(20000);
  try {
    await check("M1_real_password_code_session", async () => {
      ensure((await context.cookies()).length === 0);
      await login(page, context, "admin");
    });
    await check("M2_self_before_selection", async () => {
      const profile = await api(context, "/api/account/profile", "admin");
      ensure(profile.status === 200 && profile.body.userId === manifest.users.admin.id && profile.body.homeOrganizationId === manifest.organizations.A.id && profile.body.source === "zitadel_userinfo");
      await textContains(page, `账户 ID：${manifest.users.admin.id}`);
      const current = await api(context, "/api/workbench/context");
      ensure(current.body.selectionRequired === true && current.body.effectiveOrganizationId === null);
      await screenshot(page, "admin-profile-unselected");
    });
    const quantities = [];
    for (const key of ["B", "C", "Empty"]) {
      await check(`M3_organization_${key}`, async () => {
        await select(page, context, key);
        await page.goto(`${manifest.origins.web}${organizationPath}`);
        await textContains(page, `当前有效企业：${manifest.organizations[key].id}`);
        const organization = await api(context, "/api/account/organization", "admin", key);
        ensure(organization.status === 200 && organization.body.userId === manifest.users.admin.id && organization.body.homeOrganizationId === manifest.organizations.A.id && organization.body.effectiveOrganizationId === manifest.organizations[key].id);
        if (key === "B") await screenshot(page, "admin-organization-B");
        await page.goto(`${manifest.origins.web}${entitlementPath}`);
        await textContains(page, "实际订阅");
        const commercial = await api(context, commercialPath, "admin", key);
        ensure(commercial.status === 200 && commercial.body.organization_id === manifest.organizations[key].id);
        if (key === "Empty") {
          ensure(commercial.body.subscription === null && commercial.body.usage.every(row => row.state === "unknown" && row.committed === null));
          await textContains(page, "无订阅");
        } else {
          const quantity = commercial.body.usage.find(row => row.metric === "listingkit_generations_succeeded")?.committed;
          ensure(quantity === (key === "B" ? "1" : "2")); quantities.push(quantity);
        }
        await screenshot(page, `admin-entitlements-${key}`);
      });
    }
    ensure(quantities.length === 2 && quantities[0] !== quantities[1]);
    await check("M3_D_denied", async () => {
      const response = await context.request.put(`${manifest.origins.web}/api/workbench/context/effective-organization`, { data: { organizationId: manifest.organizations.D.id }, maxRedirects: 0 });
      ensure(response.status() === 403);
      const commercial = await api(context, commercialPath, "admin", "D");
      ensure(commercial.status === 409 && commercial.body.code === "ORGANIZATION_CONTEXT_CHANGED");
    });
    await lateSwitches(page, context);
    await realRefresh(context);
    await check("M6_logout_admin", async () => {
      await page.goto(`${manifest.origins.web}${profilePath}`);
      await textContains(page, `账户 ID：${manifest.users.admin.id}`);
      const held = await holdRealResponse(page, "/api/account/profile");
      try {
        await page.getByRole("button", { name: "刷新资料", exact: true }).click();
        await held.ready();
        await logout(page, context, "admin");
      } finally { await held.release(); }
      ensure(!(await page.locator("body").innerText()).includes(manifest.users.admin.id));
    });
    await check("M4_same_browser_viewer_password_login", async () => {
      await login(page, context, "viewer", profilePath, true);
      await textContains(page, `账户 ID：${manifest.users.viewer.id}`);
      ensure(!(await page.locator("body").innerText()).includes(manifest.users.admin.id));
      const current = await api(context, "/api/workbench/context");
      ensure(current.body.effectiveOrganizationId === null && current.body.selectionRequired);
      await select(page, context, "B");
      ensure((await api(context, "/api/account/profile", "viewer")).status === 200);
      ensure((await api(context, "/api/account/organization", "viewer", "B")).status === 200);
      await page.goto(`${manifest.origins.web}${entitlementPath}`);
      await textContains(page, "本次未取得数据");
      ensure((await api(context, commercialPath, "viewer", "B")).status === 403);
      ensure(!(await page.locator("body").innerText()).includes("实际套餐代码"));
      await screenshot(page, "viewer-commercial-denied");
    });
    await check("M6_logout_viewer", () => logout(page, context, "viewer"));
    await check("M2_no_org_self_and_sibling_gates", async () => {
      await login(page, context, "no-org", profilePath, true);
      await textContains(page, `账户 ID：${manifest.users["no-org"].id}`);
      ensure((await api(context, "/api/account/profile", "no-org")).status === 200);
      const current = await api(context, "/api/workbench/context");
      ensure(current.status === 200 && current.body.organizations.length === 0 && current.body.effectiveOrganizationId === null);
      await screenshot(page, "no-org-profile");
      for (const route of [organizationPath, entitlementPath, "/workbench/plans/options", "/workbench/stores"]) {
        await page.goto(`${manifest.origins.web}${route}`);
        await page.waitForURL(url => url.pathname === "/workbench/no-organization");
        ensure([403, 409].includes((await api(context, "/api/account/organization", "no-org", "B")).status));
        ensure([403, 409].includes((await api(context, commercialPath, "no-org", "B")).status));
      }
    });
    await page.goto(`${manifest.origins.web}${profilePath}`);
    await textContains(page, `账户 ID：${manifest.users["no-org"].id}`);
    await check("M6_logout_no_org", () => logout(page, context, "no-org"));
  } finally { await context.close(); }
}

function completeReport() {
  report.ownerRecovery = ownerMutations.snapshot();
  report.finalization = { failures: [...finalizationFailures] };
  if (cleanupFlag !== "--stop-owned-run" && !report.checks.some(item => item.name === "M11_owner_zero_write_cleanup")) {
    report.checks.push({ name: "M11_owner_zero_write_cleanup", status: "NOT_RUN" });
  }
  for (const prefix of ["M1_", "M2_", "M3_", "M4_", "M5_", "M6_", "M7_", "M8_", "M9_", "M10_"]) {
    if (!report.checks.some(item => item.name.startsWith(prefix))) report.checks.push({ name: `${prefix}prerequisite_not_reached`, status: "NOT_RUN" });
  }
  if (finalizationFailures.length > 0) {
    if (report.status !== "INTERRUPTED") report.status = "FAIL";
    process.exitCode = 1;
  } else if (report.status === "INCOMPLETE" && cleanupFlag === "--stop-owned-run") {
    report.status = "PASS";
    process.exitCode = 0;
  }
  report.finishedAt = new Date().toISOString();
  return writeFile(path.join(output, "report.json"), `${JSON.stringify(report, null, 2)}\n`).then(() => {
    console.log(`${report.status} ${report.suite}; case=${report.failure ?? activeCase}`);
  });
}

function stopOwnedRuntime() {
  if (!ownedStopPromise) ownedStopPromise = (async () => {
    const started = Date.now();
    try {
      await queuedControl("stop");
      report.cleanup = await ownerEvidence("cleanup");
      report.checks.push({ name: "M11_owner_zero_write_cleanup", status: "PASS", elapsedMs: Date.now() - started });
      console.log("PASS M11_owner_zero_write_cleanup");
    } catch {
      report.checks.push({ name: "M11_owner_zero_write_cleanup", status: "FAIL", elapsedMs: Date.now() - started });
      throw new Error("owned_run_stop_failed");
    }
  })();
  return ownedStopPromise;
}

try {
  assertBrowserDiagnosticsDisabled(process.env);
  ensure(manifestPath && runtimeSha && runtimeCheckout && (!cleanupFlag || cleanupFlag === "--stop-owned-run"));
  const webSha = execFileSync("git", ["rev-parse", "HEAD"], { cwd: uiRoot, encoding: "utf8", windowsHide: true }).trim();
  ensure(execFileSync("git", ["status", "--porcelain"], { cwd: uiRoot, encoding: "utf8", windowsHide: true }).trim() === "");
  ensure(execFileSync("git", ["rev-parse", "HEAD"], { cwd: runtimeCheckout, encoding: "utf8", windowsHide: true }).trim() === runtimeSha);
  ensure(execFileSync("git", ["status", "--porcelain"], { cwd: runtimeCheckout, encoding: "utf8", windowsHide: true }).trim() === "");
  const raw = await readFile(manifestPath);
  ensure(raw.length < 262144);
  manifest = validateBrowserHandoff(JSON.parse(raw.toString("utf8")), { manifestPath, runtimeSha, webSha, temporaryRoot: tmpdir() });
  // Realpath comparison also rejects a linked manifest outside this run.
  ensure(await realpath(manifestPath) === path.resolve(manifestPath));
  output = path.resolve(outputArgument || path.join(uiRoot, "../../.local/issue358-browser", manifest.runId));
  await mkdir(output, { recursive: true });
  Object.assign(report, {
    sourceSha: webSha,
    runtimeSha,
    runId: manifest.runId,
    origins: publicBrowserOrigins(manifest),
    signalSupport: platformSignalMatrix(process.platform),
    startedAt: new Date().toISOString(),
  });
  controlsReady = true;
  finalizer = createRunFinalizer({
    cleanupOwnedRun: cleanupFlag === "--stop-owned-run",
    stepTimeoutMs: 250000,
    closeBrowser: async () => {
      if (!browser) return;
      const activeBrowser = browser;
      browser = undefined;
      await activeBrowser.close();
    },
    recoverOwnerControls: async () => {
      await controlQueue;
      const result = await ownerMutations.recoverAll();
      report.ownerRecovery = ownerMutations.snapshot();
      if (!result.ok) throw new Error("owner_recovery_failed");
    },
    stopOwnedRun: stopOwnedRuntime,
    persistReport: completeReport,
    onInterrupt: signal => {
      interruptController.abort(new Error("runner_interrupted"));
      report.status = "INTERRUPTED";
      report.failure = `signal_${signal}`;
      report.interruption = {
        signal,
        receivedAt: new Date().toISOString(),
        cleanupRequested: cleanupFlag === "--stop-owned-run",
        ownerStatesAtSignal: [...ownerMutationStates.values()],
        ownedRunRecoveryCommand: `node scripts/issue357-runtime.mjs stop --run ${manifest.runId}`,
      };
      process.exitCode = 1;
    },
    onFailure: step => {
      finalizationFailures.push(step);
      if (report.interruption) report.interruption.failures = [...finalizationFailures];
    },
  });
  finalizer.installProcessHandlers();
  await check("M11_before_read_snapshot", async () => { await control("check"); report.beforeRead = await ownerEvidence("check"); });
  const { chromium } = await import("@playwright/test");
  browser = await chromium.launch({ headless: true, ...browserSignalOwnershipOptions() });
  await entries();
  await core();
  await controlCases();
  await check("M11_after_read_snapshot", async () => { await control("check"); report.afterRead = await ownerEvidence("check"); ensure(report.beforeRead.zeroWrite.after === report.afterRead.zeroWrite.after); });
  report.status = "INCOMPLETE";
  process.exitCode = 2;
} catch {
  if (!finalizer?.interrupted) {
    report.status = manifest ? "FAIL" : "NOT_RUN";
    report.failure = activeCase;
    process.exitCode = manifest ? 1 : 2;
  }
} finally {
  if (finalizer) {
    await finalizer.finish();
    if (!finalizer.interrupted) finalizer.removeProcessHandlers();
  } else {
    report.finishedAt = new Date().toISOString();
    if (output) await writeFile(path.join(output, "report.json"), `${JSON.stringify(report, null, 2)}\n`);
    console.log(`${report.status} ${report.suite}; case=${report.failure ?? activeCase}`);
  }
}
