"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  executeSheinActivityEnrollment,
  getSheinActivityStrategy,
  getSheinActivityCandidates,
  getSheinActivityEnrollmentRunItems,
  getSheinActivityEnrollmentRuns,
  getSheinEnrollmentDashboard,
  getSheinEnrollmentStoreSummary,
  getSheinSDSCostGroups,
  getSheinSourceSDSCostGroups,
  getSheinSyncedProducts,
  refreshSheinActivityCandidates,
  reviewSheinActivityCandidate,
  syncSheinSourceSDSProduct,
  triggerSheinStoreSync,
  updateSheinActivityStrategy,
  updateSheinSDSCostGroup,
  updateSheinSyncedProductCost,
} from "@/lib/api/shein-enrollment";
import { listingKitKeys } from "@/lib/query/keys";
import type {
  SheinActivityCandidateQuery,
  SheinEnrollmentRunItemQuery,
  SheinEnrollmentRunQuery,
  SheinEnrollmentStoreSummaryResponse,
  SheinEnrollmentSummaryQuery,
  SheinExecuteEnrollmentInput,
  SheinRefreshCandidatesInput,
  SheinReviewActivityCandidateInput,
  SheinSDSCostGroupQuery,
  SheinSyncedProductQuery,
  SheinSyncTriggerMode,
  SheinUpdateActivityStrategyInput,
} from "@/lib/types/listingkit/shein-enrollment";

const SHEIN_SYNC_SUMMARY_REFETCH_INTERVAL_MS = 2_000;

type QueryOptions = {
  enabled?: boolean;
};

export function shouldPollSheinSyncSummary(
  data?: SheinEnrollmentStoreSummaryResponse,
) {
  const status =
    data?.summary?.last_sync_job?.status || data?.summary?.last_sync_status || "";
  return status === "pending" || status === "running";
}

export function useSheinEnrollmentDashboard(
  query: SheinEnrollmentSummaryQuery,
) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentDashboard(query),
    queryFn: () => getSheinEnrollmentDashboard(query),
  });
}

export function useSheinEnrollmentStoreSummary(
  storeId: number,
  query: SheinEnrollmentSummaryQuery,
) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentStoreSummary(storeId, query),
    queryFn: () => getSheinEnrollmentStoreSummary(storeId, query),
    enabled: Number.isFinite(storeId) && storeId > 0,
    refetchInterval: (queryState) =>
      shouldPollSheinSyncSummary(queryState.state.data)
        ? SHEIN_SYNC_SUMMARY_REFETCH_INTERVAL_MS
        : false,
    refetchOnWindowFocus: true,
  });
}

export function useSheinActivityStrategy(storeId: number, activityType = "PROMOTION") {
  return useQuery({
    queryKey: listingKitKeys.sheinActivityStrategy(storeId, activityType),
    queryFn: () => getSheinActivityStrategy(storeId, activityType),
    enabled:
      Number.isFinite(storeId) && storeId > 0 && activityType.trim().length > 0,
  });
}

export function useSheinSyncedProducts(
  storeId: number,
  query: SheinSyncedProductQuery,
  options: QueryOptions = {},
) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentProducts(storeId, query),
    queryFn: () => getSheinSyncedProducts(storeId, query),
    enabled: options.enabled !== false && Number.isFinite(storeId) && storeId > 0,
  });
}

export function useSheinSDSCostGroups(
  storeId: number,
  query: SheinSDSCostGroupQuery,
  options: QueryOptions = {},
) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentSDSCostGroups(storeId, query),
    queryFn: () => getSheinSDSCostGroups(storeId, query),
    enabled: options.enabled !== false && Number.isFinite(storeId) && storeId > 0,
  });
}

export function useSheinSourceSDSCostGroups(
  storeId: number,
  query: SheinSDSCostGroupQuery,
  options: QueryOptions = {},
) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentSourceSDSCostGroups(storeId, query),
    queryFn: () => getSheinSourceSDSCostGroups(storeId, query),
    enabled: options.enabled !== false && Number.isFinite(storeId) && storeId > 0,
  });
}

export function useSheinActivityCandidates(
  storeId: number,
  query: SheinActivityCandidateQuery,
  options: QueryOptions = {},
) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentCandidates(storeId, query),
    queryFn: () => getSheinActivityCandidates(storeId, query),
    enabled:
      options.enabled !== false &&
      Number.isFinite(storeId) &&
      storeId > 0 &&
      query.activity_type.trim().length > 0,
  });
}

export function useSheinActivityEnrollmentRuns(
  storeId: number,
  query: SheinEnrollmentRunQuery,
  options: QueryOptions = {},
) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentRuns(storeId, query),
    queryFn: () => getSheinActivityEnrollmentRuns(storeId, query),
    enabled: options.enabled !== false && Number.isFinite(storeId) && storeId > 0,
  });
}

export function useSheinActivityEnrollmentRunItems(
  storeId: number,
  runId: number,
  query: SheinEnrollmentRunItemQuery,
  options: QueryOptions = {},
) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentRunItems(storeId, runId, query),
    queryFn: () => getSheinActivityEnrollmentRunItems(storeId, runId, query),
    enabled:
      options.enabled !== false &&
      Number.isFinite(storeId) &&
      storeId > 0 &&
      Number.isFinite(runId) &&
      runId > 0,
  });
}

export function useTriggerSheinStoreSync(storeId: number) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: { trigger_mode?: SheinSyncTriggerMode } = {}) =>
      triggerSheinStoreSync(storeId, input),
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: listingKitKeys.sheinEnrollmentStoreScope(storeId),
      });
    },
  });
}

export function useSyncSheinSourceSDSProduct(storeId: number) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (sourceCode: string) =>
      syncSheinSourceSDSProduct(storeId, sourceCode),
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: listingKitKeys.sheinEnrollmentStoreScope(storeId),
      });
    },
  });
}

export function useUpdateSheinActivityStrategy(storeId: number) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: SheinUpdateActivityStrategyInput) =>
      updateSheinActivityStrategy(storeId, input),
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: listingKitKeys.sheinEnrollmentStoreScope(storeId),
      });
    },
  });
}

export function useUpdateSheinSyncedProductCost(storeId: number) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({
      productId,
      manual_cost_price,
    }: {
      productId: number;
      manual_cost_price?: number | null;
    }) => updateSheinSyncedProductCost(productId, { manual_cost_price }),
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: listingKitKeys.sheinEnrollmentStoreScope(storeId),
      });
    },
  });
}

export function useUpdateSheinSDSCostGroup(storeId: number) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({
      groupKey,
      group_label,
      manual_cost_price,
    }: {
      groupKey: string;
      group_label?: string;
      manual_cost_price?: number | null;
    }) =>
      updateSheinSDSCostGroup(storeId, groupKey, {
        group_label,
        manual_cost_price,
      }),
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: listingKitKeys.sheinEnrollmentStoreScope(storeId),
      });
    },
  });
}

export function useRefreshSheinActivityCandidates(storeId: number) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: SheinRefreshCandidatesInput) =>
      refreshSheinActivityCandidates(storeId, input),
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: listingKitKeys.sheinEnrollmentStoreScope(storeId),
      });
    },
  });
}

export function useReviewSheinActivityCandidate(storeId: number) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: ({
      candidateId,
      input,
    }: {
      candidateId: number;
      input: SheinReviewActivityCandidateInput;
    }) => reviewSheinActivityCandidate(candidateId, input),
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: listingKitKeys.sheinEnrollmentStoreScope(storeId),
      });
    },
  });
}

export function useExecuteSheinActivityEnrollment(storeId: number) {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (input: SheinExecuteEnrollmentInput) =>
      executeSheinActivityEnrollment(storeId, input),
    onSuccess: async () => {
      await client.invalidateQueries({
        queryKey: listingKitKeys.sheinEnrollmentStoreScope(storeId),
      });
    },
  });
}
