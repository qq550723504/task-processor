"use client";

import { useQuery } from "@tanstack/react-query";

import { getSDSProductDetail } from "@/lib/api/sds-products";
import { listingKitKeys } from "@/lib/query/keys";

export function useSDSProductDetail(productId?: number) {
  return useQuery({
    queryKey: listingKitKeys.sdsProductDetail(productId ?? 0),
    queryFn: () => getSDSProductDetail(productId ?? 0),
    enabled: Boolean(productId && productId > 0),
  });
}
