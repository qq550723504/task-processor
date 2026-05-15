import { NextRequest, NextResponse } from "next/server";

import {
  clearZitadelCookies,
  exchangeZitadelAuthorizationCode,
  fetchZitadelDiscovery,
  getZitadelAuthOptions,
  setZitadelSessionCookie,
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
    const { session, returnTo } = await exchangeZitadelAuthorizationCode(
      request,
      options,
      discovery,
    );
    const response = NextResponse.redirect(new URL(returnTo, request.nextUrl.origin));
    clearZitadelCookies(response);
    setZitadelSessionCookie(response, session);
    return response;
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
