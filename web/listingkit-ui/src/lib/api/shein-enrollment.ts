import { apiRequest } from "@/lib/api/client";
import type {
  ExecuteSheinActivityEnrollmentResponse,
  RefreshSheinActivityCandidatesResponse,
  ReviewSheinActivityCandidateResponse,
  SheinActivityCandidateListResponse,
  SheinActivityCandidateQuery,
  SheinEnrollmentDashboardResponse,
  SheinEnrollmentRunListResponse,
  SheinEnrollmentRunQuery,
  SheinEnrollmentStoreSummaryResponse,
  SheinEnrollmentSummaryQuery,
  SheinExecuteEnrollmentInput,
  SheinRefreshCandidatesInput,
  SheinReviewActivityCandidateInput,
  SheinSyncedProductListResponse,
  SheinSyncedProductQuery,
  SheinSyncTriggerMode,
  SheinUpdateSyncedProductCostInput,
  TriggerSheinStoreSyncResponse,
} from "@/lib/types/listingkit/shein-enrollment";

export async function getSheinEnrollmentDashboard(
  query: SheinEnrollmentSummaryQuery = {},
): Promise<SheinEnrollmentDashboardResponse> {
  return apiRequest<SheinEnrollmentDashboardResponse>("/shein-sync/dashboard", {
    query,
  });
}

export async function triggerSheinStoreSync(
  storeId: number,
  input: { trigger_mode?: SheinSyncTriggerMode } = {},
): Promise<TriggerSheinStoreSyncResponse> {
  return apiRequest<TriggerSheinStoreSyncResponse>(
    `/shein-sync/stores/${storeId}/sync`,
    {
      method: "POST",
      body: input,
    },
  );
}

export async function getSheinSyncedProducts(
  storeId: number,
  query: SheinSyncedProductQuery,
): Promise<SheinSyncedProductListResponse> {
  return apiRequest<SheinSyncedProductListResponse>(
    `/shein-sync/stores/${storeId}/products`,
    {
      query,
    },
  );
}

export async function getSheinEnrollmentStoreSummary(
  storeId: number,
  query: SheinEnrollmentSummaryQuery = {},
): Promise<SheinEnrollmentStoreSummaryResponse> {
  return apiRequest<SheinEnrollmentStoreSummaryResponse>(
    `/shein-sync/stores/${storeId}/summary`,
    {
      query,
    },
  );
}

export async function updateSheinSyncedProductCost(
  productId: number,
  input: SheinUpdateSyncedProductCostInput,
): Promise<{ id?: number; manual_cost_price?: number | null }> {
  return apiRequest<{ id?: number; manual_cost_price?: number | null }>(
    `/shein-sync/products/${productId}/cost`,
    {
      method: "PATCH",
      body: input,
    },
  );
}

export async function refreshSheinActivityCandidates(
  storeId: number,
  input: SheinRefreshCandidatesInput,
): Promise<RefreshSheinActivityCandidatesResponse> {
  return apiRequest<RefreshSheinActivityCandidatesResponse>(
    `/shein-sync/stores/${storeId}/candidates/refresh`,
    {
      method: "POST",
      body: input,
    },
  );
}

export async function getSheinActivityCandidates(
  storeId: number,
  query: SheinActivityCandidateQuery,
): Promise<SheinActivityCandidateListResponse> {
  return apiRequest<SheinActivityCandidateListResponse>(
    `/shein-sync/stores/${storeId}/candidates`,
    {
      query,
    },
  );
}

export async function reviewSheinActivityCandidate(
  candidateId: number,
  input: SheinReviewActivityCandidateInput,
): Promise<ReviewSheinActivityCandidateResponse> {
  return apiRequest<ReviewSheinActivityCandidateResponse>(
    `/shein-sync/candidates/${candidateId}/review`,
    {
      method: "PATCH",
      body: input,
    },
  );
}

export async function executeSheinActivityEnrollment(
  storeId: number,
  input: SheinExecuteEnrollmentInput,
): Promise<ExecuteSheinActivityEnrollmentResponse> {
  return apiRequest<ExecuteSheinActivityEnrollmentResponse>(
    `/shein-sync/stores/${storeId}/enrollments`,
    {
      method: "POST",
      body: input,
    },
  );
}

export async function getSheinActivityEnrollmentRuns(
  storeId: number,
  query: SheinEnrollmentRunQuery,
): Promise<SheinEnrollmentRunListResponse> {
  return apiRequest<SheinEnrollmentRunListResponse>(
    `/shein-sync/stores/${storeId}/enrollment-runs`,
    {
      query,
    },
  );
}
