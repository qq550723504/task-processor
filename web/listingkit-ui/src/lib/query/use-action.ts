"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import { executeAction } from "@/lib/api/action";
import { applyActionResultToCache } from "@/lib/query/cache-updates";
import type { ActionExecutionRequest, QueueQuery } from "@/lib/types/listingkit";

export function useExecuteAction(taskId: string, baseQuery: QueueQuery) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: (request: ActionExecutionRequest) => executeAction(taskId, request),
    onSuccess: async (response) => {
      applyActionResultToCache(client, taskId, baseQuery, response);

      await client.invalidateQueries({
        predicate: (query) =>
          Array.isArray(query.queryKey) &&
          query.queryKey[0] === "listingkit" &&
          query.queryKey[1] === taskId,
      });
    },
  });
}
