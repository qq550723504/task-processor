import { NextResponse } from "next/server";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { WORKBENCH_COOKIE_NAME, workbenchProtocolError } from "./workbench-proxy";
import { newRequestLogId } from "./request-log";

const MAX_BODY_BYTES = 16 * 1024;
const MAX_RESPONSE_BYTES = 64 * 1024;

function configuredOrigin(): string | null {
  const raw = process.env.COMMERCIAL_API_ORIGIN;
  if (!raw) return null;
  try {
    const url = new URL(raw);
    return ["http:", "https:"].includes(url.protocol) && !url.username && !url.password && (raw === url.origin || raw === `${url.origin}/`) ? url.origin : null;
  } catch {
    return null;
  }
}

function selectedOrganization(request: Request): string | null {
  const cookies = (request.headers.get("cookie") ?? "").split(";").map(value => value.trim()).filter(value => value.startsWith(`${WORKBENCH_COOKIE_NAME}=`));
  if (cookies.length !== 1) return null;
  try {
    const value = decodeURIComponent(cookies[0].slice(WORKBENCH_COOKIE_NAME.length + 1));
    return /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(value) ? value : null;
  } catch {
    return null;
  }
}

const failure = (status: number, code: string) => workbenchProtocolError(status, code, "Commercial billing request could not be completed");
const safeJSON = (body: unknown, status: number) => NextResponse.json(body, { status, headers: { "Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff" } });

export async function proxyCommercialBilling(request: Request, accessToken: string): Promise<Response> {
  if (request.signal.aborted) return failure(504, "DEADLINE_EXCEEDED");
  const organization = selectedOrganization(request);
  if (!accessToken) return failure(401, "AUTHENTICATION_REQUIRED");
  if (!organization || request.headers.get("X-Expected-Organization-ID") !== organization) return failure(409, "ORGANIZATION_CONTEXT_CHANGED");
  const origin = configuredOrigin();
  if (!origin) return failure(503, "DEPENDENCY_UNAVAILABLE");

  let body: ArrayBuffer | undefined;
  if (request.method !== "GET") {
    if (request.body === null) return failure(400, "INVALID_REQUEST");
    body = await request.arrayBuffer();
    if (body.byteLength > MAX_BODY_BYTES) return failure(400, "INVALID_REQUEST");
  } else if (request.body !== null) {
    await request.body.cancel().catch(() => undefined);
    return failure(400, "INVALID_REQUEST");
  }
  const incomingURL = new URL(request.url);
  const path = `${incomingURL.pathname.replace("/api/workbench/", "/api/v1/workbench/")}${incomingURL.search}`;
  try {
    const headers = new Headers({ Accept: "application/json", Authorization: `Bearer ${accessToken}`, "X-Requested-Organization-ID": organization, "X-Request-ID": newRequestLogId() });
    if (body) headers.set("Content-Type", request.headers.get("content-type") ?? "application/json");
    const idempotencyKey = request.headers.get("Idempotency-Key");
    if (idempotencyKey) headers.set("Idempotency-Key", idempotencyKey);
    const upstream = await fetch(new URL(path, origin), { method: request.method, headers, body, cache: "no-store", redirect: "manual", signal: request.signal });
    if (upstream.status >= 300 && upstream.status < 400) {
      await upstream.body?.cancel().catch(() => undefined);
      return failure(502, "INVALID_UPSTREAM_RESPONSE");
    }
    let payload: unknown;
    try {
      payload = await readBoundedStrictJSON(upstream, upstream.status >= 200 && upstream.status < 300 ? MAX_RESPONSE_BYTES : 8192, request.signal);
    } catch {
      return request.signal.aborted ? failure(504, "DEADLINE_EXCEEDED") : failure(502, "INVALID_UPSTREAM_RESPONSE");
    }
    request.signal.throwIfAborted();
    return safeJSON(payload, upstream.status);
  } catch {
    return request.signal.aborted ? failure(504, "DEADLINE_EXCEEDED") : failure(503, "DEPENDENCY_UNAVAILABLE");
  }
}
