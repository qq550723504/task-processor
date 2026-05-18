import { apiRequest } from "@/lib/api/client";
import type {
  ListingKitRevisionHistoryDetail,
  ListingKitRevisionHistoryPage,
} from "@/lib/types/listingkit";

export type RevisionHistoryQuery = {
  limit?: number;
  before?: string;
  action_type?: "edit" | "restore" | string;
};

export type RevisionHistoryDetailQuery = {
  compare_to?: "prev" | "next" | "current" | string;
};

function withQuery(path: string, query?: Record<string, string | number | undefined>) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(query ?? {})) {
    if (value === undefined || value === null || value === "") {
      continue;
    }
    params.set(key, String(value));
  }
  const suffix = params.toString();
  return suffix ? `${path}?${suffix}` : path;
}

export function getTaskRevisionHistory(taskId: string, query?: RevisionHistoryQuery) {
  return apiRequest<ListingKitRevisionHistoryPage>(
    withQuery(`/tasks/${taskId}/revision-history`, query),
  );
}

export function getTaskRevisionHistoryDetail(
  taskId: string,
  revisionId: string,
  query?: RevisionHistoryDetailQuery,
) {
  return apiRequest<ListingKitRevisionHistoryDetail>(
    withQuery(`/tasks/${taskId}/revision-history/${revisionId}`, query),
  );
}
