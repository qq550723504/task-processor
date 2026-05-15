import { apiRequest } from "@/lib/api/client";
import { parseReviewPreviewResponse } from "@/lib/api/listingkit-response-schema";
import type {
  ConditionalState,
  QueueQuery,
  ReviewPreviewResponse,
} from "@/lib/types/listingkit";

export async function getReviewPreview(
  taskId: string,
  query: QueueQuery,
  conditional?: ConditionalState | null,
) {
  return parseReviewPreviewResponse(
    await apiRequest<ReviewPreviewResponse>(
      `/tasks/${taskId}/generation-review-preview`,
      {
        query,
        conditional,
      },
    ),
  );
}
