import { NextResponse } from "next/server";

import { fetchSDSJSON, sdsAPIErrorPayload } from "@/app/api/sds/shared";
import type { SDSShipmentArea } from "@/lib/types/sds";

export const dynamic = "force-dynamic";

export async function GET() {
  try {
    const payload = await fetchSDSJSON<SDSShipmentArea[]>("/shipment-areas");
    return NextResponse.json(payload);
  } catch (error) {
    const payload = sdsAPIErrorPayload(error, "sds_shipment_area_query_failed");
    return NextResponse.json(payload.body, { status: payload.status });
  }
}
