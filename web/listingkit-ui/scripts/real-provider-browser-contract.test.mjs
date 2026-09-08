import assert from "node:assert/strict";
import { tmpdir } from "node:os";
import path from "node:path";
import { test } from "vitest";
import { validateBrowserHandoff, publicBrowserOrigins, assertBrowserDiagnosticsDisabled, browserExpectedHeaders, classifyLateResponseDelivery, classifyRevocationRead, classifyUnavailableLogin, classifyUnavailableProviderTarget, isExpectedSettledView, isFinalApplicationLanding, retryOwnerHealth, shouldProbeUnavailableProvider, withOwnerControlRestored } from "./real-provider-browser-contract.mjs";

// Schema checks only. These objects never authenticate a browser or count as E2E.
const sha = "a".repeat(40);
const webSha = "b".repeat(40);
const runId = "7b578139-cf12-413f-9e02-3a3dbd68c128";
const root = path.join(tmpdir(), "task-processor-issue357", runId);
const manifestPath = path.join(root, "manifest.json");
function handoff() {
  return {
    schemaVersion: "issue357-v1", runId, status: "ready", sourceSha: sha, webSha, sourceDirty: false, webDirty: false,
    origins: { web: "http://localhost:43101", go: "http://127.0.0.1:43102", issuer: "http://localhost:43103" },
    instanceId: "instance-1", projectId: "project-1",
    organizations: Object.fromEntries(["A", "B", "C", "Empty", "D"].map(key => [key, { id: `org-${key}`, name: key }])),
    users: Object.fromEntries(["admin", "viewer", "no-org"].map(key => [key, { id: `user-${key}`, homeOrganizationId: "org-A", credentialFile: path.join(root, `${key}.json`) }])),
  };
}
function validate(input, overrides = {}) {
  return validateBrowserHandoff(input, { manifestPath, runtimeSha: sha, webSha, temporaryRoot: tmpdir(), ...overrides });
}
test("accepts only the explicit matching ready handoff", () => {
  assert.equal(validate(handoff()).runId, runId);
});
test("missing or old session-injecting fixtures cannot become provider acceptance", () => {
  for (const input of [null, {}, { ...handoff(), schemaVersion: "account-fixture" }, { ...handoff(), sessions: {} }]) {
    assert.throws(() => validate(input), /handoff_invalid/);
  }
});
test("planned, failed, stale source and absent exact SHA fail closed", () => {
  for (const input of [{ ...handoff(), status: "planned" }, { ...handoff(), status: "failed" }, { ...handoff(), sourceSha: webSha }, { ...handoff(), webSha: sha }]) {
    assert.throws(() => validate(input), /handoff_invalid/);
  }
  assert.throws(() => validate(handoff(), { runtimeSha: "" }), /handoff_invalid/);
});

test("development runs started from dirty or unproven source cannot become SHA acceptance", () => {
  for (const overrides of [{ sourceDirty: true }, { webDirty: true }, { sourceDirty: undefined }, { webDirty: undefined }]) {
    assert.throws(() => validate({ ...handoff(), ...overrides }), /handoff_invalid/);
  }
});
test("shared origins, credentials in origin and non-loopback endpoints are rejected", () => {
  for (const value of ["https://auth.example.test", "http://user:secret@localhost:43101", "http://localhost:43101/path", "http://localhost:43101?secret=value", "http://127.0.0.2:43101", "http://localhost:43103"]) {
    const input = handoff(); input.origins.web = value;
    assert.throws(() => validate(input), /handoff_invalid/);
  }
});
test("private credential paths must belong to the exact run", () => {
  const input = handoff(); input.users.admin.credentialFile = path.join(root, "..", "foreign", "admin.json");
  assert.throws(() => validate(input), /handoff_invalid/);
  assert.throws(() => validate(handoff(), { manifestPath: path.join(tmpdir(), "manifest.json") }), /handoff_invalid/);
});
test("Home and effective organizations and the three subjects must stay distinct", () => {
  const input = handoff(); input.organizations.B.id = input.organizations.A.id;
  assert.throws(() => validate(input), /handoff_invalid/);
  const wrongHome = handoff(); wrongHome.users["no-org"].homeOrganizationId = "org-B";
  assert.throws(() => validate(wrongHome), /handoff_invalid/);
  const duplicate = handoff(); duplicate.users.viewer.id = duplicate.users.admin.id;
  assert.throws(() => validate(duplicate), /handoff_invalid/);
});
test("invalid handoff errors never echo private inputs", () => {
  const input = handoff(); input.origins.web = "http://private-password@remote.test";
  assert.throws(() => validate(input), error => error.message === "issue358_handoff_invalid");
});
test("extra owner metadata never reaches browser report origins", () => {
  const input = handoff(); input.origins.privateMetadata = "private-sentinel";
  const result = publicBrowserOrigins(validate(input));
  assert.deepEqual(Object.keys(result).sort(), ["go", "issuer", "web"]);
  assert.equal(JSON.stringify(result).includes("private-sentinel"), false);
});
test("diagnostic environment is rejected before loading the browser library", () => {
  for (const environment of [{ DEBUG: "pw:api" }, { PWDEBUG: "1" }, { DEBUG_FILE: "private-trace.log" }, { PW_TEST_DEBUG_REPORTERS: "1" }]) {
    assert.throws(() => assertBrowserDiagnosticsDisabled(environment), error => error.message === "issue358_browser_diagnostics_forbidden");
  }
  assert.doesNotThrow(() => assertBrowserDiagnosticsDisabled({ DEBUG: "", PLAYWRIGHT_BROWSERS_PATH: "cache" }));
});
test("failed late-response delivery cannot pass without observed browser cancellation", () => {
  assert.equal(classifyLateResponseDelivery(true, null), "delivered");
  assert.equal(classifyLateResponseDelivery(false, "net::ERR_ABORTED"), "cancelled");
  for (const failure of [null, "", "private-error-detail", "net::ERR_CONNECTION_RESET"]) {
    assert.throws(() => classifyLateResponseDelivery(false, failure), error => error.message === "issue358_late_response_unproven");
  }
});

test("post-release UI must retain the selected organization's facts without an error view", () => {
  const expectation = { required: ["当前有效企业：org-C", "2 作业次"], forbidden: ["当前有效企业：org-B", "本次未取得数据"] };
  assert.equal(isExpectedSettledView("当前有效企业：org-C\n资料生成作业\n2 作业次", expectation), true);
  assert.equal(isExpectedSettledView("当前有效企业：org-C\n本次未取得数据", expectation), false);
  assert.equal(isExpectedSettledView("当前有效企业：org-B\n2 作业次", expectation), false);
});

test("a read started inside the cache window may finish after the boundary", () => {
  assert.equal(classifyRevocationRead({ status: 200, confirmedAt: 1000, requestStartedAt: 60999, responseCompletedAt: 62000 }), "cached");
  assert.equal(classifyRevocationRead({ status: 403, confirmedAt: 1000, requestStartedAt: 61001, responseCompletedAt: 62000 }), "denied");
  assert.throws(() => classifyRevocationRead({ status: 200, confirmedAt: 1000, requestStartedAt: 61001, responseCompletedAt: 62000 }), /revocation_not_converged/);
});

test("a failed or lost mutation response still attempts restoration and remains failed", async () => {
  const actions = [];
  await assert.rejects(withOwnerControlRestored(
    async () => { actions.push("mutation-applied-response-lost"); throw new Error("control_failed"); },
    async () => { actions.push("operation"); },
    async () => { actions.push("restore"); },
  ), /control_failed/);
  assert.deepEqual(actions, ["mutation-applied-response-lost", "restore"]);
});

test("restoration runs after successful controls and failed assertions", async () => {
  for (const fail of [false, true]) {
    const actions = [];
    const operation = withOwnerControlRestored(
      async () => { actions.push("mutate"); },
      async () => { actions.push("operation"); if (fail) throw new Error("assertion_failed"); return "result"; },
      async () => { actions.push("restore"); },
    );
    if (fail) await assert.rejects(operation, /assertion_failed/);
    else assert.equal(await operation, "result");
    assert.deepEqual(actions, ["mutate", "operation", "restore"]);
  }
  await assert.rejects(withOwnerControlRestored(async () => {}, async () => {}, async () => { throw new Error("restore_failed"); }), /restore_failed/);
});

test("provider outage accepts only a bounded local failure or the exact unavailable provider redirect", () => {
  assert.equal(classifyUnavailableLogin({ status: 503 }), "local-error");
  assert.equal(classifyUnavailableLogin({ status: 307, location: "http://localhost:43103/oauth/v2/authorize?private=omitted", issuer: "http://localhost:43103" }), "provider-redirect");
  assert.equal(classifyUnavailableLogin({ status: 307, location: "http://localhost:43101/api/auth/error?private=omitted", issuer: "http://localhost:43103", web: "http://localhost:43101" }), "local-error-redirect");
  assert.equal(classifyUnavailableLogin({ status: 307, location: "/api/auth/error?private=omitted", issuer: "http://localhost:43103", web: "http://localhost:43101" }), "local-error-redirect");
  for (const input of [
    { status: 200 },
    { status: 307, location: "http://localhost:43103/ui/v2/login/loginname", issuer: "http://localhost:43103" },
    { status: 307, location: "http://invalid.example/oauth/v2/authorize", issuer: "http://localhost:43103" },
  ]) assert.throws(() => classifyUnavailableLogin(input), /provider_failure_unproven/);
});

test("only an actual provider redirect is probed after provider shutdown", () => {
  assert.equal(shouldProbeUnavailableProvider("provider-redirect"), true);
  for (const outcome of ["local-error", "local-error-redirect"]) assert.equal(shouldProbeUnavailableProvider(outcome), false);
});

test("provider restoration retries the owner health check within a fixed bound", async () => {
  let attempts = 0;
  await retryOwnerHealth(async () => { attempts++; if (attempts < 3) throw new Error("not_ready"); }, { attempts: 3, wait: async () => {} });
  assert.equal(attempts, 3);
  attempts = 0;
  await assert.rejects(retryOwnerHealth(async () => { attempts++; throw new Error("not_ready"); }, { attempts: 2, wait: async () => {} }), /owner_health_not_restored/);
  assert.equal(attempts, 2);
});

test("a provider redirect is failed only by connection refusal or a bounded gateway status", () => {
  assert.equal(classifyUnavailableProviderTarget(undefined), "unreachable");
  for (const status of [500, 502, 503, 504]) assert.equal(classifyUnavailableProviderTarget(status), "gateway-error");
  for (const status of [200, 302, 401, 404]) assert.throws(() => classifyUnavailableProviderTarget(status), /provider_target_failure_unproven/);
});

test("status-only probes retain the same expected identity and organization headers", () => {
  const input = handoff();
  assert.deepEqual(browserExpectedHeaders(input, "admin"), { "X-Expected-User-ID": "user-admin" });
  assert.deepEqual(browserExpectedHeaders(input, "admin", "B"), { "X-Expected-User-ID": "user-admin", "X-Expected-Organization-ID": "org-B" });
});

test("logout cookie inspection waits for the final application landing response", () => {
  const web = "http://localhost:43101";
  assert.equal(isFinalApplicationLanding({ url: `${web}/`, status: 200 }, web), true);
  for (const response of [
    { url: `${web}/`, status: 307 },
    { url: "http://localhost:43103/", status: 302 },
    { url: `${web}/api/zitadel-auth/logout`, status: 307 },
    { url: `${web}/?returnTo=%2F`, status: 200 },
  ]) assert.equal(isFinalApplicationLanding(response, web), false);
});
