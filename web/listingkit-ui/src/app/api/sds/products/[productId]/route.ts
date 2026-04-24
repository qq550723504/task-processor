import { NextResponse } from "next/server";

import { fetchSDSJSON } from "@/app/api/sds/shared";
import type { SDSProductDetail } from "@/lib/types/sds";

export const dynamic = "force-dynamic";

export async function GET(
  _request: Request,
  { params }: { params: Promise<{ productId: string }> },
) {
  const { productId } = await params;

  try {
    const payload = await fetchSDSJSON<SDSProductDetail>(`/products/${productId}`);
    return NextResponse.json(payload);
  } catch (error) {
    return NextResponse.json(
      {
        error: "sds_product_detail_failed",
        message: error instanceof Error ? error.message : "unknown SDS error",
      },
      { status: 502 },
    );
  }
}
