import { apiRequest } from "@/lib/api/client";
import { parseQueueResponse } from "@/lib/api/queue-schema";
import type { ConditionalState, QueuePage, QueueQuery } from "@/lib/types/listingkit";

export async function getGenerationQueue(
  taskId: string,
  query: QueueQuery,
  conditional?: ConditionalState | null,
): Promise<QueuePage> {
  const payload = await apiRequest<unknown>(`/tasks/${taskId}/generation-queue`, {
    query,
    conditional,
  });
  return parseQueueResponse(payload);
}
