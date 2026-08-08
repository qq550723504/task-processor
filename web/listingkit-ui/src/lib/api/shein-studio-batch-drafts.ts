import { apiRequest } from "@/lib/api/client";
import {
  buildStudioBatchDraftUpsertPayload,
  type UpsertSheinStudioBatchDraftInput,
} from "@/lib/api/shein-studio-batch-draft-request-codec";
import {
  mapStudioBatchDraftDetailToBatch,
  mapStudioBatchDraftListResponse,
  parseStudioBatchDraftDetailResponse,
} from "@/lib/api/shein-studio-batch-draft-response-codec";

const STUDIO_BATCH_DRAFT_TIMEOUT_MS = 60_000;

type StudioBatchDraftRequestOptions = {
  signal?: AbortSignal;
  timeoutMs?: number;
  limit?: number;
};

export async function listSheinStudioBatchDrafts(
  options?: StudioBatchDraftRequestOptions,
) {
  const payload = await apiRequest<unknown>("/studio/batches", {
    query:
      typeof options?.limit === "number" && options.limit > 0
        ? { limit: options.limit }
        : undefined,
    signal: options?.signal,
    timeoutMs: options?.timeoutMs ?? STUDIO_BATCH_DRAFT_TIMEOUT_MS,
  });
  return mapStudioBatchDraftListResponse(payload);
}

export async function upsertSheinStudioBatchDraft(
  input: UpsertSheinStudioBatchDraftInput,
  options?: StudioBatchDraftRequestOptions,
) {
  const detail = parseStudioBatchDraftDetailResponse(
    await apiRequest<unknown>("/studio/batches", {
      method: "POST",
      body: buildStudioBatchDraftUpsertPayload(input),
      signal: options?.signal,
      timeoutMs: options?.timeoutMs ?? STUDIO_BATCH_DRAFT_TIMEOUT_MS,
    }),
  );
  return mapStudioBatchDraftDetailToBatch(detail);
}

export async function deleteSheinStudioBatchDraft(
  batchId: string,
  options?: StudioBatchDraftRequestOptions,
) {
  await apiRequest<unknown>(`/studio/batches/${batchId}`, {
    method: "DELETE",
    signal: options?.signal,
    timeoutMs: options?.timeoutMs ?? STUDIO_BATCH_DRAFT_TIMEOUT_MS,
  });
}
