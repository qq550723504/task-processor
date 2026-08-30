import { NextResponse } from "next/server";

import { serverAuth } from "@/auth";
import type { AuthenticatedListingKitRequest } from "@/app/api/listing-kits/proxy-auth";
import { fetchSDSJSON, sdsAPIErrorPayload } from "@/app/api/sds/shared";
import type { SDSProductDetail } from "@/lib/types/sds";

export const dynamic = "force-dynamic";

const authenticatedGET = serverAuth(async (
  request: AuthenticatedListingKitRequest,
  { params }: { params: Promise<{ productId: string }> },
) => {
  const { productId } = await params;

  try {
    const { payload } = await fetchSDSJSON<SDSProductDetail>(
      request,
      request.auth,
      `/products/${productId}`,
    );
    return NextResponse.json(payload);
  } catch (error) {
    const payload = sdsAPIErrorPayload(error, "sds_product_detail_failed");
    return NextResponse.json(payload.body, { status: payload.status });
  }
});

export { authenticatedGET as GET };
