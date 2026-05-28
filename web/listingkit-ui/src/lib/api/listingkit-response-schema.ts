import { z } from "zod";

import { parseApiResponseShape } from "@/lib/api/response-schema";
import type {
  ListingKitPreview,
  ListingKitTaskResult,
  NavigationDispatchResponse,
  ReviewPreviewResponse,
} from "@/lib/types/listingkit";

const taskResultDataSchema = z
  .object({
    task_id: z.string().optional(),
    tenant_id: z.string().optional(),
    status: z.string().optional(),
    review_reasons: z.array(z.string()).optional(),
    pod_execution: z
      .object({
        provider: z.string().optional(),
        dependency_mode: z.string().optional(),
        status: z.string().optional(),
        failure_reason: z.string().optional(),
        fallback_type: z.string().optional(),
        decision_source: z.string().optional(),
        completed_at: z.string().optional(),
        last_attempt_at: z.string().optional(),
        retry_count: z.coerce.number().optional(),
        history: z
          .array(
            z
              .object({
                kind: z.string().optional(),
                code: z.string().optional(),
                message: z.string().optional(),
                detail: z.string().optional(),
                provider: z.string().optional(),
                dependency_mode: z.string().optional(),
                decision_source: z.string().optional(),
                from_status: z.string().optional(),
                to_status: z.string().optional(),
                occurred_at: z.string().optional(),
              })
              .passthrough(),
          )
          .optional(),
      })
      .passthrough()
      .optional(),
    platforms: z.array(z.string()).optional(),
    country: z.string().optional(),
    language: z.string().optional(),
    child_tasks: z.array(z.record(z.string(), z.unknown())).optional(),
    workflow_stages: z.array(z.record(z.string(), z.unknown())).optional(),
    workflow_issues: z.array(z.record(z.string(), z.unknown())).optional(),
  })
  .passthrough();

const taskResultSchema = z
  .object({
    task_id: z.string().optional(),
    tenant_id: z.string().optional(),
    status: z.string().optional(),
    shein_workflow_status: z.string().optional(),
    shein_latest_submission_status: z.string().optional(),
    shein_latest_submission_error: z.string().optional(),
    shein_submission_remote_status: z.string().optional(),
    result: taskResultDataSchema.optional(),
    error: z.string().optional(),
    review_reasons: z.array(z.string()).optional(),
    created_at: z.string().optional(),
    completed_at: z.string().optional(),
  })
  .passthrough();

const previewSlotSchema = z.record(z.string(), z.unknown());

const previewSchema = z
  .object({
    task_id: z.string(),
    status: z.string(),
    selected_platform: z.string().optional(),
    platforms: z.array(z.string()).optional(),
    needs_review: z.boolean().optional(),
    overview: z.record(z.string(), z.unknown()).optional(),
    asset_generation_overview: z.record(z.string(), z.unknown()).optional(),
    asset_generation_queue: z
      .object({
        summary: z.record(z.string(), z.unknown()).optional(),
        items: z.array(z.record(z.string(), z.unknown())).optional(),
      })
      .passthrough()
      .optional(),
    asset_generation_tasks: z.array(z.unknown()).optional(),
    asset_render_previews: z.array(previewSlotSchema).optional(),
    platform_asset_render_previews: z.array(z.unknown()).optional(),
    amazon: z.record(z.string(), z.unknown()).optional(),
    shein: z.record(z.string(), z.unknown()).optional(),
    temu: z.record(z.string(), z.unknown()).optional(),
    walmart: z.record(z.string(), z.unknown()).optional(),
  })
  .passthrough();

const reviewPreviewSchema = z
  .object({
    task_id: z.string().optional(),
    delta_token: z.string().optional(),
    not_modified: z.boolean().optional(),
    conditional: z.record(z.string(), z.unknown()).optional(),
    resource_descriptors: z.array(z.record(z.string(), z.unknown())).optional(),
    recovery_summary: z.record(z.string(), z.unknown()).optional(),
    resolved_action_summary: z.record(z.string(), z.unknown()).optional(),
    preview: previewSlotSchema.optional(),
    scene_preset: z.record(z.string(), z.unknown()).optional(),
    review_target: z.record(z.string(), z.unknown()).optional(),
    toolbar: z.record(z.string(), z.unknown()).optional(),
    revision_status: z.string().optional(),
    revision_mismatch_reason: z.string().optional(),
  })
  .passthrough();

const dispatchResponseSchema = z
  .object({
    dispatch_kind: z.string().optional(),
    not_modified: z.boolean().optional(),
    conditional: z.record(z.string(), z.unknown()).optional(),
    resource_descriptors: z.array(z.record(z.string(), z.unknown())).optional(),
    recovery_summary: z.record(z.string(), z.unknown()).optional(),
    resolved_action_summary: z.record(z.string(), z.unknown()).optional(),
    queue: z.record(z.string(), z.unknown()).optional(),
    review_session: z.record(z.string(), z.unknown()).optional(),
    review_preview: reviewPreviewSchema.optional(),
    action: z.record(z.string(), z.unknown()).optional(),
    panel_update: z.record(z.string(), z.unknown()).optional(),
  })
  .passthrough();

export function parseTaskResultResponse(payload: unknown): ListingKitTaskResult {
  return parseApiResponseShape(
    payload,
    taskResultSchema,
    "ListingKit API returned an unexpected task result response",
  );
}

export function parsePreviewResponse(payload: unknown): ListingKitPreview {
  return parseApiResponseShape(
    payload,
    previewSchema,
    "ListingKit API returned an unexpected preview response",
  ) as ListingKitPreview;
}

export function parseReviewPreviewResponse(
  payload: unknown,
): ReviewPreviewResponse {
  return parseApiResponseShape(
    payload,
    reviewPreviewSchema,
    "ListingKit API returned an unexpected review preview response",
  );
}

export function parseDispatchResponse(
  payload: unknown,
): NavigationDispatchResponse {
  return parseApiResponseShape(
    payload,
    dispatchResponseSchema,
    "ListingKit API returned an unexpected dispatch response",
  ) as NavigationDispatchResponse;
}
