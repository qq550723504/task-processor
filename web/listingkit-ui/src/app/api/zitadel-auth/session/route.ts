import { NextResponse } from "next/server";

import { auth } from "@/auth";
import {
  buildLocalBypassIdentity,
  shouldBypassListingKitProxyAuth,
} from "@/app/api/listing-kits/proxy-auth";
import {
  authorizeZitadelIdentity,
  isZitadelAuthConfigured,
  readZitadelAccessTokenFromSession,
  readZitadelIdentityFromSession,
  readZitadelSessionError,
} from "@/lib/server/zitadel-auth";

export const dynamic = "force-dynamic";

export async function GET() {
  if (shouldBypassListingKitProxyAuth()) {
    return NextResponse.json({
      ok: true,
      identity: buildLocalBypassIdentity(),
    });
  }

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
    const session = await auth();
    const sessionError = readZitadelSessionError(session);
    if (sessionError) {
      throw new Error(sessionError);
    }
    const accessToken = readZitadelAccessTokenFromSession(session);
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
}
