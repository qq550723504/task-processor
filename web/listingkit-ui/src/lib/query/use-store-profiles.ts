"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  createStoreProfile,
  deleteStoreProfile,
  getStoreProfiles,
  updateStoreProfile,
} from "@/lib/api/store-profiles";
import { listingKitKeys } from "@/lib/query/keys";
import type { ListingKitStoreProfile } from "@/lib/types/listingkit";

export function useStoreProfiles() {
  return useQuery({
    queryKey: listingKitKeys.storeProfiles(),
    queryFn: getStoreProfiles,
  });
}

export function enabledStoreProfiles<T extends { enabled?: boolean }>(
  items: T[] | undefined,
) {
  return (items ?? []).filter((item) => item.enabled !== false);
}

export function useUpsertStoreProfile() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: ListingKitStoreProfile) =>
      input.id ? updateStoreProfile(input) : createStoreProfile(input),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: listingKitKeys.storeProfiles() });
      await client.invalidateQueries({ queryKey: listingKitKeys.storeRouting() });
    },
  });
}

export function useDeleteStoreProfile() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => deleteStoreProfile(id),
    onSuccess: async () => {
      await client.invalidateQueries({ queryKey: listingKitKeys.storeProfiles() });
      await client.invalidateQueries({ queryKey: listingKitKeys.storeRouting() });
    },
  });
}
