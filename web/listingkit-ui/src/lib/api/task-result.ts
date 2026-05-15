import { apiRequest } from "@/lib/api/client";
import { parseTaskResultResponse } from "@/lib/api/listingkit-response-schema";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

export async function getListingKitTaskResult(taskId: string) {
  return parseTaskResultResponse(
    await apiRequest<ListingKitTaskResult>(`/tasks/${taskId}`),
  );
}
