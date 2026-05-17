import { NextRequest, NextResponse } from "next/server";

import {
  getZitadelAuthOptions,
  resolvePublicAppOrigin,
} from "@/lib/server/zitadel-auth";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const options = getZitadelAuthOptions();
  if (!options) {
    return NextResponse.json(
      {
        error: "zitadel_auth_not_configured",
        message: "ZITADEL authentication is not configured",
      },
      { status: 503 },
    );
  }

  try {
    const callbackUrl = new URL(
      "/api/auth/callback/zitadel",
      resolvePublicAppOrigin(),
    );
    request.nextUrl.searchParams.forEach((value, key) => {
      callbackUrl.searchParams.set(key, value);
    });
    return NextResponse.redirect(callbackUrl);
  } catch (error) {
    return NextResponse.json(
      {
        error: "zitadel_callback_failed",
        message:
          error instanceof Error ? error.message : "ZITADEL callback failed",
      },
      { status: 401 },
    );
  }
}
