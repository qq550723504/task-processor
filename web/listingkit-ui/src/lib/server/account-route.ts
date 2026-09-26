import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { accountFailure, proxyAccount } from "./account-proxy";

const deadline = () => accountFailure(504, "DEADLINE_EXCEEDED");
const authenticated = serverAuth(async (request: NextRequest & { auth?: unknown }) => {
  if (request.signal.aborted) return deadline();
  const identity = readZitadelIdentityFromSession(request.auth as never);
  const pathname = new URL(request.url).pathname;
  const kind = pathname === "/api/account/profile" ? "profile" : pathname === "/api/account/business-profile" ? "business-profile" : pathname === "/api/account/member-ai-point-limits" || pathname.startsWith("/api/account/member-ai-point-limits/") ? "member-ai-point-limits" : pathname === "/api/account/member-allocations" || pathname.startsWith("/api/account/member-allocations/") ? "member-allocations" : "organization";
  return proxyAccount(request, readZitadelServerAccessToken(request.auth as never), String(identity?.userId ?? ""), kind);
});

export async function handleAccountGET(request: NextRequest) {
	return handleAccountRequest(request, "GET");
}
export async function handleAccountPUT(request: NextRequest) {
	return handleAccountRequest(request, "PUT");
}
async function handleAccountRequest(request: NextRequest, method: "GET" | "PUT") {
  if (request.signal.aborted) return deadline();
  // The outer deadline cannot prove whether an authenticated PUT was already
  // forwarded. Keep this new immutable-operation owner honest on response loss.
  const pointWrite = method === "PUT" && new URL(request.url).pathname.startsWith("/api/account/member-ai-point-limits/");
  const timedOut = () => pointWrite ? accountFailure(504, "RESULT_UNVERIFIED", "unknown") : deadline();
  const controller = new AbortController(); const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  const timer = setTimeout(abort, 15000);
  let finish = () => {};
  const ended = new Promise<Response>(resolve => { finish = () => resolve(timedOut()); controller.signal.addEventListener("abort", finish, { once: true }); });
  try {
    if (request.method !== method) return accountFailure(405, "INVALID_REQUEST");
    const result = await Promise.race([authenticated(new NextRequest(request, { signal: controller.signal }), { params: Promise.resolve({}) }), ended]);
    if (controller.signal.aborted) return timedOut();
    return result ?? accountFailure(503, "DEPENDENCY_UNAVAILABLE");
  } catch { return controller.signal.aborted ? timedOut() : pointWrite ? accountFailure(503, "RESULT_UNVERIFIED", "unknown") : accountFailure(503, "DEPENDENCY_UNAVAILABLE"); }
  finally { clearTimeout(timer); request.signal.removeEventListener("abort", abort); controller.signal.removeEventListener("abort", finish); }
}

const reject = () => accountFailure(405, "INVALID_REQUEST");
export const POST = reject;
export const PUT = reject;
export const PATCH = reject;
export const DELETE = reject;
export const HEAD = reject;
export const OPTIONS = reject;
