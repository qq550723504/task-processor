import { appendFile } from "node:fs/promises";
import { createOwnerMutationGuard, createRunFinalizer } from "./real-provider-browser-lifecycle.mjs";

const [scenario, log, cleanupFlag] = process.argv.slice(2);
const record = event => appendFile(log, `${JSON.stringify({ event })}\n`);
let finalizer;
const guard = createOwnerMutationGuard({ interrupted: () => finalizer?.interrupted === true });
finalizer = createRunFinalizer({
  cleanupOwnedRun: cleanupFlag === "--stop-owned-run",
  closeBrowser: () => record("close"),
  recoverOwnerControls: async () => { await record("recover"); const result = await guard.recoverAll(); if (!result.ok) throw new Error("recovery_failed"); },
  stopOwnedRun: () => record("stop"),
  persistReport: () => record("persist"),
  onInterrupt: signal => record(`interrupt:${signal}`),
  exit: code => process.exit(code),
});
finalizer.installProcessHandlers();
setInterval(() => {}, 60000);

const key = scenario === "revoke" ? "grant" : "provider";
await guard.run({
  key,
  recoveryCommand: key === "grant" ? "restore --user admin --org B" : "provider-start",
  mutate: () => record(scenario === "revoke" ? "revoke" : "provider-stop"),
  restore: () => record(`restore:${key}`),
}, async () => {
  await record("operation");
  console.log("READY_FOR_SIGNAL");
  await new Promise(() => {});
});
