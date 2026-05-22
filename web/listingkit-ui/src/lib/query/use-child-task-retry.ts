"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import { retryChildTask } from "@/lib/api/child-task-retry";

export function useRetryChildTask(taskId: string) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: (request: { kind: string }) => retryChildTask(taskId, request),
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
