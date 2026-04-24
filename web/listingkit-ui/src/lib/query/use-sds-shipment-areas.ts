"use client";

import { useQuery } from "@tanstack/react-query";

import { getSDSShipmentAreas } from "@/lib/api/sds-products";
import { listingKitKeys } from "@/lib/query/keys";

export function useSDSShipmentAreas() {
  return useQuery({
    queryKey: listingKitKeys.sdsShipmentAreas(),
    queryFn: getSDSShipmentAreas,
    staleTime: 5 * 60 * 1000,
  });
}
