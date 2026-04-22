"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  applyRevision,
  type ApplyRevisionRequest,
} from "@/lib/api/revision";

export function useApplyRevision(taskId: string) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: (request: ApplyRevisionRequest) => applyRevision(taskId, request),
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
