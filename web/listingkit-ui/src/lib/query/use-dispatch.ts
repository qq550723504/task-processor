"use client";

import { useMutation, useQueryClient } from "@tanstack/react-query";

import { dispatchNavigation } from "@/lib/api/dispatch";
import { applyDispatchResultToCache } from "@/lib/query/cache-updates";
import type { NavigationTarget, QueueQuery } from "@/lib/types/listingkit";

export function useDispatchNavigation(taskId: string, baseQuery: QueueQuery) {
  const client = useQueryClient();

  return useMutation({
    mutationFn: (target: NavigationTarget) => dispatchNavigation(taskId, target),
    onSuccess: (response) => {
      applyDispatchResultToCache(client, taskId, baseQuery, response);
    },
  });
}
