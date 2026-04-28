"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  updateSheinFinalDraft,
  type UpdateSheinFinalDraftRequest,
} from "@/lib/api/shein-final-draft";

export function useUpdateSheinFinalDraft(taskId: string) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: (request: UpdateSheinFinalDraftRequest) =>
      updateSheinFinalDraft(taskId, request),
    onSettled: async () => {
      await client.invalidateQueries({
        predicate: (query) =>
          Array.isArray(query.queryKey) &&
          query.queryKey[0] === "listingkit" &&
          query.queryKey[1] === taskId,
      });
    },
  });
}
