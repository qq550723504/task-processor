import { z } from "zod";
import { parseWorkbenchErrorEnvelopePayload, type WorkbenchErrorEnvelope } from "./workbench-context";

export const sheinDiagnosticActionSchema = z.enum(["save_draft", "publish"]);
export type SheinDiagnosticAction = z.infer<typeof sheinDiagnosticActionSchema>;
export const sheinDigestSchema = z.string().regex(/^sha256:[0-9a-f]{64}$/);
export const sheinRecordIdSchema = z.string().regex(/^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$/);
export const diagnosticOrganizationSchema = z.string().regex(/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/);
const timestamp = z.iso.datetime();
// Wire validation only. The Go evaluator remains the authority for rule outcomes.
const text = z.string();
const check = z.strictObject({
  rule: text, code: text, category: text,
  status: z.enum(["ready", "warning", "blocking"]),
  paths: z.array(text).optional(), message: text.optional(), guidance: text.optional(),
});
const evidence = z.strictObject({
  subject_digest: sheinDigestSchema, policy_version: text, source: text,
  observed_at: timestamp, valid_until: timestamp,
});
const freshness = z.union([
  z.strictObject({ status: z.literal("not_evaluated"), coverage: z.array(text).length(0) }),
  z.strictObject({ status: z.literal("valid"), coverage: z.array(text), evidence }),
]);
const diagnostic = z.strictObject({
  diagnostic_only: z.literal(true), scope: z.literal("shein.offline_package"),
  target: z.strictObject({ marketplace: z.literal("shein"), site: z.literal("") }),
  action: sheinDiagnosticActionSchema, rule_version: z.literal("shein.offline_package.v2"),
  input: z.strictObject({ actual_digest: sheinDigestSchema, binding_version: z.literal("shein.persisted-input.go-json.v1"), read_at: timestamp, evaluated_at: timestamp }),
  external_freshness: freshness, not_evaluated: z.array(text),
  not_evaluated_reasons: z.record(text, text).optional(),
  offline_checks: z.strictObject({ status: z.enum(["ready", "ready_with_warnings", "blocked"]), checks: z.array(check), blockers: z.array(check), warnings: z.array(check) }),
  action_policy: z.strictObject({ readiness_blockers_allowed: z.boolean() }),
}).refine((value) => value.external_freshness.status !== "not_evaluated" || (
  value.not_evaluated.includes("external_package_freshness") &&
  value.not_evaluated_reasons?.external_package_freshness === "no_authoritative_package_freshness"
));
export type SheinDiagnostic = z.infer<typeof diagnostic>;
const failure = z.strictObject({
  error: z.enum(["invalid_request", "unsupported_action", "unsupported_target", "permission_denied", "not_found", "stale_input", "input_too_large", "invalid_input", "evaluation_failed", "unsupported_rule_version", "unavailable", "deadline_exceeded"]),
  freshness: z.strictObject({ status: z.enum(["stale", "expired", "valid"]), coverage: z.array(text), causes: z.array(z.enum(["stale", "expired", "expired_at_evaluation", "subject_mismatch"])) }).optional(),
}).refine((value) => value.freshness === undefined || value.error === "stale_input");
export type SheinDiagnosticFailure = z.infer<typeof failure> | WorkbenchErrorEnvelope;
const diagnosticStatuses: Record<z.infer<typeof failure>["error"], number> = {
  invalid_request: 400, unsupported_action: 400, unsupported_target: 400,
  permission_denied: 403, not_found: 404, stale_input: 409,
  input_too_large: 413, invalid_input: 422, evaluation_failed: 500,
  unsupported_rule_version: 500, unavailable: 503, deadline_exceeded: 504,
};
const workbenchStatuses: Record<string, readonly number[]> = {
  INVALID_REQUEST: [400, 405], AUTHENTICATION_REQUIRED: [401],
  ORGANIZATION_SELECTION_REQUIRED: [409], ORGANIZATION_CONTEXT_CHANGED: [409],
  ORGANIZATION_ACCESS_DENIED: [403], ORGANIZATION_ACCESS_REVOKED: [403],
  ORGANIZATION_SUSPENDED: [403], PERMISSION_DENIED: [403],
  DEPENDENCY_UNAVAILABLE: [502, 503], INVALID_UPSTREAM_RESPONSE: [502], DEADLINE_EXCEEDED: [504],
};
export function parseSheinDiagnostic(value: unknown): SheinDiagnostic | null {
  const parsed = diagnostic.safeParse(value);
  return parsed.success ? parsed.data : null;
}
export function parseSheinDiagnosticFailure(value: unknown, status: number): SheinDiagnosticFailure | null {
  const parsed = failure.safeParse(value);
  if (parsed.success && diagnosticStatuses[parsed.data.error] === status) return parsed.data;
  const auth = parseWorkbenchErrorEnvelopePayload(value);
  return auth.success && workbenchStatuses[auth.data.code]?.includes(status) ? auth.data : null;
}
