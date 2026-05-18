"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import { getStoreRouting, updateStoreRouting } from "@/lib/api/store-profiles";
import { listingKitKeys } from "@/lib/query/keys";
import type { ListingKitStoreRoutingSettings } from "@/lib/types/listingkit";

export function useStoreRouting() {
  return useQuery({
    queryKey: listingKitKeys.storeRouting(),
    queryFn: getStoreRouting,
  });
}

export function useUpdateStoreRouting() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: ListingKitStoreRoutingSettings) =>
      updateStoreRouting(input),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: listingKitKeys.storeRouting() });
      await client.invalidateQueries({ queryKey: listingKitKeys.storeProfiles() });
    },
  });
}
