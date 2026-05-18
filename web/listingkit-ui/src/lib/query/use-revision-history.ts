"use client";

import { useQuery } from "@tanstack/react-query";

import {
  getTaskRevisionHistory,
  getTaskRevisionHistoryDetail,
  type RevisionHistoryDetailQuery,
  type RevisionHistoryQuery,
} from "@/lib/api/revision-history";
import { listingKitKeys } from "@/lib/query/keys";

export function useTaskRevisionHistory(taskId: string, query: RevisionHistoryQuery = {}) {
  return useQuery({
    queryKey: listingKitKeys.revisionHistory(taskId, query),
    queryFn: () => getTaskRevisionHistory(taskId, query),
    enabled: Boolean(taskId),
  });
}

export function useTaskRevisionHistoryDetail(
  taskId: string,
  revisionId?: string,
  query: RevisionHistoryDetailQuery = {},
) {
  return useQuery({
    queryKey: listingKitKeys.revisionHistoryDetail(taskId, revisionId ?? "", query.compare_to),
    queryFn: () => getTaskRevisionHistoryDetail(taskId, revisionId ?? "", query),
    enabled: Boolean(taskId && revisionId),
  });
}
