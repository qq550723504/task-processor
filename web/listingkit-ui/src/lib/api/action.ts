import { apiRequest } from "@/lib/api/client";
import type {
  ActionExecutionRequest,
  ActionExecutionResult,
} from "@/lib/types/listingkit";

export function executeAction(taskId: string, request: ActionExecutionRequest) {
  return apiRequest<ActionExecutionResult>(
    `/tasks/${taskId}/generation-actions/execute`,
    {
      method: "POST",
      body: request,
    },
  );
}
