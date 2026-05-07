"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import {
  refreshSubmissionStatus,
  submitTask,
  type SubmitTaskRequest,
} from "@/lib/api/submit";

export function useSubmitTask(taskId: string) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: (request: SubmitTaskRequest) => submitTask(taskId, request),
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

export function useRefreshSubmissionStatus(taskId: string) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: () => refreshSubmissionStatus(taskId),
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
