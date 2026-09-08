import assert from "node:assert/strict";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import path from "node:path";
import { spawn } from "node:child_process";
import { afterEach, test } from "vitest";
import { createOwnerMutationGuard, createRunFinalizer, platformSignalMatrix } from "./real-provider-browser-lifecycle.mjs";

const temporary = [];
afterEach(async () => { while (temporary.length) await rm(temporary.pop(), { recursive: true, force: true }); });
const wait = milliseconds => new Promise(resolve => setTimeout(resolve, milliseconds));
const deferred = () => { let resolve; const promise = new Promise(done => { resolve = done; }); return { promise, resolve }; };

function lifecycle({ cleanup = true, restoreFails = false, stopFails = false, timeout = 1000 } = {}) {
  const events = [];
  let finalizer;
  const guard = createOwnerMutationGuard({
    interrupted: () => finalizer?.interrupted === true,
    onState: state => events.push(`state:${state.key}:${state.status}`),
  });
  finalizer = createRunFinalizer({
    cleanupOwnedRun: cleanup,
    stepTimeoutMs: timeout,
    closeBrowser: async () => { events.push("close"); },
    recoverOwnerControls: async () => {
      events.push("recover");
      const result = await guard.recoverAll();
      if (!result.ok) throw new Error("owner_recovery_failed");
    },
    stopOwnedRun: async () => { events.push("stop"); if (stopFails) throw new Error("stop_failed"); },
    persistReport: async () => { events.push("persist"); },
    onInterrupt: signal => events.push(`interrupt:${signal}`),
    onFailure: step => events.push(`failure:${step}`),
  });
  const mutation = (key, mutate, recoveryCommand = `recover ${key}`) => ({
    key,
    recoveryCommand,
    mutate,
    restore: async () => { events.push(`restore:${key}`); if (restoreFails) throw new Error("restore_failed"); },
  });
  return { events, finalizer, guard, mutation };
}

test("signal platform matrix distinguishes catchable controls from forced termination", () => {
  const windows = platformSignalMatrix("win32");
  assert.deepEqual(windows.filter(item => item.supported).map(item => item.signal), ["SIGINT", "SIGBREAK"]);
  assert.equal(windows.find(item => item.signal === "SIGTERM").reason, "windows_force_termination_not_catchable");
  const posix = platformSignalMatrix("linux");
  assert.deepEqual(posix.filter(item => item.supported).map(item => item.signal), ["SIGINT", "SIGTERM"]);
  assert.equal(posix.find(item => item.signal === "SIGKILL").supported, false);
});

test("signal before mutation closes and stops once without inventing a restore", async () => {
  const { events, finalizer } = lifecycle();
  const first = finalizer.requestInterrupt("SIGINT");
  const second = finalizer.requestInterrupt("SIGINT");
  assert.equal(first, second);
  const result = await first;
  assert.equal(result.ok, true);
  assert.deepEqual(events, ["interrupt:SIGINT", "close", "recover", "stop", "persist"]);
  assert.throws(() => finalizer.throwIfInterrupted(), /runner_interrupted/);
});

test("confirmed revoke and provider-stop restore before owned stop and block later cases", async () => {
  for (const key of ["grant-admin-B", "provider"]) {
    const { events, finalizer, guard, mutation } = lifecycle();
    const entered = deferred();
    void guard.run(mutation(key, async () => events.push(`mutate:${key}`)), async () => {
      events.push(`operation:${key}`); entered.resolve(); await new Promise(() => {});
    }).catch(() => {});
    await entered.promise;
    const result = await finalizer.requestInterrupt("SIGINT");
    assert.equal(result.ok, true);
    assert.deepEqual(events, [
      `state:${key}:registered`, `mutate:${key}`, `state:${key}:applied`, `operation:${key}`,
      "interrupt:SIGINT", "close", "recover", `state:${key}:restoring`, `restore:${key}`, `state:${key}:restored`, "stop", "persist",
    ]);
    assert.throws(() => finalizer.throwIfInterrupted(), /runner_interrupted/);
  }
});

test("in-flight and response-late mutations settle before their single restore", async () => {
  for (const key of ["in-flight", "response-late"]) {
    const { events, finalizer, guard, mutation } = lifecycle();
    const started = deferred(); const finish = deferred();
    void guard.run(mutation(key, async () => {
      events.push(`mutate-start:${key}`); started.resolve(); await finish.promise; events.push(`mutate-end:${key}`);
    }), async () => events.push(`operation:${key}`)).catch(() => {});
    await started.promise;
    const interrupted = finalizer.requestInterrupt("SIGTERM");
    await wait(10);
    assert.deepEqual(events.slice(-3), ["interrupt:SIGTERM", "close", "recover"]);
    finish.resolve();
    const result = await interrupted;
    assert.equal(result.ok, true);
    assert.ok(events.indexOf(`mutate-end:${key}`) < events.indexOf(`restore:${key}`));
    assert.equal(events.filter(item => item === `restore:${key}`).length, 1);
    assert.equal(events.includes(`operation:${key}`), false);
  }
});

test("restore and stop failures are both attempted, persisted, and remain non-success", async () => {
  const { events, finalizer, guard, mutation } = lifecycle({ restoreFails: true, stopFails: true });
  const entered = deferred();
  void guard.run(mutation("provider", async () => {}), async () => { entered.resolve(); await new Promise(() => {}); }).catch(() => {});
  await entered.promise;
  const result = await finalizer.requestInterrupt("SIGINT");
  assert.equal(result.ok, false);
  assert.deepEqual(result.failures, ["owner-recovery", "owned-run-stop"]);
  assert.ok(events.indexOf("restore:provider") < events.indexOf("stop"));
  assert.ok(events.indexOf("stop") < events.indexOf("persist"));
  assert.deepEqual(guard.snapshot().map(item => ({ key: item.key, status: item.status, recoveryCommand: item.recoveryCommand })), [
    { key: "provider", status: "restore-failed", recoveryCommand: "recover provider" },
  ]);
});

test("bounded recovery timeout still attempts stop and persists safe recovery information", async () => {
  const { events, finalizer, guard } = lifecycle({ timeout: 20 });
  const started = deferred();
  void guard.run({ key: "grant", recoveryCommand: "restore grant", mutate: async () => { started.resolve(); await new Promise(() => {}); }, restore: async () => {} }, async () => {}).catch(() => {});
  await started.promise;
  const result = await finalizer.requestInterrupt("SIGINT");
  assert.equal(result.ok, false);
  assert.deepEqual(result.failures, ["owner-recovery"]);
  assert.ok(events.includes("stop"));
  assert.ok(events.includes("persist"));
  assert.equal(guard.snapshot()[0].recoveryCommand, "restore grant");
});

test("normal completion restores exactly once and stops only with explicit cleanup flag", async () => {
  for (const cleanup of [false, true]) {
    const { events, finalizer, guard, mutation } = lifecycle({ cleanup });
    await guard.run(mutation("grant", async () => events.push("mutate")), async () => events.push("operation"));
    const result = await finalizer.finish();
    assert.equal(result.ok, true);
    assert.equal(events.filter(item => item === "restore:grant").length, 1);
    assert.equal(events.filter(item => item === "stop").length, cleanup ? 1 : 0);
    assert.equal(events.at(-1), "persist");
  }
});

async function runSignalChild(scenario, signal) {
  const directory = await mkdtemp(path.join(tmpdir(), "issue358-signal-")); temporary.push(directory);
  const log = path.join(directory, "events.jsonl");
  const child = spawn(process.execPath, [path.join(import.meta.dirname, "real-provider-browser-lifecycle-child.mjs"), scenario, log, "--stop-owned-run"], {
    cwd: import.meta.dirname, stdio: ["ignore", "pipe", "pipe"], windowsHide: true,
  });
  let stdout = "", stderr = "";
  child.stdout.on("data", value => { stdout += value; }); child.stderr.on("data", value => { stderr += value; });
  const deadline = Date.now() + 5000;
  while (!stdout.includes("READY_FOR_SIGNAL") && Date.now() < deadline) await wait(10);
  assert.match(stdout, /READY_FOR_SIGNAL/);
  child.kill(signal);
  const exit = await new Promise(resolve => child.once("exit", (code, exitSignal) => resolve({ code, exitSignal })));
  assert.equal(stderr, "");
  return { exit, events: (await readFile(log, "utf8")).trim().split("\n").map(line => JSON.parse(line).event) };
}

test.skipIf(process.platform === "win32")("real POSIX child signals restore confirmed owner mutations before stop", async () => {
  for (const [scenario, signal, restore] of [["revoke", "SIGINT", "restore:grant"], ["provider-stop", "SIGTERM", "restore:provider"]]) {
    const result = await runSignalChild(scenario, signal);
    assert.equal(result.exit.code, signal === "SIGINT" ? 130 : 143);
    assert.equal(result.exit.exitSignal, null);
    assert.ok(result.events.indexOf(restore) < result.events.indexOf("stop"));
    assert.equal(result.events.filter(item => item === restore).length, 1);
  }
});
