"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  submitTask,
  type SubmitTaskRequest,
} from "@/lib/api/submit";

export function useSubmitTask(taskId: string) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: (request: SubmitTaskRequest) => submitTask(taskId, request),
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
