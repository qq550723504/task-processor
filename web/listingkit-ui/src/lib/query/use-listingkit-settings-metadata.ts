"use client";

import { useQuery } from "@tanstack/react-query";

import {
  getListingKitSettingsHealth,
  getListingKitSettingsSchema,
  listListingKitSettingsNamespaces,
} from "@/lib/api/listingkit-settings";
import { listingKitSettingsKeys } from "@/lib/query/listingkit-settings";

export function useListingKitSettingsNamespaces() {
  return useQuery({
    queryKey: listingKitSettingsKeys.metadataIndex(),
    queryFn: listListingKitSettingsNamespaces,
    staleTime: 60_000,
  });
}

export function useListingKitSettingsHealth() {
  return useQuery({
    queryKey: listingKitSettingsKeys.health(),
    queryFn: getListingKitSettingsHealth,
    staleTime: 30_000,
  });
}
