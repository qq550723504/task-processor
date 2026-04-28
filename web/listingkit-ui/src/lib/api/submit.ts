import { apiRequest } from "@/lib/api/client";
import type { ListingKitPreview } from "@/lib/types/listingkit";

export type SubmitTaskRequest = {
  platform: "shein";
  action?: "publish" | "save_draft";
  confirmed_final?: boolean;
};

export function submitTask(taskId: string, body: SubmitTaskRequest) {
  return apiRequest<ListingKitPreview>(`/tasks/${taskId}/submit`, {
    method: "POST",
    body,
  });
}
