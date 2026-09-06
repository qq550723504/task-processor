import { NextResponse } from "next/server";
import { diagnosticOrganizationSchema, parseSheinDiagnostic, parseSheinDiagnosticFailure, sheinDiagnosticActionSchema, sheinDigestSchema, sheinRecordIdSchema } from "@/lib/api/shein-diagnostic";
import { InvalidSheinDiagnosticResponseError, readSheinDiagnosticJSON } from "@/lib/api/shein-diagnostic-json";
import { WORKBENCH_COOKIE_NAME, workbenchProtocolError } from "./workbench-proxy";
import { configuredSheinRecordsOrigin } from "./shein-records-origin";

const unavailable = () => workbenchProtocolError(502, "DEPENDENCY_UNAVAILABLE", "Diagnostic upstream is unavailable");
const invalidResponse = () => workbenchProtocolError(502, "INVALID_UPSTREAM_RESPONSE", "Diagnostic upstream response is invalid");
const diagnosticError = (error: string) => safeJSON({ error }, 400);
function safeJSON(payload: unknown, status: number) {
  return NextResponse.json(payload, { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" } });
}
function selectedOrganization(request: Request): string | null {
  const cookies = (request.headers.get("cookie") ?? "").split(";").map((part) => part.trim());
  const values = cookies.filter((part) => part.startsWith(`${WORKBENCH_COOKIE_NAME}=`));
  if (values.length !== 1) return null;
  try {
    const value = decodeURIComponent(values[0].slice(WORKBENCH_COOKIE_NAME.length + 1)).trim();
    return diagnosticOrganizationSchema.safeParse(value).success ? value : null;
  } catch { return null; }
}
/** Called only with the access token from the verified server session. */
export async function proxySheinDiagnostic(request: Request, recordId: string, accessToken: string): Promise<Response> {
  if (request.method !== "GET") return workbenchProtocolError(405, "INVALID_REQUEST", "Method is not allowed");
  if (!accessToken) return workbenchProtocolError(401, "AUTHENTICATION_REQUIRED", "Authentication is required");
  const organization = selectedOrganization(request);
  if (!organization || organization !== request.headers.get("X-Expected-Organization-ID")) {
    return workbenchProtocolError(409, "ORGANIZATION_CONTEXT_CHANGED", "Organization context changed");
  }
  const url = new URL(request.url);
  if (!sheinRecordIdSchema.safeParse(recordId).success || url.pathname !== `/api/listing/shein-records/${recordId}/offline-diagnostic` || request.body !== null || (request.headers.has("content-length") && request.headers.get("content-length") !== "0") || request.headers.has("transfer-encoding")) {
    void request.body?.cancel().catch(() => undefined);
    return diagnosticError("invalid_request");
  }
  const rawQuery = url.search.slice(1);
  if (new TextEncoder().encode(rawQuery).length > 1024 || rawQuery.includes(";")) return diagnosticError("invalid_request");
  try { decodeURIComponent(rawQuery); } catch { return diagnosticError("invalid_request"); }
  for (const key of url.searchParams.keys()) {
    if (!["action", "expected_digest"].includes(key) || url.searchParams.getAll(key).length !== 1) return diagnosticError("invalid_request");
  }
  const action = sheinDiagnosticActionSchema.safeParse(url.searchParams.get("action"));
  if (!action.success) return diagnosticError("unsupported_action");
  const expected = url.searchParams.get("expected_digest");
  if (expected !== null && !sheinDigestSchema.safeParse(expected).success) return diagnosticError("invalid_request");
  const origin = configuredSheinRecordsOrigin();
  if (!origin) return unavailable();
  const upstreamURL = new URL(url.pathname, origin);
  upstreamURL.searchParams.set("action", action.data);
  if (expected !== null) upstreamURL.searchParams.set("expected_digest", expected);
  const controller = new AbortController();
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  if (request.signal.aborted) abort();
  const timeout = setTimeout(abort, 15000); // Go has a 5s diagnostic budget plus response headroom.
  try {
    controller.signal.throwIfAborted();
    const upstream = await fetch(upstreamURL, {
      method: "GET", headers: { Accept: "application/json", Authorization: `Bearer ${accessToken}`, "X-Requested-Organization-ID": organization },
      cache: "no-store", redirect: "manual", signal: controller.signal,
    });
    let payload: unknown;
    try { payload = await readSheinDiagnosticJSON(upstream, controller.signal); }
    catch (error) {
      if (controller.signal.aborted) throw error;
      return error instanceof InvalidSheinDiagnosticResponseError ? invalidResponse() : unavailable();
    }
    if (upstream.status === 200) {
      const result = parseSheinDiagnostic(payload);
      if (!result || result.action !== action.data || (expected !== null && result.input.actual_digest !== expected)) return invalidResponse();
      return safeJSON(result, 200);
    }
    const error = parseSheinDiagnosticFailure(payload, upstream.status);
    if (!error) return invalidResponse();
    const response = safeJSON(error, upstream.status);
    if ("code" in error && ["ORGANIZATION_ACCESS_REVOKED", "ORGANIZATION_ACCESS_DENIED"].includes(error.code)) {
      response.cookies.set(WORKBENCH_COOKIE_NAME, "", { httpOnly: true, sameSite: "lax", path: "/", secure: process.env.NODE_ENV !== "development", maxAge: 0 });
    }
    return response;
  } catch {
    return controller.signal.aborted ? workbenchProtocolError(504, "DEADLINE_EXCEEDED", "Diagnostic request ended before completion") : unavailable();
  } finally {
    clearTimeout(timeout);
    request.signal.removeEventListener("abort", abort);
  }
}
