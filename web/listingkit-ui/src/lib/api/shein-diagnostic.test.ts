import { describe, expect, it } from "vitest";
import { parseSheinDiagnostic, parseSheinDiagnosticFailure } from "./shein-diagnostic";

export const diagnosticFixture = () => ({
  diagnostic_only: true, scope: "shein.offline_package",
  target: { marketplace: "shein", site: "" }, action: "publish",
  rule_version: "shein.offline_package.v2",
  input: { actual_digest: `sha256:${"a".repeat(64)}`, binding_version: "shein.persisted-input.go-json.v1", read_at: "2026-09-06T01:02:03.123456789Z", evaluated_at: "2026-09-06T01:02:04Z" },
  external_freshness: { status: "not_evaluated", coverage: [] },
  not_evaluated: ["external_package_freshness", "submission_gate"],
  not_evaluated_reasons: { external_package_freshness: "no_authoritative_package_freshness" },
  offline_checks: { status: "blocked", checks: [], blockers: [], warnings: [] },
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
});
