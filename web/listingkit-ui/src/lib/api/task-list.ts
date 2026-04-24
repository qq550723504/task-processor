import { apiRequest } from "@/lib/api/client";
import type {
  ListingKitTaskListPage,
  ListingKitTaskListQuery,
} from "@/lib/types/listingkit";

export function getListingKitTasks(query: ListingKitTaskListQuery = {}) {
  return apiRequest<ListingKitTaskListPage>("/tasks", { query });
}
