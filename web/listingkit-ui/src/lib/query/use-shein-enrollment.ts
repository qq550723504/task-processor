"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  executeSheinActivityEnrollment,
  getSheinActivityCandidates,
  getSheinActivityEnrollmentRuns,
  getSheinEnrollmentDashboard,
  getSheinEnrollmentStoreSummary,
  getSheinSDSCostGroups,
  getSheinSyncedProducts,
  refreshSheinActivityCandidates,
  reviewSheinActivityCandidate,
  triggerSheinStoreSync,
  updateSheinSDSCostGroup,
  updateSheinSyncedProductCost,
} from "@/lib/api/shein-enrollment";
import { listingKitKeys } from "@/lib/query/keys";
import type {
  SheinActivityCandidateQuery,
  SheinEnrollmentRunQuery,
  SheinEnrollmentSummaryQuery,
  SheinExecuteEnrollmentInput,
  SheinRefreshCandidatesInput,
  SheinReviewActivityCandidateInput,
  SheinSDSCostGroupQuery,
  SheinSyncedProductQuery,
  SheinSyncTriggerMode,
} from "@/lib/types/listingkit/shein-enrollment";

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
  });
}

export function useSheinSyncedProducts(
  storeId: number,
  query: SheinSyncedProductQuery,
) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentProducts(storeId, query),
    queryFn: () => getSheinSyncedProducts(storeId, query),
    enabled: Number.isFinite(storeId) && storeId > 0,
  });
}

export function useSheinSDSCostGroups(
  storeId: number,
  query: SheinSDSCostGroupQuery,
) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentSDSCostGroups(storeId, query),
    queryFn: () => getSheinSDSCostGroups(storeId, query),
    enabled: Number.isFinite(storeId) && storeId > 0,
  });
}

export function useSheinActivityCandidates(
  storeId: number,
  query: SheinActivityCandidateQuery,
) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentCandidates(storeId, query),
    queryFn: () => getSheinActivityCandidates(storeId, query),
    enabled: Number.isFinite(storeId) && storeId > 0 && query.activity_type.trim().length > 0,
  });
}

export function useSheinActivityEnrollmentRuns(
  storeId: number,
  query: SheinEnrollmentRunQuery,
) {
  return useQuery({
    queryKey: listingKitKeys.sheinEnrollmentRuns(storeId, query),
    queryFn: () => getSheinActivityEnrollmentRuns(storeId, query),
    enabled: Number.isFinite(storeId) && storeId > 0,
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
