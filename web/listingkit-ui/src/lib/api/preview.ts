import { apiRequest } from "@/lib/api/client";
import { parsePreviewResponse } from "@/lib/api/listingkit-response-schema";
import type { ListingKitPreview } from "@/lib/types/listingkit";

export async function getListingKitPreview(taskId: string) {
  return parsePreviewResponse(
    await apiRequest<ListingKitPreview>(`/tasks/${taskId}/preview`),
  );
}
