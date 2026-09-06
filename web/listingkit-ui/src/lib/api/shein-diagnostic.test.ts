import { describe, expect, it } from "vitest";
import { parseSheinDiagnostic, parseSheinDiagnosticFailure } from "./shein-diagnostic";

const blocker = { rule: "fixture", code: "missing_asset", category: "asset", status: "blocking" };
export const diagnosticFixture = () => ({
  diagnostic_only: true, scope: "shein.offline_package",
  target: { marketplace: "shein", site: "" }, action: "publish",
  rule_version: "shein.offline_package.v2",
  input: { actual_digest: `sha256:${"a".repeat(64)}`, binding_version: "shein.persisted-input.go-json.v1", read_at: "2026-09-06T01:02:03.123456789Z", evaluated_at: "2026-09-06T01:02:04Z" },
  external_freshness: { status: "not_evaluated", coverage: [] },
  not_evaluated: ["external_package_freshness", "submission_gate"],
  not_evaluated_reasons: { external_package_freshness: "no_authoritative_package_freshness" },
  offline_checks: { status: "blocked", checks: [blocker], blockers: [blocker], warnings: [] },
  action_policy: { readiness_blockers_allowed: false },
});

describe("Shein diagnostic wire contract", () => {
  it("preserves nanoseconds, explicit unknown scope and empty arrays without inferring readiness", () => {
    const value = diagnosticFixture();
    expect(parseSheinDiagnostic(value)).toEqual(value);
  });
  it.each([
    { diagnostic_only: false }, { rule_version: "v1" }, { action: "apply" },
    { not_evaluated: null }, { not_evaluated: undefined }, { payload: "private" },
    { not_evaluated_reasons: undefined }, { not_evaluated_reasons: {} },
    { not_evaluated: ["submission_gate"] }, { target: { marketplace: "shein", site: "unknown" } },
    { external_freshness: { status: "not_evaluated", coverage: null } },
    { offline_checks: { status: "ready", checks: null, blockers: [], warnings: [] } },
  ])("rejects malformed response %j", (patch) => {
    expect(parseSheinDiagnostic({ ...diagnosticFixture(), ...patch })).toBeNull();
  });
  it("rejects impossible timestamps and malformed digest", () => {
    const value = diagnosticFixture();
    expect(parseSheinDiagnostic({ ...value, input: { ...value.input, read_at: "2026-02-30T01:00:00Z" } })).toBeNull();
    expect(parseSheinDiagnostic({ ...value, input: { ...value.input, actual_digest: "sha256:bad" } })).toBeNull();
    expect(parseSheinDiagnostic({ ...value, input: { ...value.input, read_at: "2026-09-06T01:02:04.000000001Z" } })).toBeNull();
  });
  it("validates valid freshness against its own evaluation instant, with nanosecond precision", () => {
    const base = diagnosticFixture();
    const valid = {
      ...base,
      not_evaluated: ["submission_gate"], not_evaluated_reasons: undefined,
      input: { ...base.input, evaluated_at: "2026-09-06T01:02:04.000000002Z" },
      external_freshness: { status: "valid", coverage: ["external_package_freshness"], evidence: {
        subject_digest: base.input.actual_digest, source: "owner", policy_version: "policy-v1",
        observed_at: "2026-09-06T01:02:04.000000001Z", valid_until: "2026-09-06T01:02:04.000000003Z",
      } },
    };
    expect(parseSheinDiagnostic(valid)).toEqual(valid);
    expect(parseSheinDiagnostic({ ...valid, input: { ...valid.input, read_at: "2026-09-06T01:02:04.000000003Z" } })).toBeNull();
    expect(parseSheinDiagnostic({ ...valid, input: { ...valid.input, read_at: valid.input.evaluated_at } })).not.toBeNull();
    for (const coverage of [[], ["other"], ["external_package_freshness", "other"]]) {
      expect(parseSheinDiagnostic({ ...valid, external_freshness: { ...valid.external_freshness, coverage } })).toBeNull();
    }
    for (const patch of [
      { subject_digest: `sha256:${"b".repeat(64)}` },
      { source: "" }, { policy_version: " " },
      { observed_at: "2026-09-06T01:02:04.000000003Z" },
      { valid_until: "2026-09-06T01:02:04.000000002Z" },
      { valid_until: "2026-09-06T01:02:04Z" },
    ]) {
      expect(parseSheinDiagnostic({ ...valid, external_freshness: { ...valid.external_freshness, evidence: { ...valid.external_freshness.evidence, ...patch } } })).toBeNull();
    }
    expect(parseSheinDiagnostic({ ...valid, not_evaluated: ["external_package_freshness"] })).toBeNull();
  });
  it("preserves both error families and rejects status/code conflicts or extra private fields", () => {
    const stale = { error: "stale_input", freshness: { status: "valid", coverage: ["external_package_freshness"], causes: ["expired_at_evaluation"] } };
    expect(parseSheinDiagnosticFailure(stale, 409)).toEqual(stale);
    expect(parseSheinDiagnosticFailure(stale, 200)).toBeNull();
    expect(parseSheinDiagnosticFailure({ ...stale, sql: "secret" }, 409)).toBeNull();
    expect(parseSheinDiagnosticFailure({ error: "not_found", freshness: stale.freshness }, 404)).toBeNull();
    const auth = { code: "ORGANIZATION_ACCESS_REVOKED", message: "Denied", requestId: "request", fieldErrors: [] };
    expect(parseSheinDiagnosticFailure(auth, 403)).toEqual(auth);
    expect(parseSheinDiagnosticFailure(auth, 500)).toBeNull();
  });
  it("rejects contradictory policy, summary and redundant check partitions", () => {
    const value = diagnosticFixture();
    expect(parseSheinDiagnostic({ ...value, action_policy: { readiness_blockers_allowed: true } })).toBeNull();
    expect(parseSheinDiagnostic({ ...value, action: "save_draft" })).toBeNull();
    expect(parseSheinDiagnostic({ ...value, action: "save_draft", action_policy: { readiness_blockers_allowed: true } })).not.toBeNull();
    for (const patch of [
      { status: "ready" }, { blockers: [] }, { warnings: [blocker] },
      { blockers: [{ ...blocker, message: "different" }] }, { checks: [] },
    ]) expect(parseSheinDiagnostic({ ...value, offline_checks: { ...value.offline_checks, ...patch } })).toBeNull();
    const warning = { ...blocker, status: "warning" };
    const ready = { ...blocker, status: "ready" };
    expect(parseSheinDiagnostic({ ...value, offline_checks: { status: "ready_with_warnings", checks: [warning], blockers: [], warnings: [warning] } })).not.toBeNull();
    expect(parseSheinDiagnostic({ ...value, offline_checks: { status: "ready", checks: [ready], blockers: [], warnings: [] } })).not.toBeNull();
  });
});
