export function lifecycleScenario({
  businessFailure,
  initialCleanup = cleanupResult("initial", "PASS"),
  finalCleanup = cleanupResult("final", "PASS"),
  persistFailure,
} = {}) {
  const calls = [];
  const emitted = [];
  const persisted = [];
  return {
    calls,
    emitted,
    persisted,
    options: {
      report: baseReport(),
      runBusiness: async () => {
        calls.push("business");
        if (businessFailure) throw new Error(businessFailure);
      },
      runCleanup: async (phase) => {
        calls.push(`cleanup:${phase}`);
        return structuredClone(phase === "initial" ? initialCleanup : finalCleanup);
      },
      persistReport: async (report) => {
        calls.push("persist");
        if (persistFailure) throw new Error(persistFailure);
        persisted.push(structuredClone(report));
      },
      emitFailure: (value) => emitted.push(structuredClone(value)),
      now: monotonicClock(),
    },
  };
}

export function cleanupResult(phase, status, code) {
  return {
    phase,
    status,
    actions: [],
    residuals: {
      status: status === "UNKNOWN" ? "UNKNOWN" : "PASS",
      containers: 0,
      volumes: 0,
      networks: 0,
      listeners: 0,
    },
    ...(code ? { code } : {}),
  };
}

export function cleanupActions({ failedAction, inspectionFailure } = {}) {
  const calls = [];
  const actions = ["browser", "child", "container", "runtime", "private-artifacts"].map((name) => ({
    name,
    run: async () => {
      calls.push(name);
      if (name === failedAction) throw new Error(`FAILED_${name.toUpperCase().replaceAll("-", "_")}`);
    },
  }));
  return {
    actions,
    calls,
    inspectResiduals: async () => {
      calls.push("inspect");
      if (inspectionFailure) throw new Error(inspectionFailure);
      return { containers: 0, volumes: 0, networks: 0, listeners: 0 };
    },
  };
}

export async function runCLIFaultScenario(name, { orchestrate, cleanupPass, runProcess, persistAtomic, recordRuntime, runCleanup }) {
  let scenario;
  if (name === "initial-cleanup-failure") {
    scenario = lifecycleScenario({ initialCleanup: cleanupResult("initial", "FAIL", "DESTROY_FAILED") });
  } else if (["partial-start", "browser-failure", "child-failure"].includes(name)) {
    const code = name === "partial-start" ? "PARTIAL_START_FAILED" : name === "browser-failure" ? "BROWSER_CLOSED" : "PROCESS_FAILED:CURRENT_APPLICATION";
    scenario = lifecycleScenario({ businessFailure: code });
  } else if (name === "cleanup-action-failure" || name === "cleanup-query-failure") {
    const initial = cleanupActions(name === "cleanup-action-failure" ? { failedAction: "container" } : { inspectionFailure: "QUERY_FAILED:DOCKER" });
    const final = cleanupActions();
    scenario = lifecycleScenario();
    scenario.options.runCleanup = async phase => {
      scenario.calls.push(`cleanup:${phase}`);
      const selected = phase === "initial" ? initial : final;
      return cleanupPass({ phase, actions: selected.actions, inspectResiduals: selected.inspectResiduals });
    };
    scenario.cleanupCalls = { initial: initial.calls, final: final.calls };
  } else if (name === "report-write-failure" || name === "report-half-write") {
    scenario = lifecycleScenario({ persistFailure: name === "report-write-failure" ? "REPORT_WRITE_FAILED" : "secret=resumeSecret-value" });
  } else if (name === "real-child-failure") {
    scenario = lifecycleScenario();
    scenario.options.runBusiness = async () => { scenario.calls.push("business"); await runProcess(process.execPath, ["-e", "process.exit(9)"]); };
  } else if (name === "real-report-rename-failure") {
    const root = await mkdtemp(path.join(tmpdir(), "issue413-report-fault-"));
    const destination = path.join(root, "report.json");
    await mkdir(destination);
    scenario = lifecycleScenario();
    scenario.options.persistReport = value => persistAtomic(destination, value);
    scenario.after = async () => {
      const files = await readdir(root);
      const result = { temporaryFiles: files.filter(file => file.includes(".tmp")).length, destinationRemainedDirectory: files.includes("report.json") };
      await rm(root, { recursive: true, force: true });
      return result;
    };
  } else if (name === "unreadable-dispatched-manifest") {
    const runId = randomUUID();
    const root = path.join(tmpdir(), "task-processor-issue357", runId);
    await mkdir(root, { recursive: true });
    await writeFile(path.join(root, "manifest.json"), "{broken", { mode: 0o600 });
    scenario = lifecycleScenario();
    scenario.options.runBusiness = async () => { scenario.calls.push("business"); await recordRuntime(`runId=${runId}`); };
    scenario.options.runCleanup = runCleanup;
    scenario.after = async () => { await rm(root, { recursive: true, force: true }); return { runIdObserved: true }; };
  } else {
    throw new Error("UNKNOWN_TEST_SCENARIO");
  }
  const outcome = await orchestrate(scenario.options);
  const after = scenario.after ? await scenario.after() : undefined;
  return {
    exitCode: outcome.exitCode,
    report: outcome.report,
    calls: scenario.calls,
    emitted: scenario.emitted,
    ...(scenario.cleanupCalls ? { cleanupCalls: scenario.cleanupCalls } : {}),
    ...(after ? { after } : {}),
  };
}

function baseReport() {
  return {
    schemaVersion: "issue413-referral-registration-fixture-v2",
    runId: "00000000-0000-4000-8000-000000000413",
    sourceSha: "a".repeat(40),
    webSha: "a".repeat(40),
    runnerSha256: "b".repeat(64),
    injectionPoint: "test-only",
    checks: [],
  };
}

function monotonicClock() {
  let tick = 0;
  return () => `2026-09-14T00:00:0${tick++}.000Z`;
}
import { randomUUID } from "node:crypto";
import { mkdir, mkdtemp, readdir, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
