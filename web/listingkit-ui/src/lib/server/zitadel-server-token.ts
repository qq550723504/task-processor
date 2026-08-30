import type { NextRequest } from "next/server";
import { getToken } from "next-auth/jwt";

import { getAuthJsSecret } from "@/auth.config";

async function readZitadelServerJWT(request: NextRequest) {
  const secret = getAuthJsSecret();
  if (!secret) {
    return null;
  }
  const secureCookie = request.headers
    .get("cookie")
    ?.split(";")
    .some((entry) =>
      entry.trimStart().startsWith("__Secure-authjs.session-token"),
    ) ?? false;
  return getToken({ req: request, secret, secureCookie });
}

export async function readZitadelServerAccessToken(request: NextRequest) {
  const token = await readZitadelServerJWT(request);
  return typeof token?.accessToken === "string" ? token.accessToken : "";
}

export async function readZitadelServerIDToken(request: NextRequest) {
  const token = await readZitadelServerJWT(request);
  return typeof token?.idToken === "string" ? token.idToken : "";
}
