import { z } from "zod";

import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type {
  BulkRecoverTasksRequest,
  BulkRecoverTasksResponse,
  RecoverTaskNowResponse,
} from "@/lib/types/listingkit/tasks";

const retryableBlockSchema = z
  .object({
    reason_code: z.string().optional(),
    reason_message: z.string().optional(),
    blocked_at: z.string().optional(),
    last_retry_at: z.string().optional(),
    next_retry_at: z.string().optional(),
    retry_attempts: z.coerce.number().optional(),
    max_auto_retry_attempts: z.coerce.number().optional(),
    recovery_scope: z.string().optional(),
    auto_resume_enabled: z.boolean().optional(),
    auto_retry_paused: z.boolean().optional(),
  })
  .passthrough();

const recoverTaskNowResponseSchema = z
  .object({
    task: z
      .object({
        id: z.string().optional(),
        tenant_id: z.string().optional(),
        status: z.string().optional(),
        retryable_block: retryableBlockSchema.optional(),
        error: z.string().optional(),
        created_at: z.string().optional(),
        updated_at: z.string().optional(),
      })
      .passthrough()
      .optional(),
  })
  .passthrough();

const bulkRecoverTasksResponseSchema = z
  .object({
    recovered_count: z.coerce.number(),
  })
  .passthrough();

export async function recoverTaskNow(taskId: string) {
  return parseApiResponseShape(
    await apiRequest<unknown>(`/tasks/${taskId}/recover`, {
      method: "POST",
    }),
    recoverTaskNowResponseSchema,
    "ListingKit API returned an unexpected task recovery response",
  ) as RecoverTaskNowResponse;
}

export async function bulkRecoverTasks(request: BulkRecoverTasksRequest) {
  const { due_before, recover_at, limit } = request;
  const search = new URLSearchParams();

  if (due_before) {
    search.set("due_before", due_before);
  }
  if (typeof limit === "number") {
    search.set("limit", String(limit));
  }

  return parseApiResponseShape(
    await apiRequest<unknown>(`/tasks/recovery/recover${search.size > 0 ? `?${search.toString()}` : ""}`, {
      method: "POST",
      body: recover_at ? { recover_at } : undefined,
    }),
    bulkRecoverTasksResponseSchema,
    "ListingKit API returned an unexpected bulk recovery response",
  ) as BulkRecoverTasksResponse;
}
