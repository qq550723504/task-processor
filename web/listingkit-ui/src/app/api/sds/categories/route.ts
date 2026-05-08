import { NextRequest, NextResponse } from "next/server";

import { fetchSDSJSON, sdsAPIErrorPayload } from "@/app/api/sds/shared";
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
    const payload = sdsAPIErrorPayload(error, "sds_category_query_failed");
    return NextResponse.json(payload.body, { status: payload.status });
  }
}
