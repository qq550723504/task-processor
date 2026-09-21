import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { accountFailure, proxyAccountIdentity, type AccountDispatchState } from "./account-proxy";

const operations = new Set(["profile", "email", "email/resend", "email/verify", "phone", "phone/resend", "phone/verify", "password"]);

function authenticatedFor(dispatchState: AccountDispatchState) {
  return serverAuth(async (request: NextRequest & { auth?: unknown }) => {
    const pathname = new URL(request.url).pathname;
    const prefix = "/api/account/identity/";
    const operation = pathname.startsWith(prefix) ? pathname.slice(prefix.length) : "";
    const identity = readZitadelIdentityFromSession(request.auth as never);
    if (!operations.has(operation) || !identity?.userId) return accountFailure(400, "INVALID_REQUEST");
    return proxyAccountIdentity(request, readZitadelServerAccessToken(request.auth as never), String(identity.userId), operation, dispatchState);
  });
}

async function handleAccountIdentity(request: NextRequest): Promise<Response> {
  if (request.method !== "GET" && request.method !== "PUT" && request.method !== "POST") return accountFailure(405, "INVALID_REQUEST");
  if (request.signal.aborted) return accountFailure(504, "DEADLINE_EXCEEDED");
  const controller = new AbortController();
  const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  const timer = setTimeout(abort, 15000);
  const dispatchState: AccountDispatchState = { forwarded: false };
  const deadlineFailure = () => dispatchState.forwarded
    ? accountFailure(504, "RESULT_UNVERIFIED", "unknown")
    : accountFailure(504, "DEADLINE_EXCEEDED");
  let finish = () => {};
  const ended = new Promise<Response>(resolve => {
    finish = () => resolve(deadlineFailure());
    controller.signal.addEventListener("abort", finish, { once: true });
  });
  try {
    const scoped = new NextRequest(request, { signal: controller.signal });
    const result = await Promise.race([authenticatedFor(dispatchState)(scoped, { params: Promise.resolve({}) }), ended]);
    return controller.signal.aborted ? deadlineFailure() : result ?? accountFailure(503, "DEPENDENCY_UNAVAILABLE");
  } catch { return controller.signal.aborted ? deadlineFailure() : accountFailure(503, "DEPENDENCY_UNAVAILABLE");
  } finally {
    clearTimeout(timer);
    request.signal.removeEventListener("abort", abort);
    controller.signal.removeEventListener("abort", finish);
  }
}

export const GET = handleAccountIdentity;
export const PUT = handleAccountIdentity;
export const POST = handleAccountIdentity;
export const PATCH = () => accountFailure(405, "INVALID_REQUEST");
export const DELETE = () => accountFailure(405, "INVALID_REQUEST");
export const HEAD = () => accountFailure(405, "INVALID_REQUEST");
export const OPTIONS = () => accountFailure(405, "INVALID_REQUEST");
