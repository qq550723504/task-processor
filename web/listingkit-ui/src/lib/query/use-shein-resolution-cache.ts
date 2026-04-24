"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  clearSheinResolutionCache,
  type SheinResolutionCacheKind,
} from "@/lib/api/shein-resolution-cache";

export function useClearSheinResolutionCache(taskId: string) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: (kind: SheinResolutionCacheKind) =>
      clearSheinResolutionCache(taskId, kind),
    onSuccess: async () => {
      await client.invalidateQueries({
        predicate: (query) =>
          Array.isArray(query.queryKey) &&
          query.queryKey[0] === "listingkit" &&
          query.queryKey[1] === taskId,
      });
    },
  });
}
