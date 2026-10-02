import { NextResponse } from "next/server";
import { AccountReadError, accountErrorCode } from "@/lib/api/account";
import { AUDIT_RESPONSE_MAX_BYTES, auditContentInvalidReason, auditQuery, parseAccountAudit, parseAccountAuditSummary, type AuditOptions } from "@/lib/api/account-audit";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { accountFailure } from "./account-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";

const validID = (value: string | undefined): value is string => !!value && /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(value);
const validOperation = (value: string | undefined): value is NonNullable<AuditOptions["operation"]> => !!value && ["register", "enable", "disable", "allocate_member_resource", "reclaim_member_resource", "set_member_ai_point_limit", "update", "invite", "role", "remove"].includes(value);
const validPeriod = (value: string | undefined): value is NonNullable<AuditOptions["period"]> => !!value && ["7d", "30d", "all"].includes(value);
function origin(): string | null {
  try {
    const raw = process.env.LISTINGKIT_SERVICE_API_BASE?.trim(); if (!raw) return null;
    const url = new URL(raw);
    return ["http:", "https:"].includes(url.protocol) && !url.username && !url.password && !url.search && !url.hash && ["/api/v1", "/api/v1/"].includes(url.pathname) ? url.origin : null;
  } catch { return null; }
}
export async function proxyAccountAudit(request: Request, token: string, userId: string): Promise<Response> {
  return proxyAudit(request, token, userId, false);
}
export async function proxyAccountAuditSummary(request: Request, token: string, userId: string): Promise<Response> {
  return proxyAudit(request, token, userId, true);
}
async function proxyAudit(request: Request, token: string, userId: string, isSummary: boolean): Promise<Response> {
  if (request.method !== "GET") return accountFailure(405, "INVALID_REQUEST");
  if (!token || !validID(userId)) return accountFailure(401, "AUTHENTICATION_REQUIRED");
  if (request.headers.get("X-Expected-User-ID") !== userId) return accountFailure(409, "IDENTITY_CONTEXT_CHANGED");
  const url = new URL(request.url);
  if (url.pathname !== (isSummary ? "/api/account/audit/summary" : "/api/account/audit") || isSummary && url.search !== "" || url.search.length > 4096 || request.body || (request.headers.has("content-length") && request.headers.get("content-length") !== "0") || request.headers.has("transfer-encoding")) {
    void request.body?.cancel().catch(() => undefined); return accountFailure(400, "INVALID_REQUEST");
  }
  let query: string;
  let limit: number;
  try {
    for (const name of url.searchParams.keys()) if (!["limit", "cursor", "actor", "operation", "query", "member", "period"].includes(name) || url.searchParams.getAll(name).length !== 1) throw new Error("invalid query");
    const rawLimit = url.searchParams.get("limit");
    if (rawLimit !== null && !/^[1-9][0-9]{0,2}$/.test(rawLimit)) throw new Error("invalid limit");
    const rawActor = url.searchParams.get("actor");
    if (rawActor !== null && !validID(rawActor)) throw new Error("invalid actor");
    const rawOperation = url.searchParams.get("operation");
    if (rawOperation !== null && !validOperation(rawOperation)) throw new Error("invalid operation");
    const rawContent = url.searchParams.get("query");
    if (rawContent !== null && auditContentInvalidReason(rawContent) !== null) throw new Error("invalid content");
    const rawMember = url.searchParams.get("member");
    if (rawMember !== null && !validID(rawMember)) throw new Error("invalid member");
    const rawPeriod = url.searchParams.get("period");
    if (rawPeriod !== null && !validPeriod(rawPeriod)) throw new Error("invalid period");
    limit = rawLimit === null ? 20 : Number(rawLimit);
    query = auditQuery(limit, url.searchParams.get("cursor") ?? undefined, rawActor ?? undefined, rawOperation ?? undefined, rawContent ?? undefined, rawMember ?? undefined, rawPeriod ?? undefined);
  } catch { return accountFailure(400, "INVALID_REQUEST"); }
  const selections = (request.headers.get("cookie") ?? "").split(";").map(value => value.trim()).filter(value => value.startsWith(`${WORKBENCH_COOKIE_NAME}=`));
  if (selections.length === 0) return accountFailure(409, "ORGANIZATION_SELECTION_REQUIRED");
  if (selections.length !== 1) return accountFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
  let organization: string;
  try { organization = decodeURIComponent(selections[0].slice(WORKBENCH_COOKIE_NAME.length + 1)); }
  catch { return accountFailure(409, "ORGANIZATION_CONTEXT_CHANGED"); }
  if (!validID(organization) || request.headers.get("X-Expected-Organization-ID") !== organization) return accountFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
  const service = origin(); if (!service) return accountFailure(503, "ACCOUNT_NOT_CONFIGURED");
  const controller = new AbortController(); const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true }); if (request.signal.aborted) abort();
  const timer = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const headers = new Headers({ Accept: "application/json", Authorization: `Bearer ${token}`, "X-Requested-Organization-ID": organization });
    const upstream = await fetch(`${service}/api/v1/account/audit${isSummary ? "/summary" : `?${query}`}`, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: controller.signal });
    controller.signal.throwIfAborted();
    if (upstream.status === 404) { void upstream.body?.cancel().catch(() => undefined); return accountFailure(503, "ACCOUNT_NOT_CONFIGURED"); }
    let payload: unknown;
    try { payload = await readBoundedStrictJSON(upstream, AUDIT_RESPONSE_MAX_BYTES, controller.signal); }
    catch { throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE"); }
    controller.signal.throwIfAborted();
    if (upstream.status !== 200) return accountFailure(upstream.status, accountErrorCode(upstream.status, payload));
    const result = isSummary ? parseAccountAuditSummary(payload) : parseAccountAudit(payload);
    if (result.userId !== userId) return accountFailure(409, "IDENTITY_CONTEXT_CHANGED");
    if (result.effectiveOrganizationId !== organization) return accountFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
    if ("items" in result && result.items.length > limit) return accountFailure(502, "INVALID_UPSTREAM_RESPONSE");
    return NextResponse.json(result, { headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" } });
  } catch (error) {
    if (controller.signal.aborted) return accountFailure(504, "DEADLINE_EXCEEDED");
    return error instanceof AccountReadError ? accountFailure(error.status, error.code) : accountFailure(502, "DEPENDENCY_UNAVAILABLE");
  } finally { clearTimeout(timer); request.signal.removeEventListener("abort", abort); }
}
