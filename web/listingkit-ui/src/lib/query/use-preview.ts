"use client";

import { useQuery } from "@tanstack/react-query";

import { getListingKitPreview } from "@/lib/api/preview";
import type { TargetPlatform } from "@/lib/api/generated";
import { listingKitKeys } from "@/lib/query/keys";

export function useListingKitPreview(
  taskId: string,
  freshnessKey?: string,
  platform?: TargetPlatform,
) {
  return useQuery({
    queryKey: listingKitKeys.preview(taskId, freshnessKey, platform),
    queryFn: () =>
      platform
        ? getListingKitPreview(taskId, platform)
        : getListingKitPreview(taskId),
  });
}
