import { NextResponse } from "next/server";

import { fetchSDSJSON } from "@/app/api/sds/shared";
import { sdsShipmentAreaCandidates } from "@/lib/sds/shipment-areas";
import type { SDSProductListResponse, SDSShipmentArea } from "@/lib/types/sds";

export const dynamic = "force-dynamic";

const CACHE_TTL_MS = 10 * 60 * 1000;

let cachedAreas: SDSShipmentArea[] | null = null;
let cachedAt = 0;

async function fetchShipmentAreas() {
  const now = Date.now();
  if (cachedAreas && now - cachedAt < CACHE_TTL_MS) {
    return cachedAreas;
  }

  const results = await Promise.all(
    sdsShipmentAreaCandidates.map(async (area) => {
      const query = new URLSearchParams({
        page: "1",
        size: "1",
        shipmentArea: area.value,
        preciseSearch: "0",
        t: String(now),
      });

      const payload = await fetchSDSJSON<SDSProductListResponse>("/products/page", query);
      const totalCount = payload.totalCount ?? 0;
      return totalCount > 0
        ? ({
            value: area.value,
            label: area.label,
            totalCount,
          } satisfies SDSShipmentArea)
        : null;
    }),
  );

  cachedAreas = results.filter((item): item is SDSShipmentArea => item !== null);
  cachedAt = now;
  return cachedAreas;
}

export async function GET() {
  try {
    const payload = await fetchShipmentAreas();
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
