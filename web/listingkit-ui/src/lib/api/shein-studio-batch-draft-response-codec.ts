import { z } from "zod";

import { parseApiResponseShape } from "@/lib/api/response-schema";
import type {
  SheinStudioArtworkModel,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioTransparencyMode,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";

export type StudioBatchDraftStatus =
  | "selecting"
  | "generating"
  | "generated"
  | "reviewing"
  | "failed"
  | "tasks_created";

type RawCreatedTask = {
  id?: string;
  title?: string;
  designId?: string;
  design_id?: string;
};

export type StudioBatchDraftRecordResponse = {
  id: string;
  tenant_id?: string;
  batch_name?: string;
  status?: StudioBatchDraftStatus;
  selection?: Record<string, unknown>;
  prompt?: string;
  prompt_mode?: "managed" | "raw";
  style_count?: string;
  hot_style_reference_image_urls?: string[];
  hot_style_reference_brief?: string;
  hot_style_reference_prompt?: string;
  variation_intensity?: SheinStudioVariationIntensity;
  product_image_count?: string;
  product_image_prompt?: string;
  product_image_prompts?: SheinStudioProductImagePrompt[];
  artwork_model?: SheinStudioArtworkModel;
  image_strategy?: SheinStudioImageStrategy;
  grouped_image_mode?: SheinStudioGroupedImageMode;
  selected_sds_images?: SheinStudioSelectedSDSImage[];
  groups?: Array<Record<string, unknown>>;
  grouped_selections?: Array<Record<string, unknown>>;
  transparent_background?: boolean;
  transparent_background_mode?: SheinStudioTransparencyMode;
  original_image_url?: string;
  background_removal_status?: SheinStudioGeneratedDesign["backgroundRemovalStatus"];
  background_removal_model?: string;
  background_removal_error?: string;
  render_size_images_with_sds?: boolean;
  shein_store_id?: string;
  legacy_compatibility_snapshot?: Record<string, unknown>;
  generation_job_id?: string;
  generation_jobs?: Array<{
    job_id?: string;
    target_group_key?: string;
    target_group_label?: string;
    status?: "running" | "succeeded" | "failed";
  }>;
  generation_error?: string;
  approved_design_ids?: string[];
  created_tasks?: RawCreatedTask[];
  updated_at?: string;
};

export type StudioBatchDraftDesignResponse = {
  id: string;
  tenant_id?: string;
  image_url?: string;
  prompt?: string;
  revised_prompt?: string;
  image_model?: string;
  transparent_background?: boolean;
  transparent_background_mode?: SheinStudioTransparencyMode;
  original_image_url?: string;
  background_removal_status?: SheinStudioGeneratedDesign["backgroundRemovalStatus"];
  background_removal_model?: string;
  background_removal_error?: string;
  variation_intensity?: SheinStudioVariationIntensity;
  review_note?: string;
  role?: string;
  role_label?: string;
  target_group_key?: string;
  target_group_label?: string;
  product_image_urls?: string[];
  approved?: boolean;
};

export type StudioBatchDraftDetailResponse = {
  batch?: StudioBatchDraftRecordResponse;
  designs?: StudioBatchDraftDesignResponse[];
};

export type StudioBatchListItemResponse = {
  id: string;
  tenant_id?: string;
  batch_name?: string;
  status?: StudioBatchDraftStatus;
  prompt?: string;
  prompt_mode?: "managed" | "raw";
  style_count?: string;
  hot_style_reference_image_urls?: string[];
  hot_style_reference_brief?: string;
  hot_style_reference_prompt?: string;
  variation_intensity?: SheinStudioVariationIntensity;
  product_image_count?: string;
  product_image_prompt?: string;
  product_image_prompts?: SheinStudioProductImagePrompt[];
  artwork_model?: SheinStudioArtworkModel;
  image_strategy?: SheinStudioImageStrategy;
  grouped_image_mode?: SheinStudioGroupedImageMode;
  transparent_background?: boolean;
  transparent_background_mode?: SheinStudioTransparencyMode;
  render_size_images_with_sds?: boolean;
  shein_store_id?: string;
  selection?: Record<string, unknown>;
  groups?: Array<Record<string, unknown>>;
  grouped_selections?: Array<Record<string, unknown>>;
  legacy_compatibility_snapshot?: Record<string, unknown>;
  approved_design_ids?: string[];
  created_tasks?: RawCreatedTask[];
  design_count?: number;
  updated_at?: string;
};

export type StudioBatchListResponse = {
  items?: StudioBatchListItemResponse[];
};

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

const generationJobSchema = z
  .object({
    job_id: z.string().optional(),
    target_group_key: z.string().optional(),
    target_group_label: z.string().optional(),
    status: z.enum(["running", "succeeded", "failed"]).optional(),
  })
  .passthrough();

const studioBatchDraftRecordSchema = z
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
    transparent_background_mode: z.enum(["none", "native", "removal"]).optional(),
    render_size_images_with_sds: z.boolean().optional(),
    shein_store_id: z.string().optional(),
    generation_job_id: z.string().optional(),
    generation_jobs: z.array(generationJobSchema).optional(),
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
    transparent_background_mode: z.enum(["none", "native", "removal"]).optional(),
    original_image_url: z.string().optional(),
    background_removal_status: z
      .enum(["not_requested", "pending", "succeeded", "failed"])
      .optional(),
    background_removal_model: z.string().optional(),
    background_removal_error: z.string().optional(),
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

const studioBatchDraftDetailSchema = z
  .object({
    batch: studioBatchDraftRecordSchema.optional(),
    designs: z.array(studioDesignSchema).optional(),
  })
  .passthrough();

export function parseStudioBatchDraftDetailResponse(
  payload: unknown,
): StudioBatchDraftDetailResponse {
  return parseApiResponseShape(
    payload,
    studioBatchDraftDetailSchema,
    "ListingKit API returned an unexpected studio batch draft response",
  ) as StudioBatchDraftDetailResponse;
}
