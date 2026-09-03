import { apiRequest } from "@/lib/api/client";
import type {
  ListingKitPreview as GeneratedListingKitPreview,
  TargetPlatform,
} from "@/lib/api/generated";
import { parsePreviewResponse } from "@/lib/api/listingkit-response-schema";

export async function getListingKitPreview(
  taskId: string,
  platform?: TargetPlatform,
) {
  return parsePreviewResponse(
    await apiRequest<GeneratedListingKitPreview>(`/tasks/${taskId}/preview`, {
      query: platform ? { platform } : undefined,
    }),
  );
}
