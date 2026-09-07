import assert from "node:assert/strict";
import { tmpdir } from "node:os";
import path from "node:path";
import { test } from "vitest";
import { validateBrowserHandoff } from "./real-provider-browser-contract.mjs";

// Schema checks only. These objects never authenticate a browser or count as E2E.
const sha = "a".repeat(40);
const webSha = "b".repeat(40);
const runId = "7b578139-cf12-413f-9e02-3a3dbd68c128";
const root = path.join(tmpdir(), "task-processor-issue357", runId);
const manifestPath = path.join(root, "manifest.json");
function handoff() {
  return {
    schemaVersion: "issue357-v1", runId, status: "ready", sourceSha: sha, webSha,
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
