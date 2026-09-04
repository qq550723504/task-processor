import { NextRequest, NextResponse } from "next/server";

import { signIn } from "@/auth";
import { getZitadelAuthOptions } from "@/lib/server/zitadel-auth";
import {
  isLoginEntryAvailable,
  normalizeReturnTo,
  resolveLoginEntry,
} from "@/lib/server/login-entry";

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

  const entry = resolveLoginEntry(request.nextUrl.searchParams.getAll("method"));
  if (!isLoginEntryAvailable(entry)) {
    return NextResponse.json(
      {
        error: "login_capability_unavailable",
        entry,
      },
      {
        status: 503,
        headers: { "Cache-Control": "no-store" },
      },
    );
  }

  return signIn("zitadel", {
    redirectTo: normalizeReturnTo(request.nextUrl.searchParams.get("returnTo")),
  });
}
