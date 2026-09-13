import { NextRequest } from "next/server";
import { z } from "zod";

import { serverAuth } from "@/auth";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { hasTrustedSameOriginWrite } from "./same-origin-write";
import { referralFailure, referralJSON, referralMethodNotAllowed } from "./referral-registration-route";

const id = z.string().min(1).max(128).regex(/^[A-Za-z0-9._:-]+$/);
const utc = z.string().regex(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/).refine((value) => Number.isFinite(Date.parse(value)));
const projection = z.object({ code: z.string().max(200), codeAvailability: z.enum(["available", "not_created"]), count: z.number().int().nonnegative().max(Number.MAX_SAFE_INTEGER), generatedAt: utc, earnings: z.object({ availability: z.literal("unavailable"), amount: z.null() }).strict() }).strict().superRefine((value, context) => {
  if ((value.codeAvailability === "available") !== (value.code.length > 0)) context.addIssue({ code: "custom", message: "Invalid code availability" });
});
const created = z.object({ code: z.string().min(1).max(200) }).strict();
const completed = z.object({ status: z.literal("complete"), intentID: z.string().min(1).max(200), boundAt: utc }).strict();
const safeErrors = new Set(["referral_invalid", "referral_authentication_required", "referral_missing", "referral_conflict", "referral_expired", "referral_verification_pending", "referral_outcome_unknown", "referral_capacity_exceeded", "referral_unavailable"]);

type Operation = "read" | "create" | "complete";
const authenticated = serverAuth(async (request: NextRequest & { auth?: unknown }) => {
  const operation = await operationFor(request);
  if (!operation) return referralFailure(400, "INVALID_REQUEST");
  const identity = readZitadelIdentityFromSession(request.auth as never);
  const token = readZitadelServerAccessToken(request.auth as never);
  const expected = request.headers.get("x-expected-user-id");
  if (!identity || !token) return referralFailure(401, "AUTHENTICATION_REQUIRED");
  if (!expected || expected !== String(identity.userId) || !id.safeParse(expected).success) return referralFailure(409, "IDENTITY_CONTEXT_CHANGED");
  return proxy(request, token, operation);
});

export async function handleAccountReferrals(request: NextRequest) {
  if (request.signal.aborted) return referralFailure(504, "DEADLINE_EXCEEDED");
  const pathname = new URL(request.url).pathname;
  if (request.method === "POST" && ["/api/account/referrals", "/api/account/referrals/complete"].includes(pathname) && !hasTrustedSameOriginWrite(request)) {
    return referralFailure(403, "PERMISSION_DENIED");
  }
  const controller = new AbortController();
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  const timer = setTimeout(abort, 15_000);
  let finish = () => {};
  const ended = new Promise<Response>((resolve) => {
    finish = () => resolve(referralFailure(504, "DEADLINE_EXCEEDED"));
    controller.signal.addEventListener("abort", finish, { once: true });
  });
  try {
    const scoped = new NextRequest(request, { signal: controller.signal });
    const result = await Promise.race([authenticated(scoped, { params: Promise.resolve({}) }), ended]);
    return controller.signal.aborted ? referralFailure(504, "DEADLINE_EXCEEDED") : result ?? referralFailure(503, "DEPENDENCY_UNAVAILABLE");
  } catch {
    return controller.signal.aborted ? referralFailure(504, "DEADLINE_EXCEEDED") : referralFailure(503, "DEPENDENCY_UNAVAILABLE");
  } finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
    controller.signal.removeEventListener("abort", finish);
  }
}

async function operationFor(request: Request): Promise<Operation | null> {
  const url = new URL(request.url);
  if (url.search || request.url.endsWith("?") || request.headers.has("transfer-encoding") || (request.headers.has("content-length") && request.headers.get("content-length") !== "0") || !(await hasEmptyBody(request))) return null;
  if (url.pathname === "/api/account/referrals" && request.method === "GET") return "read";
  if (url.pathname === "/api/account/referrals" && request.method === "POST") return hasTrustedSameOriginWrite(request) ? "create" : null;
  if (url.pathname === "/api/account/referrals/complete" && request.method === "POST") return hasTrustedSameOriginWrite(request) ? "complete" : null;
  return null;
}

async function hasEmptyBody(request: Request) {
  if (!request.body) return true;
  const reader = request.body.getReader();
  try {
    const { done, value } = await reader.read();
    if (!done || (value?.byteLength ?? 0) !== 0) {
      void reader.cancel().catch(() => undefined);
      return false;
    }
    return true;
  } finally {
    reader.releaseLock();
  }
}

async function proxy(request: Request, token: string, operation: Operation) {
  const origin = serviceOrigin();
  if (!origin) return referralFailure(503, "REFERRALS_NOT_CONFIGURED");
  const controller = new AbortController();
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  const timer = setTimeout(abort, 15_000);
  try {
    const suffix = operation === "complete" ? "/api/v1/account/referrals/complete" : "/api/v1/account/referrals";
    const response = await fetch(`${origin}${suffix}`, {
      method: operation === "read" ? "GET" : "POST",
      headers: new Headers({ Accept: "application/json", Authorization: `Bearer ${token}` }),
      cache: "no-store", redirect: "manual", signal: controller.signal,
    });
    const payload = await readBoundedStrictJSON(response, 16 * 1024, controller.signal);
    if (!response.ok) return upstreamFailure(response.status, payload);
    const schema = operation === "read" ? projection : operation === "create" ? created : completed;
    const parsed = schema.safeParse(payload);
    return parsed.success ? referralJSON(parsed.data) : referralFailure(502, "INVALID_UPSTREAM_RESPONSE");
  } catch {
    return controller.signal.aborted ? referralFailure(504, "DEADLINE_EXCEEDED") : referralFailure(503, "DEPENDENCY_UNAVAILABLE");
  } finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
  }
}

function serviceOrigin() {
  try {
    const raw = process.env.LISTINGKIT_SERVICE_API_BASE?.trim();
    if (!raw) return null;
    const url = new URL(raw);
    const host = url.hostname.replace(/^\[|\]$/g, "");
    if (!["127.0.0.1", "::1", "localhost"].includes(host) || !["http:", "https:"].includes(url.protocol) || url.username || url.password || url.search || url.hash || !["/api/v1", "/api/v1/"].includes(url.pathname)) return null;
    return url.origin;
  } catch { return null; }
}

function upstreamFailure(status: number, payload: unknown) {
  const code = payload && typeof payload === "object" && !Array.isArray(payload) && typeof (payload as { error?: unknown }).error === "string" ? (payload as { error: string }).error : "";
  return safeErrors.has(code) ? referralFailure(status, code) : referralFailure(502, "INVALID_UPSTREAM_RESPONSE");
}

export const rejectAccountReferralMethod = referralMethodNotAllowed;
