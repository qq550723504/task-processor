import { NextRequest, NextResponse } from "next/server";

import {
  getYudaoCheckTokenOptions,
  verifyYudaoAccessToken,
} from "@/app/api/listing-kits/[...path]/route";

export const dynamic = "force-dynamic";

export async function POST(request: NextRequest) {
  const options = getYudaoCheckTokenOptions();
  if (!options) {
    return NextResponse.json(
      {
        error: "yudao_auth_not_configured",
        message: "Yudao token verification is not configured",
      },
      { status: 503 },
    );
  }

  try {
    const identity = await verifyYudaoAccessToken(
      request.headers.get("authorization"),
      {
        ...options,
        tenantId:
          request.headers.get("tenant-id") ?? request.headers.get("x-tenant-id"),
      },
    );
    return NextResponse.json({ ok: true, identity });
  } catch (error) {
    return NextResponse.json(
      {
        error: "yudao_token_invalid",
        message:
          error instanceof Error
            ? error.message
            : "Yudao token verification failed",
      },
      { status: 401 },
    );
  }
}
