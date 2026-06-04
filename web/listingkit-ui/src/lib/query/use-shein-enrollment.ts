"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";

import {
  executeSheinActivityEnrollment,
  getSheinActivityCandidates,
  getSheinSyncedProducts,
  refreshSheinActivityCandidates,
  reviewSheinActivityCandidate,
  triggerSheinStoreSync,
  updateSheinSyncedProductCost,
} from "@/lib/api/shein-enrollment";
import { listingKitKeys } from "@/lib/query/keys";
import type {
  SheinActivityCandidateQuery,
  SheinExecuteEnrollmentInput,
  SheinRefreshCandidatesInput,
  SheinReviewActivityCandidateInput,
  SheinSyncedProductQuery,
  SheinSyncTriggerMode,
} from "@/lib/types/listingkit/shein-enrollment";

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
