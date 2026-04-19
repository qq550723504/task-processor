import { apiRequest } from "@/lib/api/client";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

export function getListingKitTaskResult(taskId: string) {
  return apiRequest<ListingKitTaskResult>(`/tasks/${taskId}`);
}
