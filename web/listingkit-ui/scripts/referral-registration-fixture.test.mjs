import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { readFile } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

const nodeTestSpecifier = "node:test";
const test = process.env.VITEST ? globalThis.test : (await import(/* @vite-ignore */ nodeTestSpecifier)).default;
const runnerPath = process.env.VITEST ? path.resolve(process.cwd(), "scripts/referral-registration-fixture.mjs") : fileURLToPath(new URL("./referral-registration-fixture.mjs", import.meta.url));

import {
  cleanupActions,
  cleanupResult,
  lifecycleScenario,
} from "./fixtures/referral-registration-fixture-fault-runtime.mjs";

async function runner() {
  return import("./referral-registration-fixture.mjs");
}

function runCLI(name) {
  const result = spawnSync(process.execPath, [runnerPath, "lifecycle-test", name], {
    encoding: "utf8",
    env: { ...process.env, ISSUE413_FIXTURE_TEST_ONLY: "1" },
  });
  const lines = result.stdout.trim().split(/\r?\n/).filter(Boolean);
  return { process: result, output: lines.length ? JSON.parse(lines.at(-1)) : null };
}

test("business success plus an initial destroy failure stays failed after final cleanup succeeds", async () => {
  const { orchestrateFixtureLifecycle } = await runner();
  const scenario = lifecycleScenario({
    initialCleanup: cleanupResult("initial", "FAIL", "DESTROY_FAILED"),
  });
  const outcome = await orchestrateFixtureLifecycle(scenario.options);

  assert.equal(outcome.exitCode, 1);
  assert.equal(outcome.report.business.status, "PASS");
  assert.equal(outcome.report.cleanup.initial.status, "FAIL");
  assert.equal(outcome.report.cleanup.initial.code, "DESTROY_FAILED");
  assert.equal(outcome.report.cleanup.final.status, "PASS");
  assert.equal(outcome.report.conclusion, "FAIL");
  assert.deepEqual(scenario.calls, ["business", "cleanup:initial", "cleanup:final", "persist"]);
});

test("a partial start failure still runs both cleanup passes and exits nonzero", async () => {
  const { orchestrateFixtureLifecycle } = await runner();
  const scenario = lifecycleScenario({ businessFailure: "PARTIAL_START_FAILED" });
  const outcome = await orchestrateFixtureLifecycle(scenario.options);

  assert.equal(outcome.exitCode, 1);
  assert.equal(outcome.report.business.status, "FAIL");
  assert.equal(outcome.report.business.code, "PARTIAL_START_FAILED");
  assert.equal(outcome.report.cleanup.initial.status, "PASS");
  assert.equal(outcome.report.cleanup.final.status, "PASS");
  assert.deepEqual(scenario.calls, ["business", "cleanup:initial", "cleanup:final", "persist"]);
});

test("browser and child-process failures remain business failures after cleanup", async () => {
  const { orchestrateFixtureLifecycle } = await runner();
  for (const code of ["BROWSER_CLOSED", "PROCESS_FAILED:CURRENT_APPLICATION"]) {
    const scenario = lifecycleScenario({ businessFailure: code });
    const outcome = await orchestrateFixtureLifecycle(scenario.options);
    assert.equal(outcome.exitCode, 1);
    assert.equal(outcome.report.business.code, code);
    assert.equal(outcome.report.cleanup.final.status, "PASS");
    assert.equal(outcome.report.conclusion, "FAIL");
  }
});

test("one cleanup action failure does not skip the remaining owned cleanup actions", async () => {
  const { runCleanupPass } = await runner();
  const scenario = cleanupActions({ failedAction: "container" });
  const result = await runCleanupPass({
    phase: "initial",
    actions: scenario.actions,
    inspectResiduals: scenario.inspectResiduals,
    now: () => "2026-09-14T00:00:00.000Z",
  });

  assert.equal(result.status, "FAIL");
  assert.deepEqual(scenario.calls, ["browser", "child", "container", "runtime", "private-artifacts", "inspect"]);
  assert.deepEqual(result.actions.map(({ name, status }) => [name, status]), [
    ["browser", "PASS"], ["child", "PASS"], ["container", "FAIL"], ["runtime", "PASS"], ["private-artifacts", "PASS"],
  ]);
  assert.equal(result.residuals.status, "PASS");
});

test("one persistent base-resource failure does not skip later owned resources", async () => {
  const { ownedResourceCleanupActions, runCleanupPass } = await runner();
  const calls = [];
  const records = Object.fromEntries(["first", "second", "third"].map((name) => [name, { kind: "container", name, id: name }]));
  const actions = ownedResourceCleanupActions(records, async record => {
    calls.push(record.name);
    if (record.name === "first") throw new Error("PERSISTENT_DELETE_FAILURE");
  });
  const result = await runCleanupPass({ phase: "initial", actions, inspectResiduals: async () => ({ containers: 1, volumes: 0, networks: 0, listeners: 0 }) });
  assert.deepEqual(calls, ["first", "second", "third"]);
  assert.equal(result.status, "FAIL");
});

test("matrix evidence and NOT_RUN fallback cannot overwrite a PASS classification", async () => {
  const { matrixNotRun, matrixRecord, report, sanitizeMatrixEvidence } = await runner();
  assert.deepEqual(sanitizeMatrixEvidence({ group: "wrong", name: "wrong", status: 200, httpStatus: 200 }), { httpStatus: 200 });
  const item = report.matrix.find(entry => entry.group === "C" && entry.name === "concurrent_first_completion_one_relationship");
  matrixRecord(item.group, item.name, "PASS", { requests: 4 });
  matrixNotRun(item.group, item.name, "STALE_FALLBACK");
  assert.deepEqual(item, { group: "C", name: item.name, status: "PASS", requests: 4 });
});

test("M1 evidence requires a fresh official verification before authenticator enrollment", async () => {
  const { evaluateReverificationControl } = await runner();
  assert.deepEqual(evaluateReverificationControl({
    subjectBefore: "subject-1",
    subjectAfter: "subject-1",
    interruptedAuthenticatorRejected: true,
    previousVerificationRejected: true,
    replacementVerificationDelivered: true,
    replacementVerificationAccepted: true,
    authenticatorConfigured: true,
    oidcCompleted: true,
  }), { sameSubject: true, staleContinuationRejected: true, officialReverification: true, oidcCompleted: true });
  assert.throws(() => evaluateReverificationControl({
    subjectBefore: "subject-1",
    subjectAfter: "subject-1",
    interruptedAuthenticatorRejected: false,
    previousVerificationRejected: true,
    replacementVerificationDelivered: true,
    replacementVerificationAccepted: true,
    authenticatorConfigured: true,
    oidcCompleted: true,
  }), /INTERRUPTED_AUTHENTICATOR_ACCEPTED/);
});

test("M1 evidence requires live enterprise revocation, personal continuity, and late isolation", async () => {
  const { evaluateEnterpriseRemovalControl } = await runner();
  assert.deepEqual(evaluateEnterpriseRemovalControl({
    subjectBefore: "subject-1",
    subjectAfter: "subject-1",
    removedOrganizationId: "org-b",
    organizationsBefore: ["org-a", "org-b"],
    organizationsAfter: ["org-a"],
    fallbackOrganizationId: "org-a",
    visibleOrganizationId: "org-a",
    lateUpstreamValidated: true,
    lateReadOutcome: "cancelled_before_delivery",
    personalProjectionCount: 1,
    adminObservedCount: 0,
    lateRemovedOrganizationVisible: false,
  }), { sameSubject: true, removedOrganizationAbsent: true, personalProjectionCount: 1, adminObservedCount: 0, lateIsolation: true });
  assert.throws(() => evaluateEnterpriseRemovalControl({
    subjectBefore: "subject-1",
    subjectAfter: "subject-1",
    removedOrganizationId: "org-b",
    organizationsBefore: ["org-a", "org-b"],
    organizationsAfter: ["org-a", "org-b"],
    personalProjectionCount: 1,
    adminObservedCount: 0,
    lateRemovedOrganizationVisible: false,
  }), /REMOVED_ORGANIZATION_STILL_VISIBLE/);
  assert.throws(() => evaluateEnterpriseRemovalControl({
    subjectBefore: "subject-1",
    subjectAfter: "subject-1",
    removedOrganizationId: "org-b",
    organizationsBefore: ["org-a", "org-b"],
    organizationsAfter: ["org-a"],
    fallbackOrganizationId: "org-a",
    visibleOrganizationId: "unknown",
    lateUpstreamValidated: true,
    lateReadOutcome: "cancelled_before_delivery",
    personalProjectionCount: 1,
    adminObservedCount: 0,
    lateRemovedOrganizationVisible: false,
  }), /ENTERPRISE_FALLBACK_NOT_VISIBLE/);
});

test("M1 evidence requires revoked sessions and prevents late identity backfill", async () => {
  const { evaluateExpiredSessionControl } = await runner();
  assert.deepEqual(evaluateExpiredSessionControl({
    positiveStatus: 200,
    revokedSessionStatus: 401,
    revokedSessionRequestObserved: true,
    revokedSessionCheckedCount: 1,
    originalSubject: "subject-1",
    identityAfterLogout: "subject-2",
    lateResponseSubject: "subject-1",
    lateUpstreamValidated: true,
    lateReadOutcome: "cancelled_before_delivery",
    visibleSubjectAfterLateResponse: "subject-2",
    oldProjectionVisible: false,
  }), { positiveControl: true, revokedSessionRejected: true, replacementIdentityPreserved: true, lateBackfillPrevented: true });
  assert.throws(() => evaluateExpiredSessionControl({
    positiveStatus: 200,
    revokedSessionStatus: 401,
    revokedSessionRequestObserved: true,
    revokedSessionCheckedCount: 1,
    originalSubject: "subject-1",
    identityAfterLogout: "subject-2",
    lateResponseSubject: "subject-1",
    lateUpstreamValidated: true,
    lateReadOutcome: "delivered_to_unmounted_identity",
    visibleSubjectAfterLateResponse: "subject-1",
    oldProjectionVisible: true,
  }), /LATE_IDENTITY_BACKFILLED/);
  assert.throws(() => evaluateExpiredSessionControl({
    positiveStatus: 200,
    revokedSessionStatus: 404,
    revokedSessionRequestObserved: true,
    revokedSessionCheckedCount: 1,
    originalSubject: "subject-1",
    identityAfterLogout: "subject-2",
    lateResponseSubject: "late_injection_failed",
    lateUpstreamValidated: false,
    lateReadOutcome: "failed",
    visibleSubjectAfterLateResponse: "subject-2",
    oldProjectionVisible: false,
  }), /LATE_PROFILE_READ_UPSTREAM_INVALID/);
  assert.throws(() => evaluateExpiredSessionControl({
    positiveStatus: 200,
    revokedSessionStatus: 404,
    revokedSessionRequestObserved: false,
    revokedSessionCheckedCount: 0,
    originalSubject: "subject-1",
    identityAfterLogout: "subject-2",
    lateResponseSubject: "subject-1",
    lateUpstreamValidated: true,
    lateReadOutcome: "cancelled_before_delivery",
    visibleSubjectAfterLateResponse: "subject-2",
    oldProjectionVisible: false,
  }), /REVOKED_SESSION_REQUEST_NOT_OBSERVED/);
});

test("M1 evidence rejects an actual OIDC user token before any referral business call", async () => {
  const { evaluateUserTokenBoundary } = await runner();
  assert.deepEqual(evaluateUserTokenBoundary({
    tokenSource: "authjs_oidc_access_token",
    positiveServiceStatus: 200,
    userTokenStatus: 401,
    referralDigestBefore: "a".repeat(32),
    referralDigestAfter: "a".repeat(32),
    rejectedBusinessCalls: 0,
    guardBeforeHandlerVerified: true,
    storageUnavailableDuringProbe: true,
  }), { actualUserToken: true, positiveControl: true, rejected: true, referralTablesChanged: false, rejectedBusinessCalls: 0 });
  assert.throws(() => evaluateUserTokenBoundary({
    tokenSource: "authjs_oidc_access_token",
    positiveServiceStatus: 200,
    userTokenStatus: 401,
    referralDigestBefore: "a".repeat(32),
    referralDigestAfter: "b".repeat(32),
    rejectedBusinessCalls: 1,
    guardBeforeHandlerVerified: true,
    storageUnavailableDuringProbe: true,
  }), /USER_TOKEN_REACHED_BUSINESS/);
});

test("M1 reverification control executes the interrupted-browser sequence and always closes it", async () => {
  const { runReverificationControl } = await runner();
  const calls = [];
  const result = await runReverificationControl({
    verifyInitial: async () => { calls.push("verify-initial"); return { subject: "subject-1", authenticatorURL: "https://login.test/authenticator/set" }; },
    openFreshBrowser: async () => { calls.push("open-fresh"); return { id: "fresh" }; },
    rejectInterruptedAuthenticator: async () => { calls.push("reject-stale"); return true; },
    requestOfficialReverification: async () => { calls.push("request-reverify"); return { subject: "subject-1", messageId: "mail-2" }; },
    verifyReplacement: async () => { calls.push("verify-replacement"); return { subject: "subject-1" }; },
    configureAuthenticator: async () => { calls.push("configure-authenticator"); },
    completeOIDC: async () => { calls.push("complete-oidc"); return { subject: "subject-1" }; },
    closeFreshBrowser: async () => { calls.push("close-fresh"); },
  });
  assert.equal(result.subjectAfter, "subject-1");
  assert.deepEqual(calls, ["verify-initial", "open-fresh", "reject-stale", "request-reverify", "verify-replacement", "configure-authenticator", "complete-oidc", "close-fresh"]);
});

test("M1 reverification control stops before enrollment on stale continuation acceptance", async () => {
  const { runReverificationControl } = await runner();
  const calls = [];
  await assert.rejects(runReverificationControl({
    verifyInitial: async () => ({ subject: "subject-1", authenticatorURL: "https://login.test/authenticator/set" }),
    openFreshBrowser: async () => ({ id: "fresh" }),
    rejectInterruptedAuthenticator: async () => false,
    requestOfficialReverification: async () => { calls.push("request-reverify"); },
    verifyReplacement: async () => { calls.push("verify-replacement"); },
    configureAuthenticator: async () => { calls.push("configure-authenticator"); },
    completeOIDC: async () => { calls.push("complete-oidc"); },
    closeFreshBrowser: async () => { calls.push("close-fresh"); },
  }), /INTERRUPTED_AUTHENTICATOR_ACCEPTED/);
  assert.deepEqual(calls, ["close-fresh"]);
});

test("M1 enterprise removal control holds a real late read across revoke and restores authorization", async () => {
  const { runEnterpriseRemovalControl } = await runner();
  const calls = [];
  let release;
  const late = new Promise(resolve => { release = resolve; });
  const result = await runEnterpriseRemovalControl({
    readContext: async phase => { calls.push(`context:${phase}`); return phase === "before" ? { subject: "viewer", organizationIds: ["org-a", "org-b", "org-c"] } : { subject: "viewer", organizationIds: ["org-c"] }; },
    switchOrganization: async id => { calls.push(`switch:${id}`); },
    beginLateOrganizationRead: () => { calls.push("late:begin"); return { ready: Promise.resolve(), result: late }; },
    revokeAuthorization: async () => { calls.push("revoke"); },
    refreshAuthorizationContext: async () => { calls.push("context:refresh"); },
    releaseLateOrganizationRead: async () => { calls.push("late:release"); release({ organizationId: "org-b", upstreamValidated: true, applied: false, outcome: "cancelled_before_delivery" }); },
    inspectVisibleOrganization: async () => { calls.push("visible"); return "org-c"; },
    readPersonalProjection: async () => { calls.push("personal"); return { subject: "viewer", count: 1 }; },
    readAdminProjection: async () => { calls.push("admin"); return { subject: "admin", count: 0 }; },
    restoreAuthorization: async () => { calls.push("restore"); },
    removedOrganizationId: "org-b",
  });
  assert.equal(result.lateRemovedOrganizationVisible, false);
  assert.equal(result.visibleOrganizationId, "org-c");
  assert.deepEqual(calls, ["context:before", "switch:org-b", "late:begin", "revoke", "context:after", "context:refresh", "switch:org-c", "late:release", "visible", "personal", "admin", "restore"]);
});

test("M1 enterprise authorization restoration retries a transient provider failure", async () => {
  const { restoreAuthorizationEventually } = await runner();
  let attempts = 0;
  await restoreAuthorizationEventually(async () => {
    attempts++;
    if (attempts < 3) throw new Error("PROVIDER_NOT_CONSISTENT_YET");
  }, { attempts: 3, pause: async () => {} });
  assert.equal(attempts, 3);
});

test("M1 enterprise authorization restoration fails closed after bounded retries", async () => {
  const { restoreAuthorizationEventually } = await runner();
  await assert.rejects(restoreAuthorizationEventually(async () => {
    throw new Error("PROVIDER_UNAVAILABLE");
  }, { attempts: 2, pause: async () => {} }), /ENTERPRISE_AUTHORIZATION_RESTORE_FAILED/);
});

test("M1 enterprise personal projection uses and closes a fresh official session", async () => {
  const { readFreshPersonalProjection } = await runner();
  const calls = [];
  const result = await readFreshPersonalProjection({
    expectedSubject: "viewer",
    openSession: async () => { calls.push("open"); return { id: "fresh" }; },
    loginAndRead: async session => { calls.push(`read:${session.id}`); return { subject: "viewer", count: 1 }; },
    closeSession: async session => { calls.push(`close:${session.id}`); },
  });
  assert.deepEqual(result, { subject: "viewer", count: 1 });
  assert.deepEqual(calls, ["open", "read:fresh", "close:fresh"]);

  await assert.rejects(readFreshPersonalProjection({
    expectedSubject: "viewer",
    openSession: async () => ({ id: "mismatch" }),
    loginAndRead: async () => ({ subject: "admin", count: 0 }),
    closeSession: async () => { calls.push("close:mismatch"); },
  }), /PERSONAL_PROJECTION_SUBJECT_CHANGED/);
  assert.equal(calls.at(-1), "close:mismatch");
});

test("M1 enterprise switch waits for committed React effects before dispatch", async () => {
  const { submitOrganizationSelection } = await runner();
  const response = { status: () => 200 };
  let evaluations = 0;

  await submitOrganizationSelection({
    switcher: { evaluate: async () => ++evaluations === 1 ? { disabled: false, value: "", optionValues: ["org-a"] } : undefined },
    organizationId: "org-a",
    responsePromise: Promise.resolve(response),
  });
  assert.equal(evaluations, 2);
});

test("M1 enterprise switch fails explicitly when no request is observed", async () => {
  const { submitOrganizationSelection } = await runner();
  let evaluations = 0;

  await assert.rejects(submitOrganizationSelection({
    switcher: { evaluate: async () => ++evaluations === 1 ? { disabled: false, value: "", optionValues: ["org-a"] } : undefined },
    organizationId: "org-a",
    responsePromise: Promise.resolve(null),
  }), /ENTERPRISE_SWITCH_REQUEST_NOT_OBSERVED/);
});

test("official login waits for hydrated input and submit controls before acting", async () => {
  const { submitOfficialLoginStep } = await runner();
  const calls = [];
  const inputHandle = { id: "input" };
  const submitHandle = { id: "submit" };
  const page = {
    waitForFunction: async (_predicate, handle) => { calls.push(`hydrate:${handle.id}`); },
  };
  const input = {
    elementHandle: async () => { calls.push("handle:input"); return inputHandle; },
    fill: async value => { calls.push(`fill:${value}`); },
  };
  const submit = {
    elementHandle: async () => { calls.push("handle:submit"); return submitHandle; },
    click: async () => { calls.push("click"); },
  };

  await submitOfficialLoginStep(page, input, submit, "credential-value");
  assert.deepEqual(calls, ["handle:input", "hydrate:input", "handle:submit", "hydrate:submit", "fill:credential-value", "click"]);
});

test("M1 expired-session control deletes the provider session before replacement identity and late release", async () => {
  const { runExpiredSessionControl } = await runner();
  const calls = [];
  let release;
  const late = new Promise(resolve => { release = resolve; });
  const result = await runExpiredSessionControl({
    positiveRead: async () => { calls.push("positive"); return { status: 200, subject: "subject-1", sessionId: "session-1" }; },
    beginLateRead: () => { calls.push("late:begin"); return { ready: Promise.resolve(), result: late }; },
    deleteProviderSession: async id => { calls.push(`delete:${id}`); },
    readWithRevokedSession: async () => { calls.push("revoked-read"); return { status: 401, requestObserved: true, checkedCount: 1 }; },
    loginReplacementIdentity: async () => { calls.push("replacement-login"); return { subject: "subject-2" }; },
    confirmReplacementIdentity: async subject => { calls.push(`replacement-visible:${subject}`); },
    releaseLateRead: async () => { calls.push("late:release"); release({ subject: "subject-1", upstreamValidated: true, outcome: "cancelled_before_delivery" }); },
    inspectVisibleIdentity: async () => { calls.push("visible"); return { subject: "subject-2", oldProjectionVisible: false }; },
  });
  assert.equal(result.visibleSubjectAfterLateResponse, "subject-2");
  assert.deepEqual(calls, ["positive", "late:begin", "delete:session-1", "revoked-read", "replacement-login", "replacement-visible:subject-2", "late:release", "visible"]);
});

test("M1 revoked-session evidence reads every deleted provider session", async () => {
  const { verifyDeletedProviderSessions } = await runner();
  const calls = [];
  const observation = await verifyDeletedProviderSessions(["session-1", "session-2"], async sessionId => {
    calls.push(sessionId);
    return 404;
  });
  assert.deepEqual(calls, ["session-1", "session-2"]);
  assert.deepEqual(observation, { status: 404, requestObserved: true, checkedCount: 2 });
  await assert.rejects(
    verifyDeletedProviderSessions(["session-1"], async () => 200),
    /PROVIDER_SESSION_READ_ACCEPTED/,
  );
});

test("M1 late-read observers reject non-200 and wrong upstream identities", async () => {
  const { observeEnterpriseLateResponse, observeProfileLateResponse } = await runner();
  const response = (status, payload) => ({ status: () => status, json: async () => payload });

  assert.deepEqual(
    await observeEnterpriseLateResponse(response(200, { userId: "subject-1", effectiveOrganizationId: "org-b" }), "subject-1", "org-b"),
    { subject: "subject-1", organizationId: "org-b", upstreamStatus: 200, upstreamValidated: true },
  );
  await assert.rejects(observeEnterpriseLateResponse(response(500, {}), "subject-1", "org-b"), /LATE_ENTERPRISE_READ_HTTP_500/);
  await assert.rejects(observeEnterpriseLateResponse(response(200, { userId: "subject-1", effectiveOrganizationId: "org-c" }), "subject-1", "org-b"), /LATE_ENTERPRISE_READ_ORGANIZATION_MISMATCH/);
  await assert.rejects(observeEnterpriseLateResponse({ status: () => 200, json: async () => { throw new Error("invalid json"); } }, "subject-1", "org-b"), /LATE_ENTERPRISE_READ_BODY_INVALID/);

  assert.deepEqual(
    await observeProfileLateResponse(response(200, { userId: "subject-1" }), "subject-1"),
    { subject: "subject-1", upstreamStatus: 200, upstreamValidated: true },
  );
  await assert.rejects(observeProfileLateResponse(response(500, {}), "subject-1"), /LATE_PROFILE_READ_HTTP_500/);
  await assert.rejects(observeProfileLateResponse(response(200, { userId: "subject-2" }), "subject-1"), /LATE_PROFILE_READ_SUBJECT_MISMATCH/);
  await assert.rejects(observeProfileLateResponse({ status: () => 200, json: async () => { throw new Error("invalid json"); } }, "subject-1"), /LATE_PROFILE_READ_BODY_INVALID/);
});

test("M1 user-token boundary validates the human OIDC token, injects storage outage, and restores storage", async () => {
  const { runUserTokenBoundaryControl } = await runner();
  const calls = [];
  const result = await runUserTokenBoundaryControl({
    loadOIDCUserToken: async () => { calls.push("token:load"); return "private-token"; },
    validateOIDCUserToken: async token => { assert.equal(token, "private-token"); calls.push("token:validate"); return { subject: "human-1", human: true }; },
    verifyGuardPrecedesHandler: async () => { calls.push("guard:path"); return true; },
    callWithServiceCredential: async () => { calls.push("positive"); return { status: 200 }; },
    referralDigest: async phase => { calls.push(`digest:${phase}`); return "a".repeat(32); },
    stopBusinessStorage: async () => { calls.push("storage:stop"); },
    callWithUserTokenAsServiceCredential: async token => { assert.equal(token, "private-token"); calls.push("user-token"); return { status: 401 }; },
    startBusinessStorage: async () => { calls.push("storage:start"); },
  });
  assert.equal(result.rejectedBusinessCalls, 0);
  assert.deepEqual(calls, ["token:load", "token:validate", "guard:path", "positive", "digest:before", "storage:stop", "user-token", "storage:start", "digest:after"]);
});

test("M1 user-token boundary restores storage when the rejection probe fails", async () => {
  const { runUserTokenBoundaryControl } = await runner();
  const calls = [];
  await assert.rejects(runUserTokenBoundaryControl({
    loadOIDCUserToken: async () => "private-token",
    validateOIDCUserToken: async () => ({ subject: "human-1", human: true }),
    verifyGuardPrecedesHandler: async () => true,
    callWithServiceCredential: async () => ({ status: 200 }),
    referralDigest: async () => "a".repeat(32),
    stopBusinessStorage: async () => { calls.push("storage:stop"); },
    callWithUserTokenAsServiceCredential: async () => { throw new Error("PROBE_FAILED"); },
    startBusinessStorage: async () => { calls.push("storage:start"); },
  }), /PROBE_FAILED/);
  assert.deepEqual(calls, ["storage:stop", "storage:start"]);
});

test("M2 provider business-read failure replays the committed receipt and restores the selective fault", async () => {
  const { runProviderBusinessReadFailureControl, evaluateProviderBusinessReadFailure } = await runner();
  const calls = [];
  const receipt = { status: "complete", intentID: "intent-1", boundAt: "2026-09-14T00:00:00Z" };
  const observation = await runProviderBusinessReadFailureControl({
    verifyIdentityHealth: async () => { calls.push("identity:healthy"); return { discoveryStatus: 200, jwksStatus: 200, authProvidersStatus: 200, currentIdentityStatus: 200, currentSubject: "subject-1" }; },
    replayReceipt: async phase => { calls.push(`receipt:${phase}`); return { status: 200, receipt }; },
    relationshipCount: async phase => { calls.push(`count:${phase}`); return 1; },
    enableSelectiveBusinessReadFailure: async () => { calls.push("fault:on"); },
    observeBusinessReadFailure: async () => { calls.push("provider:failed-read"); return { status: 503, path: "/v2/users/subject-1" }; },
    disableSelectiveBusinessReadFailure: async () => { calls.push("fault:off"); },
    observeBusinessReadRecovery: async () => { calls.push("provider:recovered"); return { status: 200, subject: "subject-1" }; },
  });
  assert.deepEqual(evaluateProviderBusinessReadFailure(observation), {
    identityHealthy: true,
    selectiveProviderReadFailed: true,
    receiptPreserved: true,
    relationshipCountPreserved: true,
    providerRecovered: true,
  });
  assert.deepEqual(calls, ["identity:healthy", "receipt:before", "count:before", "fault:on", "provider:failed-read", "receipt:during", "count:during", "fault:off", "provider:recovered"]);
});

test("M2 provider fault is always disabled and invalid observations cannot pass", async () => {
  const { runProviderBusinessReadFailureControl, evaluateProviderBusinessReadFailure } = await runner();
  const calls = [];
  await assert.rejects(runProviderBusinessReadFailureControl({
    verifyIdentityHealth: async () => ({ discoveryStatus: 200, jwksStatus: 200, authProvidersStatus: 200, currentIdentityStatus: 200, currentSubject: "subject-1" }),
    replayReceipt: async phase => phase === "before" ? { status: 200, receipt: { intentID: "intent-1" } } : { status: 500, receipt: {} },
    relationshipCount: async () => 1,
    enableSelectiveBusinessReadFailure: async () => { calls.push("fault:on"); },
    observeBusinessReadFailure: async () => ({ status: 503, path: "/v2/users/subject-1" }),
    disableSelectiveBusinessReadFailure: async () => { calls.push("fault:off"); },
    observeBusinessReadRecovery: async () => ({ status: 200, subject: "subject-1" }),
  }), /PROVIDER_FAILURE_RECEIPT_REPLAY_FAILED/);
  assert.deepEqual(calls, ["fault:on", "fault:off"]);
  assert.throws(() => evaluateProviderBusinessReadFailure({
    identityHealth: { discoveryStatus: 200, jwksStatus: 500, authProvidersStatus: 200, currentIdentityStatus: 200, currentSubject: "subject-1" },
    receiptBefore: { status: 200, receipt: { intentID: "intent-1" } },
    receiptDuring: { status: 200, receipt: { intentID: "intent-1" } },
    countBefore: 1,
    countDuring: 1,
    providerFailure: { status: 503, path: "/v2/users/subject-1" },
    providerRecovery: { status: 200, subject: "subject-1" },
  }), /IDENTITY_HEALTH_FAILED/);
});

test("M2 parallel-instance control proves both live processes share one rate-limit bucket", async () => {
  const { runParallelInstanceRateLimitControl, evaluateParallelInstanceRateLimit } = await runner();
  const calls = [];
  const observation = await runParallelInstanceRateLimitControl({
    startSecondary: async () => { calls.push("secondary:start"); return { instanceId: "secondary", goPid: 22, nextPid: 23, goPort: 41002, nextPort: 41003, databaseId: "db-1" }; },
    inspectPrimary: async () => ({ instanceId: "primary", goPid: 12, nextPid: 13, goPort: 41000, nextPort: 41001, databaseId: "db-1" }),
    bothAlive: async () => { calls.push("both:alive"); return true; },
    waitForFreshWindow: async () => { calls.push("window:fresh"); },
    request: async (instance, source, sequence) => { calls.push(`${instance}:${source}:${sequence}`); return source === "source-a" && sequence === 6 ? 429 : 200; },
    stopSecondary: async () => { calls.push("secondary:stop"); },
    inspectSecondaryReleased: async () => { calls.push("secondary:released"); return { processes: 0, listeners: 0 }; },
  });
  assert.deepEqual(evaluateParallelInstanceRateLimit(observation), {
    simultaneousInstances: true,
    sharedDatabase: true,
    bothInstancesObservedTraffic: true,
    sharedLimitEnforced: true,
    independentSourceAllowed: true,
    secondaryReleased: true,
  });
  assert.deepEqual(observation.firstSourceStatuses, [200, 200, 200, 200, 200, 429]);
  assert.equal(calls.at(-2), "secondary:stop");
  assert.equal(calls.at(-1), "secondary:released");
});

test("M2 secondary process is stopped after a rate-control failure", async () => {
  const { runParallelInstanceRateLimitControl } = await runner();
  const calls = [];
  await assert.rejects(runParallelInstanceRateLimitControl({
    startSecondary: async () => ({ instanceId: "secondary", goPid: 22, nextPid: 23, goPort: 41002, nextPort: 41003, databaseId: "db-1" }),
    inspectPrimary: async () => ({ instanceId: "primary", goPid: 12, nextPid: 13, goPort: 41000, nextPort: 41001, databaseId: "db-1" }),
    bothAlive: async () => true,
    waitForFreshWindow: async () => {},
    request: async () => 500,
    stopSecondary: async () => { calls.push("secondary:stop"); },
    inspectSecondaryReleased: async () => { calls.push("secondary:released"); return { processes: 0, listeners: 0 }; },
  }), /PARALLEL_RATE_LIMIT_PRECONDITION_FAILED/);
  assert.deepEqual(calls, ["secondary:stop", "secondary:released"]);
});

test("M2 cancellation and deadline controls observe zero late business dispatch", async () => {
  const { runCancelDeadlineControl, evaluateCancelDeadlineControl } = await runner();
  const calls = [];
  const result = requestKind => ({
    ready: Promise.resolve(),
    cancel: async () => { calls.push(`${requestKind}:cancel`); },
    result: Promise.resolve(requestKind === "deadline" ? { status: 504 } : { outcome: "client_cancelled" }),
  });
  const observation = await runCancelDeadlineControl({
    healthyRequest: async () => { calls.push("healthy"); return { status: 200, businessDispatches: 1 }; },
    beginCancelledRequest: () => { calls.push("cancel:begin"); return result("cancel"); },
    beginDeadlineRequest: () => { calls.push("deadline:begin"); return result("deadline"); },
    settle: async kind => { calls.push(`${kind}:settle`); return { businessDispatches: 0, openRequests: 0 }; },
  });
  assert.deepEqual(evaluateCancelDeadlineControl(observation), {
    healthyDispatchObserved: true,
    clientCancellationObserved: true,
    deadlineResponseObserved: true,
    zeroLateDispatch: true,
    resourcesReleased: true,
  });
  assert.deepEqual(calls, ["healthy", "cancel:begin", "cancel:cancel", "cancel:settle", "deadline:begin", "deadline:settle"]);
});

test("M2 late dispatch and leaked observer requests fail closed", async () => {
  const { evaluateCancelDeadlineControl } = await runner();
  assert.throws(() => evaluateCancelDeadlineControl({
    healthy: { status: 200, businessDispatches: 1 },
    cancelled: { outcome: "client_cancelled" },
    cancelledSettled: { businessDispatches: 1, openRequests: 0 },
    deadline: { status: 504 },
    deadlineSettled: { businessDispatches: 0, openRequests: 1 },
  }), /LATE_BUSINESS_DISPATCH_OBSERVED/);
});

test("configured restart closes browser connections and keeps executed completion evidence", async () => {
  const source = await readFile(runnerPath, "utf8");
  assert.match(source, /await Promise\.all\(\[context\.close\(\), referrerContext\.close\(\)\]\);\s+const evidence = await restartConfiguredApplications\(ports\)/);
  assert.equal(source.match(/restartContextsAndApplications\(/g)?.length, 2);
  assert.doesNotMatch(source, /matrixNotRun\("C", "concurrent_first_completion_one_relationship"/);
});

test("a cleanup-state query failure is UNKNOWN and cannot exit successfully", async () => {
  const { orchestrateFixtureLifecycle, runCleanupPass } = await runner();
  const cleanup = cleanupActions({ inspectionFailure: "QUERY_FAILED:DOCKER" });
  const unknown = await runCleanupPass({
    phase: "initial",
    actions: cleanup.actions,
    inspectResiduals: cleanup.inspectResiduals,
    now: () => "2026-09-14T00:00:00.000Z",
  });
  assert.equal(unknown.status, "UNKNOWN");
  assert.equal(unknown.residuals.status, "UNKNOWN");
  assert.equal(unknown.residuals.code, "QUERY_FAILED:DOCKER");

  const scenario = lifecycleScenario({ initialCleanup: unknown });
  const outcome = await orchestrateFixtureLifecycle(scenario.options);
  assert.equal(outcome.exitCode, 1);
  assert.equal(outcome.report.cleanup.initial.status, "UNKNOWN");
  assert.equal(outcome.report.conclusion, "FAIL");
});

test("report write and half-write failures stay observable and exit nonzero without leaking secrets", async () => {
  const { orchestrateFixtureLifecycle } = await runner();
  for (const failure of ["REPORT_WRITE_FAILED", "secret=resumeSecret-value"]) {
    const scenario = lifecycleScenario({ persistFailure: failure });
    const outcome = await orchestrateFixtureLifecycle(scenario.options);
    assert.equal(outcome.exitCode, 1);
    assert.equal(outcome.report.evidence.status, "FAIL");
    assert.equal(outcome.report.conclusion, "FAIL");
    assert.equal(scenario.emitted.length, 1);
    assert.doesNotMatch(JSON.stringify(scenario.emitted), /resumeSecret-value/);
  }
});

test("CLI preserves an initial cleanup failure after a successful business phase and final cleanup", () => {
  const result = runCLI("initial-cleanup-failure");
  assert.equal(result.process.status, 1);
  assert.equal(result.output.report.business.status, "PASS");
  assert.equal(result.output.report.cleanup.initial.status, "FAIL");
  assert.equal(result.output.report.cleanup.final.status, "PASS");
});

test("CLI reports partial start failure and still performs initial and final cleanup", () => {
  const result = runCLI("partial-start");
  assert.equal(result.process.status, 1);
  assert.equal(result.output.report.business.code, "PARTIAL_START_FAILED");
  assert.deepEqual(result.output.calls, ["business", "cleanup:initial", "cleanup:final", "persist"]);
});

test("CLI reports browser and child-process failures without changing them to cleanup success", () => {
  for (const name of ["browser-failure", "child-failure"]) {
    const result = runCLI(name);
    assert.equal(result.process.status, 1);
    assert.equal(result.output.report.business.status, "FAIL");
    assert.equal(result.output.report.conclusion, "FAIL");
  }
});

test("CLI continues owned cleanup actions after one action fails", () => {
  const result = runCLI("cleanup-action-failure");
  assert.equal(result.process.status, 1);
  assert.deepEqual(result.output.cleanupCalls.initial, ["browser", "child", "container", "runtime", "private-artifacts", "inspect"]);
  assert.equal(result.output.report.cleanup.initial.status, "FAIL");
  assert.equal(result.output.report.cleanup.final.status, "PASS");
});

test("CLI makes a cleanup query failure UNKNOWN and nonzero", () => {
  const result = runCLI("cleanup-query-failure");
  assert.equal(result.process.status, 1);
  assert.equal(result.output.report.cleanup.initial.status, "UNKNOWN");
  assert.equal(result.output.report.cleanup.initial.residuals.status, "UNKNOWN");
});

test("CLI makes report write and half-write failures observable and nonzero", () => {
  for (const name of ["report-write-failure", "report-half-write"]) {
    const result = runCLI(name);
    assert.equal(result.process.status, 1);
    assert.equal(result.output.report.evidence.status, "FAIL");
    assert.doesNotMatch(`${result.process.stdout}${result.process.stderr}`, /resumeSecret-value/);
  }
});

test("CLI observes a real child-process exit through the production process wrapper", () => {
  const result = runCLI("real-child-failure");
  assert.equal(result.process.status, 1);
  assert.equal(result.output.report.business.code, `PROCESS_FAILED:${path.basename(process.execPath).replace(/[^A-Za-z0-9_-]/g, "_").toUpperCase()}`);
});

test("atomic report persistence removes a real half-write when rename fails", () => {
  const result = runCLI("real-report-rename-failure");
  assert.equal(result.process.status, 1);
  assert.equal(result.output.report.evidence.status, "FAIL");
  assert.deepEqual(result.output.after, { temporaryFiles: 0, destinationRemainedDirectory: true });
});

test("a dispatched runtime with an unreadable manifest makes both cleanup passes UNKNOWN", () => {
  const result = runCLI("unreadable-dispatched-manifest");
  assert.equal(result.process.status, 1);
  assert.equal(result.output.report.business.code, "RUNTIME_MANIFEST_UNKNOWN");
  assert.equal(result.output.report.cleanup.initial.status, "UNKNOWN");
  assert.equal(result.output.report.cleanup.final.status, "UNKNOWN");
  assert.equal(result.output.after.runIdObserved, true);
});
