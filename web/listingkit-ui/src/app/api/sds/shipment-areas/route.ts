import { NextResponse } from "next/server";

import { fetchSDSJSON } from "@/app/api/sds/shared";
import type { SDSShipmentArea } from "@/lib/types/sds";

export const dynamic = "force-dynamic";

export async function GET() {
  try {
    const payload = await fetchSDSJSON<SDSShipmentArea[]>("/shipment-areas");
    return NextResponse.json(payload);
  } catch (error) {
    return NextResponse.json(
      {
        error: "sds_shipment_area_query_failed",
        message: error instanceof Error ? error.message : "unknown SDS error",
      },
      { status: 502 },
    );
  }
}
