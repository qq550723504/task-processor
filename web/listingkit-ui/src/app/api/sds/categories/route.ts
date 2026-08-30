import { NextResponse } from "next/server";

import { serverAuth } from "@/auth";
import type { AuthenticatedListingKitRequest } from "@/app/api/listing-kits/proxy-auth";
import { fetchSDSJSON, sdsAPIErrorPayload } from "@/app/api/sds/shared";
import type { SDSCategory } from "@/lib/types/sds";

export const dynamic = "force-dynamic";

const authenticatedGET = serverAuth(async (request: AuthenticatedListingKitRequest) => {
  const shipmentArea = request.nextUrl.searchParams.get("shipmentArea") ?? "US";

  try {
    const { payload } = await fetchSDSJSON<SDSCategory[]>(
      request,
      request.auth,
      "/categories",
      new URLSearchParams({ shipmentArea }),
    );
    return NextResponse.json(payload);
  } catch (error) {
    const payload = sdsAPIErrorPayload(error, "sds_category_query_failed");
    return NextResponse.json(payload.body, { status: payload.status });
  }
});

export { authenticatedGET as GET };
