import { NextRequest, NextResponse } from "next/server";

import { fetchSDSJSON } from "@/app/api/sds/shared";
import type { SDSCategory } from "@/lib/types/sds";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const shipmentArea = request.nextUrl.searchParams.get("shipmentArea") ?? "US";

  try {
    const payload = await fetchSDSJSON<SDSCategory[]>(
      "/categories",
      new URLSearchParams({ shipmentArea }),
    );
    return NextResponse.json(payload);
  } catch (error) {
    return NextResponse.json(
      {
        error: "sds_category_query_failed",
        message: error instanceof Error ? error.message : "unknown SDS error",
      },
      { status: 502 },
    );
  }
}
