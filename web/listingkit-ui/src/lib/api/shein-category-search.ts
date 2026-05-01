import { apiRequest } from "@/lib/api/client";
import type { SheinManualCategorySearchResult } from "@/lib/types/listingkit";

export function searchSheinCategories(taskId: string, query: string) {
  const encodedQuery = encodeURIComponent(query);
  return apiRequest<SheinManualCategorySearchResult>(
    `/tasks/${taskId}/shein/categories?query=${encodedQuery}`,
  );
}
