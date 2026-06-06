"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import { bulkRecoverTasks, recoverTaskNow } from "@/lib/api/task-recovery";

export function useRecoverTaskNow(taskId: string) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: () => recoverTaskNow(taskId),
    onSettled: async () => {
      await client.invalidateQueries({
        predicate: (query) =>
          Array.isArray(query.queryKey) &&
          query.queryKey[0] === "listingkit" &&
          (query.queryKey[1] === taskId || query.queryKey[1] === "tasks"),
      });
    },
  });
}

export function useBulkRecoverTasks() {
  const client = useQueryClient();

  return useMutation({
    mutationFn: bulkRecoverTasks,
    onSettled: async () => {
      await client.invalidateQueries({
        predicate: (query) => Array.isArray(query.queryKey) && query.queryKey[0] === "listingkit",
      });
    },
  });
}
