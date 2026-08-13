"use client";

import { useEffect, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { retryChildTask } from "@/lib/api/child-task-retry";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

export function getTaskRetryVersion(task?: ListingKitTaskResult | null) {
  if (!task) {
    return "";
  }
  const childStates =
    task.result?.child_tasks
      ?.map((child) => `${child.kind ?? ""}:${child.status ?? ""}:${child.error ?? ""}`)
      .join("|") ?? "";
  return [
    task.result?.updated_at ?? "",
    task.completed_at ?? "",
    task.status ?? "",
    task.error ?? "",
    childStates,
  ].join("|");
}

export function useRetryChildTask(taskId: string, taskVersion: string) {
  const client = useQueryClient();
  const [queuedTaskVersion, setQueuedTaskVersion] = useState<string | null>(null);

  const mutation = useMutation({
    mutationFn: (request: { kind: string }) => retryChildTask(taskId, request),
    onSuccess: () => {
      setQueuedTaskVersion(taskVersion);
    },
    onError: () => {
      setQueuedTaskVersion(null);
    },
    onSettled: async () => {
      await client.invalidateQueries({
        predicate: (query) =>
          Array.isArray(query.queryKey) &&
          query.queryKey[0] === "listingkit" &&
          query.queryKey[1] === taskId,
      });
    },
  });

  useEffect(() => {
    if (queuedTaskVersion === null || queuedTaskVersion === taskVersion) {
      return;
    }
    mutation.reset();
  }, [mutation, queuedTaskVersion, taskVersion]);

  return {
    ...mutation,
    retryQueued:
      mutation.data?.status === "queued" &&
      queuedTaskVersion === taskVersion,
  };
}
