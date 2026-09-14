import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdir, mkdtemp, readFile, rm } from "node:fs/promises";
import { createServer as createHTTPServer, request as httpRequest } from "node:http";
import { tmpdir } from "node:os";
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

function screenReaderObservation(checkpointId, sequence, overrides = {}) {
  const page = checkpointId.startsWith("registration-") ? "/referrals/register"
    : checkpointId.startsWith("completion-") ? "/workbench/account/referrals/complete"
      : "/workbench/account/referrals";
  return {
    checkpointId,
    sequence,
    page,
    runId: "11111111-1111-4111-8111-111111111111",
    nonce: `nonce-value-${sequence}`,
    checkpointAttemptId: `checkpoint-attempt-${sequence}`,
    stateObservedAt: "2026-09-14T09:59:59.000Z",
    sourceSha: "a".repeat(40),
    webSha: "a".repeat(40),
    runnerNormalizedLFSha256: "b".repeat(64),
    operator: "user-designated-operator",
    designationReference: "https://github.com/qq550723504/task-processor/issues/413#issuecomment-1",
    screenReader: { name: "NVDA", version: "2026.1" },
    browser: { name: "Chromium", version: "140.0.0" },
    operatingSystem: { name: "Windows", version: "Windows 11 Pro 10.0.26100" },
    observationSource: "live operator dictation recorded by the writer",
    runnerActions: ["prepared the task-owned state"],
    stateAttempt: "stable",
    viewport: { width: 1440, height: 1000 },
    readingSequence: ["heading", "labels", "status"],
    controlSequence: ["tab", "activate"],
    announcedText: ["expected state announced"],
    visibleErrors: [],
    announcedErrors: [],
    result: "PASS",
    observedAt: "2026-09-14T10:00:00.000Z",
    ...overrides,
  };
}

function screenReaderAuthority(runId = "11111111-1111-4111-8111-111111111111") {
  return { runId, sourceSha: "a".repeat(40), webSha: "a".repeat(40), runnerNormalizedLFSha256: "b".repeat(64) };
}

test("manual screen-reader evidence requires all eight real operator checkpoints", async () => {
  const { screenReaderCheckpointPlan, evaluateScreenReaderEvidence } = await runner();
  assert.deepEqual(screenReaderCheckpointPlan.map(item => item.id), [
    "registration-initial",
    "registration-pending",
    "registration-unknown",
    "registration-mail-pending",
    "completion-initial-pending",
    "completion-receipt-projection-unavailable",
    "overview-entry-available",
    "overview-entry-unavailable",
  ]);
  const observations = screenReaderCheckpointPlan.map((item, index) => screenReaderObservation(item.id, index + 1));
  const authority = screenReaderAuthority();
  const passed = evaluateScreenReaderEvidence(observations, authority);
  assert.equal(passed.status, "PASS");
  assert.equal(passed.sourceSha, authority.sourceSha);
  assert.equal(passed.checkpoints[0].page, "/referrals/register");
  assert.equal(passed.checkpoints[0].checkpointAttemptId, observations[0].checkpointAttemptId);
  assert.equal(passed.checkpoints[0].nonce, undefined);
  assert.equal(evaluateScreenReaderEvidence(observations.map((item, index) => index === 5 ? { ...item, result: "FAIL" } : item), authority).status, "FAIL");
  assert.equal(evaluateScreenReaderEvidence([], authority).status, "NOT_RUN");
});

test("manual evidence cannot pass from readiness, test controls, or a direct status field", async () => {
  const { screenReaderCheckpointPlan, evaluateScreenReaderEvidence } = await runner();
  const observations = screenReaderCheckpointPlan.map((item, index) => screenReaderObservation(item.id, index + 1));
  assert.throws(() => evaluateScreenReaderEvidence(observations.map(item => ({ ...item, controlReady: true, status: "PASS" })), screenReaderAuthority()), /SCREEN_READER_DIRECT_STATUS_FORBIDDEN/);
  assert.throws(() => evaluateScreenReaderEvidence(observations.map(item => ({ ...item, controlTest: true })), screenReaderAuthority()), /SCREEN_READER_TEST_EVIDENCE_FORBIDDEN/);
  const missingAnnouncements = observations.map(item => ({ ...item, announcedText: undefined }));
  assert.throws(() => evaluateScreenReaderEvidence(missingAnnouncements, screenReaderAuthority()), /SCREEN_READER_OBSERVATION_INCOMPLETE/);
  assert.throws(() => evaluateScreenReaderEvidence(observations.map((item, index) => index === 2 ? { ...item, page: "/wrong" } : item), screenReaderAuthority()), /SCREEN_READER_OBSERVATION_STATE_MISMATCH/);
});

test("manual evidence rejects missing duplicate out-of-order and inconsistent operator records", async () => {
  const { screenReaderCheckpointPlan, evaluateScreenReaderEvidence } = await runner();
  const runId = "11111111-1111-4111-8111-111111111111";
  const observations = screenReaderCheckpointPlan.map((item, index) => screenReaderObservation(item.id, index + 1));
  assert.throws(() => evaluateScreenReaderEvidence(observations.slice(0, -1), screenReaderAuthority(runId)), /SCREEN_READER_CHECKPOINT_SET_INCOMPLETE/);
  assert.throws(() => evaluateScreenReaderEvidence([...observations.slice(0, -1), observations[0]], screenReaderAuthority(runId)), /SCREEN_READER_CHECKPOINT_DUPLICATE/);
  assert.throws(() => evaluateScreenReaderEvidence([observations[1], observations[0], ...observations.slice(2)], screenReaderAuthority(runId)), /SCREEN_READER_CHECKPOINT_OUT_OF_ORDER/);
  assert.throws(() => evaluateScreenReaderEvidence(observations.map((item, index) => index === 4 ? { ...item, operator: "different-operator" } : item), screenReaderAuthority(runId)), /SCREEN_READER_OPERATOR_MISMATCH/);
  assert.throws(() => evaluateScreenReaderEvidence(observations.map((item, index) => index === 4 ? { ...item, screenReader: { name: "Narrator", version: "1" } } : item), screenReaderAuthority(runId)), /SCREEN_READER_SOFTWARE_MISMATCH/);
  assert.throws(() => evaluateScreenReaderEvidence(observations.map((item, index) => index === 4 ? { ...item, operatingSystem: { name: "Windows", version: "different" } } : item), screenReaderAuthority(runId)), /SCREEN_READER_OPERATING_SYSTEM_MISMATCH/);
});

test("manual evidence is bound to the owned run and rejects secrets or reusable links", async () => {
  const { screenReaderCheckpointPlan, evaluateScreenReaderEvidence, screenReaderObservationFromInput } = await runner();
  const observations = screenReaderCheckpointPlan.map((item, index) => screenReaderObservation(item.id, index + 1));
  assert.throws(() => evaluateScreenReaderEvidence(observations, screenReaderAuthority("22222222-2222-4222-8222-222222222222")), /SCREEN_READER_RUN_MISMATCH/);
  assert.throws(() => evaluateScreenReaderEvidence(observations, { ...screenReaderAuthority(), sourceSha: "c".repeat(40) }), /SCREEN_READER_SOURCE_MISMATCH/);
  assert.throws(() => evaluateScreenReaderEvidence(observations.map((item, index) => index === 2 ? { ...item, announcedText: ["resumeSecret=private-value"] } : item), screenReaderAuthority()), /SCREEN_READER_EVIDENCE_SECRET/);
  assert.throws(() => evaluateScreenReaderEvidence(observations.map((item, index) => index === 2 ? { ...item, announcedText: ["https:\/\/example.test\/verify?code=reusable"] } : item), screenReaderAuthority()), /SCREEN_READER_EVIDENCE_SECRET/);
  assert.throws(() => evaluateScreenReaderEvidence(observations.map((item, index) => index === 2 ? { ...item, announcedText: ["https:\/\/example.test\/verify\/reusable"] } : item), screenReaderAuthority()), /SCREEN_READER_EVIDENCE_SECRET/);
  const input = screenReaderObservation("registration-initial", 1);
  assert.throws(() => screenReaderObservationFromInput({ ...input, screenReader: { ...input.screenReader, privateLink: "synthetic-private-link" } }, {
    ...input,
    phase: "observation",
    expiresAt: "2099-09-14T10:10:00.000Z",
    runnerActions: ["prepared state"],
  }), /SCREEN_READER_OBSERVATION_SCHEMA_INVALID/);
});

test("manual decision is atomic idempotent and ack cannot overwrite abort", async () => {
  const { writeScreenReaderDecision } = await runner();
  const directory = await mkdtemp(path.join(tmpdir(), "issue413-screen-reader-decision-"));
  const decisionFile = path.join(directory, "decision.json");
  try {
    const abortDecision = { kind: "abort", runId: "run", sequence: 1 };
    assert.deepEqual(await writeScreenReaderDecision({ decisionFile, decision: abortDecision }), { recorded: true, replay: false, decision: abortDecision });
    assert.deepEqual(await writeScreenReaderDecision({ decisionFile, decision: abortDecision }), { recorded: true, replay: true, decision: abortDecision });
    await assert.rejects(writeScreenReaderDecision({ decisionFile, decision: { kind: "observation", runId: "run", sequence: 1 } }), /SCREEN_READER_DECISION_CONFLICT/);
  } finally {
    await rm(directory, { recursive: true, force: true });
  }
});

test("manual decision is invisible until the complete exclusive record is publishable", async () => {
  const { writeScreenReaderDecision } = await runner();
  const directory = await mkdtemp(path.join(tmpdir(), "issue413-screen-reader-publish-"));
  const decisionFile = path.join(directory, "decision.json");
  let publishReady;
  let releasePublish;
  const ready = new Promise(resolve => { publishReady = resolve; });
  const release = new Promise(resolve => { releasePublish = resolve; });
  try {
    const pending = writeScreenReaderDecision({
      decisionFile,
      decision: { kind: "abort", runId: "run", sequence: 1 },
      beforePublish: async () => { publishReady(); await release; },
    });
    const pausedBeforePublish = await Promise.race([ready.then(() => true), pending.then(() => false)]);
    assert.equal(pausedBeforePublish, true);
    await assert.rejects(readFile(decisionFile), error => error?.code === "ENOENT");
    releasePublish();
    await pending;
    assert.equal(JSON.parse(await readFile(decisionFile, "utf8")).kind, "abort");
  } finally {
    releasePublish?.();
    await rm(directory, { recursive: true, force: true });
  }
});

test("manual checkpoint inputs are sequence-owned", async () => {
  const { screenReaderSessionPaths } = await runner();
  const paths = screenReaderSessionPaths("11111111-1111-4111-8111-111111111111");
  assert.notEqual(paths.input(1), paths.input(2));
});

test("manual partial evidence rejects mixed identity and preserves a known failure", async () => {
  const { screenReaderPartialEvidence } = await runner();
  const first = screenReaderObservation("registration-initial", 1);
  const failed = screenReaderObservation("registration-pending", 2, { result: "FAIL" });
  const partial = screenReaderPartialEvidence([first, failed], screenReaderAuthority());
  assert.equal(partial.status, "FAIL");
  assert.equal(partial.evidenceStage, "CHECKPOINT_SNAPSHOT");
  assert.equal(partial.operator, first.operator);
  assert.equal(partial.observations[0].nonce, undefined);
  assert.equal(screenReaderPartialEvidence([first], screenReaderAuthority()).status, "NOT_RUN");
  assert.throws(() => screenReaderPartialEvidence([first, { ...failed, operator: "different-operator" }], screenReaderAuthority()), /SCREEN_READER_OPERATOR_MISMATCH/);
});

test("manual partial evidence is terminal in the persisted lifecycle report", async () => {
  const { orchestrateFixtureLifecycle } = await runner();
  const cleanup = phase => ({ phase, status: "PASS", actions: [], residuals: { status: "PASS", containers: 0, volumes: 0, networks: 0, listeners: 0 } });
  for (const code of ["SCREEN_READER_SESSION_ABORTED", "SCREEN_READER_CHECKPOINT_TIMEOUT", "SCREEN_READER_BROWSER_CLOSED", "SCREEN_READER_BUSINESS_FAILED"]) {
    const observations = code.includes("TIMEOUT") ? [{ checkpointId: "registration-initial", result: "PASS" }] : [];
    const report = { manualAccessibility: { status: code.includes("ABORTED") ? "FAIL" : "NOT_RUN", sessionStatus: "IN_PROGRESS", observations } };
    let persisted;
    let persistedManual;
    await orchestrateFixtureLifecycle({
      report,
      runBusiness: async () => { throw new Error(code); },
      runCleanup: async phase => cleanup(phase),
      persistManualEvidence: async value => { persistedManual = structuredClone(value); },
      persistReport: async value => { persisted = structuredClone(value); },
      emitFailure: () => {},
      now: () => "2026-09-14T12:00:00.000Z",
    });
    assert.deepEqual(persistedManual, persisted.manualAccessibility);
    assert.equal(persisted.manualAccessibility.status, code.includes("ABORTED") ? "FAIL" : "NOT_RUN");
    assert.equal(persisted.manualAccessibility.evidenceStage, "FINAL_SESSION_EVIDENCE");
    assert.equal(persisted.manualAccessibility.sessionStatus, "TERMINATED");
    assert.equal(persisted.manualAccessibility.terminationReason, code);
    assert.equal(persisted.manualAccessibility.finishedAt, "2026-09-14T12:00:00.000Z");
    assert.deepEqual(persisted.manualAccessibility.observations, observations);
  }
  const completed = { manualAccessibility: { status: "PASS", evidenceStage: "FINAL_SESSION_EVIDENCE", sessionStatus: "COMPLETED", observations: [{ checkpointId: "overview-entry-unavailable", result: "PASS" }] } };
  let completedManual;
  await orchestrateFixtureLifecycle({
    report: completed,
    runBusiness: async () => {},
    runCleanup: async phase => cleanup(phase),
    persistManualEvidence: async value => { completedManual = structuredClone(value); },
    persistReport: async () => {},
    emitFailure: () => {},
  });
  assert.equal(completedManual.sessionStatus, "COMPLETED");
  assert.equal(completedManual.terminationReason, undefined);

  const report = { manualAccessibility: { status: "FAIL", sessionStatus: "IN_PROGRESS" } };
  let reportPersisted = false;
  let emitted;
  const outcome = await orchestrateFixtureLifecycle({
    report,
    runBusiness: async () => { throw new Error("SCREEN_READER_SESSION_ABORTED"); },
    runCleanup: async phase => cleanup(phase),
    persistManualEvidence: async () => { throw new Error("OBSERVATIONS_WRITE_FAILED"); },
    persistReport: async () => { reportPersisted = true; },
    emitFailure: value => { emitted = value; },
  });
  assert.equal(outcome.report.manualAccessibility.status, "FAIL");
  assert.equal(outcome.report.manualAccessibility.sessionStatus, "TERMINATED");
  assert.equal(outcome.report.manualAccessibility.terminationReason, "SCREEN_READER_SESSION_ABORTED");
  assert.equal(outcome.report.evidence.status, "FAIL");
  assert.equal(outcome.report.evidence.code, "OBSERVATIONS_WRITE_FAILED");
  assert.equal(reportPersisted, false);
  assert.deepEqual(emitted.manualSession, {
    status: "FAIL",
    sessionStatus: "TERMINATED",
    terminationReason: "SCREEN_READER_SESSION_ABORTED",
  });
});

test("manual terminal evidence fails closed on actual observation and report file obstructions", async () => {
  const { orchestrateFixtureLifecycle, writeJSONAtomic } = await runner();
  const cleanup = phase => ({ phase, status: "PASS", actions: [], residuals: { status: "PASS", containers: 0, volumes: 0, networks: 0, listeners: 0 } });
  for (const blocked of ["observations", "report"]) {
    const directory = await mkdtemp(path.join(tmpdir(), `issue413-manual-${blocked}-`));
    const observationsFile = path.join(directory, "screen-reader-observations.json");
    const reportFile = path.join(directory, "report.json");
    await mkdir(blocked === "observations" ? observationsFile : reportFile);
    let emitted;
    try {
      const outcome = await orchestrateFixtureLifecycle({
        report: { schemaVersion: "test", runId: "11111111-1111-4111-8111-111111111111", manualAccessibility: { status: "FAIL", sessionStatus: "IN_PROGRESS", observations: [] } },
        runBusiness: async () => { throw new Error("SCREEN_READER_SESSION_ABORTED"); },
        runCleanup: async phase => cleanup(phase),
        persistManualEvidence: value => writeJSONAtomic(observationsFile, value),
        persistReport: value => writeJSONAtomic(reportFile, value),
        emitFailure: value => { emitted = value; },
      });
      assert.equal(outcome.exitCode, 1);
      assert.equal(outcome.report.evidence.status, "FAIL");
      assert.equal(emitted.manualSession.sessionStatus, "TERMINATED");
      assert.equal(emitted.manualSession.terminationReason, "SCREEN_READER_SESSION_ABORTED");
      if (blocked === "observations") await assert.rejects(readFile(reportFile), error => error?.code === "ENOENT");
      else assert.equal(JSON.parse(await readFile(observationsFile, "utf8")).sessionStatus, "TERMINATED");
    } finally {
      await rm(directory, { recursive: true, force: true });
    }
  }
});

test("manual checkpoint wait reports an actual Playwright page close", async () => {
  const { orchestrateFixtureLifecycle, screenReaderPageError, waitForScreenReaderDecisionOrPageClose, writeJSONAtomic } = await runner();
  const { chromium } = await import("@playwright/test");
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.setContent("<main>screen reader checkpoint control</main>");
    const waiting = waitForScreenReaderDecisionOrPageClose(signal => new Promise((resolve, reject) => signal.addEventListener("abort", () => reject(new Error("WAIT_CANCELLED")), { once: true })), page);
    const observed = waiting.catch(error => error);
    await page.close();
    const closeError = await observed;
    assert.match(closeError.message, /SCREEN_READER_BROWSER_CLOSED/);
    const directory = await mkdtemp(path.join(tmpdir(), "issue413-manual-browser-close-"));
    try {
      const cleanup = phase => ({ phase, status: "PASS", actions: [], residuals: { status: "PASS", containers: 0, volumes: 0, networks: 0, listeners: 0 } });
      const outcome = await orchestrateFixtureLifecycle({
        report: { manualAccessibility: { status: "NOT_RUN", sessionStatus: "IN_PROGRESS", observations: [] } },
        runBusiness: async () => { throw closeError; },
        runCleanup: async phase => cleanup(phase),
        persistManualEvidence: value => writeJSONAtomic(path.join(directory, "screen-reader-observations.json"), value),
        persistReport: value => writeJSONAtomic(path.join(directory, "report.json"), value),
        emitFailure: () => {},
      });
      assert.equal(outcome.exitCode, 1);
      assert.equal(JSON.parse(await readFile(path.join(directory, "screen-reader-observations.json"), "utf8")).terminationReason, "SCREEN_READER_BROWSER_CLOSED");
      assert.equal(JSON.parse(await readFile(path.join(directory, "report.json"), "utf8")).manualAccessibility.terminationReason, "SCREEN_READER_BROWSER_CLOSED");
    } finally {
      await rm(directory, { recursive: true, force: true });
    }
    const actionPage = await browser.newPage();
    const requestFailure = actionPage.waitForRequest(() => true, { timeout: 10_000 }).catch(error => error);
    await actionPage.close();
    assert.equal(screenReaderPageError(actionPage, await requestFailure).message, "SCREEN_READER_BROWSER_CLOSED");
  } finally {
    await browser.close();
  }
});

test("manual acknowledgement binds to the observed phase and exact state attempt", async () => {
  const { screenReaderObservationFromInput } = await runner();
  const base = screenReaderObservation("registration-pending", 2);
  const status = {
    schemaVersion: "issue413-screen-reader-checkpoint-v1",
    ...screenReaderAuthority(),
    checkpointId: base.checkpointId,
    sequence: base.sequence,
    phase: "observation",
    nonce: base.nonce,
    checkpointAttemptId: base.checkpointAttemptId,
    page: base.page,
    stateObservedAt: base.stateObservedAt,
    stateAttempt: { triggeredBy: "operator", transitions: [{ text: "正在提交…", disabled: true, at: base.stateObservedAt }] },
    designationReference: base.designationReference,
    browser: base.browser,
    operatingSystem: base.operatingSystem,
    viewport: base.viewport,
    runnerActions: base.runnerActions,
    expiresAt: "2099-09-14T10:10:00.000Z",
  };
  const input = {
    checkpointId: base.checkpointId,
    sequence: base.sequence,
    nonce: base.nonce,
    checkpointAttemptId: base.checkpointAttemptId,
    operator: base.operator,
    designationReference: base.designationReference,
    screenReader: base.screenReader,
    browser: base.browser,
    observationSource: base.observationSource,
    viewport: base.viewport,
    readingSequence: base.readingSequence,
    controlSequence: base.controlSequence,
    announcedText: base.announcedText,
    visibleErrors: base.visibleErrors,
    announcedErrors: base.announcedErrors,
    result: base.result,
    observedAt: base.observedAt,
  };
  assert.equal(screenReaderObservationFromInput(input, status).stateAttempt.triggeredBy, "operator");
  assert.throws(() => screenReaderObservationFromInput(input, { ...status, phase: "action-ready" }), /SCREEN_READER_ACTION_NOT_OBSERVED/);
  assert.throws(() => screenReaderObservationFromInput({ ...input, checkpointAttemptId: "another-attempt" }, status), /SCREEN_READER_OBSERVATION_STALE/);
});

test("manual session wires every required state without a passed environment switch", async () => {
  const source = await readFile(runnerPath, "utf8");
  const calls = [...source.matchAll(/collectScreenReaderObservation\("([^"]+)"/g)].map(match => match[1]);
  assert.deepEqual(calls, [
    "registration-initial",
    "registration-pending",
    "registration-unknown",
    "registration-mail-pending",
    "completion-initial-pending",
    "completion-receipt-projection-unavailable",
    "overview-entry-available",
    "overview-entry-unavailable",
  ]);
  assert.match(source, /headless: process\.env\.ISSUE413_SCREEN_READER_SESSION !== "1"/);
  assert.match(source, /matrixRecord\("F", "screen_reader", manual\.status/);
  assert.doesNotMatch(source, /SCREEN_READER_(?:PASS|PASSED)/);
  assert.match(source, /PERSONAL_COUNT_UNREADABLE_WITHOUT_REGISTRATION_DEPENDENCY/);
});

test("manual abort remains a business failure and still runs both cleanup passes", async () => {
  const { orchestrateFixtureLifecycle } = await runner();
  const scenario = lifecycleScenario({ businessFailure: "SCREEN_READER_SESSION_ABORTED" });
  const outcome = await orchestrateFixtureLifecycle(scenario.options);
  assert.equal(outcome.exitCode, 1);
  assert.equal(outcome.report.business.code, "SCREEN_READER_SESSION_ABORTED");
  assert.equal(outcome.report.cleanup.initial.status, "PASS");
  assert.equal(outcome.report.cleanup.final.status, "PASS");
  assert.deepEqual(scenario.calls, ["business", "cleanup:initial", "cleanup:final", "persist"]);
});

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
  const { matrixNotRun, matrixRecord, report, sanitizeCheckEvidence, sanitizeMatrixEvidence } = await runner();
  assert.deepEqual(sanitizeCheckEvidence({ name: "wrong", status: 200, httpStatus: 200 }), { httpStatus: 200 });
  assert.deepEqual(sanitizeMatrixEvidence({ group: "wrong", name: "wrong", status: 200, httpStatus: 200 }), { httpStatus: 200 });
  const item = report.matrix.find(entry => entry.group === "C" && entry.name === "concurrent_first_completion_one_relationship");
  matrixRecord(item.group, item.name, "PASS", { requests: 4 });
  matrixNotRun(item.group, item.name, "STALE_FALLBACK");
  assert.deepEqual(item, { group: "C", name: item.name, status: "PASS", requests: 4 });
});

test("runner provenance distinguishes executed bytes from normalized Git content", async () => {
  const { describeRunnerBytes } = await runner();
  const lf = describeRunnerBytes(Buffer.from("first\nsecond\n", "utf8"));
  const mixed = describeRunnerBytes(Buffer.from("first\r\nsecond\n", "utf8"));

  assert.notEqual(mixed.runnerSha256, lf.runnerSha256);
  assert.equal(mixed.runnerNormalizedLFSha256, lf.runnerNormalizedLFSha256);
  assert.deepEqual(mixed.runnerByteFormat, { bytes: 14, hasUTF8BOM: false, crlfCount: 1, lfCount: 1, crCount: 0 });
});

test("screen-reader NOT_RUN keeps the whole fixture incomplete and nonzero", async () => {
  const { assertMatrixMustComplete, orchestrateFixtureLifecycle, report } = await runner();
  const matrix = report.matrix.map(item => ({
    ...item,
    status: item.group === "F" && item.name === "screen_reader" ? "NOT_RUN" : "PASS",
  }));
  const scenario = lifecycleScenario();
  scenario.options.runBusiness = async () => {
    scenario.calls.push("business");
    assertMatrixMustComplete(matrix);
  };

  const outcome = await orchestrateFixtureLifecycle(scenario.options);

  assert.equal(outcome.exitCode, 1);
  assert.equal(outcome.report.business.status, "FAIL");
  assert.equal(outcome.report.business.code, "MATRIX_MUST_INCOMPLETE");
  assert.equal(outcome.report.conclusion, "FAIL");
  assert.deepEqual(scenario.calls, ["business", "cleanup:initial", "cleanup:final", "persist"]);
});

test("every planned Must must be present exactly once and PASS", async () => {
  const { assertMatrixMustComplete, report } = await runner();
  const complete = report.matrix.map(item => ({ ...item, status: "PASS" }));

  assert.doesNotThrow(() => assertMatrixMustComplete(complete));
  assert.throws(
    () => assertMatrixMustComplete(complete.map(item => item.group === "F" && item.name === "overview_desktop_narrow_keyboard_axe" ? { ...item, status: "NOT_RUN" } : item)),
    /MATRIX_MUST_INCOMPLETE/,
  );
  assert.throws(() => assertMatrixMustComplete(complete.slice(1)), /MATRIX_MUST_INCOMPLETE/);
  assert.throws(() => assertMatrixMustComplete([...complete, { ...complete[0] }]), /MATRIX_MUST_INCOMPLETE/);
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
    healthyRequest: async () => { calls.push("healthy"); return { status: 200, bffDispatches: 1, goDispatches: 1, released: true, responsesCompleted: 1, ingress: { received: 1, bodyChunksForwarded: 1, requestBodiesCompleted: 1, responsesCompleted: 1 } }; },
    beginCancelledRequest: () => { calls.push("cancel:begin"); return result("cancel"); },
    beginDeadlineRequest: () => { calls.push("deadline:begin"); return result("deadline"); },
    settle: async kind => { calls.push(`${kind}:settle`); return { bffDispatches: 1, goDispatches: 0, openHandlers: 0, clientConnectionsClosed: 1, released: true }; },
    beginBodyCancelledRequest: () => { calls.push("body:begin"); return { ...result("cancel"), ready: Promise.resolve({ localBodyChunksProduced: 1, ingressRequests: 1, ingressBodyChunksReceived: 1, bodyChunksForwardedToBFF: 1, bffConnections: 1, ingressRequestBodiesCompleted: 0 }), result: Promise.resolve({ outcome: "client_cancelled", bodyChunksProduced: 1 }) }; },
    settleBodyCancellation: async () => { calls.push("body:settle"); return { bffDispatches: 0, goDispatches: 0, openHandlers: 0, ingressAborted: 1, ingressClientConnectionsClosed: 0, bffConnectionsClosed: 1, ingressOpenHandlers: 0 }; },
  });
  assert.deepEqual(evaluateCancelDeadlineControl(observation), {
    healthyDispatchObserved: true,
    clientCancellationObserved: true,
    deadlineResponseObserved: true,
    bodyReadCancellationObserved: true,
    zeroLateDispatch: true,
    resourcesReleased: true,
  });
  assert.deepEqual(calls, ["healthy", "cancel:begin", "cancel:cancel", "cancel:settle", "deadline:begin", "deadline:settle", "body:begin", "cancel:cancel", "body:settle"]);
});

test("M2 late dispatch and leaked observer requests fail closed", async () => {
  const { evaluateCancelDeadlineControl } = await runner();
  assert.throws(() => evaluateCancelDeadlineControl({
    healthy: { status: 200, bffDispatches: 1, goDispatches: 1, released: true, responsesCompleted: 1, ingress: { received: 1, bodyChunksForwarded: 1, requestBodiesCompleted: 1, responsesCompleted: 1 } },
    cancelled: { outcome: "client_cancelled" },
    cancelledSettled: { bffDispatches: 1, goDispatches: 1, openHandlers: 0, clientConnectionsClosed: 1, released: true },
    deadline: { status: 504 },
    deadlineSettled: { bffDispatches: 1, goDispatches: 0, openHandlers: 1, clientConnectionsClosed: 1, released: true },
    bodyReady: { localBodyChunksProduced: 1, ingressRequests: 1, ingressBodyChunksReceived: 1, bodyChunksForwardedToBFF: 1, bffConnections: 1, ingressRequestBodiesCompleted: 0 },
    bodyCancelled: { outcome: "client_cancelled", bodyChunksProduced: 1 },
    bodyCancelledSettled: { bffDispatches: 0, goDispatches: 0, openHandlers: 0, ingressAborted: 1, ingressClientConnectionsClosed: 0, bffConnectionsClosed: 1, ingressOpenHandlers: 0 },
  }), /LATE_BUSINESS_DISPATCH_OBSERVED/);
});

test("M2 cancellation evidence requires observer receipt, real connection close, explicit release, and handler cleanup", async () => {
  const { evaluateCancelDeadlineControl } = await runner();
  const valid = {
    healthy: { status: 200, bffDispatches: 1, goDispatches: 1, released: true, responsesCompleted: 1, ingress: { received: 1, bodyChunksForwarded: 1, requestBodiesCompleted: 1, responsesCompleted: 1 } },
    cancelled: { outcome: "client_cancelled" },
    cancelledSettled: { bffDispatches: 1, goDispatches: 0, openHandlers: 0, clientConnectionsClosed: 1, released: true },
    deadline: { status: 504 },
    deadlineSettled: { bffDispatches: 1, goDispatches: 0, openHandlers: 0, clientConnectionsClosed: 1, released: true },
    bodyReady: { localBodyChunksProduced: 1, ingressRequests: 1, ingressBodyChunksReceived: 1, bodyChunksForwardedToBFF: 1, bffConnections: 1, ingressRequestBodiesCompleted: 0 },
    bodyCancelled: { outcome: "client_cancelled", bodyChunksProduced: 1 },
    bodyCancelledSettled: { bffDispatches: 0, goDispatches: 0, openHandlers: 0, ingressAborted: 1, ingressClientConnectionsClosed: 0, bffConnectionsClosed: 1, ingressOpenHandlers: 0 },
  };
  assert.doesNotThrow(() => evaluateCancelDeadlineControl(valid));
  assert.throws(() => evaluateCancelDeadlineControl({ ...valid, cancelledSettled: { ...valid.cancelledSettled, bffDispatches: 0 } }), /BFF_DISPATCH_NOT_OBSERVED/);
  assert.throws(() => evaluateCancelDeadlineControl({ ...valid, deadlineSettled: { ...valid.deadlineSettled, clientConnectionsClosed: 0 } }), /OBSERVER_CLIENT_CLOSE_NOT_OBSERVED/);
  assert.throws(() => evaluateCancelDeadlineControl({ ...valid, cancelledSettled: { ...valid.cancelledSettled, released: false } }), /OBSERVER_DELAY_NOT_RELEASED/);
  assert.throws(() => evaluateCancelDeadlineControl({ ...valid, deadlineSettled: { ...valid.deadlineSettled, openHandlers: 1 } }), /OBSERVER_REQUEST_NOT_RELEASED/);
  assert.throws(() => evaluateCancelDeadlineControl({ ...valid, bodyReady: { ...valid.bodyReady, ingressRequests: 0 } }), /BODY_READ_INGRESS_NOT_OBSERVED/);
  assert.throws(() => evaluateCancelDeadlineControl({ ...valid, bodyCancelledSettled: { ...valid.bodyCancelledSettled, bffDispatches: 1 } }), /BODY_READ_LATE_DISPATCH_OBSERVED/);
});

test("M2 delayed observer forwards a still-live request through the same held path", async () => {
  const upstreamRequests = [];
  const upstream = createHTTPServer(async (request, response) => {
    upstreamRequests.push(request.url);
    for await (const _chunk of request) { /* consume the actual request body */ }
    response.writeHead(200, { "content-type": "application/json" });
    response.end(JSON.stringify({ ok: true }));
  });
  await new Promise((resolve, reject) => { upstream.once("error", reject); upstream.listen(0, "127.0.0.1", resolve); });
  const upstreamAddress = upstream.address();
  assert.ok(upstreamAddress && typeof upstreamAddress === "object");
  const { startDispatchObserver, stopDispatchObserver } = await runner();
  const observer = await startDispatchObserver(0, `http://127.0.0.1:${upstreamAddress.port}`, { releaseTimeoutMs: 500 });
  try {
    observer.arm("cancel");
    const pending = fetch(`http://127.0.0.1:${observer.port}/api/v1/referral-registration/intents`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ request: "held-positive-control" }),
      signal: AbortSignal.timeout(500),
    }).catch(error => error);
    await observer.waitReceived("cancel");
    observer.release?.("cancel");
    const response = await pending;

    assert.ok(response instanceof Response);
    assert.equal(response.status, 200);
    assert.deepEqual(upstreamRequests, ["/api/v1/referral-registration/intents"]);
    assert.equal(observer.snapshot("cancel").goDispatches, 1);

    observer.arm("cancel");
    const controller = new AbortController();
    const cancelled = fetch(`http://127.0.0.1:${observer.port}/api/v1/referral-registration/intents`, {
      method: "POST",
      headers: { "content-type": "application/json" },
      body: JSON.stringify({ request: "held-cancel-control" }),
      signal: controller.signal,
    }).catch(error => error);
    await observer.waitReceived("cancel");
    controller.abort();
    assert.ok((await cancelled) instanceof Error);
    await observer.waitClientClosed("cancel");
    observer.release("cancel");
    await observer.waitReleased("cancel");
    const snapshot = observer.snapshot("cancel");
    assert.equal(snapshot.bffDispatches, 1);
    assert.equal(snapshot.goDispatches, 0);
    assert.equal(snapshot.clientConnectionsClosed, 1);
    assert.equal(snapshot.openHandlers, 0);
    assert.equal(snapshot.released, true);
    assert.deepEqual(upstreamRequests, ["/api/v1/referral-registration/intents"]);
  } finally {
    observer.release("cancel");
    await stopDispatchObserver();
    upstream.closeAllConnections?.();
    await new Promise((resolve, reject) => upstream.close(error => error ? reject(error) : resolve()));
  }
});

test("M2 BFF ingress observer proves a partial body reached the upstream connection before cancellation", async () => {
  const upstreamObservation = { chunks: 0, bytes: 0, aborted: 0 };
  const upstream = createHTTPServer(request => {
    request.on("data", chunk => {
      upstreamObservation.chunks++;
      upstreamObservation.bytes += chunk.length;
    });
    request.once("aborted", () => { upstreamObservation.aborted++; });
  });
  await new Promise((resolve, reject) => { upstream.once("error", reject); upstream.listen(0, "127.0.0.1", resolve); });
  const upstreamAddress = upstream.address();
  assert.ok(upstreamAddress && typeof upstreamAddress === "object");
  const { startBFFIngressObserver, stopBFFIngressObserver } = await runner();
  const ingress = await startBFFIngressObserver(0, upstreamAddress.port);
  let client;
  try {
    ingress.arm("body-cancel");
    client = httpRequest({ hostname: "127.0.0.1", port: ingress.port, method: "POST", path: "/api/referral-registration", headers: { "content-type": "application/json" } });
    client.on("error", () => {});
    client.write(Buffer.from('{"partial":', "utf8"));
    await ingress.waitBodyForwarded("body-cancel");
    const ready = ingress.snapshot("body-cancel");
    assert.equal(ready.received, 1);
    assert.ok(ready.bodyChunksReceived > 0);
    assert.ok(ready.bodyChunksForwarded > 0);
    assert.equal(ready.requestBodiesCompleted, 0);
    assert.equal(ready.upstreamConnections, 1);
    assert.ok(upstreamObservation.chunks > 0);
    assert.ok(upstreamObservation.bytes > 0);

    client.destroy();
    await ingress.waitClosed("body-cancel");
    const settled = ingress.snapshot("body-cancel");
    assert.ok(settled.inboundAborted > 0 || settled.clientConnectionsClosed > 0);
    assert.ok(settled.upstreamConnectionsClosed > 0);
    assert.equal(settled.openHandlers, 0);
  } finally {
    client?.destroy();
    await stopBFFIngressObserver();
    upstream.closeAllConnections?.();
    await new Promise((resolve, reject) => upstream.close(error => error ? reject(error) : resolve()));
  }
});

test("M2 ingress completion tracks only the armed idempotency key", async () => {
  const upstream = createHTTPServer(async (request, response) => {
    for await (const _chunk of request) { /* consume the actual request body */ }
    response.writeHead(200, { "content-type": "application/json" });
    response.end(JSON.stringify({ ok: true }));
  });
  await new Promise((resolve, reject) => { upstream.once("error", reject); upstream.listen(0, "127.0.0.1", resolve); });
  const upstreamAddress = upstream.address();
  assert.ok(upstreamAddress && typeof upstreamAddress === "object");
  const { startBFFIngressObserver, stopBFFIngressObserver } = await runner();
  const ingress = await startBFFIngressObserver(0, upstreamAddress.port);
  try {
    ingress.arm("healthy", "target-key");
    for (const key of ["background-key", "target-key"]) {
      const response = await fetch(`http://127.0.0.1:${ingress.port}/api/referral-registration`, {
        method: "POST",
        headers: { "content-type": "application/json", "idempotency-key": key },
        body: JSON.stringify({ key }),
      });
      assert.equal(response.status, 200);
      await response.arrayBuffer();
    }
    await ingress.waitCompleted("healthy");
    const target = ingress.snapshot("healthy");
    assert.equal(target.received, 1);
    assert.equal(target.requestBodiesCompleted, 1);
    assert.equal(target.responsesCompleted, 1);
    assert.equal(target.openHandlers, 0);
    assert.equal(ingress.snapshot("pass").received, 1);
  } finally {
    await stopBFFIngressObserver();
    upstream.closeAllConnections?.();
    await new Promise((resolve, reject) => upstream.close(error => error ? reject(error) : resolve()));
  }
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
