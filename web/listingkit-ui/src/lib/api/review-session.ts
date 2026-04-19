import { apiRequest } from "@/lib/api/client";
import type {
  ConditionalState,
  QueueQuery,
  ReviewSessionResponse,
} from "@/lib/types/listingkit";

export function getReviewSession(
  taskId: string,
  query: QueueQuery,
  conditional?: ConditionalState | null,
) {
  return apiRequest<ReviewSessionResponse>(
    `/tasks/${taskId}/generation-review-session`,
    {
      query,
      conditional,
    },
  );
}
