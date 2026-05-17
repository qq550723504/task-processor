import { NextRequest, NextResponse } from "next/server";

import {
  signIn,
} from "@/auth";
import {
  getZitadelAuthOptions,
  normalizeReturnTo,
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

  return signIn("zitadel", {
    redirectTo: normalizeReturnTo(request.nextUrl.searchParams.get("returnTo")),
  });
}
