import { NextRequest, NextResponse } from "next/server";

import { fetchSDSJSON, sdsAPIErrorPayload } from "@/app/api/sds/shared";
import type { SDSProductDetail } from "@/lib/types/sds";

export const dynamic = "force-dynamic";

export async function GET(
  request: NextRequest,
  { params }: { params: Promise<{ productId: string }> },
) {
  const { productId } = await params;

  try {
    const { payload } = await fetchSDSJSON<SDSProductDetail>(
      request,
      `/products/${productId}`,
    );
    return NextResponse.json(payload);
  } catch (error) {
    const payload = sdsAPIErrorPayload(error, "sds_product_detail_failed");
    return NextResponse.json(payload.body, { status: payload.status });
  }
}
