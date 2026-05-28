import { z } from "zod";

import { parseApiResponseShape } from "@/lib/api/response-schema";
import type { StudioSessionDetailResponse } from "@/lib/api/shein-studio-sessions";

const productImagePromptSchema = z
  .object({
    role: z.string(),
    label: z.string(),
    prompt: z.string(),
  })
  .passthrough();

const createdTaskSchema = z
  .object({
    id: z.string(),
    title: z.string(),
    designId: z.string().optional(),
    design_id: z.string().optional(),
  })
  .passthrough();

const selectedSdsImageSchema = z
  .object({
    imageUrl: z.string(),
    variantSku: z.string().optional(),
    color: z.string().optional(),
  })
  .passthrough();

const groupedSelectionSchema = z
  .object({
    selection_id: z.string().optional(),
    selectionId: z.string().optional(),
    selection: z.record(z.string(), z.unknown()),
    baseline_key: z.string().optional(),
    baselineKey: z.string().optional(),
    baseline_status: z.string().optional(),
    baselineStatus: z.string().optional(),
    baseline_reason: z.string().optional(),
    baselineReason: z.string().optional(),
    baseline_reason_code: z.string().optional(),
    baselineReasonCode: z.string().optional(),
    shein_store_id: z.string().optional(),
    sheinStoreId: z.string().optional(),
    eligible: z.boolean().optional(),
    eligibility_reason: z.string().optional(),
    eligibilityReason: z.string().optional(),
  })
  .passthrough();

const studioSessionSchema = z
  .object({
    id: z.string(),
    tenant_id: z.string().optional(),
    status: z.string().optional(),
    selection: z.record(z.string(), z.unknown()).optional(),
    prompt: z.string().optional(),
    style_count: z.string().optional(),
    variation_intensity: z.string().optional(),
    product_image_count: z.string().optional(),
    product_image_prompt: z.string().optional(),
    product_image_prompts: z.array(productImagePromptSchema).optional(),
    artwork_model: z.string().optional(),
    image_strategy: z.string().optional(),
    grouped_image_mode: z.string().optional(),
    selected_sds_images: z.array(selectedSdsImageSchema).optional(),
    grouped_selections: z.array(groupedSelectionSchema).optional(),
    transparent_background: z.boolean().optional(),
    render_size_images_with_sds: z.boolean().optional(),
    shein_store_id: z.string().optional(),
    generation_job_id: z.string().optional(),
    generation_error: z.string().optional(),
    approved_design_ids: z.array(z.string()).optional(),
    created_tasks: z.array(createdTaskSchema).optional(),
    updated_at: z.string().optional(),
  })
  .passthrough();

const studioDesignSchema = z
  .object({
    id: z.string(),
    tenant_id: z.string().optional(),
    image_url: z.string().optional(),
    prompt: z.string().optional(),
    revised_prompt: z.string().optional(),
    image_model: z.string().optional(),
    transparent_background: z.boolean().optional(),
    variation_intensity: z.string().optional(),
    review_note: z.string().optional(),
    role: z.string().optional(),
    role_label: z.string().optional(),
    target_group_key: z.string().optional(),
    target_group_label: z.string().optional(),
    product_image_urls: z.array(z.string()).optional(),
    approved: z.boolean().optional(),
  })
  .passthrough();

const studioSessionDetailSchema = z
  .object({
    session: studioSessionSchema.optional(),
    designs: z.array(studioDesignSchema).optional(),
  })
  .passthrough();

export function parseStudioSessionDetailResponse(
  payload: unknown,
): StudioSessionDetailResponse {
  return parseApiResponseShape(
    payload,
    studioSessionDetailSchema,
    "ListingKit API returned an unexpected studio session response",
  ) as StudioSessionDetailResponse;
}
