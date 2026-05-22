import { apiRequest } from "@/lib/api/client";
import { parseTaskResultResponse } from "@/lib/api/listingkit-response-schema";
import type { ListingKitTaskResult } from "@/lib/types/listingkit/tasks";

export type RetryChildTaskRequest = {
  kind: string;
};

export async function retryChildTask(taskId: string, body: RetryChildTaskRequest) {
  return parseTaskResultResponse(
    await apiRequest<ListingKitTaskResult>(`/tasks/${taskId}/child-tasks/retry`, {
      method: "POST",
      body,
    }),
  );
}
