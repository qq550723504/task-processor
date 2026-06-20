import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type {
  SheinStudioBatchRun,
  SheinStudioBatchRunFailurePolicy,
  SheinStudioBatchRunItem,
  SheinStudioBatchRunItemStatus,
  SheinStudioBatchRunMode,
  SheinStudioBatchRunStartResponse,
  SheinStudioBatchRunStatus,
} from "@/lib/types/shein-studio-batch-runs";
import { z } from "zod";

const studioBatchRunModeSchema = z.enum(["generate", "create_tasks"]);
const studioBatchRunFailurePolicySchema = z.enum([
  "continue_on_error",
  "stop_on_error",
]);
const studioBatchRunStatusSchema = z.enum([
  "pending",
  "running",
  "succeeded",
  "partially_succeeded",
  "failed",
  "cancelled",
]);
const studioBatchRunItemStatusSchema = z.enum([
  "pending",
  "running",
  "succeeded",
  "failed",
  "cancelled",
]);

const studioBatchRunSchema = z
  .object({
    id: z.string(),
    mode: studioBatchRunModeSchema,
    failure_policy: studioBatchRunFailurePolicySchema,
    status: studioBatchRunStatusSchema,
    current_batch_id: z.string().optional(),
    current_index: z.number(),
    total_batches: z.number(),
    completed_batches: z.number(),
    succeeded_batches: z.number(),
    failed_batches: z.number(),
    last_error: z.string().optional(),
    cancel_requested: z.boolean(),
    started_at: z.string().optional(),
    finished_at: z.string().optional(),
    created_at: z.string(),
    updated_at: z.string(),
  })
  .passthrough();

const studioBatchRunItemSchema = z
  .object({
    id: z.string(),
    run_id: z.string(),
    batch_id: z.string(),
    position: z.number(),
    status: studioBatchRunItemStatusSchema,
    session_id: z.string().optional(),
    async_job_id: z.string().optional(),
    error_message: z.string().optional(),
    batch_status: z.string().optional(),
    batch_last_error: z.string().optional(),
    started_at: z.string().optional(),
    finished_at: z.string().optional(),
    created_at: z.string(),
    updated_at: z.string(),
  })
  .passthrough();

const studioBatchRunStartResponseSchema = z
  .object({
    run: studioBatchRunSchema,
    items: z.array(studioBatchRunItemSchema).optional(),
  })
  .passthrough();

const studioBatchRunResponseSchema = z
  .object({
    run: studioBatchRunSchema,
  })
  .passthrough();

const studioBatchRunItemsResponseSchema = z
  .object({
    items: z.array(studioBatchRunItemSchema).optional(),
  })
  .passthrough();

export function mapStudioBatchRun(
  payload: z.infer<typeof studioBatchRunSchema>,
): SheinStudioBatchRun {
  return {
    id: payload.id,
    mode: payload.mode as SheinStudioBatchRunMode,
    failurePolicy:
      payload.failure_policy as SheinStudioBatchRunFailurePolicy,
    status: payload.status as SheinStudioBatchRunStatus,
    currentBatchId: payload.current_batch_id,
    currentIndex: payload.current_index,
    totalBatches: payload.total_batches,
    completedBatches: payload.completed_batches,
    succeededBatches: payload.succeeded_batches,
    failedBatches: payload.failed_batches,
    lastError: payload.last_error,
    cancelRequested: payload.cancel_requested,
    startedAt: payload.started_at,
    finishedAt: payload.finished_at,
    createdAt: payload.created_at,
    updatedAt: payload.updated_at,
  };
}

export function mapStudioBatchRunItem(
  payload: z.infer<typeof studioBatchRunItemSchema>,
): SheinStudioBatchRunItem {
  return {
    id: payload.id,
    runId: payload.run_id,
    batchId: payload.batch_id,
    position: payload.position,
    status: payload.status as SheinStudioBatchRunItemStatus,
    sessionId: payload.session_id,
    asyncJobId: payload.async_job_id,
    errorMessage: payload.error_message,
    batchStatus: payload.batch_status,
    batchLastError: payload.batch_last_error,
    startedAt: payload.started_at,
    finishedAt: payload.finished_at,
    createdAt: payload.created_at,
    updatedAt: payload.updated_at,
  };
}

export function parseSheinStudioBatchRunStartResponse(
  payload: unknown,
): SheinStudioBatchRunStartResponse {
  const parsed = parseApiResponseShape(
    payload,
    studioBatchRunStartResponseSchema,
    "ListingKit API returned an unexpected studio batch run response",
  );
  return {
    run: mapStudioBatchRun(parsed.run),
    items: (parsed.items ?? []).map(mapStudioBatchRunItem),
  };
}

export function parseSheinStudioBatchRunResponse(
  payload: unknown,
): SheinStudioBatchRun {
  const parsed = parseApiResponseShape(
    payload,
    studioBatchRunResponseSchema,
    "ListingKit API returned an unexpected studio batch run response",
  );
  return mapStudioBatchRun(parsed.run);
}

export function parseSheinStudioBatchRunItemsResponse(
  payload: unknown,
): SheinStudioBatchRunItem[] {
  const parsed = parseApiResponseShape(
    payload,
    studioBatchRunItemsResponseSchema,
    "ListingKit API returned an unexpected studio batch run items response",
  );
  return (parsed.items ?? []).map(mapStudioBatchRunItem);
}

export async function startSheinStudioBatchRun(
  batchIds: string[],
): Promise<SheinStudioBatchRunStartResponse> {
  const payload = await apiRequest<unknown>("/studio/batch-runs", {
    method: "POST",
    body: { batch_ids: batchIds },
  });
  return parseSheinStudioBatchRunStartResponse(payload);
}

export async function getSheinStudioBatchRun(
  runId: string,
): Promise<SheinStudioBatchRun> {
  const payload = await apiRequest<unknown>(`/studio/batch-runs/${runId}`);
  return parseSheinStudioBatchRunResponse(payload);
}

export async function listSheinStudioBatchRunItems(
  runId: string,
): Promise<SheinStudioBatchRunItem[]> {
  const payload = await apiRequest<unknown>(`/studio/batch-runs/${runId}/items`);
  return parseSheinStudioBatchRunItemsResponse(payload);
}

export async function cancelSheinStudioBatchRun(runId: string): Promise<void> {
  await apiRequest<unknown>(`/studio/batch-runs/${runId}/cancel`, {
    method: "POST",
  });
}

export async function recoverSheinStudioBatchRun(runId: string): Promise<void> {
  await apiRequest<unknown>(`/studio/batch-runs/${runId}/recover`, {
    method: "POST",
  });
}
