import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import type {
  SheinStudioBatchDetail,
  SheinStudioBatchItem,
  SheinStudioBatchItemStatus,
  SheinStudioBatchRecord,
  SheinStudioBatchStatus,
  SheinStudioItemizedBatchItem,
  SheinStudioMaterializedDesign,
} from "@/lib/types/shein-studio";
import { z } from "zod";

const studioBatchStatusSchema = z.enum([
  "draft",
  "generating",
  "partially_materialized",
  "review_ready",
  "partially_failed",
  "failed",
  "tasks_created",
]);

const studioBatchItemStatusSchema = z.enum([
  "pending",
  "generating",
  "awaiting_materialization",
  "review_ready",
  "failed",
]);

const studioBatchSchema = z
  .object({
    id: z.string(),
    status: studioBatchStatusSchema,
    prompt: z.string().optional(),
    style_count: z.string().optional(),
    shein_store_id: z.number().optional(),
    created_at: z.string(),
    updated_at: z.string(),
  })
  .passthrough();

const studioMaterializedDesignReviewStatusSchema = z.enum([
  "unreviewed",
  "approved",
  "rejected",
]);

const studioBatchItemSchema = z
  .object({
    id: z.string(),
    batch_id: z.string(),
    target_group_key: z.string(),
    target_group_label: z.string().optional(),
    status: studioBatchItemStatusSchema,
    selection_count: z.number().optional(),
    last_error: z.string().optional(),
    created_at: z.string(),
    updated_at: z.string(),
  })
  .passthrough();

const studioMaterializedDesignSchema = z
  .object({
    id: z.string(),
    batch_id: z.string(),
    item_id: z.string(),
    source_attempt_id: z.string(),
    target_group_key: z.string(),
    target_group_label: z.string().optional(),
    image_url: z.string(),
    review_status: studioMaterializedDesignReviewStatusSchema,
    review_note: z.string().optional(),
    role: z.string().optional(),
    role_label: z.string().optional(),
    product_image_urls: z.array(z.string()).optional(),
    created_at: z.string(),
    updated_at: z.string(),
  })
  .passthrough();

const studioBatchDetailItemSchema = z
  .object({
    item: studioBatchItemSchema,
    designs: z.array(studioMaterializedDesignSchema).optional(),
  })
  .passthrough();

const studioBatchDetailResponseSchema = z
  .object({
    batch: studioBatchSchema,
    items: z.array(studioBatchDetailItemSchema).optional(),
  })
  .passthrough();

function mapStudioBatch(
  payload: z.infer<typeof studioBatchSchema>,
): SheinStudioBatchRecord {
  return {
    id: payload.id,
    status: payload.status as SheinStudioBatchStatus,
    prompt: payload.prompt ?? "",
    styleCount: payload.style_count ?? "1",
    sheinStoreId: payload.shein_store_id ?? "",
    createdAt: payload.created_at,
    updatedAt: payload.updated_at,
  };
}

function mapStudioBatchItem(
  payload: z.infer<typeof studioBatchItemSchema>,
): SheinStudioBatchItem {
  return {
    id: payload.id,
    batchId: payload.batch_id,
    targetGroupKey: payload.target_group_key,
    targetGroupLabel: payload.target_group_label,
    status: payload.status as SheinStudioBatchItemStatus,
    selectionCount: payload.selection_count ?? 0,
    lastError: payload.last_error,
    createdAt: payload.created_at,
    updatedAt: payload.updated_at,
  };
}

function mapStudioMaterializedDesign(
  payload: z.infer<typeof studioMaterializedDesignSchema>,
): SheinStudioMaterializedDesign {
  return {
    id: payload.id,
    batchId: payload.batch_id,
    itemId: payload.item_id,
    sourceAttemptId: payload.source_attempt_id,
    targetGroupKey: payload.target_group_key,
    targetGroupLabel: payload.target_group_label,
    imageUrl: payload.image_url,
    reviewStatus: payload.review_status,
    reviewNote: payload.review_note,
    role: payload.role,
    roleLabel: payload.role_label,
    productImageUrls: payload.product_image_urls,
    createdAt: payload.created_at,
    updatedAt: payload.updated_at,
  };
}

function mapStudioBatchDetailItem(
  payload: z.infer<typeof studioBatchDetailItemSchema>,
): SheinStudioItemizedBatchItem {
  return {
    item: mapStudioBatchItem(payload.item),
    designs: (payload.designs ?? []).map(mapStudioMaterializedDesign),
  };
}

export function parseSheinStudioBatchDetailResponse(
  payload: unknown,
): SheinStudioBatchDetail {
  const parsed = parseApiResponseShape(
    payload,
    studioBatchDetailResponseSchema,
    "ListingKit API returned an unexpected studio batch detail response",
  );

  return {
    batch: mapStudioBatch(parsed.batch),
    items: (parsed.items ?? []).map(mapStudioBatchDetailItem),
  };
}

export async function getSheinStudioBatchDetail(
  batchId: string,
): Promise<SheinStudioBatchDetail> {
  const payload = await apiRequest<unknown>(`/studio/batches/${batchId}`);
  return parseSheinStudioBatchDetailResponse(payload);
}

export async function approveSheinStudioBatchDesigns(
  batchId: string,
  designIds: string[],
): Promise<SheinStudioBatchDetail> {
  const payload = await apiRequest<unknown>(
    `/studio/batches/${batchId}/design-approvals`,
    {
      method: "POST",
      body: { design_ids: designIds },
    },
  );
  return parseSheinStudioBatchDetailResponse(payload);
}
