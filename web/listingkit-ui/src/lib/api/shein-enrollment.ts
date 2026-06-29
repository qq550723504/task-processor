import { apiRequest } from "@/lib/api/client";
import type {
  ExecuteSheinActivityEnrollmentResponse,
  RefreshSheinActivityCandidatesResponse,
  ReviewSheinActivityCandidateResponse,
  SheinActivityCandidateListResponse,
  SheinActivityCandidateQuery,
  SheinActivityStrategyResponse,
  SheinEnrollmentDashboardResponse,
  SheinEnrollmentRunItemListResponse,
  SheinEnrollmentRunItemQuery,
  SheinEnrollmentRunListResponse,
  SheinEnrollmentRunQuery,
  SheinEnrollmentStoreSummaryResponse,
  SheinEnrollmentSummaryQuery,
  SheinExecuteEnrollmentInput,
  SheinRefreshCandidatesInput,
  SheinReviewActivityCandidateInput,
  SheinSDSCostGroupListResponse,
  SheinSDSCostGroupQuery,
  SheinSourceSDSCostGroupListResponse,
  SheinSourceSDSMetadataResponse,
  SheinSyncedProductListResponse,
  SheinSyncedProductQuery,
  SheinSyncTriggerMode,
  SheinUpdateSDSCostGroupInput,
  SheinUpdateSyncedProductCostInput,
  SheinUpdateActivityStrategyInput,
  SyncSheinSourceSDSProductResponse,
  TriggerSheinStoreSyncResponse,
  UpdateSheinSDSCostGroupResponse,
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

export async function syncSheinSourceSDSProduct(
  storeId: number,
  sourceCode: string,
): Promise<SyncSheinSourceSDSProductResponse> {
  return apiRequest<SyncSheinSourceSDSProductResponse>(
    `/shein-sync/stores/${storeId}/source-sds-products/${encodeURIComponent(sourceCode)}/sync`,
    {
      method: "POST",
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

export async function getSheinActivityStrategy(
  storeId: number,
  activityType = "PROMOTION",
): Promise<SheinActivityStrategyResponse> {
  return apiRequest<SheinActivityStrategyResponse>(
    `/shein-sync/stores/${storeId}/activity-strategy`,
    {
      query: {
        activity_type: activityType,
      },
    },
  );
}

export async function updateSheinActivityStrategy(
  storeId: number,
  input: SheinUpdateActivityStrategyInput,
): Promise<SheinActivityStrategyResponse> {
  return apiRequest<SheinActivityStrategyResponse>(
    `/shein-sync/stores/${storeId}/activity-strategy`,
    {
      method: "PATCH",
      body: input,
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

export async function getSheinSDSCostGroups(
  storeId: number,
  query: SheinSDSCostGroupQuery = {},
): Promise<SheinSDSCostGroupListResponse> {
  return apiRequest<SheinSDSCostGroupListResponse>(
    `/shein-sync/stores/${storeId}/sds-cost-groups`,
    {
      query,
    },
  );
}

export async function getSheinSourceSDSCostGroups(
  storeId: number,
  query: SheinSDSCostGroupQuery = {},
): Promise<SheinSourceSDSCostGroupListResponse> {
  return apiRequest<SheinSourceSDSCostGroupListResponse>(
    `/shein-sync/stores/${storeId}/source-sds-cost-groups`,
    {
      query,
    },
  );
}

export async function getSheinSourceSDSMetadata(
  storeId: number,
  sourceCodes: string[],
): Promise<SheinSourceSDSMetadataResponse> {
  return apiRequest<SheinSourceSDSMetadataResponse>(
    `/shein-sync/stores/${storeId}/source-sds-metadata`,
    {
      query: {
        source_codes: sourceCodes.join(","),
      },
    },
  );
}

export async function updateSheinSDSCostGroup(
  storeId: number,
  groupKey: string,
  input: SheinUpdateSDSCostGroupInput,
): Promise<UpdateSheinSDSCostGroupResponse> {
  return apiRequest<UpdateSheinSDSCostGroupResponse>(
    `/shein-sync/stores/${storeId}/sds-cost-groups/${encodeURIComponent(groupKey)}/cost`,
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

export async function getSheinActivityEnrollmentRunItems(
  storeId: number,
  runId: number,
  query: SheinEnrollmentRunItemQuery,
): Promise<SheinEnrollmentRunItemListResponse> {
  return apiRequest<SheinEnrollmentRunItemListResponse>(
    `/shein-sync/stores/${storeId}/enrollment-runs/${runId}/items`,
    {
      query,
    },
  );
}
