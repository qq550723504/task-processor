"use client";

import { useQuery, useQueryClient } from "@tanstack/react-query";

import { getReviewPreview } from "@/lib/api/review-preview";
import { nextConditional } from "@/lib/query/conditional";
import { listingKitKeys } from "@/lib/query/keys";
import { normalizeConditionalResponse } from "@/lib/query/normalize";
import type { QueueQuery, ReviewPreviewResponse } from "@/lib/types/listingkit";

export function useReviewPreview(
  taskId: string,
  query: QueueQuery,
  enabled = true,
) {
  const client = useQueryClient();
  const key = listingKitKeys.reviewPreview(taskId, query);

  return useQuery({
    queryKey: key,
    enabled,
    queryFn: async () => {
      const previous = client.getQueryData<ReviewPreviewResponse>(key);
      const response = await getReviewPreview(
        taskId,
        query,
        nextConditional(previous?.conditional, previous?.delta_token),
      );
      return normalizeConditionalResponse(response, previous);
    },
  });
}
