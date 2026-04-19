"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";

import { getGenerationQueue } from "@/lib/api/queue";
import { nextConditional } from "@/lib/query/conditional";
import { listingKitKeys } from "@/lib/query/keys";
import { normalizeConditionalResponse } from "@/lib/query/normalize";
import type { QueuePage, QueueQuery } from "@/lib/types/listingkit";

export function useGenerationQueue(taskId: string, query: QueueQuery) {
  const client = useQueryClient();
  const key = listingKitKeys.queue(taskId, query);

  return useQuery({
    queryKey: key,
    queryFn: async () => {
      const previous = client.getQueryData<QueuePage>(key);
      const response = await getGenerationQueue(
        taskId,
        query,
        nextConditional(previous?.conditional, previous?.delta_token),
      );
      return normalizeConditionalResponse(response, previous);
    },
  });
}
