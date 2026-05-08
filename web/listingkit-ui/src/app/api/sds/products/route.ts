import { NextRequest, NextResponse } from "next/server";

import { fetchSDSJSON, sdsAPIErrorPayload } from "@/app/api/sds/shared";
import type { SDSProductListResponse } from "@/lib/types/sds";

export const dynamic = "force-dynamic";

export async function GET(request: NextRequest) {
  const page = Number(request.nextUrl.searchParams.get("page") ?? "1") || 1;
  const size = Number(request.nextUrl.searchParams.get("size") ?? "12") || 12;
  const weightBand = request.nextUrl.searchParams.get("weightBand")?.trim() ?? "";
  const cycleBand = request.nextUrl.searchParams.get("cycleBand")?.trim() ?? "";
  const query = new URLSearchParams();
  query.set("page", String(page));
  query.set("size", String(size));
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
  if (weightBand) {
    query.set("weightBand", weightBand);
  }
  if (cycleBand) {
    query.set("cycleBand", cycleBand);
  }

  try {
    const payload = await fetchSDSJSON<SDSProductListResponse>("/products", query);
    return NextResponse.json(payload);
  } catch (error) {
    const payload = sdsAPIErrorPayload(error, "sds_product_query_failed");
    return NextResponse.json(payload.body, { status: payload.status });
  }
}
