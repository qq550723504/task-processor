import { apiRequest } from "@/lib/api/client";
import type { ListingKitPreview } from "@/lib/types/listingkit";

export function getListingKitPreview(taskId: string) {
  return apiRequest<ListingKitPreview>(`/tasks/${taskId}/preview`);
}
