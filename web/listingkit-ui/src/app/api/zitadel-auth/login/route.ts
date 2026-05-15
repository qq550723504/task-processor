import { NextRequest, NextResponse } from "next/server";

import {
  createZitadelAuthRequest,
  fetchZitadelDiscovery,
  getZitadelAuthOptions,
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

  const discovery = await fetchZitadelDiscovery(options);
  return createZitadelAuthRequest(request, options, discovery);
}
