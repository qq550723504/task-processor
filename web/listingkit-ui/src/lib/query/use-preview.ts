"use client";

import { useQuery } from "@tanstack/react-query";

import { getListingKitPreview } from "@/lib/api/preview";
import { listingKitKeys } from "@/lib/query/keys";

export function useListingKitPreview(taskId: string) {
  return useQuery({
    queryKey: listingKitKeys.preview(taskId),
    queryFn: () => getListingKitPreview(taskId),
  });
}
