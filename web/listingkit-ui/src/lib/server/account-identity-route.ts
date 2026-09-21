import { NextRequest } from "next/server";
import { serverAuth } from "@/auth";
import { readZitadelIdentityFromSession } from "./zitadel-auth";
import { readZitadelServerAccessToken } from "./zitadel-server-token";
import { accountFailure, proxyAccountIdentity } from "./account-proxy";

const operations = new Set(["profile", "email", "email/resend", "email/verify", "phone", "phone/resend", "phone/verify", "password"]);

const authenticated = serverAuth(async (request: NextRequest & { auth?: unknown }) => {
  const pathname = new URL(request.url).pathname;
  const prefix = "/api/account/identity/";
  const operation = pathname.startsWith(prefix) ? pathname.slice(prefix.length) : "";
  const identity = readZitadelIdentityFromSession(request.auth as never);
  if (!operations.has(operation) || !identity?.userId) return accountFailure(400, "INVALID_REQUEST");
  return proxyAccountIdentity(request, readZitadelServerAccessToken(request.auth as never), String(identity.userId), operation);
});

export async function handleAccountIdentity(request: NextRequest): Promise<Response> {
  if (request.method !== "GET" && request.method !== "PUT" && request.method !== "POST") return accountFailure(405, "INVALID_REQUEST");
  try {
    return (await authenticated(request, { params: Promise.resolve({}) })) ?? accountFailure(503, "DEPENDENCY_UNAVAILABLE");
  } catch {
    return accountFailure(503, "DEPENDENCY_UNAVAILABLE");
  }
}

export const GET = handleAccountIdentity;
export const PUT = handleAccountIdentity;
export const POST = handleAccountIdentity;
export const PATCH = () => accountFailure(405, "INVALID_REQUEST");
export const DELETE = () => accountFailure(405, "INVALID_REQUEST");
export const HEAD = () => accountFailure(405, "INVALID_REQUEST");
export const OPTIONS = () => accountFailure(405, "INVALID_REQUEST");
