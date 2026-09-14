import assert from "node:assert/strict";
import { execFile as execFileCallback, spawn } from "node:child_process";
import { createHash, randomBytes, randomUUID } from "node:crypto";
import { readFile, writeFile, mkdir, unlink, rm, rename, cp, symlink, link } from "node:fs/promises";
import { createServer as createHTTPServer, request as httpRequest } from "node:http";
import { request as httpsRequest } from "node:https";
import { createServer as createNetServer } from "node:net";
import { release, tmpdir } from "node:os";
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
const screenReaderSessionLimitMs = 90 * 60_000;
const screenReaderStableCheckpointLimitMs = 15 * 60_000;
const screenReaderCreateRecoveryLimitMs = 120_000;
const matrixPlan = {
  A: ["admission_response_loss_same_request_and_receipt", "same_key_original_receipt_and_different_payload_conflict", "multiple_referral_code_conflict", "reload_with_recovery_fragment_resumes_original", "fresh_browser_empty_form_no_auto_submit", "lost_credentials_safe_rejection"],
  B: ["existing_account_cannot_be_bound_to_new_intent", "new_browser_invalid_verification_does_not_verify", "verified_without_authenticator_cannot_complete", "official_verification_interruption_new_browser_reverify", "another_subject_cannot_claim_intent"],
  C: ["completion_response_loss_restart_receipt_replay", "concurrent_first_completion_one_relationship", "concurrent_completion_replays_one_durable_receipt", "provider_business_read_failure_preserves_receipt", "projection_failure_preserves_completion_receipt"],
  D: ["authenticated_get_is_pure_on_referral_tables", "admin_reads_only_own_personal_projection", "no_enterprise_user_can_read_own_empty_projection", "enterprise_removed_switching", "logout_removes_personal_projection", "expired_session_and_late_response"],
  E: ["trusted_proxy_overwrites_forged_forwarding", "bff_and_go_reject_untrusted_credentials_and_csrf", "real_source_ips_and_cross_process_rate_limit", "parallel_application_instances_share_rate_limit", "real_user_token_rejected_as_service_credential", "missing_dependency_disables_invite_entry", "cancel_deadline_zero_late_dispatch"],
  F: ["registration_desktop_axe", "registration_narrow_keyboard_axe", "completion_desktop_narrow_keyboard_axe", "overview_desktop_narrow_keyboard_axe", "screen_reader"],
};
export const screenReaderCheckpointPlan = Object.freeze([
  { id: "registration-initial", page: "/referrals/register", state: "labels_focus_validation" },
  { id: "registration-pending", page: "/referrals/register", state: "actual_start_pending" },
  { id: "registration-unknown", page: "/referrals/register", state: "original_request_unknown" },
  { id: "registration-mail-pending", page: "/referrals/register", state: "official_mail_pending" },
  { id: "completion-initial-pending", page: "/workbench/account/referrals/complete", state: "actual_completion_pending" },
  { id: "completion-receipt-projection-unavailable", page: "/workbench/account/referrals/complete", state: "receipt_and_projection_unavailable" },
  { id: "overview-entry-available", page: "/workbench/account/referrals", state: "real_count_and_entry_available" },
  { id: "overview-entry-unavailable", page: "/workbench/account/referrals", state: "real_count_and_entry_unavailable" },
]);
export const report = {
  schemaVersion: "issue413-referral-registration-fixture-v2",
  status: "NOT_RUN",
  checks: [],
  matrix: Object.entries(matrixPlan).flatMap(([group, names]) => names.map(name => ({ group, name, status: "NOT_RUN" }))),
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
let secondaryApplication;
let dispatchObserver;
let bffIngressObserver;
let secondaryGeneration = 0;
let screenReaderSession;

function ensure(value, code = "ASSERTION_FAILED") {
  assert.ok(value, code);
}

async function check(name, operation, recordEvidence = true) {
  const started = Date.now();
  try {
    const evidence = await operation();
    report.checks.push({ name, status: "PASS", elapsedMs: Date.now() - started, ...(recordEvidence ? sanitizeCheckEvidence(evidence) : {}) });
    console.log(`PASS ${name}`);
    return evidence;
  } catch (error) {
    report.checks.push({ name, status: "FAIL", elapsedMs: Date.now() - started, code: safeCode(error) });
    throw error;
  }
}

export function sanitizeCheckEvidence(evidence) {
  const details = evidence && typeof evidence === "object" ? { ...evidence } : {};
  delete details.name;
  delete details.status;
  return details;
}

async function matrixCheck(group, name, operation) {
  const started = Date.now();
  try {
    const evidence = await operation();
    const details = sanitizeMatrixEvidence(evidence);
    matrixRecord(group, name, "PASS", { elapsedMs: Date.now() - started, ...details });
    console.log(`PASS ${group}.${name}`);
    return evidence;
  } catch (error) {
    matrixRecord(group, name, "FAIL", { elapsedMs: Date.now() - started, code: safeCode(error) });
    console.error(`FAIL ${group}.${name} ${safeCode(error)}`);
    return undefined;
  }
}

export function matrixRecord(group, name, status, evidence = {}) {
  const item = report.matrix.find(entry => entry.group === group && entry.name === name);
  ensure(item, "UNKNOWN_MATRIX_ITEM");
  Object.assign(item, { status, ...sanitizeMatrixEvidence(evidence) });
}

export function sanitizeMatrixEvidence(evidence) {
  const details = evidence && typeof evidence === "object" ? { ...evidence } : {};
  delete details.group; delete details.name; delete details.status;
  return details;
}

export function matrixNotRun(group, name, reason) {
  if (report.matrix.find(entry => entry.group === group && entry.name === name)?.status === "NOT_RUN") matrixRecord(group, name, "NOT_RUN", { reason });
}

export function assertMatrixMustComplete(matrix) {
  ensure(Array.isArray(matrix), "MATRIX_MUST_INCOMPLETE");
  const expected = Object.entries(matrixPlan).flatMap(([group, names]) => names.map(name => `${group}.${name}`));
  const observed = new Map();
  for (const item of matrix) {
    const key = `${item?.group}.${item?.name}`;
    observed.set(key, [...(observed.get(key) ?? []), item]);
  }
  ensure(matrix.length === expected.length, "MATRIX_MUST_INCOMPLETE");
  for (const key of expected) {
    const matches = observed.get(key);
    ensure(matches?.length === 1 && matches[0].status === "PASS", "MATRIX_MUST_INCOMPLETE");
  }
  return true;
}

export function evaluateScreenReaderEvidence(observations, authority = {}) {
  ensure(Array.isArray(observations), "SCREEN_READER_OBSERVATIONS_INVALID");
  if (observations.length === 0) return { status: "NOT_RUN", checkpoints: [] };
  ensure(observations.length === screenReaderCheckpointPlan.length, "SCREEN_READER_CHECKPOINT_SET_INCOMPLETE");
  const seen = new Set();
  let identity;
  let failed = false;
  const checkpoints = observations.map((observation, index) => {
    validateScreenReaderObservationFields(observation);
    const expected = screenReaderCheckpointPlan[index];
    ensure(!seen.has(observation.checkpointId), "SCREEN_READER_CHECKPOINT_DUPLICATE");
    seen.add(observation.checkpointId);
    ensure(observation.checkpointId === expected.id && observation.sequence === index + 1, "SCREEN_READER_CHECKPOINT_OUT_OF_ORDER");
    ensure(observation.runId === authority.runId, "SCREEN_READER_RUN_MISMATCH");
    ensure(observation.sourceSha === authority.sourceSha && observation.webSha === authority.webSha && observation.runnerNormalizedLFSha256 === authority.runnerNormalizedLFSha256, "SCREEN_READER_SOURCE_MISMATCH");
    if (identity) assertScreenReaderIdentity(observation, identity);
    else identity = observation;
    if (observation.result === "FAIL") failed = true;
    return screenReaderCheckpointEvidence(observation, expected);
  });
  return {
    status: failed ? "FAIL" : "PASS",
    runId: authority.runId,
    sourceSha: authority.sourceSha,
    webSha: authority.webSha,
    runnerNormalizedLFSha256: authority.runnerNormalizedLFSha256,
    operator: identity.operator,
    designationReference: identity.designationReference,
    screenReader: identity.screenReader,
    browser: identity.browser,
    operatingSystem: identity.operatingSystem,
    checkpoints,
  };
}

function screenReaderCheckpointEvidence(observation, expected) {
  return {
    checkpointId: expected.id,
    sequence: observation.sequence,
    page: observation.page,
    checkpointAttemptId: observation.checkpointAttemptId,
    stateObservedAt: observation.stateObservedAt,
    result: observation.result,
    viewport: observation.viewport,
    observedAt: observation.observedAt,
    readingSequence: observation.readingSequence,
    controlSequence: observation.controlSequence,
    announcedText: observation.announcedText,
    visibleErrors: observation.visibleErrors,
    announcedErrors: observation.announcedErrors,
    observationSource: observation.observationSource,
    runnerActions: observation.runnerActions,
    stateAttempt: observation.stateAttempt,
  };
}

function assertScreenReaderIdentity(observation, reference) {
  ensure(observation.operator === reference.operator, "SCREEN_READER_OPERATOR_MISMATCH");
  ensure(observation.designationReference === reference.designationReference, "SCREEN_READER_DESIGNATION_MISMATCH");
  ensure(JSON.stringify(observation.screenReader) === JSON.stringify(reference.screenReader), "SCREEN_READER_SOFTWARE_MISMATCH");
  ensure(JSON.stringify(observation.browser) === JSON.stringify(reference.browser), "SCREEN_READER_BROWSER_MISMATCH");
  ensure(JSON.stringify(observation.operatingSystem) === JSON.stringify(reference.operatingSystem), "SCREEN_READER_OPERATING_SYSTEM_MISMATCH");
}

export function screenReaderPartialEvidence(observations, authority = {}) {
  ensure(Array.isArray(observations) && observations.length > 0 && observations.length <= screenReaderCheckpointPlan.length, "SCREEN_READER_OBSERVATIONS_INVALID");
  observations.forEach((observation, index) => {
    validateScreenReaderObservationFields(observation);
    ensure(observation.checkpointId === screenReaderCheckpointPlan[index].id && observation.sequence === index + 1, "SCREEN_READER_CHECKPOINT_OUT_OF_ORDER");
    ensure(observation.runId === authority.runId, "SCREEN_READER_RUN_MISMATCH");
    ensure(observation.sourceSha === authority.sourceSha && observation.webSha === authority.webSha && observation.runnerNormalizedLFSha256 === authority.runnerNormalizedLFSha256, "SCREEN_READER_SOURCE_MISMATCH");
    if (index > 0) assertScreenReaderIdentity(observation, observations[0]);
  });
  const identity = observations[0];
  return {
    status: observations.some(item => item.result === "FAIL") ? "FAIL" : "NOT_RUN",
    sessionStatus: "IN_PROGRESS",
    ...authority,
    operator: identity.operator,
    designationReference: identity.designationReference,
    screenReader: identity.screenReader,
    browser: identity.browser,
    operatingSystem: identity.operatingSystem,
    observations: observations.map((item, index) => screenReaderCheckpointEvidence(item, screenReaderCheckpointPlan[index])),
  };
}

function validateScreenReaderObservationFields(observation) {
  ensure(observation && typeof observation === "object" && !Array.isArray(observation), "SCREEN_READER_OBSERVATION_INCOMPLETE");
  ensure(!Object.hasOwn(observation, "status"), "SCREEN_READER_DIRECT_STATUS_FORBIDDEN");
  ensure(observation.controlTest !== true, "SCREEN_READER_TEST_EVIDENCE_FORBIDDEN");
  ensure(boundedText(observation.nonce, 8, 128) && boundedText(observation.checkpointAttemptId, 8, 128) && boundedText(observation.operator, 1, 120), "SCREEN_READER_OBSERVATION_INCOMPLETE");
  ensure(Number.isInteger(observation.sequence) && screenReaderCheckpointPlan[observation.sequence - 1]?.page === observation.page, "SCREEN_READER_OBSERVATION_STATE_MISMATCH");
  ensure(Number.isFinite(Date.parse(observation.stateObservedAt)), "SCREEN_READER_STATE_TIME_INVALID");
  ensure(/^https:\/\/github\.com\/qq550723504\/task-processor\/issues\/413#issuecomment-\d+$/.test(observation.designationReference ?? ""), "SCREEN_READER_DESIGNATION_INVALID");
  ensure(exactObjectKeys(observation.screenReader, ["name", "version"]) && boundedText(observation.screenReader.name, 1, 80) && boundedText(observation.screenReader.version, 1, 80), "SCREEN_READER_OBSERVATION_SCHEMA_INVALID");
  ensure(exactObjectKeys(observation.browser, ["name", "version"]) && boundedText(observation.browser.name, 1, 80) && boundedText(observation.browser.version, 1, 120), "SCREEN_READER_OBSERVATION_SCHEMA_INVALID");
  ensure(exactObjectKeys(observation.operatingSystem, ["name", "version"]) && boundedText(observation.operatingSystem.name, 1, 80) && boundedText(observation.operatingSystem.version, 1, 120), "SCREEN_READER_OBSERVATION_SCHEMA_INVALID");
  ensure(boundedText(observation.observationSource, 1, 240), "SCREEN_READER_OBSERVATION_INCOMPLETE");
  ensure(boundedTextArray(observation.runnerActions, 1), "SCREEN_READER_OBSERVATION_INCOMPLETE");
  const stateAttempt = JSON.stringify(observation.stateAttempt);
  ensure(stateAttempt && stateAttempt.length <= 8_000 && !screenReaderSensitiveText(stateAttempt), "SCREEN_READER_EVIDENCE_SECRET");
  ensure(exactObjectKeys(observation.viewport, ["width", "height"]) && Number.isInteger(observation.viewport.width) && observation.viewport.width >= 320 && observation.viewport.width <= 7680 && Number.isInteger(observation.viewport.height) && observation.viewport.height >= 320 && observation.viewport.height <= 4320, "SCREEN_READER_VIEWPORT_INVALID");
  for (const name of ["readingSequence", "controlSequence", "announcedText", "visibleErrors", "announcedErrors"]) ensure(boundedTextArray(observation[name], name === "visibleErrors" || name === "announcedErrors" ? 0 : 1), "SCREEN_READER_OBSERVATION_INCOMPLETE");
  ensure(["PASS", "FAIL"].includes(observation.result), "SCREEN_READER_RESULT_INVALID");
  ensure(Number.isFinite(Date.parse(observation.observedAt)), "SCREEN_READER_OBSERVED_AT_INVALID");
  ensure(Date.parse(observation.observedAt) >= Date.parse(observation.stateObservedAt) - 5_000, "SCREEN_READER_OBSERVATION_STALE");
  ensure(!screenReaderEvidenceContainsSecret(observation), "SCREEN_READER_EVIDENCE_SECRET");
  return observation;
}

function exactObjectKeys(value, keys) {
  return value && typeof value === "object" && !Array.isArray(value) && Object.keys(value).length === keys.length && keys.every(key => Object.hasOwn(value, key));
}

function boundedText(value, min, max) {
  return typeof value === "string" && value === value.trim() && value.length >= min && value.length <= max && !/[\u0000-\u001f\u007f]/u.test(value);
}

function boundedTextArray(value, min) {
  return Array.isArray(value) && value.length >= min && value.length <= 40 && value.every(item => boundedText(item, 1, 500));
}

function screenReaderEvidenceContainsSecret(observation) {
  const values = [
    observation.operator,
    observation.screenReader?.name,
    observation.screenReader?.version,
    observation.browser?.name,
    observation.browser?.version,
    observation.operatingSystem?.name,
    observation.operatingSystem?.version,
    observation.observationSource,
    ...observation.readingSequence,
    ...observation.controlSequence,
    ...observation.announcedText,
    ...observation.visibleErrors,
    ...observation.announcedErrors,
    ...observation.runnerActions,
  ];
  return values.some(screenReaderSensitiveText);
}

function screenReaderSensitiveText(value) {
  return (
    /(?:password|resume.?secret|cookie|bearer|access.?token|id.?token|otp|proof)\s*[:=]/i.test(value) ||
    /https?:\/\/\S+/i.test(value) ||
    /\b[A-Z0-9._%+-]+@[A-Z0-9.-]+\.[A-Z]{2,}\b/i.test(value) ||
    /\b(?:[a-f0-9]{32,}|[A-Za-z0-9]{43,})\b/i.test(value));
}

function safeCode(error) {
  const raw = error instanceof Error ? error.message : String(error);
  return /^[A-Z0-9_:-]{1,160}$/.test(raw) ? raw : "FIXTURE_STEP_FAILED";
}

export function describeRunnerBytes(input) {
  const bytes = Buffer.from(input);
  const hasUTF8BOM = bytes.length >= 3 && bytes[0] === 0xef && bytes[1] === 0xbb && bytes[2] === 0xbf;
  const text = bytes.toString("utf8");
  const crlfCount = text.match(/\r\n/g)?.length ?? 0;
  const lfCount = text.match(/(?<!\r)\n/g)?.length ?? 0;
  const crCount = text.match(/\r(?!\n)/g)?.length ?? 0;
  const normalized = Buffer.from(text.replaceAll("\r\n", "\n").replaceAll("\r", "\n"), "utf8");
  return {
    runnerSha256: createHash("sha256").update(bytes).digest("hex"),
    runnerNormalizedLFSha256: createHash("sha256").update(normalized).digest("hex"),
    runnerByteFormat: { bytes: bytes.length, hasUTF8BOM, crlfCount, lfCount, crCount },
  };
}

export function evaluateReverificationControl(observation) {
  ensure(observation.subjectBefore && observation.subjectBefore === observation.subjectAfter, "REVERIFICATION_SUBJECT_CHANGED");
  ensure(observation.interruptedAuthenticatorRejected === true, "INTERRUPTED_AUTHENTICATOR_ACCEPTED");
  ensure(observation.previousVerificationRejected === true, "STALE_VERIFICATION_CHECK_ACCEPTED");
  ensure(observation.replacementVerificationDelivered === true && observation.replacementVerificationAccepted === true, "OFFICIAL_REVERIFICATION_INCOMPLETE");
  ensure(observation.authenticatorConfigured === true, "AUTHENTICATOR_NOT_CONFIGURED");
  ensure(observation.oidcCompleted === true, "OIDC_NOT_COMPLETED");
  return { sameSubject: true, staleContinuationRejected: true, officialReverification: true, oidcCompleted: true };
}

export function evaluateEnterpriseRemovalControl(observation) {
  ensure(observation.subjectBefore && observation.subjectBefore === observation.subjectAfter, "ENTERPRISE_REMOVAL_SUBJECT_CHANGED");
  ensure(observation.organizationsBefore?.includes(observation.removedOrganizationId), "REMOVED_ORGANIZATION_PRECONDITION_MISSING");
  ensure(!observation.organizationsAfter?.includes(observation.removedOrganizationId), "REMOVED_ORGANIZATION_STILL_VISIBLE");
  ensure(observation.organizationsAfter?.includes(observation.fallbackOrganizationId), "ENTERPRISE_FALLBACK_AUTHORIZATION_MISSING");
  ensure(observation.visibleOrganizationId === observation.fallbackOrganizationId, "ENTERPRISE_FALLBACK_NOT_VISIBLE");
  ensure(observation.lateUpstreamValidated === true, "LATE_ENTERPRISE_READ_UPSTREAM_INVALID");
  ensure(["delivered_to_cancelled_request", "cancelled_before_delivery"].includes(observation.lateReadOutcome), "LATE_ENTERPRISE_READ_OBSERVATION_INVALID");
  ensure(observation.personalProjectionCount === 1, "PERSONAL_PROJECTION_NOT_PRESERVED");
  ensure(observation.adminObservedCount === 0, "ADMIN_OBSERVED_OTHER_PERSONAL_PROJECTION");
  ensure(observation.lateRemovedOrganizationVisible === false, "LATE_REMOVED_ORGANIZATION_BACKFILLED");
  return { sameSubject: true, removedOrganizationAbsent: true, personalProjectionCount: 1, adminObservedCount: 0, lateIsolation: true };
}

export function evaluateExpiredSessionControl(observation) {
  ensure(observation.positiveStatus === 200, "SESSION_POSITIVE_CONTROL_FAILED");
  ensure(observation.revokedSessionRequestObserved === true && Number.isInteger(observation.revokedSessionCheckedCount) && observation.revokedSessionCheckedCount > 0, "REVOKED_SESSION_REQUEST_NOT_OBSERVED");
  ensure([401, 403, 404].includes(observation.revokedSessionStatus), "REVOKED_SESSION_ACCEPTED");
  ensure(observation.lateUpstreamValidated === true, "LATE_PROFILE_READ_UPSTREAM_INVALID");
  ensure(["delivered_to_unmounted_identity", "cancelled_before_delivery"].includes(observation.lateReadOutcome), "LATE_SESSION_READ_OBSERVATION_INVALID");
  ensure(observation.originalSubject && observation.lateResponseSubject === observation.originalSubject, "LATE_SESSION_READ_SUBJECT_INVALID");
  ensure(observation.identityAfterLogout && observation.identityAfterLogout !== observation.lateResponseSubject, "REPLACEMENT_IDENTITY_INVALID");
  ensure(observation.visibleSubjectAfterLateResponse === observation.identityAfterLogout && observation.oldProjectionVisible === false, "LATE_IDENTITY_BACKFILLED");
  return { positiveControl: true, revokedSessionRejected: true, replacementIdentityPreserved: true, lateBackfillPrevented: true };
}

export function evaluateUserTokenBoundary(observation) {
  ensure(observation.tokenSource === "authjs_oidc_access_token", "USER_TOKEN_SOURCE_NOT_PROVEN");
  ensure(observation.guardBeforeHandlerVerified === true && observation.storageUnavailableDuringProbe === true, "USER_TOKEN_DISPATCH_BOUNDARY_NOT_PROVEN");
  ensure(observation.positiveServiceStatus === 200, "SERVICE_CREDENTIAL_POSITIVE_CONTROL_FAILED");
  ensure([401, 403].includes(observation.userTokenStatus), "USER_TOKEN_ACCEPTED_AS_SERVICE_CREDENTIAL");
  ensure(observation.referralDigestBefore === observation.referralDigestAfter && observation.rejectedBusinessCalls === 0, "USER_TOKEN_REACHED_BUSINESS");
  return { actualUserToken: true, positiveControl: true, rejected: true, referralTablesChanged: false, rejectedBusinessCalls: 0 };
}

export function evaluateProviderBusinessReadFailure(observation) {
  ensure(observation.identityHealth?.discoveryStatus === 200 && observation.identityHealth?.jwksStatus === 200 && observation.identityHealth?.authProvidersStatus === 200 && observation.identityHealth?.currentIdentityStatus === 200 && Boolean(observation.identityHealth?.currentSubject), "IDENTITY_HEALTH_FAILED");
  ensure(observation.receiptBefore?.status === 200 && observation.receiptDuring?.status === 200, "PROVIDER_FAILURE_RECEIPT_REPLAY_FAILED");
  ensure(JSON.stringify(observation.receiptBefore.receipt) === JSON.stringify(observation.receiptDuring.receipt), "PROVIDER_FAILURE_RECEIPT_CHANGED");
  ensure(Number.isInteger(observation.countBefore) && observation.countBefore === observation.countDuring, "PROVIDER_FAILURE_RELATIONSHIP_COUNT_CHANGED");
  ensure(observation.providerFailure?.status === 503 && /^\/v2\/users\//.test(observation.providerFailure?.path ?? ""), "SELECTIVE_PROVIDER_READ_FAILURE_NOT_OBSERVED");
  ensure(observation.providerRecovery?.status === 200 && Boolean(observation.providerRecovery?.subject), "PROVIDER_BUSINESS_READ_NOT_RECOVERED");
  return { identityHealthy: true, selectiveProviderReadFailed: true, receiptPreserved: true, relationshipCountPreserved: true, providerRecovered: true };
}

export async function runProviderBusinessReadFailureControl(operations) {
  const identityHealth = await operations.verifyIdentityHealth();
  ensure(identityHealth?.discoveryStatus === 200 && identityHealth?.jwksStatus === 200 && identityHealth?.authProvidersStatus === 200 && identityHealth?.currentIdentityStatus === 200 && Boolean(identityHealth?.currentSubject), "IDENTITY_HEALTH_FAILED");
  const receiptBefore = await operations.replayReceipt("before");
  ensure(receiptBefore?.status === 200, "PROVIDER_BASELINE_RECEIPT_REPLAY_FAILED");
  const countBefore = await operations.relationshipCount("before");
  let enabled = false;
  let providerFailure;
  let receiptDuring;
  let countDuring;
  try {
    await operations.enableSelectiveBusinessReadFailure();
    enabled = true;
    providerFailure = await operations.observeBusinessReadFailure();
    ensure(providerFailure?.status === 503, "SELECTIVE_PROVIDER_READ_FAILURE_NOT_OBSERVED");
    receiptDuring = await operations.replayReceipt("during");
    ensure(receiptDuring?.status === 200, "PROVIDER_FAILURE_RECEIPT_REPLAY_FAILED");
    countDuring = await operations.relationshipCount("during");
  } finally {
    if (enabled) await operations.disableSelectiveBusinessReadFailure();
  }
  const providerRecovery = await operations.observeBusinessReadRecovery();
  const observation = { identityHealth, receiptBefore, receiptDuring, countBefore, countDuring, providerFailure, providerRecovery };
  evaluateProviderBusinessReadFailure(observation);
  return observation;
}

export function evaluateParallelInstanceRateLimit(observation) {
  const { primary, secondary } = observation;
  ensure(primary?.instanceId && secondary?.instanceId && primary.instanceId !== secondary.instanceId, "PARALLEL_INSTANCE_IDENTITY_REUSED");
  ensure(primary.goPid !== secondary.goPid && primary.nextPid !== secondary.nextPid && primary.goPort !== secondary.goPort && primary.nextPort !== secondary.nextPort, "PARALLEL_PROCESS_OR_PORT_REUSED");
  ensure(observation.simultaneouslyAlive === true, "PARALLEL_INSTANCES_NOT_SIMULTANEOUS");
  ensure(primary.databaseId && primary.databaseId === secondary.databaseId, "PARALLEL_DATABASE_NOT_SHARED");
  ensure(observation.instanceRequests?.primary > 0 && observation.instanceRequests?.secondary > 0, "PARALLEL_INSTANCE_TRAFFIC_MISSING");
  ensure(observation.firstSourceStatuses?.slice(0, 5).every(status => status === 200), "PARALLEL_RATE_LIMIT_PRECONDITION_FAILED");
  ensure(observation.firstSourceStatuses?.[5] === 429, "PARALLEL_SHARED_RATE_LIMIT_NOT_ENFORCED");
  ensure(observation.secondSourceStatus === 200, "PARALLEL_CONTROL_SOURCE_REJECTED");
  ensure(observation.secondaryReleased?.processes === 0 && observation.secondaryReleased?.listeners === 0, "SECONDARY_INSTANCE_NOT_RELEASED");
  return { simultaneousInstances: true, sharedDatabase: true, bothInstancesObservedTraffic: true, sharedLimitEnforced: true, independentSourceAllowed: true, secondaryReleased: true };
}

export async function runParallelInstanceRateLimitControl(operations) {
  const secondary = await operations.startSecondary();
  let observation;
  let primary;
  let firstSourceStatuses = [];
  let secondSourceStatus;
  let simultaneouslyAlive = false;
  const instanceRequests = { primary: 0, secondary: 0 };
  let failure;
  let secondaryReleased;
  try {
    primary = await operations.inspectPrimary();
    simultaneouslyAlive = await operations.bothAlive(primary, secondary);
    ensure(simultaneouslyAlive, "PARALLEL_INSTANCES_NOT_SIMULTANEOUS");
    ensure(primary?.databaseId && primary.databaseId === secondary?.databaseId, "PARALLEL_DATABASE_NOT_SHARED");
    await operations.waitForFreshWindow();
    for (let sequence = 1; sequence <= 5; sequence++) {
      const instance = sequence % 2 === 0 ? "secondary" : "primary";
      instanceRequests[instance]++;
      firstSourceStatuses.push(await operations.request(instance, "source-a", sequence));
    }
    ensure(firstSourceStatuses.every(status => status === 200), "PARALLEL_RATE_LIMIT_PRECONDITION_FAILED");
    instanceRequests.secondary++;
    firstSourceStatuses.push(await operations.request("secondary", "source-a", 6));
    instanceRequests.primary++;
    secondSourceStatus = await operations.request("primary", "source-b", 7);
  } catch (error) {
    failure = error;
  } finally {
    try {
      await operations.stopSecondary(secondary);
      secondaryReleased = await operations.inspectSecondaryReleased(secondary);
      ensure(secondaryReleased?.processes === 0 && secondaryReleased?.listeners === 0, "SECONDARY_INSTANCE_NOT_RELEASED");
    } catch (cleanupError) {
      failure = failure ? new AggregateError([failure, cleanupError], "PARALLEL_INSTANCE_CONTROL_FAILED") : cleanupError;
    }
  }
  if (failure) throw failure;
  observation = { primary, secondary, simultaneouslyAlive, firstSourceStatuses, secondSourceStatus, instanceRequests, secondaryReleased };
  evaluateParallelInstanceRateLimit(observation);
  return observation;
}

export function evaluateCancelDeadlineControl(observation) {
  ensure(observation.healthy?.status === 200 && observation.healthy?.bffDispatches === 1 && observation.healthy?.goDispatches === 1, "HEALTHY_BUSINESS_DISPATCH_NOT_OBSERVED");
  ensure(observation.healthy?.released === true && observation.healthy?.responsesCompleted === 1, "HEALTHY_HELD_PATH_NOT_RELEASED");
  ensure(observation.healthy?.ingress?.received === 1 && observation.healthy?.ingress?.bodyChunksForwarded > 0 && observation.healthy?.ingress?.requestBodiesCompleted === 1 && observation.healthy?.ingress?.responsesCompleted === 1, "HEALTHY_INGRESS_PATH_NOT_OBSERVED");
  ensure(observation.cancelled?.outcome === "client_cancelled", "CLIENT_CANCELLATION_NOT_OBSERVED");
  ensure(observation.deadline?.status === 504, "DEADLINE_RESPONSE_NOT_OBSERVED");
  ensure(observation.cancelledSettled?.bffDispatches === 1 && observation.deadlineSettled?.bffDispatches === 1, "BFF_DISPATCH_NOT_OBSERVED");
  ensure(observation.cancelledSettled?.goDispatches === 0 && observation.deadlineSettled?.goDispatches === 0, "LATE_BUSINESS_DISPATCH_OBSERVED");
  ensure(observation.cancelledSettled?.clientConnectionsClosed === 1 && observation.deadlineSettled?.clientConnectionsClosed === 1, "OBSERVER_CLIENT_CLOSE_NOT_OBSERVED");
  ensure(observation.cancelledSettled?.released === true && observation.deadlineSettled?.released === true, "OBSERVER_DELAY_NOT_RELEASED");
  ensure(observation.cancelledSettled?.openHandlers === 0 && observation.deadlineSettled?.openHandlers === 0, "OBSERVER_REQUEST_NOT_RELEASED");
  ensure(observation.bodyReady?.localBodyChunksProduced > 0, "BODY_STREAM_NOT_PRODUCED");
  ensure(observation.bodyReady?.ingressRequests === 1 && observation.bodyReady?.ingressBodyChunksReceived > 0 && observation.bodyReady?.bodyChunksForwardedToBFF > 0 && observation.bodyReady?.bffConnections > 0 && observation.bodyReady?.ingressRequestBodiesCompleted === 0, "BODY_READ_INGRESS_NOT_OBSERVED");
  ensure(observation.bodyCancelled?.outcome === "client_cancelled" && observation.bodyCancelled?.bodyChunksProduced > 0, "BODY_READ_CANCELLATION_NOT_OBSERVED");
  ensure((observation.bodyCancelledSettled?.ingressAborted > 0 || observation.bodyCancelledSettled?.ingressClientConnectionsClosed > 0) && observation.bodyCancelledSettled?.bffConnectionsClosed > 0, "BODY_READ_CONNECTION_CLOSE_NOT_OBSERVED");
  ensure(observation.bodyCancelledSettled?.ingressOpenHandlers === 0, "BODY_READ_INGRESS_HANDLER_NOT_RELEASED");
  ensure(observation.bodyCancelledSettled?.bffDispatches === 0 && observation.bodyCancelledSettled?.goDispatches === 0, "BODY_READ_LATE_DISPATCH_OBSERVED");
  ensure(observation.bodyCancelledSettled?.openHandlers === 0, "BODY_READ_OBSERVER_REQUEST_NOT_RELEASED");
  return { healthyDispatchObserved: true, clientCancellationObserved: true, deadlineResponseObserved: true, bodyReadCancellationObserved: true, zeroLateDispatch: true, resourcesReleased: true };
}

export async function runCancelDeadlineControl(operations) {
  const healthy = await operations.healthyRequest();
  ensure(healthy?.status === 200 && healthy?.goDispatches === 1, "HEALTHY_BUSINESS_DISPATCH_NOT_OBSERVED");
  const cancelling = operations.beginCancelledRequest();
  await cancelling.ready;
  await cancelling.cancel();
  const cancelled = await cancelling.result;
  const cancelledSettled = await operations.settle("cancel");
  const timingOut = operations.beginDeadlineRequest();
  await timingOut.ready;
  const deadline = await timingOut.result;
  const deadlineSettled = await operations.settle("deadline");
  const bodyCancelling = operations.beginBodyCancelledRequest();
  const bodyReady = await bodyCancelling.ready;
  await bodyCancelling.cancel();
  const bodyCancelled = await bodyCancelling.result;
  const bodyCancelledSettled = await operations.settleBodyCancellation();
  const observation = { healthy, cancelled, cancelledSettled, deadline, deadlineSettled, bodyReady, bodyCancelled, bodyCancelledSettled };
  evaluateCancelDeadlineControl(observation);
  return observation;
}

export async function runReverificationControl(operations) {
  const initial = await operations.verifyInitial();
  ensure(initial?.subject && initial?.authenticatorURL, "INITIAL_VERIFICATION_OBSERVATION_INVALID");
  const fresh = await operations.openFreshBrowser(initial.authenticatorURL);
  try {
    ensure(await operations.rejectInterruptedAuthenticator(fresh, initial), "INTERRUPTED_AUTHENTICATOR_ACCEPTED");
    const replacement = await operations.requestOfficialReverification(initial.subject);
    ensure(replacement?.subject === initial.subject, "REVERIFICATION_SUBJECT_CHANGED");
    const verified = await operations.verifyReplacement(fresh, replacement);
    ensure(verified?.subject === initial.subject, "REVERIFICATION_SUBJECT_CHANGED");
    await operations.configureAuthenticator(fresh, verified);
    const oidc = await operations.completeOIDC(fresh, verified);
    ensure(oidc?.subject === initial.subject, "OIDC_SUBJECT_CHANGED");
    return { subjectBefore: initial.subject, subjectAfter: oidc.subject, interruptedAuthenticatorRejected: true, previousVerificationRejected: true, replacementVerificationDelivered: true, replacementVerificationAccepted: true, authenticatorConfigured: true, oidcCompleted: true };
  } finally {
    await operations.closeFreshBrowser(fresh);
  }
}

export async function observeEnterpriseLateResponse(response, expectedSubject, expectedOrganizationId) {
  const status = response.status();
  ensure(status === 200, `LATE_ENTERPRISE_READ_HTTP_${status}`);
  const payload = await response.json().catch(() => { throw new Error("LATE_ENTERPRISE_READ_BODY_INVALID"); });
  ensure(payload?.userId === expectedSubject, "LATE_ENTERPRISE_READ_SUBJECT_MISMATCH");
  ensure(payload?.effectiveOrganizationId === expectedOrganizationId, "LATE_ENTERPRISE_READ_ORGANIZATION_MISMATCH");
  return { subject: payload.userId, organizationId: payload.effectiveOrganizationId, upstreamStatus: status, upstreamValidated: true };
}

export async function observeProfileLateResponse(response, expectedSubject) {
  const status = response.status();
  ensure(status === 200, `LATE_PROFILE_READ_HTTP_${status}`);
  const payload = await response.json().catch(() => { throw new Error("LATE_PROFILE_READ_BODY_INVALID"); });
  ensure(payload?.userId === expectedSubject, "LATE_PROFILE_READ_SUBJECT_MISMATCH");
  return { subject: payload.userId, upstreamStatus: status, upstreamValidated: true };
}

export async function runEnterpriseRemovalControl(operations) {
  let revoked = false;
  let lateReleased = false;
  const before = await operations.readContext("before");
  ensure(before?.subject && before.organizationIds?.includes(operations.removedOrganizationId), "ENTERPRISE_REMOVAL_PRECONDITION_FAILED");
  await operations.switchOrganization(operations.removedOrganizationId);
  const late = operations.beginLateOrganizationRead();
  await late.ready;
  try {
    revoked = true;
    await operations.revokeAuthorization();
    const after = await operations.readContext("after");
    const fallbackOrganizationId = after?.organizationIds?.find(id => id !== operations.removedOrganizationId);
    ensure(fallbackOrganizationId, "ENTERPRISE_FALLBACK_AUTHORIZATION_MISSING");
    await operations.refreshAuthorizationContext();
    await operations.switchOrganization(fallbackOrganizationId);
    await operations.releaseLateOrganizationRead();
    lateReleased = true;
    const lateResult = await late.result;
    const visibleOrganization = await operations.inspectVisibleOrganization();
    const personal = await operations.readPersonalProjection();
    const admin = await operations.readAdminProjection();
    return {
      subjectBefore: before.subject,
      subjectAfter: after.subject,
      removedOrganizationId: operations.removedOrganizationId,
      organizationsBefore: before.organizationIds,
      organizationsAfter: after.organizationIds,
      fallbackOrganizationId,
      visibleOrganizationId: visibleOrganization,
      lateUpstreamValidated: lateResult?.upstreamValidated,
      lateUpstreamStatus: lateResult?.upstreamStatus,
      lateReadOutcome: lateResult?.outcome,
      personalProjectionCount: personal.count,
      adminObservedCount: admin.count,
      lateRemovedOrganizationVisible: visibleOrganization === operations.removedOrganizationId || lateResult?.applied === true,
    };
  } finally {
    if (!lateReleased) await operations.releaseLateOrganizationRead().catch(() => {});
    if (revoked) await operations.restoreAuthorization();
  }
}

export async function restoreAuthorizationEventually(operation, options = {}) {
  const attempts = options.attempts ?? 4;
  const pause = options.pause ?? (() => delay(1_000));
  ensure(Number.isInteger(attempts) && attempts > 0, "ENTERPRISE_AUTHORIZATION_RESTORE_ATTEMPTS_INVALID");
  for (let attempt = 1; attempt <= attempts; attempt++) {
    try {
      await operation();
      return;
    } catch {
      if (attempt === attempts) throw new Error("ENTERPRISE_AUTHORIZATION_RESTORE_FAILED");
      await pause();
    }
  }
}

export async function readFreshPersonalProjection(operations) {
  const session = await operations.openSession();
  try {
    const projection = await operations.loginAndRead(session);
    ensure(projection?.subject === operations.expectedSubject, "PERSONAL_PROJECTION_SUBJECT_CHANGED");
    ensure(Number.isInteger(projection.count) && projection.count >= 0, "PERSONAL_PROJECTION_COUNT_INVALID");
    return projection;
  } finally {
    await operations.closeSession(session);
  }
}

export async function submitOrganizationSelection({ switcher, organizationId, responsePromise }) {
  const readiness = await switcher.evaluate(async element => {
    await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
    return {
      disabled: element.disabled,
      value: element.value,
      optionValues: Array.from(element.options, option => option.value),
    };
  });
  ensure(!readiness.disabled, "ENTERPRISE_SWITCHER_DISABLED");
  ensure(readiness.optionValues.includes(organizationId), "ENTERPRISE_SWITCH_TARGET_MISSING");
  const [, responseResult] = await Promise.allSettled([
    switcher.evaluate((element, id) => {
      const valueSetter = Object.getOwnPropertyDescriptor(Object.getPrototypeOf(element), "value")?.set;
      if (valueSetter) valueSetter.call(element, id);
      else element.value = id;
      element.dispatchEvent(new Event("change", { bubbles: true }));
    }, organizationId),
    responsePromise,
  ]);
  const response = responseResult.status === "fulfilled" ? responseResult.value : null;
  if (!response) {
    const error = new Error("ENTERPRISE_SWITCH_REQUEST_NOT_OBSERVED");
    error.fixtureDiagnostic = readiness;
    throw error;
  }
  ensure(response.status() === 200, `ENTERPRISE_SWITCH_HTTP_${response.status()}`);
}

export async function runExpiredSessionControl(operations) {
  const positive = await operations.positiveRead();
  ensure(positive?.status === 200 && positive?.subject && positive?.sessionId, "SESSION_POSITIVE_CONTROL_FAILED");
  const late = operations.beginLateRead();
  await late.ready;
  let lateReleased = false;
  try {
    await operations.deleteProviderSession(positive.sessionId);
    const rejected = await operations.readWithRevokedSession();
    const replacement = await operations.loginReplacementIdentity();
    await operations.confirmReplacementIdentity(replacement.subject);
    await operations.releaseLateRead();
    lateReleased = true;
    const lateResult = await late.result;
    const visible = await operations.inspectVisibleIdentity();
    return {
      positiveStatus: positive.status,
      revokedSessionStatus: rejected.status,
      revokedSessionRequestObserved: rejected.requestObserved,
      revokedSessionCheckedCount: rejected.checkedCount,
      originalSubject: positive.subject,
      identityAfterLogout: replacement.subject,
      lateResponseSubject: lateResult.subject,
      lateUpstreamValidated: lateResult.upstreamValidated,
      lateUpstreamStatus: lateResult.upstreamStatus,
      lateReadOutcome: lateResult.outcome,
      visibleSubjectAfterLateResponse: visible.subject,
      oldProjectionVisible: visible.oldProjectionVisible,
    };
  } finally {
    if (!lateReleased) await operations.releaseLateRead().catch(() => {});
  }
}

export async function verifyDeletedProviderSessions(sessionIds, readStatus) {
  ensure(Array.isArray(sessionIds) && sessionIds.length > 0, "PROVIDER_SESSION_NOT_FOUND");
  const statuses = [];
  for (const sessionId of sessionIds) statuses.push(await readStatus(sessionId));
  ensure(statuses.every(status => [401, 403, 404].includes(status)), "PROVIDER_SESSION_READ_ACCEPTED");
  return { status: statuses[0], requestObserved: true, checkedCount: statuses.length };
}

export async function runUserTokenBoundaryControl(operations) {
  const token = await operations.loadOIDCUserToken();
  ensure(typeof token === "string" && token.length > 0, "OIDC_USER_TOKEN_MISSING");
  const tokenIdentity = await operations.validateOIDCUserToken(token);
  ensure(tokenIdentity?.human === true && tokenIdentity?.subject, "OIDC_USER_TOKEN_NOT_HUMAN");
  ensure(await operations.verifyGuardPrecedesHandler(), "SERVICE_GUARD_ORDER_INVALID");
  const positive = await operations.callWithServiceCredential();
  const before = await operations.referralDigest("before");
  let storageStopAttempted = true;
  let rejected;
  try {
    await operations.stopBusinessStorage();
    rejected = await operations.callWithUserTokenAsServiceCredential(token);
  } finally {
    if (storageStopAttempted) await operations.startBusinessStorage();
  }
  const after = await operations.referralDigest("after");
  return {
    tokenSource: "authjs_oidc_access_token",
    positiveServiceStatus: positive.status,
    userTokenStatus: rejected.status,
    referralDigestBefore: before,
    referralDigestAfter: after,
    rejectedBusinessCalls: [401, 403].includes(rejected.status) ? 0 : 1,
    tokenSubject: tokenIdentity.subject,
    guardBeforeHandlerVerified: true,
    storageUnavailableDuringProbe: true,
    directBusinessDispatchObserver: false,
    zeroDispatchEvidence: "actual_401_during_storage_outage_plus_exact_guard_before_handler_path",
  };
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
  const server = createNetServer();
  await new Promise((resolve, reject) => { server.once("error", reject); server.listen(preferred, "127.0.0.1", resolve); });
  const address = server.address();
  ensure(address && typeof address === "object", "PORT_ALLOCATION_FAILED");
  await new Promise(resolve => server.close(resolve));
  return address.port;
}

async function fixturePorts() {
  const reserved = new Set(Object.values(manifest.ports));
  const allocated = {};
  for (const name of ["next", "provider", "providerFault", "public", "publicSecondary", "mail", "goSecondary", "nextSecondary", "observer", "bffIngress"]) {
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

export function screenReaderSessionPaths(runId) {
  ensure(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/.test(runId ?? ""), "INVALID_RUN_ID");
  const runDirectory = path.resolve(tmpdir(), "task-processor-issue357", runId);
  const directory = path.resolve(runDirectory, "screen-reader-session");
  ensure(path.dirname(directory) === runDirectory, "SCREEN_READER_PATH_INVALID");
  return {
    runDirectory,
    directory,
    status: path.join(directory, "status.json"),
    input: sequence => path.join(directory, `operator-observation-${String(sequence).padStart(2, "0")}.json`),
    decision: sequence => path.join(directory, `decision-${String(sequence).padStart(2, "0")}.json`),
  };
}

async function readBoundedJSON(file, maxBytes = 64 * 1024) {
  const bytes = await readFile(file);
  ensure(bytes.byteLength > 0 && bytes.byteLength <= maxBytes, "SCREEN_READER_INPUT_SIZE_INVALID");
  return JSON.parse(bytes.toString("utf8"));
}

function validateScreenReaderStatus(status, owner) {
  ensure(status?.schemaVersion === "issue413-screen-reader-checkpoint-v1", "SCREEN_READER_STATUS_INVALID");
  ensure(status.runId === owner.runId && status.sourceSha === owner.sourceSha && status.webSha === owner.webSha, "SCREEN_READER_STATUS_OWNER_MISMATCH");
  ensure(/^[0-9a-f]{64}$/.test(status.runnerNormalizedLFSha256 ?? ""), "SCREEN_READER_STATUS_SOURCE_INVALID");
  ensure(status.sequence >= 1 && status.sequence <= screenReaderCheckpointPlan.length && screenReaderCheckpointPlan[status.sequence - 1]?.id === status.checkpointId, "SCREEN_READER_STATUS_SEQUENCE_INVALID");
  ensure(["action-ready", "observation"].includes(status.phase), "SCREEN_READER_STATUS_INVALID");
  ensure(Date.now() < Date.parse(status.expiresAt), "SCREEN_READER_STATUS_EXPIRED");
  return status;
}


async function ensureScreenReaderCommandSource(status) {
  const current = describeRunnerBytes(await readFile(fileURLToPath(import.meta.url)));
  ensure(current.runnerNormalizedLFSha256 === status.runnerNormalizedLFSha256, "SCREEN_READER_STATUS_SOURCE_INVALID");
}

export function screenReaderObservationFromInput(input, status) {
  ensure(status.phase === "observation", "SCREEN_READER_ACTION_NOT_OBSERVED");
  ensure(input && typeof input === "object" && !Array.isArray(input), "SCREEN_READER_OBSERVATION_INCOMPLETE");
  ensure(!Object.hasOwn(input, "status") && input.controlTest !== true, Object.hasOwn(input, "status") ? "SCREEN_READER_DIRECT_STATUS_FORBIDDEN" : "SCREEN_READER_TEST_EVIDENCE_FORBIDDEN");
  ensure(input.checkpointId === status.checkpointId && input.sequence === status.sequence && input.nonce === status.nonce && input.checkpointAttemptId === status.checkpointAttemptId, "SCREEN_READER_OBSERVATION_STALE");
  ensure(input.designationReference === status.designationReference, "SCREEN_READER_DESIGNATION_MISMATCH");
  ensure(JSON.stringify(input.browser) === JSON.stringify(status.browser) && JSON.stringify(input.viewport) === JSON.stringify(status.viewport), "SCREEN_READER_OBSERVATION_STATE_MISMATCH");
  ensure(Date.parse(input.observedAt) <= Date.parse(status.expiresAt) && Date.parse(input.observedAt) <= Date.now() + 60_000, "SCREEN_READER_OBSERVATION_STALE");
  const observation = {
    checkpointId: status.checkpointId,
    sequence: status.sequence,
    runId: status.runId,
    nonce: status.nonce,
    checkpointAttemptId: status.checkpointAttemptId,
    page: status.page,
    stateObservedAt: status.stateObservedAt,
    sourceSha: status.sourceSha,
    webSha: status.webSha,
    runnerNormalizedLFSha256: status.runnerNormalizedLFSha256,
    operator: input.operator,
    designationReference: input.designationReference,
    screenReader: input.screenReader,
    browser: input.browser,
    operatingSystem: status.operatingSystem,
    observationSource: input.observationSource,
    viewport: input.viewport,
    readingSequence: input.readingSequence,
    controlSequence: input.controlSequence,
    announcedText: input.announcedText,
    visibleErrors: input.visibleErrors,
    announcedErrors: input.announcedErrors,
    result: input.result,
    observedAt: input.observedAt,
    runnerActions: status.runnerActions,
    stateAttempt: status.stateAttempt,
  };
  return validateScreenReaderObservationFields(observation);
}

export async function writeScreenReaderDecision({ decisionFile, decision, beforePublish }) {
  const encoded = `${JSON.stringify(decision, null, 2)}\n`;
  const temporary = `${decisionFile}.${process.pid}.${randomBytes(8).toString("hex")}.tmp`;
  try {
    await writeFile(temporary, encoded, { encoding: "utf8", flag: "wx", mode: 0o600 });
    await beforePublish?.();
    try {
      await link(temporary, decisionFile);
      return { recorded: true, replay: false, decision };
    } catch (error) {
      if (error.code !== "EEXIST") throw error;
      const existing = await readBoundedJSON(decisionFile);
      ensure(JSON.stringify(existing) === JSON.stringify(decision), "SCREEN_READER_DECISION_CONFLICT");
      return { recorded: true, replay: true, decision: existing };
    }
  } finally {
    await unlink(temporary).catch(error => { if (error.code !== "ENOENT") throw error; });
  }
}

async function screenReaderStatusCommand(runId) {
  const paths = screenReaderSessionPaths(runId);
  const owner = await readBoundedJSON(path.join(paths.runDirectory, "manifest.json"));
  const status = validateScreenReaderStatus(await readBoundedJSON(paths.status), owner);
  await ensureScreenReaderCommandSource(status);
  console.log(JSON.stringify({ ...status, inputFile: paths.input(status.sequence) }));
}

async function screenReaderAckCommand(runId) {
  const paths = screenReaderSessionPaths(runId);
  const owner = await readBoundedJSON(path.join(paths.runDirectory, "manifest.json"));
  const status = validateScreenReaderStatus(await readBoundedJSON(paths.status), owner);
  await ensureScreenReaderCommandSource(status);
  const inputFile = paths.input(status.sequence);
  const input = await readBoundedJSON(inputFile);
  const observation = screenReaderObservationFromInput(input, status);
  const result = await writeScreenReaderDecision({ decisionFile: paths.decision(status.sequence), decision: { kind: "observation", observation } });
  await unlink(inputFile).catch(error => { if (error.code !== "ENOENT") throw error; });
  console.log(result.replay ? "SCREEN_READER_OBSERVATION_ALREADY_RECORDED" : "SCREEN_READER_OBSERVATION_RECORDED");
}

async function screenReaderAbortCommand(runId) {
  const paths = screenReaderSessionPaths(runId);
  const owner = await readBoundedJSON(path.join(paths.runDirectory, "manifest.json"));
  const status = validateScreenReaderStatus(await readBoundedJSON(paths.status), owner);
  await ensureScreenReaderCommandSource(status);
  const result = await writeScreenReaderDecision({ decisionFile: paths.decision(status.sequence), decision: { kind: "abort", runId, checkpointId: status.checkpointId, sequence: status.sequence, nonce: status.nonce } });
  console.log(result.replay ? "SCREEN_READER_ABORT_ALREADY_RECORDED" : "SCREEN_READER_ABORT_RECORDED");
}

async function waitForScreenReaderDecision(status, timeoutMs, signal) {
  const decisionFile = screenReaderSession.paths.decision(status.sequence);
  const end = Math.min(Date.now() + timeoutMs, screenReaderSession.deadlineAt, Date.parse(status.expiresAt));
  while (Date.now() < end) {
    if (signal?.aborted) throw new Error("SCREEN_READER_WAIT_CANCELLED");
    try {
      const decision = await readBoundedJSON(decisionFile);
      ensure(decision.kind === "abort" || decision.kind === "observation", "SCREEN_READER_DECISION_INVALID");
      ensure(decision.runId === undefined || decision.runId === status.runId, "SCREEN_READER_RUN_MISMATCH");
      return decision;
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
    }
    await delay(250);
  }
  throw new Error("SCREEN_READER_CHECKPOINT_TIMEOUT");
}

async function collectScreenReaderObservation(checkpointId, page, options = {}) {
  if (!screenReaderSession) return undefined;
  const sequence = screenReaderSession.observations.length + 1;
  ensure(screenReaderCheckpointPlan[sequence - 1]?.id === checkpointId, "SCREEN_READER_CHECKPOINT_OUT_OF_ORDER");
  const url = new URL(page.url());
  const viewport = page.viewportSize();
  ensure(url.pathname === screenReaderCheckpointPlan[sequence - 1].page && viewport, "SCREEN_READER_OBSERVATION_STATE_MISMATCH");
  const now = Date.now();
  const timeoutMs = Math.min(options.timeoutMs ?? screenReaderStableCheckpointLimitMs, screenReaderSession.deadlineAt - now);
  ensure(timeoutMs > 0, "SCREEN_READER_SESSION_TIMEOUT");
  const status = {
    schemaVersion: "issue413-screen-reader-checkpoint-v1",
    runId: manifest.runId,
    sourceSha: report.sourceSha,
    webSha: report.webSha,
    runnerNormalizedLFSha256: report.runnerNormalizedLFSha256,
    checkpointId,
    sequence,
    phase: "observation",
    nonce: randomBytes(24).toString("base64url"),
    checkpointAttemptId: options.checkpointAttemptId ?? randomUUID(),
    stateAttempt: options.stateAttempt ?? "stable",
    stateObservedAt: options.stateObservedAt ?? new Date(now).toISOString(),
    openedAt: new Date(now).toISOString(),
    expiresAt: new Date(now + timeoutMs).toISOString(),
    designationReference: screenReaderSession.designationReference,
    browser: screenReaderSession.browser,
    operatingSystem: screenReaderSession.operatingSystem,
    viewport,
    page: url.pathname,
    expected: options.expected ?? [],
    runnerActions: options.runnerActions ?? [],
  };
  await unlink(screenReaderSession.paths.input(sequence)).catch(error => { if (error.code !== "ENOENT") throw error; });
  await writeJSONAtomic(screenReaderSession.paths.status, status);
  console.log(`SCREEN_READER_CHECKPOINT ${checkpointId} ${manifest.runId}`);
  const decision = await waitForScreenReaderDecision(status, timeoutMs);
  if (decision.kind === "abort") throw new Error("SCREEN_READER_SESSION_ABORTED");
  ensure(!page.isClosed() && new URL(page.url()).pathname === status.page && JSON.stringify(page.viewportSize()) === JSON.stringify(status.viewport), "SCREEN_READER_OBSERVATION_STATE_MISMATCH");
  const observation = decision.observation;
  ensure(observation.checkpointId === checkpointId && observation.sequence === sequence && observation.nonce === status.nonce && observation.checkpointAttemptId === status.checkpointAttemptId, "SCREEN_READER_OBSERVATION_STALE");
  validateScreenReaderObservationFields(observation);
  const observations = [...screenReaderSession.observations, observation];
  const partialEvidence = screenReaderPartialEvidence(observations, {
    runId: manifest.runId,
    sourceSha: report.sourceSha,
    webSha: report.webSha,
    runnerNormalizedLFSha256: report.runnerNormalizedLFSha256,
  });
  screenReaderSession.observations.push(observation);
  report.manualAccessibility = partialEvidence;
  if (partialEvidence.status === "FAIL") matrixRecord("F", "screen_reader", "FAIL", { checkpoints: partialEvidence.observations.map(item => ({ checkpointId: item.checkpointId, result: item.result })) });
  await writeJSONAtomic(path.join(outputDirectory, "screen-reader-observations.json"), partialEvidence);
  return observation;
}

async function prepareScreenReaderAction(checkpointId, page, button, requestPredicate, options = {}) {
  if (!screenReaderSession) {
    await button.click();
    return undefined;
  }
  const sequence = screenReaderSession.observations.length + 1;
  ensure(screenReaderCheckpointPlan[sequence - 1]?.id === checkpointId, "SCREEN_READER_CHECKPOINT_OUT_OF_ORDER");
  const now = Date.now();
  const timeoutMs = Math.min(options.timeoutMs ?? screenReaderStableCheckpointLimitMs, screenReaderSession.deadlineAt - now);
  ensure(timeoutMs > 0, "SCREEN_READER_SESSION_TIMEOUT");
  const checkpointAttemptId = randomUUID();
  await button.evaluate((element, attemptId) => {
    window.__issue413ScreenReaderTransitions ??= {};
    const transitions = [];
    const record = () => transitions.push({ text: element.textContent?.trim() ?? "", disabled: "disabled" in element && Boolean(element.disabled), at: new Date().toISOString() });
    record();
    const observer = new MutationObserver(record);
    observer.observe(element, { childList: true, subtree: true, attributes: true, attributeFilter: ["disabled", "aria-disabled"] });
    window.__issue413ScreenReaderTransitions[attemptId] = { transitions, disconnect: () => observer.disconnect() };
  }, checkpointAttemptId);
  const status = {
    schemaVersion: "issue413-screen-reader-checkpoint-v1",
    runId: manifest.runId,
    sourceSha: report.sourceSha,
    webSha: report.webSha,
    runnerNormalizedLFSha256: report.runnerNormalizedLFSha256,
    checkpointId,
    sequence,
    phase: "action-ready",
    nonce: randomBytes(24).toString("base64url"),
    checkpointAttemptId,
    stateAttempt: "operator_action_ready",
    stateObservedAt: new Date(now).toISOString(),
    openedAt: new Date(now).toISOString(),
    expiresAt: new Date(now + timeoutMs).toISOString(),
    designationReference: screenReaderSession.designationReference,
    browser: screenReaderSession.browser,
    operatingSystem: screenReaderSession.operatingSystem,
    viewport: page.viewportSize(),
    page: new URL(page.url()).pathname,
    expected: options.expected ?? [],
    runnerActions: ["waited_for_operator_action", "did_not_dispatch_or_extend_product_budget"],
  };
  await unlink(screenReaderSession.paths.input(sequence)).catch(error => { if (error.code !== "ENOENT") throw error; });
  await writeJSONAtomic(screenReaderSession.paths.status, status);
  console.log(`SCREEN_READER_ACTION_READY ${checkpointId} ${manifest.runId}`);
  const request = page.waitForRequest(requestPredicate, { timeout: timeoutMs }).then(value => ({ kind: "request", value }), error => ({ kind: "error", error }));
  const waitController = new AbortController();
  const decision = waitForScreenReaderDecision(status, timeoutMs, waitController.signal).then(value => ({ kind: "decision", value }), error => ({ kind: "error", error }));
  const outcome = await Promise.race([request, decision]);
  waitController.abort();
  if (outcome.kind === "error") throw outcome.error;
  if (outcome.kind === "decision") {
    ensure(outcome.value.kind === "abort", "SCREEN_READER_OBSERVATION_BEFORE_ACTION");
    throw new Error("SCREEN_READER_SESSION_ABORTED");
  }
  return { checkpointAttemptId, requestObservedAt: new Date().toISOString() };
}

async function finishScreenReaderAction(page, action) {
  if (!screenReaderSession || !action) return undefined;
  return page.evaluate(attemptId => {
    const state = window.__issue413ScreenReaderTransitions?.[attemptId];
    state?.disconnect?.();
    if (window.__issue413ScreenReaderTransitions) delete window.__issue413ScreenReaderTransitions[attemptId];
    return state?.transitions ?? [];
  }, action.checkpointAttemptId);
}

async function initializeScreenReaderSession() {
  if (process.env.ISSUE413_SCREEN_READER_SESSION !== "1") return;
  const designationReference = process.env.ISSUE413_SCREEN_READER_DESIGNATION ?? "";
  ensure(/^https:\/\/github\.com\/qq550723504\/task-processor\/issues\/413#issuecomment-\d+$/.test(designationReference), "SCREEN_READER_DESIGNATION_INVALID");
  const paths = screenReaderSessionPaths(manifest.runId);
  await mkdir(paths.directory, { recursive: true, mode: 0o700 });
  screenReaderSession = {
    paths,
    designationReference,
    browser: { name: "Chromium", version: browser.version() },
    operatingSystem: { name: "Windows", version: release() },
    startedAt: Date.now(),
    deadlineAt: Date.now() + screenReaderSessionLimitMs,
    observations: [],
  };
  report.manualAccessibility = { status: "NOT_RUN", sessionStatus: "IN_PROGRESS", designationReference, browser: screenReaderSession.browser, operatingSystem: screenReaderSession.operatingSystem };
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
  const providerFailureResponse = JSON.stringify({ code: 13, message: "run-owned selective business read failure" });
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

:81 {
  reverse_proxy host.docker.internal:${ports.bffIngress} {
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
https://localhost:445 {
  tls internal
  @user_business_read {
    method GET
    path /v2/users/*
  }
  @metadata_business_read {
    method POST
    path_regexp metadata_read ^/v2/users/[^/]+/metadata/search$
  }
  handle @user_business_read {
    respond \`${providerFailureResponse}\` 503
  }
  handle @metadata_business_read {
    respond \`${providerFailureResponse}\` 503
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
https://localhost:446 {
  tls internal
  reverse_proxy host.docker.internal:${ports.bffIngress} {
    header_up -X-Referral-Service-Credential
    header_up -X-Referral-Client-IP
    header_up X-ListingKit-Client-IP {remote_host}
  }
}
`);
  await docker(["run", "-d", "--name", caddyName, "--label", `${ownerLabel}=${manifest.runId}`, "--network", network,
    "-p", `127.0.0.1:${manifest.ports.web}:80`, "-p", `127.0.0.1:${ports.provider}:443`, "-p", `127.0.0.1:${ports.providerFault}:445`,
    "-p", `127.0.0.1:${ports.public}:444`, "-p", `127.0.0.1:${ports.publicSecondary}:446`,
    "--mount", `type=bind,source=${config},target=/etc/caddy/Caddyfile,readonly`, "--mount", `type=bind,source=${caddyData},target=/data`,
    caddyImage, "caddy", "run", "--config", "/etc/caddy/Caddyfile", "--adapter", "caddyfile"]);
  const caFile = path.join(caddyData, "caddy", "pki", "authorities", "local", "root.crt");
  const leafFile = path.join(caddyData, "caddy", "certificates", "local", "localhost", "localhost.crt");
  try {
    await until(async () => {
      const pem = await readFile(caFile, "utf8");
      const leaf = await readFile(leafFile, "utf8");
      return pem.includes("BEGIN CERTIFICATE") && pem.length < 64 * 1024 && leaf.includes("BEGIN CERTIFICATE") && leaf.length < 64 * 1024;
    }, "CADDY_CERTIFICATES", 60_000);
  } catch (error) {
    const inspected = await inspectDockerResource("container", caddyName).catch(() => null);
    const logs = await execFile("docker", ["--host", dockerHost, "logs", "--tail", "200", caddyName], { windowsHide: true, timeout: 5_000, maxBuffer: 512 * 1024 }).catch(() => ({ stdout: "", stderr: "" }));
    await writePrivate(path.join(outputDirectory, "m2-caddy-start-diagnostic.log"), `${logs.stdout}\n${logs.stderr}`.slice(-128 * 1024));
    await writeJSON(path.join(outputDirectory, "m2-caddy-start-diagnostic.json"), { running: inspected?.State?.Running === true, exitCode: inspected?.State?.ExitCode ?? null, code: safeCode(error) });
    throw error;
  }
  await until(async () => (await fetch(`http://127.0.0.1:${ports.mail}/api/v1/info`, { signal: AbortSignal.timeout(2_000) })).ok, "MAILPIT", 60_000);
  report.imageDigests = { ...manifest.imageDigests };
  for (const [name, image] of Object.entries({ caddy: caddyImage, mailpit: mailImage })) { const inspected = JSON.parse(await docker(["image", "inspect", image]))[0]; report.imageDigests[name] = { image, id: inspected.Id, digests: inspected.RepoDigests }; }
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
  const secondaryPublicOrigin = `https://localhost:${ports.publicSecondary}`;
  const providerOrigin = `https://localhost:${ports.provider}`;
  const providerFaultOrigin = `https://localhost:${ports.providerFault}`;
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
  return { publicOrigin, secondaryPublicOrigin, providerOrigin, providerFaultOrigin, caFile };
}

async function startConfiguredApplications(ports) {
  for (const name of ["stop-go", "stop-next", "stop-services", "go-ready.json", "next-ready.json", "services-stopped.json"]) await unlink(path.join(manifest.directory, name)).catch(error => { if (error.code !== "ENOENT") throw error; });
  const child = spawn(process.execPath, [path.join(repo, "scripts", "issue357", "serve.mjs"), manifest.directory], { cwd: manifest.directory, detached: true, windowsHide: true, stdio: "ignore" });
  await new Promise((resolve, reject) => { child.once("spawn", resolve); child.once("error", () => reject(new Error("SUPERVISOR_START_FAILED"))); });
  child.unref();
  await until(async () => { await readJSON(path.join(manifest.directory, "go-ready.json")); await readJSON(path.join(manifest.directory, "next-ready.json")); return true; }, "CONFIGURED_APPLICATION_START", 300_000);
  const current = await readJSON(path.join(manifest.directory, "manifest.json"));
  const processes = await readJSON(path.join(manifest.directory, "processes.json"));
  ensure(current.runId === manifest.runId && processes.supervisor, "CONFIGURED_APPLICATION_START_FAILED");
  current.status = "ready"; current.supervisor = processes.supervisor;
  await writeJSON(path.join(manifest.directory, "manifest.json"), current);
  manifest = { ...current, directory: manifest.directory };
  const go = await fetch(`${manifest.origins.go}/api/v1/account/profile`, { signal: AbortSignal.timeout(10_000) });
  ensure(go.status === 401, "GO_HEALTH_FAILED");
  const providersResponse = await fetch(`http://127.0.0.1:${ports.next}/api/auth/providers`, { signal: AbortSignal.timeout(90_000) });
  ensure(providersResponse.ok, "NEXT_HEALTH_FAILED");
  const providers = await providersResponse.json();
  ensure(providers.zitadel, "NEXT_PROVIDER_MISSING");
  return { ready: true };
}

export async function startDispatchObserver(port, upstreamOrigin, options = {}) {
  if (dispatchObserver) return dispatchObserver;
  const observations = new Map();
  const selectors = new Map();
  let mode = "idle";
  const createObservation = () => {
    let release;
    const releasePromise = new Promise(resolve => { release = resolve; });
    return {
      received: 0,
      businessDispatches: 0,
      openRequests: 0,
      closedRequests: 0,
      clientConnectionsClosed: 0,
      responsesCompleted: 0,
      released: false,
      release,
      releasePromise,
    };
  };
  const state = name => {
    if (!observations.has(name)) observations.set(name, createObservation());
    return observations.get(name);
  };
  const server = createHTTPServer(async (request, response) => {
    const selected = selectors.get(String(request.headers["idempotency-key"] ?? "")) ?? mode;
    const record = state(selected);
    record.received++;
    record.openRequests++;
    let clientConnectionClosed = response.destroyed;
    const observeClientClose = () => {
      if (!response.writableEnded && !clientConnectionClosed) {
        clientConnectionClosed = true;
        record.clientConnectionsClosed++;
      }
    };
    response.once("close", observeClientClose);
    try {
      const chunks = [];
      let bytes = 0;
      for await (const chunk of request) {
        bytes += chunk.length;
        ensure(bytes <= 16 * 1024, "OBSERVER_REQUEST_TOO_LARGE");
        chunks.push(chunk);
      }
      if (["healthy", "cancel", "deadline"].includes(selected)) {
        let releaseTimer;
        try {
          await Promise.race([
            record.releasePromise,
            new Promise((_, reject) => { releaseTimer = setTimeout(() => reject(new Error("OBSERVER_RELEASE_TIMEOUT")), options.releaseTimeoutMs ?? 35_000); }),
          ]);
        } finally {
          clearTimeout(releaseTimer);
        }
        if (clientConnectionClosed || response.destroyed) return;
      }
      record.businessDispatches++;
      const headers = new Headers();
      for (const [name, value] of Object.entries(request.headers)) {
        if (["host", "connection", "transfer-encoding", "content-length"].includes(name) || value === undefined) continue;
        headers.set(name, Array.isArray(value) ? value.join(",") : value);
      }
      const upstream = await fetch(`${upstreamOrigin}${request.url}`, {
        method: request.method,
        headers,
        body: chunks.length ? Buffer.concat(chunks) : undefined,
        redirect: "manual",
        signal: AbortSignal.timeout(30_000),
      });
      const body = Buffer.from(await upstream.arrayBuffer());
      response.statusCode = upstream.status;
      response.setHeader("content-type", upstream.headers.get("content-type") ?? "application/json");
      response.setHeader("cache-control", "private, no-store");
      response.end(body);
      record.responsesCompleted++;
    } catch {
      if (!response.headersSent) {
        response.statusCode = 503;
        response.setHeader("content-type", "application/json");
      }
      if (!response.destroyed) response.end(JSON.stringify({ error: "referral_unavailable" }));
    } finally {
      response.removeListener("close", observeClientClose);
      record.openRequests--;
      record.closedRequests++;
    }
  });
  await new Promise((resolve, reject) => { server.once("error", reject); server.listen(port, "127.0.0.1", resolve); });
  const address = server.address();
  ensure(address && typeof address === "object", "OBSERVER_ADDRESS_MISSING");
  dispatchObserver = {
    port: address.port,
    server,
    arm(name, key) {
      if (key) selectors.set(key, name);
      else mode = name;
      observations.set(name, createObservation());
    },
    release(name) {
      const record = state(name);
      record.released = true;
      record.release();
    },
    async waitReceived(name) { return until(() => state(name).received > 0, `OBSERVER_${name.toUpperCase()}_RECEIVED`, 30_000); },
    async waitClientClosed(name) { return until(() => state(name).clientConnectionsClosed > 0, `OBSERVER_${name.toUpperCase()}_CLIENT_CLOSED`, 30_000); },
    async waitReleased(name) { return until(() => state(name).received > 0 && state(name).openRequests === 0, `OBSERVER_${name.toUpperCase()}_RELEASED`, 35_000); },
    snapshot(name) {
      const record = state(name);
      return {
        bffDispatches: record.received,
        goDispatches: record.businessDispatches,
        openHandlers: record.openRequests,
        completedHandlers: record.closedRequests,
        clientConnectionsClosed: record.clientConnectionsClosed,
        responsesCompleted: record.responsesCompleted,
        released: record.released,
      };
    },
  };
  return dispatchObserver;
}

export async function stopDispatchObserver() {
  if (!dispatchObserver) return;
  const current = dispatchObserver;
  dispatchObserver = undefined;
  current.server.closeAllConnections?.();
  await new Promise((resolve, reject) => current.server.close(error => error ? reject(error) : resolve()));
  ensure(await freePort(current.port) === current.port, "OBSERVER_PORT_NOT_RELEASED");
}

export async function startBFFIngressObserver(port, upstreamPort) {
  if (bffIngressObserver) return bffIngressObserver;
  const observations = new Map();
  const selectors = new Map();
  let mode = "pass";
  const state = name => {
    if (!observations.has(name)) observations.set(name, {
      received: 0,
      bodyChunksReceived: 0,
      bodyChunksForwarded: 0,
      requestBodiesCompleted: 0,
      upstreamConnections: 0,
      upstreamConnectionsClosed: 0,
      upstreamResponses: 0,
      responsesCompleted: 0,
      inboundAborted: 0,
      clientConnectionsClosed: 0,
      openHandlers: 0,
      completedHandlers: 0,
    });
    return observations.get(name);
  };
  const server = createHTTPServer((request, response) => {
    const selected = selectors.get(String(request.headers["idempotency-key"] ?? "")) ?? mode;
    const record = state(selected);
    record.received++;
    record.openHandlers++;
    let handlerCompleted = false;
    let upstreamResponseCompleted = false;
    const finishHandler = () => {
      if (handlerCompleted) return;
      handlerCompleted = true;
      record.openHandlers--;
      record.completedHandlers++;
    };
    response.once("finish", finishHandler);
    response.once("close", () => {
      if (!response.writableEnded) record.clientConnectionsClosed++;
      finishHandler();
    });
    const upstream = httpRequest({
      hostname: "127.0.0.1",
      port: upstreamPort,
      method: request.method,
      path: request.url,
      headers: request.headers,
    }, upstreamResponse => {
      record.upstreamResponses++;
      response.writeHead(upstreamResponse.statusCode ?? 502, upstreamResponse.headers);
      upstreamResponse.on("end", () => {
        upstreamResponseCompleted = true;
        record.responsesCompleted++;
      });
      upstreamResponse.pipe(response);
    });
    upstream.once("socket", socket => {
      const connected = () => { record.upstreamConnections++; };
      if (socket.connecting) socket.once("connect", connected);
      else connected();
    });
    upstream.once("close", () => {
      if (!upstreamResponseCompleted) record.upstreamConnectionsClosed++;
    });
    upstream.once("error", () => {
      if (!response.headersSent && !response.destroyed) response.writeHead(502, { "content-type": "application/json" });
      if (!response.destroyed) response.end(JSON.stringify({ error: "bff_unavailable" }));
    });
    request.on("data", chunk => {
      record.bodyChunksReceived++;
      if (!upstream.destroyed) upstream.write(chunk, () => { record.bodyChunksForwarded++; });
    });
    request.once("end", () => {
      record.requestBodiesCompleted++;
      if (!upstream.destroyed) upstream.end();
    });
    request.once("aborted", () => {
      record.inboundAborted++;
      upstream.destroy();
    });
    request.once("error", () => upstream.destroy());
  });
  await new Promise((resolve, reject) => { server.once("error", reject); server.listen(port, "127.0.0.1", resolve); });
  const address = server.address();
  ensure(address && typeof address === "object", "BFF_INGRESS_ADDRESS_MISSING");
  bffIngressObserver = {
    port: address.port,
    server,
    arm(name, key) {
      if (key) selectors.set(key, name);
      else mode = name;
      observations.delete(name);
      state(name);
    },
    async waitBodyForwarded(name) {
      return until(() => {
        const record = state(name);
        return record.received > 0 && record.bodyChunksReceived > 0 && record.bodyChunksForwarded > 0 && record.upstreamConnections > 0;
      }, `BFF_INGRESS_${name.toUpperCase()}_BODY_FORWARDED`, 30_000);
    },
    async waitClosed(name) {
      return until(() => {
        const record = state(name);
        return (record.inboundAborted > 0 || record.clientConnectionsClosed > 0) && record.upstreamConnectionsClosed > 0 && record.openHandlers === 0;
      }, `BFF_INGRESS_${name.toUpperCase()}_CLOSED`, 30_000);
    },
    async waitCompleted(name) {
      return until(() => state(name).received > 0 && state(name).openHandlers === 0, `BFF_INGRESS_${name.toUpperCase()}_COMPLETED`, 30_000);
    },
    snapshot(name) { return { ...state(name) }; },
  };
  return bffIngressObserver;
}

export async function stopBFFIngressObserver() {
  if (!bffIngressObserver) return;
  const current = bffIngressObserver;
  bffIngressObserver = undefined;
  current.server.closeAllConnections?.();
  await new Promise((resolve, reject) => current.server.close(error => error ? reject(error) : resolve()));
  ensure(await freePort(current.port) === current.port, "BFF_INGRESS_PORT_NOT_RELEASED");
}

async function startSecondaryApplication(origins, ports) {
  if (secondaryApplication) {
    ensure([secondaryApplication.supervisorPid, secondaryApplication.goPid, secondaryApplication.nextPid].every(processAlive), "SECONDARY_APPLICATION_STALE");
    return secondaryApplication;
  }
  const directory = path.resolve(manifest.directory, "m2-secondary");
  ensure(path.dirname(directory) === path.resolve(manifest.directory), "SECONDARY_DIRECTORY_INVALID");
  await unlink(path.join(directory, "ui", "node_modules")).catch(error => { if (error.code !== "ENOENT") throw error; });
  await rm(directory, { recursive: true, force: true });
  await mkdir(directory, { recursive: true });
  const current = structuredClone(await readJSON(path.join(manifest.directory, "current-application.json")));
  current.listen.port = ports.goSecondary;
  current.referrals.providerOrigin = origins.providerFaultOrigin;
  const currentConfig = path.join(directory, "current-application.json");
  await writeJSON(currentConfig, current);
  const services = structuredClone(await readJSON(path.join(manifest.directory, "services.json")));
  const sourceUI = path.resolve(services.uiDirectory);
  ensure(path.dirname(sourceUI) === path.resolve(manifest.directory), "SECONDARY_UI_SOURCE_INVALID");
  const secondaryUI = path.join(directory, "ui");
  await cp(sourceUI, secondaryUI, {
    recursive: true,
    filter: source => {
      const relative = path.relative(sourceUI, source);
      const first = relative.split(path.sep)[0];
      return !["node_modules", ".next"].includes(first);
    },
  });
  await symlink(path.join(sourceUI, "node_modules"), path.join(secondaryUI, "node_modules"), "junction");
  services.uiDirectory = secondaryUI;
  services.goArgs = ["-config", currentConfig, "-shutdown-file", path.join(directory, "stop-go")];
  services.goEnvironment = {};
  services.goPort = ports.goSecondary;
  services.webPort = ports.nextSecondary;
  Object.assign(services.nextEnvironment, {
    AUTH_URL: origins.secondaryPublicOrigin,
    LISTINGKIT_PUBLIC_BASE_URL: origins.secondaryPublicOrigin,
    ZITADEL_REDIRECT_URI: `${origins.secondaryPublicOrigin}/api/auth/callback/zitadel`,
    ZITADEL_POST_LOGOUT_REDIRECT_URI: origins.secondaryPublicOrigin,
    LISTINGKIT_SERVICE_API_BASE: `http://127.0.0.1:${ports.observer}/api/v1`,
    COMMERCIAL_API_ORIGIN: `http://127.0.0.1:${ports.goSecondary}`,
  });
  await writeJSON(path.join(directory, "services.json"), services);
  for (const name of ["stop-go", "stop-next", "stop-services", "go-ready.json", "next-ready.json", "services-stopped.json", "processes.json"]) await unlink(path.join(directory, name)).catch(error => { if (error.code !== "ENOENT") throw error; });
  const child = spawn(process.execPath, [path.join(repo, "scripts", "issue357", "serve.mjs"), directory], { cwd: directory, detached: true, windowsHide: true, stdio: "ignore" });
  await new Promise((resolve, reject) => { child.once("spawn", resolve); child.once("error", () => reject(new Error("SECONDARY_SUPERVISOR_START_FAILED"))); });
  child.unref();
  secondaryApplication = {
    instanceId: `${manifest.runId}:secondary:${++secondaryGeneration}`,
    directory,
    supervisorPid: child.pid,
    goPid: undefined,
    nextPid: undefined,
    goPort: ports.goSecondary,
    nextPort: ports.nextSecondary,
    databaseId: manifest.resources?.[`${manifest.project}-commercial-db`]?.id,
    ready: false,
  };
  const started = await until(async () => {
    const stopped = await readJSON(path.join(directory, "services-stopped.json")).catch(error => error.code === "ENOENT" ? null : Promise.reject(error));
    if (stopped) return { failed: true };
    try {
      await readJSON(path.join(directory, "go-ready.json"));
      await readJSON(path.join(directory, "next-ready.json"));
      return { ready: true };
    } catch (error) {
      if (error.code !== "ENOENT") throw error;
      return false;
    }
  }, "SECONDARY_APPLICATION_START", 300_000);
  ensure(started.ready === true && !started.failed, "SECONDARY_APPLICATION_START_FAILED");
  const processes = await readJSON(path.join(directory, "processes.json"));
  ensure([processes.supervisor?.pid, processes.go?.pid, processes.next?.pid].every(processAlive), "SECONDARY_APPLICATION_PROCESS_MISSING");
  const provider = await fetch(`http://127.0.0.1:${ports.nextSecondary}/api/auth/providers`, { signal: AbortSignal.timeout(90_000) });
  ensure(provider.status === 200 && (await provider.json()).zitadel, "SECONDARY_AUTH_PROVIDER_MISSING");
  const unauthenticated = await fetch(`http://127.0.0.1:${ports.goSecondary}/api/v1/account/profile`, { signal: AbortSignal.timeout(10_000) });
  ensure(unauthenticated.status === 401, "SECONDARY_GO_HEALTH_FAILED");
  const databaseId = manifest.resources?.[`${manifest.project}-commercial-db`]?.id;
  ensure(databaseId, "SECONDARY_DATABASE_IDENTITY_MISSING");
  secondaryApplication = {
    ...secondaryApplication,
    supervisorPid: processes.supervisor.pid,
    goPid: processes.go.pid,
    nextPid: processes.next.pid,
    databaseId,
    ready: true,
  };
  report.secondaryApplicationStarts = secondaryGeneration;
  return secondaryApplication;
}

async function stopSecondaryApplication(application = secondaryApplication) {
  if (!application) return;
  await writePrivate(path.join(application.directory, "stop-services"), "stop\n");
  const latest = await readJSON(path.join(application.directory, "processes.json")).catch(error => error.code === "ENOENT" ? {} : Promise.reject(error));
  application.supervisorPid = latest.supervisor?.pid ?? application.supervisorPid;
  application.goPid = latest.go?.pid ?? application.goPid;
  application.nextPid = latest.next?.pid ?? application.nextPid;
  await until(() => !processAlive(application.supervisorPid) && !processAlive(application.goPid) && !processAlive(application.nextPid), "SECONDARY_APPLICATION_STOP", 40_000);
  const stopped = await readJSON(path.join(application.directory, "services-stopped.json")).catch(error => error.code === "ENOENT" ? null : Promise.reject(error));
  if (application.ready) ensure(stopped?.passed === true, "SECONDARY_APPLICATION_STOP_FAILED");
  for (const port of [application.goPort, application.nextPort]) ensure(await freePort(port) === port, "SECONDARY_APPLICATION_PORT_NOT_RELEASED");
  if (secondaryApplication?.instanceId === application.instanceId) secondaryApplication = undefined;
}

async function inspectSecondaryReleased(application) {
  return {
    processes: [application.supervisorPid, application.goPid, application.nextPid].filter(processAlive).length,
    listeners: (await Promise.all([application.goPort, application.nextPort].map(async port => {
      try { return await freePort(port) === port ? 0 : 1; } catch { return 1; }
    }))).reduce((sum, value) => sum + value, 0),
  };
}

async function restartConfiguredApplications(ports) {
  const before = await readJSON(path.join(manifest.directory, "processes.json"));
  const databaseID = manifest.resources?.[`${manifest.project}-commercial-db`]?.id;
  ensure(databaseID, "DATABASE_IDENTITY_MISSING");
  await writePrivate(path.join(manifest.directory, "stop-next"), "stop\n");
  await until(() => !processAlive(before.next?.pid), "CONFIGURED_NEXT_STOP", 35_000);
  await writePrivate(path.join(manifest.directory, "stop-services"), "stop\n");
  await until(() => !processAlive(before.go?.pid) && !processAlive(before.supervisor?.pid), "CONFIGURED_APPLICATION_STOP", 35_000);
  const stopped = await readJSON(path.join(manifest.directory, "services-stopped.json"));
  ensure(stopped.passed === true, "CONFIGURED_APPLICATION_STOP_FAILED");
  ensure(!processAlive(before.go?.pid) && !processAlive(before.next?.pid), "OLD_APPLICATION_PROCESS_ALIVE");
  await startConfiguredApplications(ports);
  const after = await readJSON(path.join(manifest.directory, "processes.json"));
  ensure(["go", "next"].every(name => before[name]?.pid !== after[name]?.pid && before[name]?.started !== after[name]?.started), "APPLICATION_PROCESS_IDENTITY_REUSED");
  const current = await readJSON(path.join(manifest.directory, "manifest.json"));
  const database = await inspectDockerResource("container", `${manifest.project}-commercial-db`);
  ensure(current.runId === manifest.runId && current.resources?.[`${manifest.project}-commercial-db`]?.id === databaseID && database?.Id === databaseID, "DATABASE_IDENTITY_CHANGED");
  manifest = { ...current, directory: manifest.directory };
  return { processMemoryLost: true, oldProcessesExited: true, processIdentitiesChanged: true, persistentDatabaseRetained: true };
}

async function providerStatus(pathname, body, token, method = "POST") {
  const response = await fetch(`${manifest.origins.issuer}${pathname}`, {
    method,
    headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json", "Connect-Protocol-Version": "1" },
    body: body === undefined ? undefined : JSON.stringify(body),
    redirect: "error",
    signal: AbortSignal.timeout(30_000),
  });
  const bytes = new Uint8Array(await response.arrayBuffer());
  ensure(bytes.byteLength <= 1024 * 1024, "PROVIDER_RESPONSE_TOO_LARGE");
  return response.status;
}

async function runBaseRuntimeControl(action, user, organization) {
  ensure(manifest?.runId, "RUNTIME_ID_MISSING");
  const args = [runtimeScript, action, "--run", manifest.runId];
  if (user) args.push("--user", user);
  if (organization) args.push("--org", organization);
  await run(process.execPath, args);
}

function processAlive(pid) { try { process.kill(pid, 0); return true; } catch { return false; } }

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

async function providerTLSRead(origin, pathname, caFile, token) {
  const ca = await readFile(caFile);
  const target = new URL(pathname, origin);
  return new Promise((resolve, reject) => {
    const request = httpsRequest(target, {
      method: "GET",
      ca,
      rejectUnauthorized: true,
      headers: { Authorization: `Bearer ${token}`, "Content-Type": "application/json" },
      timeout: 10_000,
    }, response => {
      const chunks = [];
      let size = 0;
      response.on("data", chunk => {
        size += chunk.length;
        if (size > 1024 * 1024) request.destroy(new Error("PROVIDER_TLS_RESPONSE_TOO_LARGE"));
        else chunks.push(chunk);
      });
      response.on("end", () => {
        let payload = {};
        try { payload = chunks.length ? JSON.parse(Buffer.concat(chunks).toString("utf8")) : {}; } catch {}
        resolve({ status: response.statusCode, payload });
      });
    });
    request.once("timeout", () => request.destroy(new Error("PROVIDER_TLS_TIMEOUT")));
    request.once("error", reject);
    request.end();
  });
}

async function relationshipCountForSubject(subject) {
  ensure(/^[A-Za-z0-9._:-]{1,200}$/.test(subject ?? ""), "RELATIONSHIP_SUBJECT_INVALID");
  const value = await run("docker", ["--host", dockerHost, "exec", `${manifest.project}-commercial-db`, "psql", "-At", "-U", "issue357", "-d", "issue357", "-c", `SELECT COUNT(*) FROM public.referral_relations WHERE subject='${subject}'`]);
  ensure(/^\d+$/.test(value), "RELATIONSHIP_COUNT_INVALID");
  return Number(value);
}

async function postCompletion(api, origin, expectedSubject) {
  const response = await api.post(`${origin}/api/account/referrals/complete`, {
    headers: { Origin: origin, "Sec-Fetch-Site": "same-origin", "X-Expected-User-ID": expectedSubject },
  });
  const receipt = await response.json().catch(() => ({}));
  return { status: response.status(), receipt };
}

export async function submitOfficialLoginStep(page, input, submit, value) {
  await waitForReactHydration(page, input);
  await waitForReactHydration(page, submit);
  await input.fill(value);
  await submit.click();
}

async function login(page, origin, credential, target) {
  let stage = "entry";
  try {
    await page.goto(`${origin}${target}`, { waitUntil: "load" }).catch(() => { throw new Error("PUBLIC_PROXY_PAGE_LOAD_FAILED"); });
    stage = "username";
    const username = page.getByTestId("username-text-input");
    await username.waitFor({ state: "visible", timeout: 45_000 }).catch(() => { throw new Error("OFFICIAL_USERNAME_PAGE_MISSING"); });
    await submitOfficialLoginStep(page, username, page.getByTestId("submit-button"), credential.username);
    stage = "password";
    const password = page.getByTestId("password-text-input");
    await password.waitFor({ state: "visible", timeout: 30_000 }).catch(() => { throw new Error("OFFICIAL_PASSWORD_PAGE_MISSING"); });
    await submitOfficialLoginStep(page, password, page.getByTestId("submit-button"), credential.password);
    stage = "callback";
    await page.waitForURL(url => url.origin === origin && url.pathname === target, { timeout: 45_000 })
      .catch(() => { throw new Error("OIDC_CALLBACK_DID_NOT_RETURN"); });
  } catch (error) {
    if (outputDirectory) {
      const diagnosticIndex = report.loginDiagnostics = (report.loginDiagnostics ?? 0) + 1;
      const diagnosticPath = path.join(outputDirectory, `official-login-diagnostic-${diagnosticIndex}.json`);
      await writeJSON(diagnosticPath, {
        stage,
        code: safeCode(error),
        pathname: (() => { try { return new URL(page.url()).pathname; } catch { return "unknown"; } })(),
        usernameVisible: await page.getByTestId("username-text-input").isVisible().catch(() => false),
        passwordVisible: await page.getByTestId("password-text-input").isVisible().catch(() => false),
        errorVisible: await page.getByTestId("error").isVisible().catch(() => false),
      }).catch(() => {});
    }
    throw error;
  }
}

async function waitForMessage(mailPort, email, providerOrigin, excludedMessageId) {
  let latest = { messages: [] };
  let matching;
  let detail;
  try {
    return await until(async () => {
      latest = await (await fetch(`http://127.0.0.1:${mailPort}/api/v1/messages`, { signal: AbortSignal.timeout(2_000) })).json();
      matching = latest.messages?.find(message => message.ID !== excludedMessageId && message.To?.some(recipient => recipient.Address === email));
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
  browser = await chromium.launch({ headless: process.env.ISSUE413_SCREEN_READER_SESSION !== "1" });
  await initializeScreenReaderSession();
  const referrer = await readJSON(path.join(manifest.directory, "viewer.credentials.json"));
  let referrerContext = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN", ignoreHTTPSErrors: true });
  let referrerPage = await referrerContext.newPage();
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

  let context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN", ignoreHTTPSErrors: true,
    extraHTTPHeaders: { "X-Forwarded-For": "127.0.0.1", "X-ListingKit-Client-IP": "127.0.0.1" } });
  let page = await context.newPage();
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
  const restartContextsAndApplications = async onResponse => {
    const urls = [page.url(), referrerPage.url()];
    const states = await Promise.all([context.storageState(), referrerContext.storageState()]);
    await Promise.all([context.close(), referrerContext.close()]);
    const evidence = await restartConfiguredApplications(ports);
    context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN", ignoreHTTPSErrors: true, storageState: states[0], extraHTTPHeaders: { "X-Forwarded-For": "127.0.0.1", "X-ListingKit-Client-IP": "127.0.0.1" } });
    referrerContext = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN", ignoreHTTPSErrors: true, storageState: states[1] });
    page = await context.newPage(); referrerPage = await referrerContext.newPage(); if (onResponse) page.on("response", onResponse);
    await Promise.all([page.goto(urls[0], { waitUntil: "load" }), referrerPage.goto(urls[1], { waitUntil: "load" })]);
    return evidence;
  };
  await check("registration_ui_desktop_and_automated_accessibility", async () => {
    try {
      await page.goto(`${origins.publicOrigin}/referrals/register?code=${encodeURIComponent(code)}`);
      await page.getByRole("heading", { name: "接受好友邀请" }).waitFor({ state: "visible", timeout: 30_000 });
      if (screenReaderSession) {
        await collectScreenReaderObservation("registration-initial", page, {
          expected: ["main_and_accept_invitation_heading", "readonly_invitation_code", "email_given_name_family_name_labels", "keyboard_focus", "native_validation_without_post"],
          runnerActions: ["opened_empty_registration_page", "did_not_submit_registration"],
        });
        ensure(registrationRequests.start === 0 && await page.locator("input:invalid").count() >= 3, "SCREEN_READER_REGISTRATION_VALIDATION_STATE_INVALID");
      }
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
      const registrationAction = await prepareScreenReaderAction(
        "registration-pending",
        page,
        submitButton,
        request => request.method() === "POST" && new URL(request.url()).pathname === "/api/referral-registration",
        { expected: ["operator_activates_start_registration", "focused_button_changes_to_submitting_and_disabled", "original_client_and_bff_deadlines_unchanged"] },
      );
      await responseLost;
      const registrationTransitions = await finishScreenReaderAction(page, registrationAction);
      if (screenReaderSession) ensure(registrationTransitions.some(item => item.text === "正在提交…" && item.disabled === true), "SCREEN_READER_REGISTRATION_PENDING_NOT_OBSERVED");
      ensure(lostAdmission?.status === 200, "ADMISSION_LOST_RESPONSE_NOT_COMMITTED");
      ensure(/^[A-Za-z0-9._:-]{1,200}$/.test(lostAdmission.payload?.intentID ?? ""), "ADMISSION_LOST_INTENT_INVALID");
      const fixedSubject = await run("docker", ["--host", dockerHost, "exec", `${manifest.project}-commercial-db`, "psql", "-At", "-U", "issue357", "-d", "issue357", "-c", `SELECT subject FROM public.registration_intents WHERE id='${lostAdmission.payload.intentID}'`]);
      ensure(/^[A-Za-z0-9._:-]{1,200}$/.test(fixedSubject), "ADMISSION_FIXED_SUBJECT_INVALID");
      await page.getByText("暂时无法确认注册结果").waitFor({ state: "visible", timeout: 15_000 });
      ensure(await page.getByLabel("邮箱").isDisabled() || await page.getByLabel("邮箱").getAttribute("readonly") !== null, "ADMISSION_INPUT_NOT_LOCKED");
      if (screenReaderSession) {
        await collectScreenReaderObservation("registration-pending", page, {
          checkpointAttemptId: registrationAction.checkpointAttemptId,
          timeoutMs: 60_000,
          stateObservedAt: registrationTransitions.find(item => item.text === "正在提交…")?.at,
          expected: ["actual_operator_action", "actual_pending_transition", "request_completed_without_manual_deadline_extension"],
          stateAttempt: { triggeredBy: "operator", requestObservedAt: registrationAction.requestObservedAt, transitions: registrationTransitions },
          runnerActions: ["filled_valid_registration_fields", "armed_existing_response_loss_injection", "did_not_extend_product_deadlines"],
        });
        await collectScreenReaderObservation("registration-unknown", page, {
          timeoutMs: screenReaderCreateRecoveryLimitMs,
          expected: ["alert_unknown_outcome", "locked_original_fields", "original_request_retry_control"],
          runnerActions: ["committed_upstream_then_lost_browser_response", "will_activate_original_retry_after_operator_observation"],
        });
      }
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
      if (screenReaderSession) await collectScreenReaderObservation("registration-mail-pending", page, {
        timeoutMs: 5 * 60_000,
        expected: ["official_mail_pending_status", "official_verification_explanation", "continue_to_official_login_control"],
        runnerActions: ["replayed_original_key_and_payload", "resumed_original_intent", "did_not_pause_during_create_lease_or_provider_call"],
      });
      await page.screenshot({ path: path.join(outputDirectory, "registration-mail-pending-desktop.png"), fullPage: true });
      ensure(admissionRequest?.body && /^[A-Za-z0-9_-]{43,128}$/.test(admissionRequest.key), "ADMISSION_REQUEST_NOT_OBSERVED");
      const replayedSubject = await run("docker", ["--host", dockerHost, "exec", `${manifest.project}-commercial-db`, "psql", "-At", "-U", "issue357", "-d", "issue357", "-c", `SELECT subject FROM public.registration_intents WHERE id='${admittedIntentID}'`]);
      ensure(replayedSubject === fixedSubject, "ADMISSION_REPLAY_CHANGED_SUBJECT");
      matrixRecord("A", "admission_response_loss_same_request_and_receipt", "PASS", { originalIntentPreserved: true, originalSubjectPreserved: true, originalSecretPreserved: true });
      matrixRecord("F", "registration_desktop_axe", "PASS", { axe: desktopAxe });
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
      data: { ...JSON.parse(admissionRequest.body), code: `${code}x` },
      headers: { Origin: origins.publicOrigin, "Sec-Fetch-Site": "same-origin", "Idempotency-Key": admissionRequest.key },
      maxRedirects: 0,
    });
    ensure(changed.status() === 409, "IDEMPOTENCY_PAYLOAD_CONFLICT_NOT_REJECTED");
    matrixRecord("A", "same_key_original_receipt_and_different_payload_conflict", "PASS", { replayStatus: 200, conflictStatus: 409 });
    matrixRecord("A", "multiple_referral_code_conflict", "PASS", { conflictStatus: 409 });
    matrixRecord("E", "trusted_proxy_overwrites_forged_forwarding", "PASS", { untrustedDirectStatus: 403 });
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
    matrixRecord("A", "reload_with_recovery_fragment_resumes_original", "PASS");
    matrixRecord("F", "registration_narrow_keyboard_axe", "PASS", { axe });
    return { viewport: "390x844", automated: "PASS", ...axe };
  });
  await matrixCheck("A", "fresh_browser_empty_form_no_auto_submit", async () => {
    const fresh = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", ignoreHTTPSErrors: true });
    const freshPage = await fresh.newPage();
    let writes = 0;
    freshPage.on("request", request => { if (request.method() === "POST" && new URL(request.url()).pathname.startsWith("/api/referral-registration")) writes++; });
    await freshPage.goto(`${origins.publicOrigin}/referrals/register?code=${encodeURIComponent(code)}`);
    await freshPage.getByRole("button", { name: "开始注册" }).waitFor({ state: "visible" });
    await freshPage.reload();
    await freshPage.getByRole("button", { name: "开始注册" }).waitFor({ state: "visible" });
    ensure(writes === 0, "FRESH_BROWSER_SILENTLY_RECOVERED");
    await freshPage.goto(`${origins.publicOrigin}/referrals/register?code=${encodeURIComponent(code)}#intentID=${encodeURIComponent(admittedIntentID)}`);
    await freshPage.getByRole("button", { name: "开始注册" }).waitFor({ state: "visible" });
    ensure(writes === 0, "INCOMPLETE_RECOVERY_CAPABILITY_DISPATCHED");
    matrixRecord("A", "lost_credentials_safe_rejection", "PASS", { recoveryDispatched: false, identitySwitched: false });
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
  const initialVerification = await check("official_email_verification", async () => {
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
    await page.waitForURL(url => url.origin === origins.providerOrigin && url.pathname === "/ui/v2/login/authenticator/set", { timeout: 45_000 })
      .catch(() => { throw new Error("BLOCKER_NO_OFFICIAL_AUTHENTICATOR_SETUP"); });
    return { sameSubject: true, officialProvider: true, subject, authenticatorURL: page.url() };
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
  const reverified = await matrixCheck("B", "official_verification_interruption_new_browser_reverify", async () => {
    const observation = await runReverificationControl({
      verifyInitial: async () => {
        await context.close();
        return { subject, authenticatorURL: initialVerification.authenticatorURL };
      },
      openFreshBrowser: async authenticatorURL => {
        const freshContext = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN", ignoreHTTPSErrors: true });
        return { context: freshContext, page: await freshContext.newPage(), authenticatorURL };
      },
      rejectInterruptedAuthenticator: async fresh => {
        await fresh.page.goto(fresh.authenticatorURL, { waitUntil: "load" });
        await delay(1_000);
        return !(await fresh.page.getByRole("link", { name: "Password" }).isVisible().catch(() => false));
      },
      requestOfficialReverification: async expectedSubject => {
        await provider(`/v2/users/${encodeURIComponent(expectedSubject)}/invite_code`, {
          sendCode: { urlTemplate: `${origins.providerOrigin}/ui/v2/login/verify?code={{.Code}}&userId={{.UserID}}&organization={{.OrgID}}&invite=true` },
        }, machine.token);
        const replacement = await waitForMessage(ports.mail, email, origins.providerOrigin, message.id);
        return { subject: expectedSubject, messageId: replacement.id, link: replacement.link };
      },
      verifyReplacement: async (fresh, replacement) => {
        const replacementURL = new URL(replacement.link);
        ensure(replacementURL.searchParams.get("userId") === subject, "REVERIFICATION_SUBJECT_CHANGED");
        await fresh.page.goto(replacement.link, { waitUntil: "load" });
        const input = fresh.page.getByTestId("code-text-input");
        const replacementCode = replacementURL.searchParams.get("code");
        ensure(replacementCode, "REVERIFICATION_CODE_MISSING");
        await input.waitFor({ state: "visible", timeout: 30_000 });
        await input.fill(replacementCode);
        await fresh.page.getByTestId("submit-button").click();
        await until(async () => {
          const user = await provider(`/v2/users/${encodeURIComponent(subject)}`, undefined, machine.token, "GET");
          return user.user?.human?.email?.isVerified === true;
        }, "EMAIL_REVERIFIED", 45_000);
        await fresh.page.waitForURL(url => url.origin === origins.providerOrigin && url.pathname === "/ui/v2/login/authenticator/set", { timeout: 45_000 });
        return { subject };
      },
      configureAuthenticator: async fresh => {
        await check("official_authenticator_setup", async () => {
          const passwordChoice = fresh.page.getByRole("link", { name: "Password" });
          await passwordChoice.waitFor({ state: "visible", timeout: 20_000 }).catch(() => { throw new Error("BLOCKER_NO_OFFICIAL_PASSWORD_CHOICE"); });
          await passwordChoice.click();
          await fresh.page.waitForURL(url => url.origin === origins.providerOrigin && url.pathname === "/ui/v2/login/password/set", { timeout: 30_000 });
          const password = fresh.page.getByTestId("password-set-text-input");
          const confirmation = fresh.page.getByTestId("password-set-confirm-text-input");
          await password.waitFor({ state: "visible", timeout: 20_000 });
          registeredPassword = `A9!${randomBytes(18).toString("hex")}`;
          await password.fill(registeredPassword);
          await confirmation.fill(registeredPassword);
          const submit = fresh.page.getByTestId("submit-button");
          const submitElement = await submit.elementHandle();
          ensure(submitElement, "BLOCKER_OFFICIAL_PASSWORD_ACTION_MISSING");
          await fresh.page.waitForFunction(button => !button.disabled, submitElement, { timeout: 30_000 });
          await submit.click();
          await fresh.page.waitForURL(url => url.pathname !== "/ui/v2/login/password/set", { timeout: 45_000 });
          return { subject, method: "password" };
        });
      },
      completeOIDC: async fresh => {
        return check("generic_oidc_authjs_login", async () => {
          ensure(typeof registeredPassword === "string" && registeredPassword.length >= 20, "OFFICIAL_PASSWORD_NOT_RETAINED");
          await fresh.context.clearCookies();
          await login(fresh.page, origins.publicOrigin, { username: email, password: registeredPassword }, "/workbench/account/referrals/complete");
          return { subject };
        });
      },
      closeFreshBrowser: fresh => fresh.context.close(),
    });
    return { ...evaluateReverificationControl(observation), precondition: "verified_without_authenticator", injection: "fresh_browser_without_verification_check", positiveControl: "replacement_official_verification", observation: "same_subject_authenticator_and_oidc_completed", invariants: ["fixed_subject", "official_login_only"] };
  });
  ensure(reverified, "M1_REVERIFICATION_REQUIRED");

  context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN", ignoreHTTPSErrors: true,
    extraHTTPHeaders: { "X-Forwarded-For": "127.0.0.1", "X-ListingKit-Client-IP": "127.0.0.1" } });
  page = await context.newPage();
  await login(page, origins.publicOrigin, { username: email, password: registeredPassword }, "/workbench/account/referrals/complete");

  await matrixCheck("B", "another_subject_cannot_claim_intent", async () => {
    const response = await referrerContext.request.post(`${origins.publicOrigin}/api/account/referrals/complete`, {
      headers: { Origin: origins.publicOrigin, "Sec-Fetch-Site": "same-origin", "X-Expected-User-ID": manifest.users.viewer.id },
    });
    ensure(response.status() === 404, "OTHER_SUBJECT_CLAIM_NOT_REJECTED");
    return { rejectedStatus: 404 };
  });

  let completionReceipt;
  let firstCompletionReceiptsMatched = false;
  await check("same_subject_completion", async () => {
    let button = page.getByRole("button", { name: "完成推广关系" });
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
            const headers = { Origin: origins.publicOrigin, "Sec-Fetch-Site": "same-origin", "X-Expected-User-ID": subject };
            const responses = await Promise.all([route.fetch(), ...Array.from({ length: 3 }, () => context.request.post(`${origins.publicOrigin}/api/account/referrals/complete`, { headers }))]);
            const payloads = await Promise.all(responses.map(response => response.json()));
            ensure(responses.every(response => response.status() === 200), "CONCURRENT_FIRST_COMPLETION_FAILED");
            firstCompletionReceiptsMatched = payloads.every(payload => JSON.stringify(payload) === JSON.stringify(payloads[0]));
            ensure(firstCompletionReceiptsMatched, "CONCURRENT_FIRST_COMPLETION_CHANGED_RECEIPT");
            lostReceipt = { status: responses[0].status(), payload: payloads[0] };
            await route.abort("connectionreset");
            resolve();
          } catch { reject(new Error("COMPLETION_RESPONSE_LOSS_INJECTION_FAILED")); }
        }, { times: 1 }).catch(reject);
      });
      const completionAction = await prepareScreenReaderAction(
        "completion-initial-pending",
        page,
        button,
        request => request.method() === "POST" && new URL(request.url()).pathname === "/api/account/referrals/complete",
        { expected: ["completion_heading_and_explanation", "real_projection_and_unavailable_revenue", "operator_activates_complete", "focused_button_changes_to_confirming_and_disabled"] },
      );
      await completionLost;
      const completionTransitions = await finishScreenReaderAction(page, completionAction);
      if (screenReaderSession) ensure(completionTransitions.some(item => item.text === "正在确认…" && item.disabled === true), "SCREEN_READER_COMPLETION_PENDING_NOT_OBSERVED");
      ensure(lostReceipt?.status === 200, "COMPLETION_LOST_RESPONSE_NOT_COMMITTED");
      await page.getByText("推广服务暂不可用").waitFor({ state: "visible", timeout: 30_000 })
        .catch(async () => {
          if (await page.getByText("推广关系已确认").isVisible().catch(() => false)) throw new Error("COMPLETION_RESPONSE_LOSS_NOT_OBSERVED");
          throw new Error("COMPLETION_RESPONSE_LOSS_UI_TIMEOUT");
        });
      if (screenReaderSession) await collectScreenReaderObservation("completion-initial-pending", page, {
        checkpointAttemptId: completionAction.checkpointAttemptId,
        stateObservedAt: completionTransitions.find(item => item.text === "正在确认…")?.at,
        expected: ["initial_completion_content", "actual_operator_action", "actual_pending_transition", "request_completed_without_manual_deadline_extension"],
        stateAttempt: { triggeredBy: "operator", requestObservedAt: completionAction.requestObservedAt, transitions: completionTransitions },
        runnerActions: ["armed_existing_completion_response_loss_injection", "did_not_extend_client_or_bff_deadlines"],
      });
      const restartEvidence = await restartContextsAndApplications(observe);
      button = page.getByRole("button", { name: "完成推广关系" });
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
      if (screenReaderSession) await collectScreenReaderObservation("completion-receipt-projection-unavailable", page, {
        expected: ["committed_receipt_status_and_time", "projection_unavailable_without_rewriting_receipt", "real_relationship_count_context"],
        runnerActions: ["replayed_durable_completion_receipt", "injected_existing_projection_503", "did_not_create_another_relationship"],
      });
      await page.screenshot({ path: path.join(outputDirectory, "completion-receipt-with-projection-failure.png"), fullPage: true });
      matrixRecord("C", "completion_response_loss_restart_receipt_replay", "PASS", { ...restartEvidence, originalReceiptPreserved: true });
      matrixRecord("C", "projection_failure_preserves_completion_receipt", "PASS", { injectionBoundary: "browser projection response", projectionStatus: 503 });
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
      matrixRecord("F", "completion_desktop_narrow_keyboard_axe", "PASS", { desktop: desktopAxe, narrow: narrowAxe });
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
      await page.screenshot({ path: path.join(outputDirectory, "completion-stage-failure.png"), fullPage: true }).catch(() => {});
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
    matrixRecord("C", "concurrent_first_completion_one_relationship", "PASS", { requests: 4, receiptMatched: firstCompletionReceiptsMatched, relations: 1 });
    return { requests: 4, relations: 1, receipts: 1, consumedIntents: 1 };
  });
  await check("referrer_real_count", async () => {
    await referrerPage.reload();
    await referrerPage.getByText("已建立关系").waitFor({ state: "visible" });
    ensure((await referrerPage.locator("article").filter({ hasText: "已建立关系" }).locator("strong").textContent()) === "1", "REFERRER_COUNT_NOT_ONE");
    if (screenReaderSession) await collectScreenReaderObservation("overview-entry-available", referrerPage, {
      expected: ["overview_heading", "real_relationship_count_one", "revenue_unavailable", "referral_code", "open_invitation_link_and_return"],
      runnerActions: ["loaded_viewer_personal_projection", "did_not_activate_invitation_link"],
    });
    return { count: 1 };
  });
  await matrixCheck("D", "authenticated_get_is_pure_on_referral_tables", async () => {
    const before = await referralTablesDigest();
    const response = await referrerContext.request.get(`${origins.publicOrigin}/api/account/referrals`, {
      headers: { "X-Expected-User-ID": manifest.users.viewer.id },
    });
    ensure(response.status() === 200, "SELF_READ_FAILED");
    ensure(await referralTablesDigest() === before, "SELF_READ_MUTATED_REFERRAL_TABLES");
    return { httpStatus: 200, referralTablesChanged: false };
  });
  await matrixCheck("D", "enterprise_removed_switching", async () => {
    let enterpriseStage = "precondition";
    const enterpriseContext = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN", ignoreHTTPSErrors: true,
      extraHTTPHeaders: { "X-Forwarded-For": "127.0.0.1", "X-ListingKit-Client-IP": "127.0.0.1" } });
    const enterprisePage = await enterpriseContext.newPage();
    let lateRelease;
    let lateReadyResolve;
    let lateResultResolve;
    const lateReady = new Promise(resolve => { lateReadyResolve = resolve; });
    const lateResult = new Promise(resolve => { lateResultResolve = resolve; });
    let observation;
    try {
      enterpriseStage = "dedicated_viewer_login";
      const viewerCredential = await readJSON(path.join(manifest.directory, "viewer.credentials.json"));
      await login(enterprisePage, origins.publicOrigin, viewerCredential, "/workbench/account/organization");
      observation = await runEnterpriseRemovalControl({
      removedOrganizationId: manifest.organizations.B.id,
      readContext: async phase => {
        enterpriseStage = `context_${phase}`;
        return until(async () => {
          const response = await enterpriseContext.request.get(`${origins.publicOrigin}/api/workbench/context`);
          if (response.status() !== 200) return null;
          const payload = await response.json();
          const organizationIds = payload.organizations?.map(organization => organization.id) ?? [];
          if (phase === "after" && organizationIds.includes(manifest.organizations.B.id)) return null;
          return { subject: payload.user?.id, organizationIds };
        }, `ENTERPRISE_CONTEXT_${phase.toUpperCase()}`, phase === "after" ? 75_000 : 30_000);
      },
      switchOrganization: async organizationId => {
        enterpriseStage = organizationId === manifest.organizations.B.id ? "switch_removed" : "switch_fallback";
        if (new URL(enterprisePage.url()).pathname !== "/workbench/account/organization") {
          await enterprisePage.goto(`${origins.publicOrigin}/workbench/account/organization`, { waitUntil: "load" });
        }
        await enterprisePage.getByLabel("当前企业").waitFor({ state: "visible", timeout: 30_000 });
        const switcher = enterprisePage.getByRole("combobox", { name: "当前企业" });
        if (await switcher.count()) {
          if (await switcher.inputValue() !== organizationId) {
            enterpriseStage = organizationId === manifest.organizations.B.id ? "switch_removed_submit" : "switch_fallback_submit";
            await waitForReactHydration(enterprisePage, switcher);
            const responsePromise = enterprisePage.waitForResponse(response => {
              const url = new URL(response.url());
              return response.request().method() === "PUT" && url.pathname === "/api/workbench/context/effective-organization";
            }, { timeout: 30_000 }).catch(() => null);
            await submitOrganizationSelection({ switcher, organizationId, responsePromise });
          }
          enterpriseStage = organizationId === manifest.organizations.B.id ? "switch_removed_visible" : "switch_fallback_visible";
          await enterprisePage.waitForFunction(({ id }) => document.querySelector("select")?.value === id, { id: organizationId }, { timeout: 45_000 });
        }
        if (organizationId === manifest.organizations.B.id) {
          await enterprisePage.getByText(`当前有效企业：${organizationId}`).waitFor({ state: "visible", timeout: 45_000 });
        }
      },
      beginLateOrganizationRead: () => {
        enterpriseStage = "late_read";
        const gate = new Promise(resolve => { lateRelease = resolve; });
        void (async () => {
          try {
            await enterprisePage.route("**/api/account/organization", async route => {
              let upstream;
              try {
                upstream = await route.fetch();
                const observed = await observeEnterpriseLateResponse(upstream, manifest.users.viewer.id, manifest.organizations.B.id);
                lateReadyResolve();
                await gate;
                try {
                  await route.fulfill({ response: upstream });
                  lateResultResolve({ ...observed, applied: false, outcome: "delivered_to_cancelled_request" });
                } catch {
                  lateResultResolve({ ...observed, applied: false, outcome: "cancelled_before_delivery" });
                }
              } catch {
                lateReadyResolve();
                lateResultResolve({ organizationId: "unknown", upstreamValidated: false, applied: true, outcome: "late_response_invalid" });
                if (upstream) await route.fulfill({ response: upstream }).catch(() => {});
                else await route.abort().catch(() => {});
              }
            }, { times: 1 });
            const refresh = enterprisePage.getByRole("button", { name: "刷新资料" });
            await refresh.waitFor({ state: "visible", timeout: 30_000 });
            await waitForReactHydration(enterprisePage, refresh);
            await refresh.click();
          } catch {
            lateReadyResolve();
            lateResultResolve({ applied: true, outcome: "late_injection_failed" });
          }
        })();
        return { ready: Promise.race([lateReady, delay(30_000).then(() => { throw new Error("LATE_ENTERPRISE_READ_NOT_CAPTURED"); })]), result: lateResult };
      },
      revokeAuthorization: () => { enterpriseStage = "revoke"; return runBaseRuntimeControl("revoke", "viewer", "B"); },
      refreshAuthorizationContext: async () => {
        enterpriseStage = "refresh_after_revoke";
        await enterprisePage.reload({ waitUntil: "load" });
        await enterprisePage.getByLabel("当前企业").waitFor({ state: "visible", timeout: 30_000 });
      },
      releaseLateOrganizationRead: async () => lateRelease(),
      inspectVisibleOrganization: async () => {
        enterpriseStage = "visible_context";
        await delay(500);
        const body = await enterprisePage.locator("body").innerText();
        if (body.includes(`当前有效企业：${manifest.organizations.B.id}`)) return manifest.organizations.B.id;
        const switcher = enterprisePage.getByRole("combobox", { name: "当前企业" });
        if (await switcher.count()) return switcher.inputValue();
        return body.includes(manifest.organizations.A.name) ? manifest.organizations.A.id : "unknown";
      },
      readPersonalProjection: async () => {
        enterpriseStage = "personal_projection";
        return readFreshPersonalProjection({
          expectedSubject: manifest.users.viewer.id,
          openSession: async () => {
            const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN", ignoreHTTPSErrors: true });
            return { context, page: await context.newPage() };
          },
          loginAndRead: async session => {
            await login(session.page, origins.publicOrigin, viewerCredential, "/workbench/account/referrals");
            const profile = await session.context.request.get(`${origins.publicOrigin}/api/account/profile`, { headers: { "X-Expected-User-ID": manifest.users.viewer.id } });
            ensure(profile.status() === 200, "PERSONAL_PROJECTION_IDENTITY_READ_FAILED");
            const identity = await profile.json();
            await session.page.getByText("已建立关系").waitFor({ state: "visible", timeout: 30_000 });
            return { subject: identity.userId, count: Number(await session.page.locator("article").filter({ hasText: "已建立关系" }).locator("strong").textContent()) };
          },
          closeSession: session => session.context.close(),
        });
      },
      readAdminProjection: async () => {
        enterpriseStage = "admin_projection";
        const credential = await readJSON(path.join(manifest.directory, "admin.credentials.json"));
        const adminContext = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", ignoreHTTPSErrors: true });
        try {
          const adminPage = await adminContext.newPage();
          await login(adminPage, origins.publicOrigin, credential, "/workbench/account/referrals");
          await adminPage.getByText("已建立关系").waitFor({ state: "visible", timeout: 30_000 });
          return { subject: manifest.users.admin.id, count: Number(await adminPage.locator("article").filter({ hasText: "已建立关系" }).locator("strong").textContent()) };
        } finally { await adminContext.close(); }
      },
      restoreAuthorization: () => {
        const primaryStage = enterpriseStage;
        return restoreAuthorizationEventually(() => runBaseRuntimeControl("restore", "viewer", "B"))
          .then(() => { enterpriseStage = primaryStage; })
          .catch(() => { enterpriseStage = "restore"; throw new Error("ENTERPRISE_AUTHORIZATION_RESTORE_FAILED"); });
      },
      });
    } catch (error) {
      await writeJSON(path.join(outputDirectory, "m1-enterprise-diagnostic.json"), { stage: enterpriseStage, code: safeCode(error), switcher: error?.fixtureDiagnostic });
      throw error;
    } finally {
      await enterpriseContext.close();
    }
    await writeJSON(path.join(outputDirectory, "m1-enterprise-observation.json"), observation);
    return { ...evaluateEnterpriseRemovalControl(observation), precondition: "viewer_selected_enterprise_B", injection: "official_authorization_deactivation", positiveControl: "live_switch_to_surviving_authorized_enterprise", observation: "late_enterprise_read_not_applied", invariants: ["same_subject", "personal_projection", "admin_isolation"] };
  });
  await matrixCheck("D", "admin_reads_only_own_personal_projection", async () => {
    const admin = await readJSON(path.join(manifest.directory, "admin.credentials.json"));
    const adminContext = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN", ignoreHTTPSErrors: true });
    try {
      const adminPage = await adminContext.newPage();
      await login(adminPage, origins.publicOrigin, admin, "/workbench/account/referrals");
      await adminPage.getByText("已建立关系").waitFor({ state: "visible", timeout: 30_000 });
      ensure((await adminPage.locator("article").filter({ hasText: "已建立关系" }).locator("strong").textContent()) === "0", "ADMIN_READ_OTHER_RELATION");
      return { targetRelationVisible: false, ownCount: 0 };
    } finally { await adminContext.close(); }
  });
  await matrixCheck("D", "no_enterprise_user_can_read_own_empty_projection", async () => {
    const credential = await readJSON(path.join(manifest.directory, "no-org.credentials.json"));
    const isolated = await browser.newContext({ viewport: { width: 390, height: 844 }, locale: "zh-CN", ignoreHTTPSErrors: true });
    try {
      const isolatedPage = await isolated.newPage();
      await login(isolatedPage, origins.publicOrigin, credential, "/workbench/account/referrals");
      await isolatedPage.getByText("已建立关系").waitFor({ state: "visible", timeout: 30_000 });
      ensure((await isolatedPage.locator("article").filter({ hasText: "已建立关系" }).locator("strong").textContent()) === "0", "NO_ENTERPRISE_SELF_READ_INVALID");
      return { ownCount: 0 };
    } finally { await isolated.close(); }
  });
  await matrixCheck("D", "expired_session_and_late_response", async () => {
    let expiredStage = "positive_read";
    const expiredContext = await browser.newContext({ viewport: { width: 1440, height: 1000 }, locale: "zh-CN", ignoreHTTPSErrors: true,
      extraHTTPHeaders: { "X-Forwarded-For": "127.0.0.1", "X-ListingKit-Client-IP": "127.0.0.1" } });
    const expiredPage = await expiredContext.newPage();
    const bootstrap = (await readFile(path.join(manifest.directory, "bootstrap.pat"), "utf8")).trim();
    let lateRelease;
    let lateReadyResolve;
    let lateResultResolve;
    let deletedSessionIds = [];
    const lateReady = new Promise(resolve => { lateReadyResolve = resolve; });
    const lateResult = new Promise(resolve => { lateResultResolve = resolve; });
    let observation;
    try {
      expiredStage = "dedicated_session_login";
      await login(expiredPage, origins.publicOrigin, { username: email, password: registeredPassword }, "/workbench/account/profile");
      await expiredPage.getByText(`账户 ID：${subject}`).waitFor({ state: "visible", timeout: 30_000 });
      observation = await runExpiredSessionControl({
      positiveRead: async () => {
        expiredStage = "positive_read";
        const response = await expiredContext.request.get(`${origins.publicOrigin}/api/account/profile`, { headers: { "X-Expected-User-ID": subject } });
        ensure(response.status() === 200, "SESSION_POSITIVE_CONTROL_FAILED");
        const sessions = await provider("/v2/sessions/search", { query: { offset: 0, limit: 100, asc: true }, queries: [{ userIdQuery: { id: subject } }] }, bootstrap);
        const sessionIds = (sessions.sessions ?? []).map(session => session.id).filter(id => typeof id === "string" && id.length > 0);
        ensure(sessionIds.length > 0, "PROVIDER_SESSION_NOT_FOUND");
        return { status: response.status(), subject, sessionId: sessionIds };
      },
      beginLateRead: () => {
        expiredStage = "late_read";
        const gate = new Promise(resolve => { lateRelease = resolve; });
        void (async () => {
          try {
            await expiredPage.goto(`${origins.publicOrigin}/workbench/account/profile`, { waitUntil: "load" });
            await expiredPage.getByText(`账户 ID：${subject}`).waitFor({ state: "visible", timeout: 30_000 });
            await expiredPage.route("**/api/account/profile", async route => {
              let upstream;
              try {
                upstream = await route.fetch();
                const observed = await observeProfileLateResponse(upstream, subject);
                lateReadyResolve();
                await gate;
                try {
                  await route.fulfill({ response: upstream });
                  lateResultResolve({ ...observed, outcome: "delivered_to_unmounted_identity" });
                } catch {
                  lateResultResolve({ ...observed, outcome: "cancelled_before_delivery" });
                }
              } catch {
                lateReadyResolve();
                lateResultResolve({ subject: "unknown", upstreamValidated: false, outcome: "late_response_invalid" });
                if (upstream) await route.fulfill({ response: upstream }).catch(() => {});
                else await route.abort().catch(() => {});
              }
            }, { times: 1 });
            const refresh = expiredPage.getByRole("button", { name: "刷新资料" });
            await refresh.waitFor({ state: "visible", timeout: 30_000 });
            await waitForReactHydration(expiredPage, refresh);
            await refresh.click();
          } catch {
            lateReadyResolve();
            lateResultResolve({ subject: "late_injection_failed", outcome: "failed" });
          }
        })();
        return { ready: Promise.race([lateReady, delay(30_000).then(() => { throw new Error("LATE_PROFILE_READ_NOT_CAPTURED"); })]), result: lateResult };
      },
      deleteProviderSession: async sessionIds => {
        expiredStage = "delete_provider_session";
        deletedSessionIds = [...sessionIds];
        for (const sessionId of sessionIds) await provider(`/v2/sessions/${encodeURIComponent(sessionId)}`, {}, bootstrap, "DELETE");
      },
      readWithRevokedSession: async () => {
        expiredStage = "revoked_session_read";
        const sessions = await provider("/v2/sessions/search", { query: { offset: 0, limit: 100, asc: true }, queries: [{ userIdQuery: { id: subject } }] }, bootstrap);
        const remaining = (sessions.sessions ?? []).filter(session => session.factors?.user?.id === subject);
        ensure(remaining.length === 0, "PROVIDER_SESSION_STILL_LISTED");
        return verifyDeletedProviderSessions(deletedSessionIds, sessionId => providerStatus(`/v2/sessions/${encodeURIComponent(sessionId)}`, undefined, bootstrap, "GET"));
      },
      loginReplacementIdentity: async () => {
        expiredStage = "replacement_login";
        const admin = await readJSON(path.join(manifest.directory, "admin.credentials.json"));
        await expiredPage.goto(`${origins.publicOrigin}/api/zitadel-auth/logout`, { waitUntil: "commit", timeout: 45_000 });
        await login(expiredPage, origins.publicOrigin, admin, "/workbench/account/profile");
        await expiredPage.reload({ waitUntil: "load" });
        return { subject: manifest.users.admin.id };
      },
      confirmReplacementIdentity: async expectedSubject => {
        expiredStage = "replacement_confirmation";
        const response = await expiredContext.request.get(`${origins.publicOrigin}/api/account/profile`, { headers: { "X-Expected-User-ID": expectedSubject } });
        ensure(response.status() === 200, "REPLACEMENT_PROFILE_READ_FAILED");
        const payload = await response.json();
        ensure(payload.userId === expectedSubject, "REPLACEMENT_PROFILE_SUBJECT_MISMATCH");
        await expiredPage.getByText(`账户 ID：${expectedSubject}`).waitFor({ state: "visible", timeout: 30_000 });
      },
      releaseLateRead: async () => lateRelease(),
      inspectVisibleIdentity: async () => {
        expiredStage = "visible_identity";
        await expiredPage.getByText(`账户 ID：${manifest.users.admin.id}`).waitFor({ state: "visible", timeout: 5_000 }).catch(() => {});
        const body = await expiredPage.locator("body").innerText();
        return { subject: body.includes(`账户 ID：${manifest.users.admin.id}`) ? manifest.users.admin.id : "unknown", oldProjectionVisible: body.includes(`账户 ID：${subject}`) };
      },
      });
    } catch (error) {
      const body = await expiredPage.locator("body").innerText().catch(() => "");
      await writeJSON(path.join(outputDirectory, "m1-expired-session-diagnostic.json"), {
        stage: expiredStage,
        code: safeCode(error),
        pathname: new URL(expiredPage.url()).pathname,
        replacementVisible: body.includes(`账户 ID：${manifest.users.admin.id}`),
        oldProjectionVisible: body.includes(`账户 ID：${subject}`),
        identityChangedStateVisible: body.includes("登录身份已变化"),
        authenticationRequiredStateVisible: body.includes("登录已失效"),
      });
      throw error;
    } finally {
      await expiredContext.close();
    }
    await writeJSON(path.join(outputDirectory, "m1-expired-session-observation.json"), observation);
    return { ...evaluateExpiredSessionControl(observation), precondition: "authenticated_profile_read_200", injection: "official_session_delete_and_old_profile_response_hold", positiveControl: "admin_login_after_logout", observation: "old_response_not_visible_after_identity_change", invariants: ["provider_session_deleted", "identity_keyed_projection"] };
  });
  await matrixCheck("E", "bff_and_go_reject_untrusted_credentials_and_csrf", async () => {
    const csrf = await context.request.post(`${origins.publicOrigin}/api/referral-registration`, { data: JSON.parse(admissionRequest.body), headers: { "Idempotency-Key": admissionRequest.key } })
      .catch(() => { throw new Error("CSRF_REQUEST_FAILED"); });
    const goURL = `${manifest.origins.go}/api/v1/referral-registration/intents`;
    const direct = headers => fetch(goURL, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": admissionRequest.key, ...headers }, body: admissionRequest.body });
    const attempts = await Promise.allSettled([direct({}), direct({ "X-Referral-Service-Credential": "0".repeat(64) }), direct({ "X-Referral-Service-Credential": machine.token })]);
    const statuses = attempts.map(result => result.status === "fulfilled" ? result.value.status : -1);
    ensure(csrf.status() === 403 && statuses.every(status => status === 401 || status === 403), `SERVICE_BOUNDARY_STATUS:${csrf.status()}:${statuses.join(":")}`);
    return { csrfStatus: 403, directCredentialFailures: statuses };
  });
  await matrixCheck("E", "real_user_token_rejected_as_service_credential", async () => {
    const goURL = `${manifest.origins.go}/api/v1/referral-registration/intents`;
    const databaseRecord = manifest.resources?.[`${manifest.project}-commercial-db`];
    ensure(databaseRecord?.id, "BUSINESS_STORAGE_ID_MISSING");
    const observation = await runUserTokenBoundaryControl({
      loadOIDCUserToken: () => until(async () => {
        const value = (await readFile(path.join(manifest.directory, "ui", ".local", "image-agent-acceptance", "user-token.txt"), "utf8").catch(() => "")).trim();
        return value.length >= 32 ? value : null;
      }, "AUTHJS_OIDC_USER_TOKEN", 30_000),
      validateOIDCUserToken: async token => {
        const response = await fetch(`${manifest.origins.issuer}/oidc/v1/userinfo`, { headers: { Authorization: `Bearer ${token}` }, signal: AbortSignal.timeout(30_000) });
        ensure(response.status === 200, "OIDC_USERINFO_REJECTED_TOKEN");
        const payload = await response.json();
        ensure(typeof payload.sub === "string" && payload.sub.length > 0, "OIDC_USERINFO_SUBJECT_MISSING");
        const user = await provider(`/v2/users/${encodeURIComponent(payload.sub)}`, undefined, machine.token, "GET");
        return { subject: payload.sub, human: Boolean(user.user?.human) };
      },
      verifyGuardPrecedesHandler: async () => {
        const source = await readFile(path.join(repo, "internal", "app", "httpapi", "referral_registration.go"), "utf8");
        const entry = source.indexOf("func (m referralHTTPModule) start(c *gin.Context)");
        const guard = source.indexOf("m.trustedCommand(c)", entry);
        const dispatch = source.indexOf("m.commands.Start", entry);
        return entry >= 0 && guard > entry && dispatch > guard && source.slice(guard, dispatch).includes("return");
      },
      callWithServiceCredential: async () => {
        const credential = (await readFile(path.join(manifest.directory, "referral-service.secret"), "utf8")).trim();
        const positiveBody = JSON.stringify({ code, email: `service-positive.${manifest.runId.slice(0, 8)}@example.test`, givenName: "Service", familyName: "Positive" });
        const response = await fetch(goURL, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": randomBytes(32).toString("hex"), "X-Referral-Service-Credential": credential, "X-Referral-Client-IP": "127.0.0.1" }, body: positiveBody, signal: AbortSignal.timeout(30_000) });
        await response.arrayBuffer();
        return { status: response.status };
      },
      referralDigest: () => referralTablesDigest(),
      stopBusinessStorage: async () => {
        await docker(["container", "stop", "--time", "10", databaseRecord.id]);
        const inspected = await inspectDockerResource("container", `${manifest.project}-commercial-db`);
        ensure(inspected?.State?.Running === false, "BUSINESS_STORAGE_STOP_FAILED");
      },
      callWithUserTokenAsServiceCredential: async token => {
        const response = await fetch(goURL, { method: "POST", headers: { "Content-Type": "application/json", "Idempotency-Key": randomBytes(32).toString("hex"), "X-Referral-Service-Credential": token, "X-Referral-Client-IP": "127.0.0.1" }, body: admissionRequest.body, signal: AbortSignal.timeout(30_000) });
        await response.arrayBuffer();
        return { status: response.status };
      },
      startBusinessStorage: async () => {
        await docker(["container", "start", databaseRecord.id]);
        await until(async () => {
          const inspected = await inspectDockerResource("container", `${manifest.project}-commercial-db`);
          return inspected?.State?.Running === true && inspected?.State?.Health?.Status === "healthy";
        }, "BUSINESS_STORAGE_RESTART", 60_000);
      },
    });
    await writeJSON(path.join(outputDirectory, "m1-user-token-observation.json"), { ...observation, tokenSubject: "redacted-human-subject" });
    return { ...evaluateUserTokenBoundary(observation), precondition: "human_oidc_userinfo_200_and_service_credential_200", injection: "oidc_user_token_in_service_credential_header_while_storage_stopped", positiveControl: "independent_service_credential_admission", observation: observation.zeroDispatchEvidence, invariants: ["user_token_not_service_credential", "referral_digest_unchanged"], directBusinessDispatchObserver: observation.directBusinessDispatchObserver };
  });
  await startBFFIngressObserver(ports.bffIngress, ports.nextSecondary);
  await startDispatchObserver(ports.observer, `http://127.0.0.1:${ports.goSecondary}`);
  await matrixCheck("C", "provider_business_read_failure_preserves_receipt", async () => {
    const providerContext = await browser.newContext({ viewport: { width: 900, height: 700 }, locale: "zh-CN", ignoreHTTPSErrors: true });
    const providerPage = await providerContext.newPage();
    try {
      await login(providerPage, origins.publicOrigin, { username: email, password: registeredPassword }, "/workbench/account/referrals/complete");
      const observation = await runProviderBusinessReadFailureControl({
      verifyIdentityHealth: async () => {
        const discoveryResponse = await fetch(`${manifest.origins.issuer}/.well-known/openid-configuration`, { signal: AbortSignal.timeout(10_000) });
        const discovery = await discoveryResponse.json();
        const jwksResponse = await fetch(discovery.jwks_uri, { signal: AbortSignal.timeout(10_000) });
        const authProviders = await fetch(`http://127.0.0.1:${ports.next}/api/auth/providers`, { signal: AbortSignal.timeout(90_000) });
        const identity = await providerContext.request.get(`${origins.publicOrigin}/api/account/profile`, { headers: { "X-Expected-User-ID": subject } });
        const identityPayload = await identity.json().catch(() => ({}));
        if (identityPayload.userId !== subject) {
          await writeJSON(path.join(outputDirectory, "m2-provider-identity-diagnostic.json"), { status: identity.status(), expectedSubject: subject, observedSubject: typeof identityPayload.userId === "string" ? identityPayload.userId : "missing", code: typeof identityPayload.code === "string" ? identityPayload.code : "none" });
          throw new Error("CURRENT_IDENTITY_SUBJECT_MISMATCH");
        }
        return { discoveryStatus: discoveryResponse.status, jwksStatus: jwksResponse.status, authProvidersStatus: authProviders.status, currentIdentityStatus: identity.status(), currentSubject: identityPayload.userId };
      },
      replayReceipt: phase => postCompletion(providerContext.request, phase === "before" ? origins.publicOrigin : origins.secondaryPublicOrigin, subject),
      relationshipCount: () => relationshipCountForSubject(subject),
      enableSelectiveBusinessReadFailure: () => startSecondaryApplication(origins, ports),
      observeBusinessReadFailure: async () => {
        const result = await providerTLSRead(origins.providerFaultOrigin, `/v2/users/${encodeURIComponent(subject)}`, origins.caFile, machine.token);
        return { status: result.status, path: `/v2/users/${subject}` };
      },
      disableSelectiveBusinessReadFailure: () => stopSecondaryApplication(),
      observeBusinessReadRecovery: async () => {
        const result = await providerTLSRead(origins.providerOrigin, `/v2/users/${encodeURIComponent(subject)}`, origins.caFile, machine.token);
        return { status: result.status, subject: result.payload?.user?.userId };
      },
      });
      await writeJSON(path.join(outputDirectory, "m2-provider-business-read-observation.json"), observation);
      return { ...evaluateProviderBusinessReadFailure(observation), precondition: "fresh_official_current_identity_discovery_jwks_authjs_and_committed_receipt_healthy", injection: "secondary_go_uses_run_owned_provider_business_read_only_503_origin", positiveControl: "healthy_provider_user_read_after_secondary_stop", observation: "same_durable_receipt_and_relationship_count_during_selective_provider_read_failure", invariants: ["receipt_first_replay", "one_relationship", "identity_stack_healthy"] };
    } finally {
      await providerContext.close();
    }
  });
  await matrixCheck("E", "cancel_deadline_zero_late_dispatch", async () => {
    await startSecondaryApplication(origins, ports);
    const cancelContext = await browser.newContext({ viewport: { width: 900, height: 700 }, locale: "zh-CN", ignoreHTTPSErrors: true });
    const controlPage = await cancelContext.newPage();
    try {
      await controlPage.goto(`${origins.secondaryPublicOrigin}/referrals/register?code=${encodeURIComponent(code)}`, { waitUntil: "load", timeout: 90_000 });
      const bodyFor = suffix => ({ code, email: `m2.${suffix}.${manifest.runId.slice(0, 8)}@example.test`, givenName: "Deadline", familyName: "Control" });
      const begin = (mode, suffix) => {
        const key = createHash("sha256").update(`${manifest.runId}:${mode}`).digest("hex");
        bffIngressObserver.arm(mode, key);
        dispatchObserver.arm(mode, key);
        const result = controlPage.evaluate(async ({ body, key, mode }) => {
          const controller = new AbortController();
          window.__issue413M2AbortController = controller;
          try {
            const response = await fetch("/api/referral-registration", {
              method: "POST",
              headers: { "Content-Type": "application/json", "Idempotency-Key": key },
              body: JSON.stringify(body),
              signal: controller.signal,
            });
            await response.arrayBuffer();
            return { status: response.status, outcome: "response" };
          } catch {
            return { outcome: controller.signal.aborted ? "client_cancelled" : `${mode}_request_failed` };
          }
        }, { body: bodyFor(suffix), key, mode });
        return {
          ready: dispatchObserver.waitReceived(mode),
          cancel: () => controlPage.evaluate(() => window.__issue413M2AbortController?.abort()),
          result,
        };
      };
      const beginBodyCancellation = () => {
        const mode = "body-cancel";
        const key = createHash("sha256").update(`${manifest.runId}:body-cancel`).digest("hex");
        bffIngressObserver.arm(mode, key);
        dispatchObserver.arm(mode, key);
        const result = controlPage.evaluate(async ({ body, key }) => {
          const controller = new AbortController();
          const encoded = new TextEncoder().encode(JSON.stringify(body));
          const state = { chunksProduced: 0 };
          window.__issue413M2BodyAbortController = controller;
          window.__issue413M2BodyState = state;
          const stream = new ReadableStream({
            start(streamController) {
              streamController.enqueue(encoded.slice(0, Math.min(16, encoded.byteLength)));
              state.chunksProduced++;
            },
          });
          try {
            const response = await fetch("/api/referral-registration", {
              method: "POST",
              headers: { "Content-Type": "application/json", "Idempotency-Key": key },
              body: stream,
              duplex: "half",
              signal: controller.signal,
            });
            await response.arrayBuffer();
            return { status: response.status, outcome: "response", bodyChunksProduced: state.chunksProduced };
          } catch {
            return { outcome: controller.signal.aborted ? "client_cancelled" : "body_request_failed", bodyChunksProduced: state.chunksProduced };
          }
        }, { body: bodyFor("body-cancel"), key });
        return {
          ready: (async () => {
            const localBodyChunksProduced = await until(() => controlPage.evaluate(() => window.__issue413M2BodyState?.chunksProduced), "BODY_STREAM_STARTED", 10_000);
            await bffIngressObserver.waitBodyForwarded(mode);
            const ingress = bffIngressObserver.snapshot(mode);
            ensure(ingress.requestBodiesCompleted === 0, "BODY_STREAM_COMPLETED_BEFORE_CANCELLATION");
            return {
              localBodyChunksProduced,
              ingressRequests: ingress.received,
              ingressBodyChunksReceived: ingress.bodyChunksReceived,
              bodyChunksForwardedToBFF: ingress.bodyChunksForwarded,
              bffConnections: ingress.upstreamConnections,
              ingressRequestBodiesCompleted: ingress.requestBodiesCompleted,
            };
          })(),
          cancel: () => controlPage.evaluate(() => window.__issue413M2BodyAbortController?.abort()),
          result,
        };
      };
      const observation = await runCancelDeadlineControl({
        healthyRequest: async () => {
          const request = begin("healthy", "healthy");
          await request.ready;
          dispatchObserver.release("healthy");
          const result = await request.result;
          await dispatchObserver.waitReleased("healthy");
          await bffIngressObserver.waitCompleted("healthy");
          const snapshot = dispatchObserver.snapshot("healthy");
          return { status: result.status, ...snapshot, ingress: bffIngressObserver.snapshot("healthy") };
        },
        beginCancelledRequest: () => begin("cancel", "cancel"),
        beginDeadlineRequest: () => begin("deadline", "deadline"),
        settle: async mode => {
          await dispatchObserver.waitClientClosed(mode);
          dispatchObserver.release(mode);
          await dispatchObserver.waitReleased(mode);
          await bffIngressObserver.waitCompleted(mode);
          await delay(1_000);
          return { ...dispatchObserver.snapshot(mode), ingress: bffIngressObserver.snapshot(mode) };
        },
        beginBodyCancelledRequest: beginBodyCancellation,
        settleBodyCancellation: async () => {
          await bffIngressObserver.waitClosed("body-cancel");
          await delay(16_000);
          const ingress = bffIngressObserver.snapshot("body-cancel");
          return {
            ...dispatchObserver.snapshot("body-cancel"),
            ingressAborted: ingress.inboundAborted,
            ingressClientConnectionsClosed: ingress.clientConnectionsClosed,
            bffConnectionsClosed: ingress.upstreamConnectionsClosed,
            ingressOpenHandlers: ingress.openHandlers,
            ingressCompletedHandlers: ingress.completedHandlers,
          };
        },
      });
      await writeJSON(path.join(outputDirectory, "m2-cancel-deadline-observation.json"), observation);
      return { ...evaluateCancelDeadlineControl(observation), precondition: "same_held_observer_path_releases_to_real_go_once_and_partial_body_is_flushed_through_ingress_to_real_bff", injection: "browser_cancel_after_incomplete_body_reaches_bff_plus_browser_cancel_and_bff_15_second_deadline_after_bff_dispatch_to_held_observer", positiveControl: "same_ingress_and_observer_path_completes_body_then_explicitly_releases_live_connection_and_dispatches_once_to_go", observation: "incomplete_body_ingress_and_bff_connections_close_before_any_business_dispatch; post_body_cancel_and_deadline_each_close_real_bff_observer_connection_before_release and remain_zero_go_dispatch", invariants: ["pre_dispatch_incomplete_body_cancel_zero_late_dispatch_past_total_deadline", "post_bff_dispatch_connection_close_zero_go_dispatch_after_release", "ingress_and_observer_handlers_and_connections_released"] };
    } catch (error) {
      await writeJSON(path.join(outputDirectory, "m2-cancel-deadline-diagnostic.json"), {
        code: safeCode(error),
        dispatch: Object.fromEntries(["healthy", "cancel", "deadline", "body-cancel"].map(name => [name, dispatchObserver?.snapshot(name)])),
        ingress: Object.fromEntries(["healthy", "cancel", "deadline", "body-cancel"].map(name => [name, bffIngressObserver?.snapshot(name)])),
      }).catch(() => {});
      throw error;
    } finally {
      await cancelContext.close();
    }
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
    if (new Date().getUTCSeconds() > 30) await delay((61 - new Date().getUTCSeconds()) * 1_000);
    const first = [];
    for (let index = 1; index <= 5; index++) first.push(await status(ownedClientNames[0], index));
    ensure(first.every(value => value === 200), "SOURCE_RATE_LIMIT_PRECONDITION_FAILED");
    await restartContextsAndApplications();
    const limited = await status(ownedClientNames[0], 6);
    const independent = await status(ownedClientNames[1], 7);
    ensure(limited === 429 && independent === 200, `CROSS_PROCESS_RATE_STATUS:${first.join(":")}:${limited}:${independent}`);
    return { firstSourceStatuses: first, afterRestartSameSource: limited, secondSource: independent };
  });
  await matrixCheck("E", "parallel_application_instances_share_rate_limit", async () => {
    const network = `${manifest.project}-network`;
    const status = async (instance, source, sequence) => {
      const name = source === "source-a" ? ownedClientNames[0] : ownedClientNames[1];
      const origin = instance === "primary" ? origins.publicOrigin : origins.secondaryPublicOrigin;
      const portSuffix = instance === "primary" ? "" : ":81";
      const body = JSON.stringify({ code, email: `parallel.${sequence}.${manifest.runId.slice(0, 8)}@example.test`, givenName: "Parallel", familyName: "Limit" });
      const key = createHash("sha256").update(`${manifest.runId}:parallel:${sequence}`).digest("hex");
      const shell = `wget -S -O /dev/null --header='Origin: ${origin}' --header='Sec-Fetch-Site: same-origin' --header='Content-Type: application/json' --header='Idempotency-Key: ${key}' --post-data='${body}' http://${caddyName}${portSuffix}/api/referral-registration 2>&1 | awk '/  HTTP\\// {s=$2} END {print s}'`;
      return Number(await docker(["exec", name, "sh", "-c", shell]));
    };
    const observation = await runParallelInstanceRateLimitControl({
      startSecondary: () => startSecondaryApplication(origins, ports),
      inspectPrimary: async () => {
        const processes = await readJSON(path.join(manifest.directory, "processes.json"));
        return { instanceId: `${manifest.runId}:primary`, goPid: processes.go.pid, nextPid: processes.next.pid, goPort: manifest.ports.go, nextPort: ports.next, databaseId: manifest.resources?.[`${manifest.project}-commercial-db`]?.id };
      },
      bothAlive: async (primary, secondary) => {
        const alive = [primary.goPid, primary.nextPid, secondary.supervisorPid, secondary.goPid, secondary.nextPid].every(processAlive);
        const health = await Promise.all([
          fetch(`http://127.0.0.1:${primary.goPort}/api/v1/account/profile`, { signal: AbortSignal.timeout(5_000) }),
          fetch(`http://127.0.0.1:${secondary.goPort}/api/v1/account/profile`, { signal: AbortSignal.timeout(5_000) }),
          fetch(`http://127.0.0.1:${primary.nextPort}/api/auth/providers`, { signal: AbortSignal.timeout(30_000) }),
          fetch(`http://127.0.0.1:${secondary.nextPort}/api/auth/providers`, { signal: AbortSignal.timeout(30_000) }),
        ]);
        return alive && health[0].status === 401 && health[1].status === 401 && health[2].status === 200 && health[3].status === 200;
      },
      waitForFreshWindow: async () => {
        const seconds = new Date().getUTCSeconds();
        await delay((61 - seconds) * 1_000);
        dispatchObserver.arm("parallel");
      },
      request: status,
      stopSecondary: application => stopSecondaryApplication(application),
      inspectSecondaryReleased,
    });
    const observer = dispatchObserver.snapshot("parallel");
    ensure(observer.goDispatches >= 3 && observer.openHandlers === 0, "SECONDARY_BACKEND_TRAFFIC_NOT_OBSERVED");
    const evidence = { ...observation, secondaryBackendObserver: observer };
    await writeJSON(path.join(outputDirectory, "m2-parallel-rate-limit-observation.json"), evidence);
    return { ...evaluateParallelInstanceRateLimit(observation), precondition: "two_live_go_next_process_pairs_and_one_owned_postgresql", injection: "one_source_alternates_primary_secondary_in_same_database_window", positiveControl: "second_source_allowed", observation: "five_total_200_then_cross_instance_429_with_secondary_backend_dispatches", invariants: ["shared_database_rate_bucket", "both_instances_receive_traffic", "secondary_processes_and_ports_released"], secondaryBackendDispatches: observer.goDispatches };
  });
  await matrixCheck("E", "missing_dependency_disables_invite_entry", async () => {
    const secret = path.join(manifest.directory, "referral-service.secret");
    const disabled = `${secret}.disabled`;
    let probe;
    await rename(secret, disabled);
    try {
      probe = await referrerContext.newPage();
      await probe.goto(`${origins.publicOrigin}/workbench/account/referrals`, { waitUntil: "load" });
      await probe.getByText("注册入口暂不可用").waitFor({ state: "visible", timeout: 30_000 });
      ensure(!(await probe.getByRole("link", { name: "打开邀请链接" }).isVisible().catch(() => false)), "INVITE_LINK_ENABLED_WITHOUT_DEPENDENCY");
      const unavailableCount = await probe.locator("article").filter({ hasText: "已建立关系" }).locator("strong").textContent();
      ensure(unavailableCount === "1", "PERSONAL_COUNT_UNREADABLE_WITHOUT_REGISTRATION_DEPENDENCY");
      if (screenReaderSession) await collectScreenReaderObservation("overview-entry-unavailable", probe, {
        expected: ["same_viewer_personal_count_one", "registration_entry_unavailable", "no_active_invitation_link"],
        runnerActions: ["temporarily_disabled_run_owned_registration_dependency", "kept_personal_projection_readable", "will_restore_dependency_in_finally"],
      });
    } finally {
      await probe?.close().catch(() => {});
      await rename(disabled, secret).catch(() => {});
    }
    return { inviteEnabled: false };
  });
  await referrerPage.reload({ waitUntil: "load" });
  await referrerPage.getByText("已建立关系").waitFor({ state: "visible", timeout: 30_000 });
  await referrerPage.setViewportSize({ width: 390, height: 844 });
  const firstControl = referrerPage.locator("a[href],button:not([disabled])").first();
  await firstControl.focus();
  ensure(await firstControl.evaluate(element => element === document.activeElement), "OVERVIEW_INITIAL_FOCUS_MISSING");
  await referrerPage.keyboard.press("Tab");
  ensure(await referrerPage.evaluate(() => document.activeElement !== document.body), "OVERVIEW_KEYBOARD_FOCUS_MISSING");
  const overviewAxe = await assertNoSeriousA11y(referrerPage);
  await referrerPage.screenshot({ path: path.join(outputDirectory, "referrals-overview-narrow.png"), fullPage: true });
  matrixRecord("F", "overview_desktop_narrow_keyboard_axe", "PASS", { narrow: overviewAxe });
  await matrixCheck("D", "logout_removes_personal_projection", async () => {
    await referrerPage.goto(`${origins.publicOrigin}/api/zitadel-auth/logout`, { waitUntil: "commit", timeout: 45_000 });
    await referrerPage.goto(`${origins.publicOrigin}/workbench/account/referrals`, { waitUntil: "domcontentloaded", timeout: 45_000 });
    await delay(2_000);
    const referralDataVisible = await referrerPage.getByText("已建立关系").isVisible().catch(() => false);
    ensure(!referralDataVisible, "LOGOUT_LEFT_REFERRAL_DATA_VISIBLE");
    return { referralDataVisible: false, protectedPathVisible: new URL(referrerPage.url()).pathname === "/workbench/account/referrals" };
  });
  matrixNotRun("D", "enterprise_removed_switching", "CURRENT_RUN_HAS_NO_SAFE_ENTERPRISE_SWITCH_CONTROL");
  matrixNotRun("D", "expired_session_and_late_response", "CURRENT_RUN_HAS_NO_SAFE_TOKEN_TIME_CONTROL");
  matrixNotRun("B", "official_verification_interruption_new_browser_reverify", "OFFICIAL_REVERIFY_CONTINUATION_NOT_EXECUTED");
  matrixNotRun("C", "provider_business_read_failure_preserves_receipt", "ONLY_BROWSER_PROJECTION_FAILURE_EXECUTED");
  matrixNotRun("E", "parallel_application_instances_share_rate_limit", "ONLY_SEQUENTIAL_PROCESS_RESTART_EXECUTED");
  matrixNotRun("E", "real_user_token_rejected_as_service_credential", "ONLY_PROVIDER_MACHINE_TOKEN_REJECTED");
  matrixNotRun("E", "cancel_deadline_zero_late_dispatch", "CURRENT_PRODUCT_HAS_NO_TASK_OWNED_DISPATCH_OBSERVER");
  if (screenReaderSession) {
    const manual = evaluateScreenReaderEvidence(screenReaderSession.observations, {
      runId: manifest.runId,
      sourceSha: report.sourceSha,
      webSha: report.webSha,
      runnerNormalizedLFSha256: report.runnerNormalizedLFSha256,
    });
    report.manualAccessibility = manual;
    matrixRecord("F", "screen_reader", manual.status, {
      operator: manual.operator,
      designationReference: manual.designationReference,
      screenReader: manual.screenReader,
      browser: manual.browser,
      operatingSystem: manual.operatingSystem,
      checkpoints: manual.checkpoints.map(item => ({ checkpointId: item.checkpointId, result: item.result, page: item.page, viewport: item.viewport, observedAt: item.observedAt })),
    });
    await writeJSONAtomic(path.join(outputDirectory, "screen-reader-observations.json"), manual);
  } else {
    matrixNotRun("F", "screen_reader", "REAL_SCREEN_READER_NOT_EXECUTED");
    report.manualAccessibility = "NOT_RUN";
  }
  assertMatrixMustComplete(report.matrix);
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
    { name: "secondary-application", run: async () => stopSecondaryApplication() },
    { name: "dispatch-observer", run: async () => stopDispatchObserver() },
    { name: "bff-ingress-observer", run: async () => stopBFFIngressObserver() },
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
      if (current.status !== "destroyed") {
        try {
          await run(process.execPath, [runtimeScript, "destroy", "--run", manifest.runId]);
        } catch (error) {
          const diagnostic = typeof error?.privateOutput === "string" && error.privateOutput.trim() ? error.privateOutput : String(error?.message ?? error);
          if (outputDirectory) await writePrivate(path.join(outputDirectory, `base-runtime-cleanup-${phase}.log`), diagnostic);
          throw error;
        }
      }
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
  for (const name of [caddyName, mailName, ...ownedClientNames].filter(Boolean)) {
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
  const secondaryRoot = path.resolve(expected, "m2-secondary");
  ensure(path.dirname(secondaryRoot) === expected, "CLEANUP_PATH_INVALID");
  await unlink(path.join(secondaryRoot, "ui", "node_modules")).catch(error => { if (error.code !== "ENOENT") throw error; });
  await rm(secondaryRoot, { recursive: true, force: true });
  const screenReaderRoot = path.resolve(expected, "screen-reader-session");
  ensure(path.dirname(screenReaderRoot) === expected, "CLEANUP_PATH_INVALID");
  await rm(screenReaderRoot, { recursive: true, force: true });
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
  Object.assign(report, describeRunnerBytes(await readFile(fileURLToPath(import.meta.url))));
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
      if (process.env.ISSUE413_CONFIGURATION_SMOKE === "1") throw new Error("CONFIGURATION_SMOKE_COMPLETE");
      if (process.env.ISSUE413_M2_CONFIGURATION_SMOKE === "1") {
        await check("m2_bff_ingress_observer_start", () => startBFFIngressObserver(ports.bffIngress, ports.nextSecondary));
        await check("m2_dispatch_observer_start", () => startDispatchObserver(ports.observer, `http://127.0.0.1:${ports.goSecondary}`));
        await check("m2_secondary_application_start", () => startSecondaryApplication(origins, ports));
        await check("m2_bff_ingress_proxy_health", async () => {
          const health = await providerTLSRead(origins.secondaryPublicOrigin, "/api/auth/providers", origins.caFile, "");
          ensure(health.status === 200 && health.payload?.zitadel, "BFF_INGRESS_PROXY_HEALTH_FAILED");
          return { httpStatus: health.status };
        });
        await check("m2_secondary_application_stop", () => stopSecondaryApplication());
        await check("m2_dispatch_observer_stop", () => stopDispatchObserver());
        await check("m2_bff_ingress_observer_stop", () => stopBFFIngressObserver());
        throw new Error("M2_CONFIGURATION_SMOKE_COMPLETE");
      }
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
  else if (process.argv[2] === "screen-reader-status") await screenReaderStatusCommand(process.argv[3]);
  else if (process.argv[2] === "screen-reader-ack") await screenReaderAckCommand(process.argv[3]);
  else if (process.argv[2] === "screen-reader-abort") await screenReaderAbortCommand(process.argv[3]);
  else await main();
}
