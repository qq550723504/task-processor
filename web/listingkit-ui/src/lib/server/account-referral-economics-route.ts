import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readBoundedStrictJSON } from "@/lib/api/strict-json-response";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { hasTrustedSameOriginWrite } from "./same-origin-write";
import { referralFailure, referralJSON, referralMethodNotAllowed } from "./referral-registration-route";

const validID = (value: string | null | undefined): value is string => typeof value === "string" && /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(value);
function authenticatedFor(dispatchState: { forwarded: boolean }) {
  return serverAuth(async (request: NextRequest & { auth?: unknown }) => {
    const identity = readZitadelIdentityFromSession(request.auth as never);
    const token = readZitadelServerAccessToken(request.auth as never);
    const expected = request.headers.get("x-expected-user-id");
    if (!identity || !token) return referralFailure(401, "AUTHENTICATION_REQUIRED");
    if (!expected || expected !== String(identity.userId) || !validID(expected)) return referralFailure(409, "IDENTITY_CONTEXT_CHANGED");
    return proxy(request, token, dispatchState);
  });
}

export async function handleAccountReferralEconomics(request: NextRequest) {
  if (request.signal.aborted) return referralFailure(504, "DEADLINE_EXCEEDED");
  if (request.method === "POST" && !hasTrustedSameOriginWrite(request)) return referralFailure(403, "PERMISSION_DENIED");
  const controller = new AbortController();
  const dispatchState = { forwarded: false };
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  const timer = setTimeout(abort, 15_000);
  let finish = () => {};
  const ended = new Promise<Response>(resolve => {
    finish = () => resolve(referralFailure(504, "DEADLINE_EXCEEDED"));
    controller.signal.addEventListener("abort", finish, { once: true });
  });
  const deadline = () => request.method === "POST" && dispatchState.forwarded
    ? referralFailure(504, "RESULT_UNVERIFIED", "unknown")
    : referralFailure(504, "DEADLINE_EXCEEDED");
  try {
    const result = await Promise.race([authenticatedFor(dispatchState)(new NextRequest(request, { signal: controller.signal }), { params: Promise.resolve({}) }), ended]);
    return controller.signal.aborted ? deadline() : result;
  } catch { return controller.signal.aborted ? deadline() : referralFailure(503, "DEPENDENCY_UNAVAILABLE"); }
  finally { clearTimeout(timer); request.signal.removeEventListener("abort", abort); controller.signal.removeEventListener("abort", finish); }
}

async function proxy(request: Request, token: string, dispatchState: { forwarded: boolean }) {
  const path = new URL(request.url).pathname;
  const mapping: Record<string, string> = {
    "/api/account/referral-earnings": "/api/v1/account/referrals/earnings",
    "/api/account/referral-rules": "/api/v1/account/referrals/rules",
    "/api/account/referral-payout-methods": "/api/v1/account/referrals/payout-methods",
    "/api/account/referral-withdrawals": "/api/v1/account/referrals/withdrawals",
    "/api/account/referral-withdrawals/review-queue": "/api/v1/account/referrals/withdrawals/review-queue",
  };
  const suffix = mapping[path] ?? (path.match(/^\/api\/account\/referral-withdrawals\/[^/]+\/(?:cancel|review)$/) ? path.replace("/api/account/referral-withdrawals", "/api/v1/account/referrals/withdrawals") : "");
  if (!suffix || request.method === "GET" && !["/api/account/referral-earnings", "/api/account/referral-rules", "/api/account/referral-payout-methods", "/api/account/referral-withdrawals", "/api/account/referral-withdrawals/review-queue"].includes(path) || request.method === "POST" && path === "/api/account/referral-earnings" || !["GET", "POST"].includes(request.method)) return referralFailure(400, "INVALID_REQUEST");
  const origin = serviceOrigin();
  if (!origin) return referralFailure(503, "REFERRALS_NOT_CONFIGURED");
  const headers = new Headers({ Accept: "application/json", Authorization: `Bearer ${token}` });
  if (request.method === "POST") {
    if (request.headers.get("content-type")?.split(";", 1)[0].trim().toLowerCase() !== "application/json") return referralFailure(400, "INVALID_REQUEST");
    const body = await readBoundedStrictJSON(new Response(request.body, { headers: { "Content-Type": "application/json" } }), 16 * 1024, request.signal).catch(() => undefined);
    if (body === undefined) return referralFailure(400, "INVALID_REQUEST");
    headers.set("Content-Type", "application/json");
    const key = request.headers.get("Idempotency-Key");
    if (!key || key.length > 128) return referralFailure(400, "INVALID_REQUEST");
    headers.set("Idempotency-Key", key);
    dispatchState.forwarded = true;
    const response = await fetch(`${origin}${suffix}`, { method: "POST", headers, body: JSON.stringify(body), cache: "no-store", redirect: "manual", signal: request.signal });
    return upstream(response);
  }
  const response = await fetch(`${origin}${suffix}`, { method: "GET", headers, cache: "no-store", redirect: "manual", signal: request.signal });
  return upstream(response);
}

async function upstream(response: Response) {
  const payload = await readBoundedStrictJSON(response, 128 * 1024).catch(() => undefined);
  if (payload === undefined) return referralFailure(502, "INVALID_UPSTREAM_RESPONSE");
  return response.ok ? referralJSON(payload, response.status) : referralFailure(response.status, errorCode(payload));
}
function errorCode(payload: unknown) { return payload && typeof payload === "object" && !Array.isArray(payload) && typeof (payload as { code?: unknown }).code === "string" ? (payload as { code: string }).code : "DEPENDENCY_UNAVAILABLE"; }
function serviceOrigin() {
  try { const raw = process.env.LISTINGKIT_SERVICE_API_BASE?.trim(); if (!raw) return null; const url = new URL(raw); return ["http:", "https:"].includes(url.protocol) && !url.username && !url.password && !url.search && !url.hash && ["/api/v1", "/api/v1/"].includes(url.pathname) ? url.origin : null; } catch { return null; }
}

export const rejectAccountReferralEconomicsMethod = referralMethodNotAllowed;
