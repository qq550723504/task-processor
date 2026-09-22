import { NextResponse } from "next/server";
import { AccountReadError, accountErrorCode, parseAccountPayload } from "@/lib/api/account";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";

const validID = (value: string | null | undefined): value is string => typeof value === "string" && /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(value);
function json(value: unknown, status: number) { return NextResponse.json(value, { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" } }); }
export function accountFailure(status: number, code: string, outcome?: "not_sent" | "unknown") { return json({ code, message: outcome === "unknown" ? "操作结果待核实，请刷新资料确认状态" : "Account request could not be completed", requestId: "", fieldErrors: [], ...(outcome === "unknown" ? { outcome } : {}) }, status); }
function serviceOrigin(): string | null {
  try {
    const raw = process.env.LISTINGKIT_SERVICE_API_BASE?.trim(); if (!raw) return null;
    const u = new URL(raw);
    return ["http:", "https:"].includes(u.protocol) && !u.username && !u.password && !u.search && !u.hash && ["/api/v1", "/api/v1/"].includes(u.pathname) ? u.origin : null;
  } catch { return null; }
}

export async function proxyAccount(request: Request, token: string, sessionUserId: string, kind: "profile" | "organization" | "business-profile" | "member-allocations"): Promise<Response> {
  if (!["business-profile", "member-allocations"].includes(kind) && request.method !== "GET" || ["business-profile", "member-allocations"].includes(kind) && !["GET", "PUT"].includes(request.method)) return accountFailure(405, "INVALID_REQUEST");
  if (!token || !validID(sessionUserId)) return accountFailure(401, "AUTHENTICATION_REQUIRED");
  if (request.headers.get("X-Expected-User-ID") !== sessionUserId) return accountFailure(409, "IDENTITY_CONTEXT_CHANGED");
  const url = new URL(request.url);
  const memberPath = kind === "member-allocations" && url.pathname.startsWith("/api/account/member-allocations/") ? url.pathname.slice("/api/account/member-allocations/".length) : "";
  if ((kind !== "member-allocations" ? url.pathname !== `/api/account/${kind}` : request.method === "GET" ? url.pathname !== "/api/account/member-allocations" : !validID(memberPath)) || url.search || request.url.endsWith("?") || (!["business-profile", "member-allocations"].includes(kind) || request.method !== "PUT") && (request.body || (request.headers.has("content-length") && request.headers.get("content-length") !== "0") || request.headers.has("transfer-encoding"))) {
    void request.body?.cancel().catch(() => undefined); return accountFailure(400, "INVALID_REQUEST");
  }
  let organization: string | undefined;
  if (kind === "organization" || kind === "member-allocations" || kind === "business-profile") {
    const selections = (request.headers.get("cookie") ?? "").split(";").map(v => v.trim()).filter(v => v.startsWith(`${WORKBENCH_COOKIE_NAME}=`));
    if (selections.length === 0) return accountFailure(409, "ORGANIZATION_SELECTION_REQUIRED");
    if (selections.length !== 1) return accountFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
    try { organization = decodeURIComponent(selections[0].slice(WORKBENCH_COOKIE_NAME.length + 1)); } catch { return accountFailure(409, "ORGANIZATION_CONTEXT_CHANGED"); }
    if (!validID(organization) || request.headers.get("X-Expected-Organization-ID") !== organization) return accountFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
  }
  const origin = serviceOrigin(); if (!origin) return accountFailure(503, "ACCOUNT_NOT_CONFIGURED");
  const controller = new AbortController(); const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true }); if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const headers = new Headers({ Accept: "application/json", Authorization: `Bearer ${token}` });
    let body: string | undefined;
    if ((kind === "business-profile" || kind === "member-allocations") && request.method === "PUT") {
      if (request.headers.get("content-type")?.split(";", 1)[0].trim().toLowerCase() !== "application/json") return accountFailure(400, "INVALID_REQUEST");
      try { body = await readAccountRequestBody(request, 16 * 1024, controller.signal); } catch { return accountFailure(400, "INVALID_REQUEST"); }
      headers.set("Content-Type", "application/json");
      if (kind === "member-allocations") {
        const idempotency = request.headers.get("Idempotency-Key");
        if (!idempotency || idempotency.length > 128) return accountFailure(400, "INVALID_REQUEST");
        headers.set("Idempotency-Key", idempotency);
      }
    }
    if (organization) headers.set("X-Requested-Organization-ID", organization);
    const servicePath = kind === "member-allocations" ? `organization/resources/member-allocations${memberPath ? `/${memberPath}` : ""}` : kind;
    const response = await fetch(`${origin}/api/v1/account/${servicePath}`, { method: request.method, headers, ...(body === undefined ? {} : { body }), cache: "no-store", redirect: "manual", signal: controller.signal });
    controller.signal.throwIfAborted();
    if (response.status === 404) {
      void response.body?.cancel().catch(() => undefined);
      return accountFailure(503, "ACCOUNT_NOT_CONFIGURED");
    }
    let payload: unknown;
    try { payload = await readBoundedStrictJSON(response, 16 * 1024, controller.signal); }
    catch { throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE"); }
    controller.signal.throwIfAborted();
    if (response.status !== 200) return accountFailure(response.status, accountErrorCode(response.status, payload));
    if (kind === "member-allocations" && request.method === "GET") {
      if (!payload || typeof payload !== "object" || !("organizationId" in payload) || payload.organizationId !== organization) return accountFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
    }
    if (kind === "member-allocations") return json(payload, 200);
    const result = parseAccountPayload(kind, payload);
    if (result.userId !== sessionUserId) return accountFailure(409, "IDENTITY_CONTEXT_CHANGED");
    if ("effectiveOrganizationId" in result && result.effectiveOrganizationId !== organization) return accountFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
    return json(result, 200);
  } catch (error) {
    if (controller.signal.aborted) return accountFailure(504, "DEADLINE_EXCEEDED");
    return error instanceof AccountReadError ? accountFailure(error.status, error.code) : accountFailure(502, "DEPENDENCY_UNAVAILABLE");
  } finally { clearTimeout(timer); request.signal.removeEventListener("abort", abort); }
}

const identityOperations: Record<string, readonly ("GET" | "PUT" | "POST")[]> = {
  profile: ["GET", "PUT"],
  email: ["PUT"],
  "email/resend": ["POST"],
  "email/verify": ["POST"],
  phone: ["PUT"],
  "phone/resend": ["POST"],
  "phone/verify": ["POST"],
  password: ["PUT"],
};

export type AccountDispatchState = { forwarded: boolean };

export async function proxyAccountIdentity(request: Request, token: string, sessionUserId: string, operation: string, dispatchState: AccountDispatchState = { forwarded: false }): Promise<Response> {
  if (!token) return accountFailure(401, "AUTHENTICATION_REQUIRED");
  if (!validID(sessionUserId)) return accountFailure(400, "INVALID_REQUEST");
  const methods = identityOperations[operation];
  if (!methods || !methods.includes(request.method as "GET" | "PUT" | "POST")) return accountFailure(400, "INVALID_REQUEST");
  if (request.headers.get("X-Expected-User-ID") !== sessionUserId) return accountFailure(409, "IDENTITY_CONTEXT_CHANGED");
  const url = new URL(request.url);
  const isRead = request.method === "GET";
  if (url.pathname !== `/api/account/identity/${operation}` || url.search || request.url.endsWith("?") || (!isRead && request.headers.get("content-type")?.split(";", 1)[0].trim().toLowerCase() !== "application/json") || (isRead && (request.body || (request.headers.has("content-length") && request.headers.get("content-length") !== "0") || request.headers.has("transfer-encoding")))) {
    void request.body?.cancel().catch(() => undefined);
    return accountFailure(400, "INVALID_REQUEST");
  }
  const origin = serviceOrigin();
  if (!origin) return accountFailure(503, "ACCOUNT_NOT_CONFIGURED");
  const controller = new AbortController(); const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true }); if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const body = isRead ? undefined : await readAccountRequestBody(request, 16 * 1024, controller.signal);
    const headers = new Headers({ Accept: "application/json", Authorization: `Bearer ${token}` });
    if (!isRead) headers.set("Content-Type", "application/json");
    dispatchState.forwarded = true;
    const response = await fetch(`${origin}/api/v1/account/identity/${operation}`, { method: request.method, headers, ...(body === undefined ? {} : { body }), cache: "no-store", redirect: "manual", signal: controller.signal });
    controller.signal.throwIfAborted();
    let payload: unknown;
    try { payload = await readBoundedStrictJSON(response, 16 * 1024, controller.signal); } catch { throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE"); }
    controller.signal.throwIfAborted();
    if (response.status !== 200) {
      const code = accountErrorCode(response.status, payload);
      return accountFailure(response.status, code, code === "RESULT_UNVERIFIED" ? "unknown" : undefined);
    }
    return json(payload, 200);
  } catch (error) {
    if (dispatchState.forwarded) return accountFailure(controller.signal.aborted ? 504 : 502, "RESULT_UNVERIFIED", "unknown");
    if (controller.signal.aborted) return accountFailure(504, "DEADLINE_EXCEEDED");
    return error instanceof AccountReadError ? accountFailure(error.status, error.code) : accountFailure(400, "INVALID_REQUEST");
  } finally { clearTimeout(timer); request.signal.removeEventListener("abort", abort); }
}

async function readAccountRequestBody(request: Request, maxBytes: number, signal: AbortSignal) {
  const reader = request.body?.getReader();
  if (!reader) throw new Error("missing request body");
  const chunks: Uint8Array[] = []; let size = 0;
  try {
    while (true) { signal.throwIfAborted(); const next = await reader.read(); if (next.done) break; size += next.value.byteLength; if (size > maxBytes) throw new Error("request body too large"); chunks.push(next.value); }
  } finally { reader.releaseLock(); }
  const bytes = new Uint8Array(size); let offset = 0; for (const chunk of chunks) { bytes.set(chunk, offset); offset += chunk.byteLength; }
  return new TextDecoder("utf-8", { fatal: true }).decode(bytes);
}
