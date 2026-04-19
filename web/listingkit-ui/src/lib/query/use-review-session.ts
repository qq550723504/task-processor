"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";

import { getReviewSession } from "@/lib/api/review-session";
import { nextConditional } from "@/lib/query/conditional";
import { listingKitKeys } from "@/lib/query/keys";
import { normalizeConditionalResponse } from "@/lib/query/normalize";
import type { QueueQuery, ReviewSessionResponse } from "@/lib/types/listingkit";

export function useReviewSession(taskId: string, query: QueueQuery) {
  const client = useQueryClient();
  const key = listingKitKeys.reviewSession(taskId, query);

  return useQuery({
    queryKey: key,
    queryFn: async () => {
      const previous = client.getQueryData<ReviewSessionResponse>(key);
      const response = await getReviewSession(
        taskId,
        query,
        nextConditional(previous?.conditional, previous?.delta_token),
      );
      return normalizeConditionalResponse(response, previous);
    },
  });
}
