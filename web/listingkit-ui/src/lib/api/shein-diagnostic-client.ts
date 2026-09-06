import { diagnosticOrganizationSchema, parseSheinDiagnostic, parseSheinDiagnosticFailure, sheinDiagnosticActionSchema, sheinDigestSchema, sheinRecordIdSchema, type SheinDiagnostic, type SheinDiagnosticAction, type SheinDiagnosticFailure } from "./shein-diagnostic";
import { readSheinDiagnosticJSON } from "./shein-diagnostic-json";

export class SheinDiagnosticError extends Error {
  constructor(public readonly status: number, public readonly code: string, public readonly payload: SheinDiagnosticFailure) {
    super(`Diagnostic request failed (${code})`);
    this.name = "SheinDiagnosticError";
  }
}
const invalidResponse = () => new SheinDiagnosticError(502, "INVALID_UPSTREAM_RESPONSE", { code: "INVALID_UPSTREAM_RESPONSE", message: "Diagnostic response is invalid", requestId: "", fieldErrors: [] });

export async function fetchSheinDiagnostic(input: {
  recordId: string;
  action: SheinDiagnosticAction;
  organizationId: string;
  expectedDigest?: string;
  signal?: AbortSignal;
}): Promise<SheinDiagnostic> {
  if (!sheinRecordIdSchema.safeParse(input.recordId).success || !sheinDiagnosticActionSchema.safeParse(input.action).success || !diagnosticOrganizationSchema.safeParse(input.organizationId).success || (input.expectedDigest !== undefined && !sheinDigestSchema.safeParse(input.expectedDigest).success)) {
    throw new SheinDiagnosticError(400, "invalid_request", { error: "invalid_request" });
  }
  const query = new URLSearchParams({ action: input.action });
  if (input.expectedDigest !== undefined) query.set("expected_digest", input.expectedDigest);
  const response = await fetch(`/api/listing/shein-records/${input.recordId}/offline-diagnostic?${query}`, {
    method: "GET", credentials: "same-origin", cache: "no-store", redirect: "error",
    headers: { Accept: "application/json", "X-Expected-Organization-ID": input.organizationId },
    ...(input.signal ? { signal: input.signal } : {}),
  });
  let payload: unknown;
  try { payload = await readSheinDiagnosticJSON(response, input.signal); }
  catch { input.signal?.throwIfAborted(); throw invalidResponse(); }
  if (response.status === 200) {
    const parsed = parseSheinDiagnostic(payload);
    if (!parsed || parsed.action !== input.action || (input.expectedDigest !== undefined && parsed.input.actual_digest !== input.expectedDigest)) throw invalidResponse();
    return parsed;
  }
  const error = parseSheinDiagnosticFailure(payload, response.status);
  if (!error) throw invalidResponse();
  throw new SheinDiagnosticError(response.status, "error" in error ? error.error : error.code, error);
}
