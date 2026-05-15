import { apiRequest } from "@/lib/api/client";
import { parseTaskListResponse } from "@/lib/api/task-list-schema";
import type {
  ListingKitTaskListPage,
  ListingKitTaskListQuery,
} from "@/lib/types/listingkit";

export async function getListingKitTasks(
  query: ListingKitTaskListQuery = {},
): Promise<ListingKitTaskListPage> {
  const payload = await apiRequest<unknown>("/tasks", { query });
  return parseTaskListResponse(payload);
}
