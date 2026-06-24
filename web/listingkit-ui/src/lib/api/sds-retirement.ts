import { apiRequest } from "@/lib/api/client";
import type {
  CreateSDSRetirementRunInput,
  SDSRetirementRunDetail,
  SDSRetirementSelectionUpdate,
} from "@/lib/types/sds-retirement";

export function createSDSRetirementRun(input: CreateSDSRetirementRunInput) {
  return apiRequest<SDSRetirementRunDetail>("/sds/retirements", {
    method: "POST",
    body: input,
  });
}

export function getSDSRetirementRun(runId: string) {
  return apiRequest<SDSRetirementRunDetail>(`/sds/retirements/${runId}`, {
    method: "GET",
  });
}

export function updateSDSRetirementSelection(
  runId: string,
  items: SDSRetirementSelectionUpdate[],
) {
  return apiRequest<SDSRetirementRunDetail>(`/sds/retirements/${runId}/items`, {
    method: "PATCH",
    body: { items },
  });
}

export function confirmSDSRetirementRun(runId: string) {
  return apiRequest<SDSRetirementRunDetail>(`/sds/retirements/${runId}/confirm`, {
    method: "POST",
    body: {},
  });
}

export function retrySDSRetirementRun(runId: string) {
  return apiRequest<SDSRetirementRunDetail>(`/sds/retirements/${runId}/retry`, {
    method: "POST",
    body: {},
  });
}
