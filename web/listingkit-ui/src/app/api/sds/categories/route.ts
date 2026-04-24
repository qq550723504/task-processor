import { NextRequest, NextResponse } from "next/server";

import { fetchSDSJSON } from "@/app/api/sds/shared";
import type { SDSCategory, SDSProductListResponse } from "@/lib/types/sds";

export const dynamic = "force-dynamic";

const PAGE_SIZE = 100;
const MAX_PAGES = 5;
const CACHE_TTL_MS = 10 * 60 * 1000;

const cache = new Map<string, { data: SDSCategory[]; at: number }>();

async function loadCategories(shipmentArea: string) {
  const cached = cache.get(shipmentArea);
  const now = Date.now();
  if (cached && now - cached.at < CACHE_TTL_MS) {
    return cached.data;
  }

  const categoryMap = new Map<number, SDSCategory>();
  let totalCount = 0;
  let fetched = 0;

  for (let page = 1; page <= MAX_PAGES; page += 1) {
    const query = new URLSearchParams({
      page: String(page),
      size: String(PAGE_SIZE),
      shipmentArea,
      preciseSearch: "0",
      t: String(now + page),
    });

    const payload = await fetchSDSJSON<SDSProductListResponse>("/products/page", query);
    totalCount = payload.totalCount ?? totalCount;
    const items = payload.items ?? [];
    fetched += items.length;

    for (const item of items) {
      const leaf = item.categories?.at(-1);
      if (!leaf) {
        continue;
      }
      const existing = categoryMap.get(leaf.id);
      if (existing) {
        existing.count += 1;
      } else {
        categoryMap.set(leaf.id, {
          id: leaf.id,
          name: leaf.name,
          count: 1,
        });
      }
    }

    if (items.length < PAGE_SIZE || fetched >= totalCount) {
      break;
    }
  }

  const data = Array.from(categoryMap.values()).sort((a, b) => {
    if (b.count !== a.count) {
      return b.count - a.count;
    }
    return a.name.localeCompare(b.name);
  });

  cache.set(shipmentArea, { data, at: now });
  return data;
}

export async function GET(request: NextRequest) {
  const shipmentArea = request.nextUrl.searchParams.get("shipmentArea") ?? "US";

  try {
    const payload = await loadCategories(shipmentArea);
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
