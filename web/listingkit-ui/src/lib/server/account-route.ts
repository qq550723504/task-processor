import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { accountFailure, proxyAccount } from "./account-proxy";

const deadline = () => accountFailure(504, "DEADLINE_EXCEEDED");
const authenticated = serverAuth(async (request: NextRequest & { auth?: unknown }) => {
  if (request.signal.aborted) return deadline();
  const identity = readZitadelIdentityFromSession(request.auth as never);
  const kind = new URL(request.url).pathname === "/api/account/profile" ? "profile" : "organization";
  return proxyAccount(request, readZitadelServerAccessToken(request.auth as never), String(identity?.userId ?? ""), kind);
});

export async function handleAccountGET(request: NextRequest) {
  if (request.signal.aborted) return deadline();
  const controller = new AbortController(); const abort = () => controller.abort();
  request.signal.addEventListener("abort", abort, { once: true });
  const timer = setTimeout(abort, 15000);
  let finish = () => {};
  const ended = new Promise<Response>(resolve => { finish = () => resolve(deadline()); controller.signal.addEventListener("abort", finish, { once: true }); });
  try {
    const result = await Promise.race([authenticated(new NextRequest(request, { signal: controller.signal }), { params: Promise.resolve({}) }), ended]);
    if (controller.signal.aborted) return deadline();
    return result ?? accountFailure(503, "DEPENDENCY_UNAVAILABLE");
  } catch { return controller.signal.aborted ? deadline() : accountFailure(503, "DEPENDENCY_UNAVAILABLE"); }
  finally { clearTimeout(timer); request.signal.removeEventListener("abort", abort); controller.signal.removeEventListener("abort", finish); }
}

const reject = () => accountFailure(405, "INVALID_REQUEST");
export const POST = reject;
export const PUT = reject;
export const PATCH = reject;
export const DELETE = reject;
export const HEAD = reject;
export const OPTIONS = reject;
