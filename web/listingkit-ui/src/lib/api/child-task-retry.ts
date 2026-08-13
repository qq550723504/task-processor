import { apiRequest } from "@/lib/api/client";
import { parseTaskChildRetryAcceptedResponse } from "@/lib/api/listingkit-response-schema";
import type { TaskChildRetryAccepted } from "@/lib/types/listingkit/tasks";

export type RetryChildTaskRequest = {
  kind: string;
};

export async function retryChildTask(taskId: string, body: RetryChildTaskRequest): Promise<TaskChildRetryAccepted> {
  return parseTaskChildRetryAcceptedResponse(
    await apiRequest<TaskChildRetryAccepted>(`/tasks/${taskId}/child-tasks/retry`, {
      method: "POST",
      body,
    }),
  );
}
