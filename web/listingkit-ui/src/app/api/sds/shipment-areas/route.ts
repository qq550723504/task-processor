import { NextResponse } from "next/server";

import { serverAuth } from "@/auth";
import type { AuthenticatedListingKitRequest } from "@/app/api/listing-kits/proxy-auth";
import { fetchSDSJSON, sdsAPIErrorPayload } from "@/app/api/sds/shared";
import type { SDSShipmentArea } from "@/lib/types/sds";

export const dynamic = "force-dynamic";

const authenticatedGET = serverAuth(async (request: AuthenticatedListingKitRequest) => {
  try {
    const { payload } = await fetchSDSJSON<SDSShipmentArea[]>(
      request,
      request.auth,
      "/shipment-areas",
    );
    return NextResponse.json(payload);
  } catch (error) {
    const payload = sdsAPIErrorPayload(error, "sds_shipment_area_query_failed");
    return NextResponse.json(payload.body, { status: payload.status });
  }
});

export { authenticatedGET as GET };
