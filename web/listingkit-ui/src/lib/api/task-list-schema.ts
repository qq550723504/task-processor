import { z } from "zod";

import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { ListingKitTaskListPage } from "@/lib/types/listingkit";

const taskListItemSchema = z
  .object({
    task_id: z.string(),
    tenant_id: z.string().optional(),
    status: z.string().optional(),
    platforms: z.array(z.string()).optional(),
    title: z.string().optional(),
    image_count: z.coerce.number().optional(),
    product_name: z.string().optional(),
    variant_label: z.string().optional(),
    sds_sync_status: z.string().optional(),
    shein_workflow_status: z.string().optional(),
    shein_blocking_keys: z.array(z.string()).optional(),
    shein_warning_keys: z.array(z.string()).optional(),
    shein_work_queue: z.string().optional(),
    shein_action_queue: z.string().optional(),
    shein_latest_submission_status: z.string().optional(),
    shein_latest_submission_error: z.string().optional(),
    shein_submission_remote_status: z.string().optional(),
    shein_submission_remote_checked_at: z.string().optional(),
    shein_submission_remote_record_id: z.string().optional(),
    error: z.string().optional(),
    created_at: z.string().optional(),
    updated_at: z.string().optional(),
    completed_at: z.string().optional(),
  })
  .passthrough();

const taskListPageSchema = z
  .object({
    page: z.coerce.number().int().nonnegative(),
    page_size: z.coerce.number().int().positive(),
    total: z.coerce.number().int().nonnegative(),
    summary: z.record(z.string(), z.unknown()).optional(),
    taxonomy: z.record(z.string(), z.unknown()).optional(),
    items: z.array(taskListItemSchema).optional(),
  })
  .passthrough();

export function parseTaskListResponse(payload: unknown): ListingKitTaskListPage {
  return parseApiResponseShape(
    payload,
    taskListPageSchema,
    "ListingKit API returned an unexpected task list response",
  );
}
