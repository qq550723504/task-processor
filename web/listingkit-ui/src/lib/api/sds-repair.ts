import { apiRequest } from "@/lib/api/client";
import { parseTaskResultResponse } from "@/lib/api/listingkit-response-schema";
import type { ApplyTaskSDSRepairRequest, ListingKitTaskResult, TaskSDSRepairSession } from "@/lib/types/listingkit/tasks";

export const getTaskSDSRepair = (taskId: string) =>
  apiRequest<TaskSDSRepairSession>(`/tasks/${taskId}/sds-repair`);

export const repairAndRetryTaskSDS = async (taskId: string, body: ApplyTaskSDSRepairRequest) =>
  parseTaskResultResponse(await apiRequest<ListingKitTaskResult>(`/tasks/${taskId}/sds-repair/retry`, { method: "POST", body }));
