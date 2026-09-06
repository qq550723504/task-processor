import { NextResponse } from "next/server";
import { InvalidStrictJSONResponseError, readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { parseSheinRecordList, parseSheinRecordListFailure, sheinRecordCursorSchema, type SheinRecordList } from "@/lib/api/shein-records";
import { WORKBENCH_COOKIE_NAME, workbenchProtocolError } from "./workbench-proxy";
import { configuredSheinRecordsOrigin } from "./shein-records-origin";

const MAX_BYTES = 128 * 1024;
const unavailable = () => workbenchProtocolError(502, "DEPENDENCY_UNAVAILABLE", "SHEIN record list upstream is unavailable");
const invalidResponse = () => workbenchProtocolError(502, "INVALID_UPSTREAM_RESPONSE", "SHEIN record list upstream response is invalid");
const invalidRequest = () => safeJSON({ error: "invalid_request" }, 400);
function safeJSON(payload: unknown, status: number) { return NextResponse.json(payload, { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" } }); }
function selectedOrganization(request: Request): string | null {
  const values = (request.headers.get("cookie") ?? "").split(";").map((part) => part.trim()).filter((part) => part.startsWith(`${WORKBENCH_COOKIE_NAME}=`));
  if (values.length !== 1) return null;
  try {
    const value = decodeURIComponent(values[0].slice(WORKBENCH_COOKIE_NAME.length + 1)).trim();
    return /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(value) ? value : null;
  } catch { return null; }
}

export async function proxySheinRecords(request: Request, accessToken: string): Promise<Response> {
  if (request.method !== "GET") return workbenchProtocolError(405, "INVALID_REQUEST", "Method is not allowed");
  if (!accessToken) return workbenchProtocolError(401, "AUTHENTICATION_REQUIRED", "Authentication is required");
  const organization = selectedOrganization(request);
  if (!organization || organization !== request.headers.get("X-Expected-Organization-ID")) return workbenchProtocolError(409, "ORGANIZATION_CONTEXT_CHANGED", "Organization context changed");
  const incoming = new URL(request.url);
  if (incoming.pathname !== "/api/listing/shein-records" || request.body !== null || (request.headers.has("content-length") && request.headers.get("content-length") !== "0") || request.headers.has("transfer-encoding")) {
    void request.body?.cancel().catch(() => undefined); return invalidRequest();
  }
  const rawQuery = incoming.search.slice(1);
  if (new TextEncoder().encode(rawQuery).length > 1024 || rawQuery.includes(";")) return invalidRequest();
  try { decodeURIComponent(rawQuery); } catch { return invalidRequest(); }
  for (const key of incoming.searchParams.keys()) if (!["limit", "cursor"].includes(key) || incoming.searchParams.getAll(key).length !== 1) return invalidRequest();
  const limit = incoming.searchParams.get("limit") ?? "20";
  if (!/^[1-9][0-9]{0,2}$/.test(limit) || Number(limit) > 100) return invalidRequest();
  const cursor = incoming.searchParams.get("cursor");
  if (cursor !== null && !sheinRecordCursorSchema.safeParse(cursor).success) return invalidRequest();
  const origin = configuredSheinRecordsOrigin();
  if (!origin) return unavailable();
  const upstreamURL = new URL("/api/listing/shein-records", origin);
  upstreamURL.searchParams.set("limit", limit);
  if (cursor !== null) upstreamURL.searchParams.set("cursor", cursor);
  const controller = new AbortController();
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  if (request.signal.aborted) abort();
  const timeout = setTimeout(abort, 15000);
  try {
    controller.signal.throwIfAborted();
    const upstream = await fetch(upstreamURL, { method: "GET", headers: { Accept: "application/json", Authorization: `Bearer ${accessToken}`, "X-Requested-Organization-ID": organization }, cache: "no-store", redirect: "manual", signal: controller.signal });
    let payload: unknown;
    try { payload = await readBoundedStrictJSON(upstream, MAX_BYTES, controller.signal); }
    catch (error) {
      if (controller.signal.aborted) throw error;
      return error instanceof InvalidStrictJSONResponseError ? invalidResponse() : unavailable();
    }
    let result: SheinRecordList | null = null;
    if (upstream.status === 200) result = parseSheinRecordList(payload);
    if (result) return safeJSON(result, 200);
    const failure = parseSheinRecordListFailure(payload, upstream.status);
    if (!failure) return invalidResponse();
    const response = safeJSON(failure, upstream.status);
    if ("code" in failure && ["ORGANIZATION_ACCESS_REVOKED", "ORGANIZATION_ACCESS_DENIED"].includes(failure.code)) response.cookies.set(WORKBENCH_COOKIE_NAME, "", { httpOnly: true, sameSite: "lax", path: "/", secure: process.env.NODE_ENV !== "development", maxAge: 0 });
    return response;
  } catch {
    return controller.signal.aborted ? workbenchProtocolError(504, "DEADLINE_EXCEEDED", "SHEIN record list request ended before completion") : unavailable();
  } finally {
    clearTimeout(timeout); request.signal.removeEventListener("abort", abort);
  }
}
