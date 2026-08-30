import { NextResponse } from "next/server";

import { serverAuth } from "@/auth";

import {
  authorizeZitadelIdentity,
  isZitadelAuthConfigured,
  readZitadelIdentityFromSession,
  readZitadelSessionError,
} from "@/lib/server/zitadel-auth";
import { readZitadelServerAccessToken } from "@/lib/server/zitadel-server-token";

export const dynamic = "force-dynamic";

const authenticatedGET = serverAuth(async (request) => {
  if (!isZitadelAuthConfigured()) {
    return NextResponse.json(
      {
        error: "zitadel_auth_not_configured",
        message: "ZITADEL authentication is not configured",
      },
      { status: 503 },
    );
  }

  try {
    const session = request.auth;
    const sessionError = readZitadelSessionError(session);
    if (sessionError) {
      throw new Error(sessionError);
    }
    const accessToken = readZitadelServerAccessToken(session);
    const identity = readZitadelIdentityFromSession(session);
    if (!accessToken || !identity) {
      throw new Error("Missing ZITADEL session");
    }
    const authorization = authorizeZitadelIdentity(identity);
    if (!authorization.authorized) {
      return NextResponse.json(
        {
          error: "zitadel_access_denied",
          message: authorization.reason ?? "ZITADEL access denied",
          identity,
        },
        { status: 403 },
      );
    }
    return NextResponse.json({ ok: true, identity });
  } catch (error) {
    return NextResponse.json(
      {
        error: "zitadel_token_invalid",
        message:
          error instanceof Error
            ? error.message
            : "ZITADEL token verification failed",
      },
      { status: 401 },
    );
  }
});

export { authenticatedGET as GET };
