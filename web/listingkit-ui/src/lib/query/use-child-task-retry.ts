"use client";

import { useEffect, useRef, useState } from "react";
import { useMutation, useQueryClient } from "@tanstack/react-query";

import { retryChildTask } from "@/lib/api/child-task-retry";
import { listingKitKeys } from "@/lib/query/keys";
import type {
  ListingKitChildRetry,
  ListingKitTaskResult,
} from "@/lib/types/listingkit";

export function getTaskRetryVersion(task?: ListingKitTaskResult | null) {
  if (!task) {
    return "";
  }
  const childStates =
    task.result?.child_tasks
      ?.map((child) => `${child.kind ?? ""}:${child.status ?? ""}:${child.error ?? ""}`)
      .join("|") ?? "";
  const childRetries =
    task.child_retries
      ?.map(
        (retry) =>
          `${retry.kind ?? ""}:${retry.status ?? ""}:${retry.attempt ?? ""}:${retry.last_error ?? ""}:${retry.updated_at ?? ""}`,
      )
      .join("|") ?? "";
  return [
    task.result?.updated_at ?? "",
    task.completed_at ?? "",
    task.status ?? "",
    task.error ?? "",
    childStates,
    childRetries,
  ].join("|");
}

export function useRetryChildTask(
  taskId: string,
  taskVersion: string,
  durableRetries: ListingKitChildRetry[] = [],
) {
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

  const durableRetryQueued = durableRetries.some(
    (retry) => retry.status === "queued" || retry.status === "running",
  );
  const retryQueued =
    durableRetries.length > 0
      ? durableRetryQueued
      : mutation.data?.status === "queued" && queuedTaskVersion === taskVersion;

  const previousDurableRetryQueued = useRef<boolean | null>(null);
  useEffect(() => {
    const previous = previousDurableRetryQueued.current;
    if (previous === true && !durableRetryQueued) {
      void client.invalidateQueries({
        predicate: (query) =>
          Array.isArray(query.queryKey) &&
          query.queryKey[0] === "listingkit" &&
          query.queryKey[1] === taskId,
      });
    }
    previousDurableRetryQueued.current = durableRetryQueued;
  }, [client, durableRetryQueued, taskId]);

  useEffect(() => {
    if (!retryQueued) {
      return;
    }
    const interval = window.setInterval(() => {
      void client.refetchQueries({
        queryKey: listingKitKeys.taskResult(taskId),
      });
    }, 5000);
    return () => window.clearInterval(interval);
  }, [client, retryQueued, taskId]);

  return {
    ...mutation,
    retryQueued,
  };
}

