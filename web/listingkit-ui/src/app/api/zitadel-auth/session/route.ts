import { NextRequest, NextResponse } from "next/server";

import {
  fetchZitadelDiscovery,
  getZitadelAuthOptions,
  getZitadelBearerToken,
  verifyZitadelAccessToken,
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
    const discovery = await fetchZitadelDiscovery(options);
    const identity = await verifyZitadelAccessToken(
      getZitadelBearerToken(request),
      options,
      discovery,
    );
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
