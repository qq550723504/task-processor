import { apiRequest } from "@/lib/api/client";
import type { ConditionalState, QueuePage, QueueQuery } from "@/lib/types/listingkit";

export function getGenerationQueue(
  taskId: string,
  query: QueueQuery,
  conditional?: ConditionalState | null,
) {
  return apiRequest<QueuePage>(`/tasks/${taskId}/generation-queue`, {
    query,
    conditional,
  });
}
