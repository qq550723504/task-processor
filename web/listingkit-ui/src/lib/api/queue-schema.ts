import { z } from "zod";

import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { QueuePage } from "@/lib/types/listingkit";

export const queueSummarySchema = z
  .object({
    total_items: z.coerce.number().int().nonnegative(),
    ready_items: z.coerce.number().int().nonnegative(),
    fallback_items: z.coerce.number().int().nonnegative(),
    missing_items: z.coerce.number().int().nonnegative(),
    queued_items: z.coerce.number().int().nonnegative(),
    running_items: z.coerce.number().int().nonnegative(),
    completed_items: z.coerce.number().int().nonnegative(),
    failed_items: z.coerce.number().int().nonnegative(),
    retryable_items: z.coerce.number().int().nonnegative(),
    previewable_items: z.coerce.number().int().nonnegative(),
    preview_capability_counts: z.record(z.string(), z.coerce.number()).optional(),
    quality_grade_counts: z.record(z.string(), z.coerce.number()).optional(),
    approved_sections: z.coerce.number().int().nonnegative(),
    deferred_sections: z.coerce.number().int().nonnegative(),
    review_pending_sections: z.coerce.number().int().nonnegative(),
  })
  .passthrough();

const queueItemSchema = z
  .object({
    task_id: z.string().optional(),
    generation_task: z.string().optional(),
    platform: z.string().optional(),
    slot: z.string().optional(),
    purpose: z.string().optional(),
    state: z.string().optional(),
    retryable: z.boolean().optional(),
    retry_hint: z.string().optional(),
    quality_grade: z.string().optional(),
    quality_grade_label: z.string().optional(),
    execution_quality: z.string().optional(),
    render_preview_available: z.boolean().optional(),
    preview_capabilities: z.array(z.string()).optional(),
    review_decision: z.string().optional(),
    review_status: z.string().optional(),
    review_blocked: z.boolean().optional(),
    selected_asset_id: z.string().optional(),
  })
  .passthrough();

const queuePageSchema = z
  .object({
    task_id: z.string(),
    delta_token: z.string().optional(),
    not_modified: z.boolean().optional(),
    summary: queueSummarySchema.optional(),
    page: z.coerce.number().int().nonnegative(),
    page_size: z.coerce.number().int().positive(),
    total: z.coerce.number().int().nonnegative(),
    items: z.array(queueItemSchema).optional(),
  })
  .passthrough();

export function parseQueueResponse(payload: unknown): QueuePage {
  return parseApiResponseShape(
    payload,
    queuePageSchema,
    "ListingKit API returned an unexpected generation queue response",
  );
}
