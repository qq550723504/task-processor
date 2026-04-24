import { NextRequest, NextResponse } from "next/server";

import { fetchSDSJSON } from "@/app/api/sds/shared";
import { matchesCycleBand, matchesWeightBand } from "@/lib/sds/product-filters";
import type { SDSProductListResponse } from "@/lib/types/sds";

export const dynamic = "force-dynamic";

const FILTER_PAGE_SIZE = 100;
const MAX_FILTER_PAGES = 8;

function paginateItems<T>(items: T[], page: number, size: number) {
  const start = (page - 1) * size;
  return items.slice(start, start + size);
}

export async function GET(request: NextRequest) {
  const page = Number(request.nextUrl.searchParams.get("page") ?? "1") || 1;
  const size = Number(request.nextUrl.searchParams.get("size") ?? "12") || 12;
  const weightBand = request.nextUrl.searchParams.get("weightBand")?.trim() ?? "";
  const cycleBand = request.nextUrl.searchParams.get("cycleBand")?.trim() ?? "";
  const needsLocalFiltering = Boolean(weightBand || cycleBand);
  const query = new URLSearchParams();
  query.set("page", String(page));
  query.set("size", String(needsLocalFiltering ? FILTER_PAGE_SIZE : size));
  query.set("shipmentArea", request.nextUrl.searchParams.get("shipmentArea") ?? "US");
  query.set("preciseSearch", request.nextUrl.searchParams.get("preciseSearch") ?? "0");
  query.set("t", Date.now().toString());

  const keyword = request.nextUrl.searchParams.get("keyword")?.trim();
  if (keyword) {
    query.set("keyword", keyword);
  }

  const categoryId = request.nextUrl.searchParams.get("categoryId")?.trim();
  if (categoryId) {
    query.set("categoryId", categoryId);
  }

  const onSaleStatus = request.nextUrl.searchParams.get("onSaleStatus")?.trim();
  if (onSaleStatus) {
    query.set("onSaleStatus", onSaleStatus);
  }

  const hotSellStatus = request.nextUrl.searchParams.get("hotSellStatus")?.trim();
  if (hotSellStatus) {
    query.set("hotSellStatus", hotSellStatus);
  }

  const sortField = request.nextUrl.searchParams.get("sortField")?.trim();
  if (sortField) {
    query.set("sortField", sortField);
  }

  const sortType = request.nextUrl.searchParams.get("sortType")?.trim();
  if (sortType) {
    query.set("sortType", sortType);
  }

  try {
    if (!needsLocalFiltering) {
      const payload = await fetchSDSJSON<SDSProductListResponse>("/products/page", query);
      return NextResponse.json(payload);
    }

    const filteredItems = [];
    let totalCount = 0;
    let fetchedCount = 0;

    for (let current = 1; current <= MAX_FILTER_PAGES; current += 1) {
      query.set("page", String(current));
      query.set("t", String(Date.now() + current));
      const payload = await fetchSDSJSON<SDSProductListResponse>("/products/page", query);
      totalCount = payload.totalCount ?? totalCount;
      const items = payload.items ?? [];
      fetchedCount += items.length;
      filteredItems.push(
        ...items.filter(
          (item) => matchesWeightBand(item, weightBand) && matchesCycleBand(item, cycleBand),
        ),
      );

      if (items.length < FILTER_PAGE_SIZE || fetchedCount >= totalCount) {
        break;
      }
    }

    return NextResponse.json({
      page,
      size,
      totalCount: filteredItems.length,
      items: paginateItems(filteredItems, page, size),
    } satisfies SDSProductListResponse);
  } catch (error) {
    return NextResponse.json(
      {
        error: "sds_product_query_failed",
        message: error instanceof Error ? error.message : "unknown SDS error",
      },
      { status: 502 },
    );
  }
}
