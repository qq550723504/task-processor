import { z } from "zod";

import { queueSummarySchema } from "@/lib/api/queue-schema";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { ReviewSessionResponse } from "@/lib/types/listingkit";

const reviewSectionSchema = z
  .object({
    capability: z.string().optional(),
    capability_label: z.string().optional(),
    section_key: z.string().optional(),
    title: z.string().optional(),
    description: z.string().optional(),
    empty_state: z.string().optional(),
    selected: z.boolean().optional(),
    item_count: z.coerce.number().int().nonnegative(),
    primary_action_key: z.string().optional(),
    workflow_state: z.string().optional(),
    workflow_message: z.string().optional(),
    review_decision: z.string().optional(),
    review_status: z.string().optional(),
  })
  .passthrough();

const reviewSessionSchema = z
  .object({
    selected_platform: z.string().optional(),
    selected_slot: z.string().optional(),
    focus_capability: z.string().optional(),
    focused_section_key: z.string().optional(),
    review_summary: z.record(z.string(), z.unknown()).optional(),
    queue: z
      .object({
        summary: queueSummarySchema.optional(),
        items: z.array(z.record(z.string(), z.unknown())).optional(),
      })
      .passthrough()
      .optional(),
    overview: z.record(z.string(), z.unknown()).optional(),
    platform_cards: z.array(z.record(z.string(), z.unknown())).optional(),
    slot_navigation: z.array(z.record(z.string(), z.unknown())).optional(),
    sections: z.array(reviewSectionSchema).optional(),
  })
  .passthrough();

const reviewSessionResponseSchema = z
  .object({
    task_id: z.string().optional(),
    delta_token: z.string().optional(),
    not_modified: z.boolean().optional(),
    response_mode: z.string().optional(),
    conditional: z.record(z.string(), z.unknown()).optional(),
    resource_descriptors: z.array(z.record(z.string(), z.unknown())).optional(),
    recovery_summary: z.record(z.string(), z.unknown()).optional(),
    resolved_action_summary: z.record(z.string(), z.unknown()).optional(),
    patch: z.record(z.string(), z.unknown()).optional(),
    session: reviewSessionSchema.optional(),
  })
  .passthrough();

export function parseReviewSessionResponse(payload: unknown): ReviewSessionResponse {
  return parseApiResponseShape(
    payload,
    reviewSessionResponseSchema,
    "ListingKit API returned an unexpected review session response",
  ) as ReviewSessionResponse;
}
