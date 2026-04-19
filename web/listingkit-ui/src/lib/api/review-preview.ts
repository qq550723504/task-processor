import { apiRequest } from "@/lib/api/client";
import type {
  ConditionalState,
  QueueQuery,
  ReviewPreviewResponse,
} from "@/lib/types/listingkit";

export function getReviewPreview(
  taskId: string,
  query: QueueQuery,
  conditional?: ConditionalState | null,
) {
  return apiRequest<ReviewPreviewResponse>(
    `/tasks/${taskId}/generation-review-preview`,
    {
      query,
      conditional,
    },
  );
}
