import { apiRequest } from "@/lib/api/client";
import { parseReviewSessionResponse } from "@/lib/api/review-session-schema";
import type {
  ConditionalState,
  QueueQuery,
  ReviewSessionResponse,
} from "@/lib/types/listingkit";

export async function getReviewSession(
  taskId: string,
  query: QueueQuery,
  conditional?: ConditionalState | null,
) : Promise<ReviewSessionResponse> {
  const payload = await apiRequest<unknown>(
    `/tasks/${taskId}/generation-review-session`,
    {
      query,
      conditional,
    },
  );
  return parseReviewSessionResponse(payload);
}
