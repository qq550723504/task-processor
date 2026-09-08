const signalExitCodes = { SIGINT: 130, SIGTERM: 143, SIGBREAK: 149 };

export function platformSignalMatrix(platform) {
  if (platform === "win32") return [
    { signal: "SIGINT", trigger: "Ctrl+C", supported: true, automated: false, reason: "windows_console_control" },
    { signal: "SIGBREAK", trigger: "Ctrl+Break", supported: true, automated: false, reason: "windows_console_control" },
    { signal: "SIGTERM", trigger: "TerminateProcess", supported: false, automated: false, reason: "windows_force_termination_not_catchable" },
    { signal: "SIGKILL", trigger: "TerminateProcess", supported: false, automated: false, reason: "windows_force_termination_not_catchable" },
  ];
  return [
    { signal: "SIGINT", trigger: "SIGINT", supported: true, automated: true, reason: "posix_catchable_signal" },
    { signal: "SIGTERM", trigger: "SIGTERM", supported: true, automated: true, reason: "posix_catchable_signal" },
    { signal: "SIGKILL", trigger: "SIGKILL", supported: false, automated: false, reason: "posix_uncatchable_signal" },
  ];
}

export function createOwnerMutationGuard({ interrupted = () => false, onState = () => {} } = {}) {
  const records = new Map();

  const publish = record => onState({ key: record.key, status: record.status, recoveryCommand: record.recoveryCommand });
  const recover = record => {
    if (!record.restorePromise) record.restorePromise = (async () => {
      await record.mutationPromise.catch(() => {});
      record.status = "restoring"; publish(record);
      try {
        await record.restore();
        record.status = "restored"; publish(record);
      } catch {
        record.status = "restore-failed"; publish(record);
        throw new Error("owner_restore_failed");
      }
    })();
    return record.restorePromise;
  };

  return {
    async run({ key, recoveryCommand, mutate, restore }, operation) {
      if (interrupted()) throw new Error("runner_interrupted");
      if (!key || records.has(key) || typeof mutate !== "function" || typeof restore !== "function") throw new Error("owner_mutation_invalid");
      const record = { key, recoveryCommand, mutate, restore, status: "registered" };
      records.set(key, record); publish(record);
      record.mutationPromise = (async () => {
        try {
          await mutate();
          record.status = "applied"; publish(record);
        } catch {
          record.status = "uncertain"; publish(record);
          throw new Error("owner_mutation_failed");
        }
      })();
      try {
        await record.mutationPromise;
        if (interrupted()) throw new Error("runner_interrupted");
        return await operation();
      } finally { await recover(record); }
    },
    async recoverAll() {
      const pending = [...records.values()].filter(record => record.status !== "restored").reverse();
      const results = [];
      for (const record of pending) {
        try { await recover(record); results.push({ key: record.key, status: record.status }); }
        catch { results.push({ key: record.key, status: record.status }); }
      }
      return { ok: results.every(item => item.status === "restored"), results };
    },
    snapshot() {
      return [...records.values()].map(({ key, status, recoveryCommand }) => ({ key, status, recoveryCommand }));
    },
  };
}

export function createRunFinalizer({
  cleanupOwnedRun,
  closeBrowser,
  recoverOwnerControls,
  stopOwnedRun,
  persistReport,
  onInterrupt = () => {},
  onFailure = () => {},
  exit = code => process.exit(code),
  stepTimeoutMs = 125000,
  platform = process.platform,
} = {}) {
  let interruptSignal;
  let finalization;
  let installed;
  let exitScheduled = false;

  const step = async (name, operation, failures) => {
    let timer;
    try {
      await Promise.race([
        Promise.resolve().then(operation),
        new Promise((_, reject) => { timer = setTimeout(() => reject(new Error("cleanup_timeout")), stepTimeoutMs); timer.unref?.(); }),
      ]);
    } catch {
      failures.push(name); onFailure(name);
    } finally { clearTimeout(timer); }
  };

  const finalize = async () => {
    const failures = [];
    await step("browser-close", closeBrowser, failures);
    await step("owner-recovery", recoverOwnerControls, failures);
    if (cleanupOwnedRun) await step("owned-run-stop", stopOwnedRun, failures);
    await step("report-persist", persistReport, failures);
    return { ok: failures.length === 0, failures, signal: interruptSignal };
  };

  const requestInterrupt = signal => {
    if (!interruptSignal) { interruptSignal = signal; onInterrupt(signal); }
    if (!finalization) finalization = finalize();
    return finalization;
  };

  return {
    get interrupted() { return Boolean(interruptSignal); },
    get signal() { return interruptSignal; },
    requestInterrupt,
    finish() { if (!finalization) finalization = finalize(); return finalization; },
    throwIfInterrupted() { if (interruptSignal) throw new Error("runner_interrupted"); },
    installProcessHandlers(target = process) {
      if (installed) return;
      const handlers = new Map();
      for (const { signal, supported } of platformSignalMatrix(platform)) {
        if (!supported) continue;
        const handler = () => {
          const pending = requestInterrupt(signal);
          if (!exitScheduled) {
            exitScheduled = true;
            void pending.then(result => exit(result.ok ? signalExitCodes[signal] ?? 1 : 1), () => exit(1));
          }
        };
        target.on(signal, handler); handlers.set(signal, handler);
      }
      installed = { target, handlers };
    },
    removeProcessHandlers() {
      if (!installed) return;
      for (const [signal, handler] of installed.handlers) installed.target.off(signal, handler);
      installed = undefined;
    },
  };
}
