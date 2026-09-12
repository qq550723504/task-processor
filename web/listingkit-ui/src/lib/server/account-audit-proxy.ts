import { NextResponse } from "next/server";
import { AccountReadError, accountErrorCode } from "@/lib/api/account";
import { AUDIT_RESPONSE_MAX_BYTES, auditQuery, parseAccountAudit } from "@/lib/api/account-audit";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { accountFailure } from "./account-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";

const validID = (value: string | undefined): value is string => !!value && /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(value);
function origin(): string | null {
  try {
    const raw = process.env.LISTINGKIT_SERVICE_API_BASE?.trim(); if (!raw) return null;
    const url = new URL(raw);
    return ["http:", "https:"].includes(url.protocol) && !url.username && !url.password && !url.search && !url.hash && ["/api/v1", "/api/v1/"].includes(url.pathname) ? url.origin : null;
  } catch { return null; }
}
export async function proxyAccountAudit(request: Request, token: string, userId: string): Promise<Response> {
  if (request.method !== "GET") return accountFailure(405, "INVALID_REQUEST");
  if (!token || !validID(userId)) return accountFailure(401, "AUTHENTICATION_REQUIRED");
  if (request.headers.get("X-Expected-User-ID") !== userId) return accountFailure(409, "IDENTITY_CONTEXT_CHANGED");
  const url = new URL(request.url);
  if (url.pathname !== "/api/account/audit" || url.search.length > 2300 || request.body || (request.headers.has("content-length") && request.headers.get("content-length") !== "0") || request.headers.has("transfer-encoding")) {
    void request.body?.cancel().catch(() => undefined); return accountFailure(400, "INVALID_REQUEST");
  }
  let query: string;
  let limit: number;
  try {
    for (const name of url.searchParams.keys()) if (!["limit", "cursor"].includes(name) || url.searchParams.getAll(name).length !== 1) throw new Error("invalid query");
    const rawLimit = url.searchParams.get("limit");
    if (rawLimit !== null && !/^[1-9][0-9]{0,2}$/.test(rawLimit)) throw new Error("invalid limit");
    limit = rawLimit === null ? 20 : Number(rawLimit);
    query = auditQuery(limit, url.searchParams.get("cursor") ?? undefined);
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
    const upstream = await fetch(`${service}/api/v1/account/audit?${query}`, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: controller.signal });
    controller.signal.throwIfAborted();
    if (upstream.status === 404) { void upstream.body?.cancel().catch(() => undefined); return accountFailure(503, "ACCOUNT_NOT_CONFIGURED"); }
    let payload: unknown;
    try { payload = await readBoundedStrictJSON(upstream, AUDIT_RESPONSE_MAX_BYTES, controller.signal); }
    catch { throw new AccountReadError(502, "INVALID_UPSTREAM_RESPONSE"); }
    controller.signal.throwIfAborted();
    if (upstream.status !== 200) return accountFailure(upstream.status, accountErrorCode(upstream.status, payload));
    const result = parseAccountAudit(payload);
    if (result.userId !== userId) return accountFailure(409, "IDENTITY_CONTEXT_CHANGED");
    if (result.effectiveOrganizationId !== organization) return accountFailure(409, "ORGANIZATION_CONTEXT_CHANGED");
    if (result.items.length > limit) return accountFailure(502, "INVALID_UPSTREAM_RESPONSE");
    return NextResponse.json(result, { headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" } });
  } catch (error) {
    if (controller.signal.aborted) return accountFailure(504, "DEADLINE_EXCEEDED");
    return error instanceof AccountReadError ? accountFailure(error.status, error.code) : accountFailure(502, "DEPENDENCY_UNAVAILABLE");
  } finally { clearTimeout(timer); request.signal.removeEventListener("abort", abort); }
}
