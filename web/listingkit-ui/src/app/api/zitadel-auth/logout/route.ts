import { NextRequest, NextResponse } from "next/server";

import {
  buildZitadelLogoutResponse,
  clearZitadelCookies,
  fetchZitadelDiscovery,
  getZitadelAuthOptions,
} from "@/lib/server/zitadel-auth";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const options = getZitadelAuthOptions();
  if (!options) {
    const response = NextResponse.redirect(new URL("/", request.nextUrl.origin));
    clearZitadelCookies(response);
    return response;
  }

  const discovery = await fetchZitadelDiscovery(options);
  return buildZitadelLogoutResponse(request, options, discovery);
}
