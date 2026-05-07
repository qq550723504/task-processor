import { apiRequest } from "@/lib/api/client";
import type { ListingKitPreview } from "@/lib/types/listingkit";

export type SubmitTaskRequest = {
  platform: "shein";
  action?: "publish" | "save_draft";
  confirmed_final?: boolean;
  idempotency_key?: string;
};

function createSubmitIdempotencyKey() {
  return globalThis.crypto?.randomUUID?.() ?? `submit-${Date.now()}`;
}

export function submitTask(taskId: string, body: SubmitTaskRequest) {
  return apiRequest<ListingKitPreview>(`/tasks/${taskId}/submit`, {
    method: "POST",
    body: {
      ...body,
      idempotency_key: body.idempotency_key ?? createSubmitIdempotencyKey(),
    },
  });
}

export function refreshSubmissionStatus(taskId: string) {
  return apiRequest<ListingKitPreview>(
    `/tasks/${taskId}/submission-status/refresh`,
    {
      method: "POST",
    },
  );
}
