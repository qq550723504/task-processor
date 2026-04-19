"use client";

import { useQuery } from "@tanstack/react-query";

import { shouldPollTaskResult } from "@/components/listingkit/task-status-query";
import { getListingKitTaskResult } from "@/lib/api/task-result";
import { listingKitKeys } from "@/lib/query/keys";

export function useListingKitTaskResult(taskId: string) {
  return useQuery({
    queryKey: listingKitKeys.taskResult(taskId),
    queryFn: () => getListingKitTaskResult(taskId),
    refetchInterval: (query) =>
      shouldPollTaskResult(query.state.data?.status) ? 5000 : false,
    refetchOnWindowFocus: true,
  });
}
