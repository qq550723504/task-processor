"use client";

import { useQuery } from "@tanstack/react-query";

import {
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

export function useListingKitSettingsSchema(namespace: string) {
  return useQuery({
    queryKey: listingKitSettingsKeys.schema(namespace),
    queryFn: () => getListingKitSettingsSchema(namespace),
    enabled: namespace.length > 0,
    staleTime: 60_000,
  });
}
