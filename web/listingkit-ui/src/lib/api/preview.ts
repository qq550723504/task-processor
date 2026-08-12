import { apiRequest } from "@/lib/api/client";
import type { components } from "@/lib/api/generated/listingkit-asset";
import { parsePreviewResponse } from "@/lib/api/listingkit-response-schema";
import type { ListingKitPreview } from "@/lib/types/listingkit";

type GeneratedListingKitPreview = components["schemas"]["ListingKitPreview"];

export async function getListingKitPreview(taskId: string) {
  return parsePreviewResponse(
    await apiRequest<GeneratedListingKitPreview>(`/tasks/${taskId}/preview`),
  );
}
