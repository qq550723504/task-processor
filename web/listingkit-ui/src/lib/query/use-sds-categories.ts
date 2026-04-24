"use client";

import { useQuery } from "@tanstack/react-query";

import { getSDSCategories } from "@/lib/api/sds-products";
import { listingKitKeys } from "@/lib/query/keys";

export function useSDSCategories(shipmentArea: string) {
  return useQuery({
    queryKey: listingKitKeys.sdsCategories(shipmentArea),
    queryFn: () => getSDSCategories(shipmentArea),
    staleTime: 5 * 60 * 1000,
  });
}
