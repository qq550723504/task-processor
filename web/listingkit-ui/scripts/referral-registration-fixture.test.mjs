import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";

import {
  cleanupActions,
  cleanupResult,
  lifecycleScenario,
} from "./fixtures/referral-registration-fixture-fault-runtime.mjs";

async function runner() {
  return import("./referral-registration-fixture.mjs");
}

function runCLI(name) {
  const result = spawnSync(process.execPath, [fileURLToPath(new URL("./referral-registration-fixture.mjs", import.meta.url)), "lifecycle-test", name], {
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
