import { apiRequest } from "@/lib/api/client";
import { parseApiResponseShape } from "@/lib/api/response-schema";
import {
  normalizeGroupedSelectionsResponse,
  normalizeSelectionResponse,
} from "@/lib/api/shein-studio-batch-drafts";
import { normalizeSelectedSDSImages } from "@/lib/shein-studio/sds-selectable-images";
import type {
  SheinStudioBatchDetail,
  SheinStudioBatchItem,
  SheinStudioBatchItemStatus,
  SheinStudioBatchRecord,
  SheinStudioBatchStatus,
  SheinStudioBatchStatusGroup,
  SheinStudioBatchStatusGroups,
  SheinStudioCreatedTask,
  SheinStudioFailedTask,
  SheinStudioItemizedBatchItem,
  SheinStudioMaterializedDesign,
} from "@/lib/types/shein-studio";
import type { QueueQuery } from "@/lib/types/listingkit";
import { z } from "zod";

const studioBatchStatusSchema = z.enum([
  "draft",
  "generating",
  "partially_materialized",
  "review_ready",
  "partially_failed",
  "failed",
  "tasks_creating",
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
    shein_store_id: z.union([z.number(), z.string()]).optional(),
    created_at: z.string(),
    draft_updated_at: z.string().optional(),
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

const studioCreatedTaskSchema = z
  .object({
    id: z.string().optional(),
    title: z.string().optional(),
    designId: z.string().optional(),
    design_id: z.string().optional(),
  })
  .passthrough();

const studioFailedTaskSchema = z
  .object({
    designId: z.string().optional(),
    design_id: z.string().optional(),
    title: z.string().optional(),
    message: z.string().optional(),
  })
  .passthrough();

const studioBatchStatusGroupSchema = z
  .object({
    key: z.string(),
    label: z.string(),
    count: z.number().optional(),
    ids: z.array(z.string()).optional(),
  })
  .passthrough();

const studioBatchStatusGroupsSchema = z
  .object({
    items: z.array(studioBatchStatusGroupSchema).optional(),
    by_key: z.record(z.string(), studioBatchStatusGroupSchema).optional(),
  })
  .passthrough();

const studioBatchDetailResponseSchema = z
  .object({
    batch: studioBatchSchema,
    items: z.array(studioBatchDetailItemSchema).optional(),
    created_tasks: z.array(studioCreatedTaskSchema).optional(),
    failed_tasks: z.array(studioFailedTaskSchema).optional(),
    status_groups: studioBatchStatusGroupsSchema.optional(),
  })
  .passthrough();

const studioBatchTaskCreationResponseSchema = z
  .object({
    batch: studioBatchSchema,
    items: z.array(studioBatchDetailItemSchema).optional(),
    created_tasks: z.array(studioCreatedTaskSchema).optional(),
    failed_tasks: z.array(studioFailedTaskSchema).optional(),
    status_groups: studioBatchStatusGroupsSchema.optional(),
  })
  .passthrough();

export type SheinStudioBatchTaskCreationResult = SheinStudioBatchDetail & {
  createdTasks: SheinStudioCreatedTask[];
  failedTasks: SheinStudioFailedTask[];
};

function mapStudioBatch(
  payload: z.infer<typeof studioBatchSchema>,
): SheinStudioBatchRecord {
  const selection = normalizeSelectionResponse(
    payload.selection as Record<string, unknown> | undefined,
  );
  return {
    id: payload.id,
    tenantId: typeof payload.tenant_id === "string" ? payload.tenant_id.trim() || undefined : undefined,
    status: payload.status as SheinStudioBatchStatus,
    prompt: payload.prompt ?? "",
    styleCount: payload.style_count ?? "1",
    sheinStoreId: Number(payload.shein_store_id ?? 0) || 0,
    variationIntensity:
      payload.variation_intensity === "light" ||
      payload.variation_intensity === "medium" ||
      payload.variation_intensity === "strong"
        ? payload.variation_intensity
        : undefined,
    artworkModel:
      typeof payload.artwork_model === "string" ? payload.artwork_model : undefined,
    transparentBackground:
      typeof payload.transparent_background === "boolean"
        ? payload.transparent_background
        : undefined,
    groupedImageMode:
      payload.grouped_image_mode === "per_product" ||
      payload.grouped_image_mode === "shared_by_size"
        ? payload.grouped_image_mode
        : undefined,
    selectedSdsImages: normalizeSelectedSDSImages(payload.selected_sds_images),
    selectionVariantId: selection?.variantId,
    selection,
    groupedSelections: normalizeGroupedSelectionsResponse(
      payload.grouped_selections as Array<Record<string, unknown>> | undefined,
      selection,
    ),
    createdAt: payload.created_at,
    draftUpdatedAt: payload.draft_updated_at,
    updatedAt: payload.updated_at,
  };
}

type SheinStudioBatchRequestOptions = {
  tenantId?: string;
};

function buildStudioBatchQuery(options?: SheinStudioBatchRequestOptions) {
  const tenantId = options?.tenantId?.trim();
  return tenantId ? ({ tenant_id: tenantId } as QueueQuery) : undefined;
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

function mapStudioCreatedTasks(
  payload: z.infer<typeof studioCreatedTaskSchema>[],
): SheinStudioCreatedTask[] {
  return payload
    .map((item) => {
      const id = item.id?.trim();
      const title = item.title?.trim();
      if (!id || !title) {
        return null;
      }
      return {
        id,
        title,
        designId: item.designId?.trim() || item.design_id?.trim() || "",
      } satisfies SheinStudioCreatedTask;
    })
    .filter((item): item is SheinStudioCreatedTask => Boolean(item));
}

function mapStudioFailedTasks(
  payload: z.infer<typeof studioFailedTaskSchema>[],
): SheinStudioFailedTask[] {
  return payload
    .map((item) => {
      const designId = item.designId?.trim() || item.design_id?.trim() || "";
      const title = item.title?.trim() || "";
      const message = item.message?.trim() || "";
      if (!designId || !title || !message) {
        return null;
      }
      return {
        designId,
        title,
        message,
      } satisfies SheinStudioFailedTask;
    })
    .filter((item): item is SheinStudioFailedTask => Boolean(item));
}

function mapStudioBatchStatusGroup(
  payload: z.infer<typeof studioBatchStatusGroupSchema>,
): SheinStudioBatchStatusGroup {
  return {
    key: payload.key,
    label: payload.label,
    count: payload.count ?? 0,
    ids: payload.ids,
  };
}

function mapStudioBatchStatusGroups(
  payload: z.infer<typeof studioBatchStatusGroupsSchema> | undefined,
): SheinStudioBatchStatusGroups | undefined {
  if (!payload) {
    return undefined;
  }
  return {
    items: (payload.items ?? []).map(mapStudioBatchStatusGroup),
    byKey: Object.fromEntries(
      Object.entries(payload.by_key ?? {}).map(([key, group]) => [
        key,
        mapStudioBatchStatusGroup(group),
      ]),
    ),
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
    createdTasks: mapStudioCreatedTasks(parsed.created_tasks ?? []),
    failedTasks: mapStudioFailedTasks(parsed.failed_tasks ?? []),
    statusGroups: mapStudioBatchStatusGroups(parsed.status_groups),
  };
}

export function parseSheinStudioBatchTaskCreationResponse(
  payload: unknown,
): SheinStudioBatchTaskCreationResult {
  const parsed = parseApiResponseShape(
    payload,
    studioBatchTaskCreationResponseSchema,
    "ListingKit API returned an unexpected studio batch task creation response",
  );

  return {
    batch: mapStudioBatch(parsed.batch),
    items: (parsed.items ?? []).map(mapStudioBatchDetailItem),
    createdTasks: mapStudioCreatedTasks(parsed.created_tasks ?? []),
    failedTasks: mapStudioFailedTasks(parsed.failed_tasks ?? []),
    statusGroups: mapStudioBatchStatusGroups(parsed.status_groups),
  };
}

export async function getSheinStudioBatchDetail(
  batchId: string,
  options?: SheinStudioBatchRequestOptions,
): Promise<SheinStudioBatchDetail> {
  const payload = await apiRequest<unknown>(`/studio/batches/${batchId}`, {
    query: buildStudioBatchQuery(options),
  });
  return parseSheinStudioBatchDetailResponse(payload);
}

export async function approveSheinStudioBatchDesigns(
  batchId: string,
  designIds: string[],
  options?: SheinStudioBatchRequestOptions,
): Promise<SheinStudioBatchDetail> {
  const payload = await apiRequest<unknown>(
    `/studio/batches/${batchId}/design-approvals`,
    {
      method: "POST",
      query: buildStudioBatchQuery(options),
      body: { design_ids: designIds },
    },
  );
  return parseSheinStudioBatchDetailResponse(payload);
}

export async function generateSheinStudioBatch(
  batchId: string,
  options?: SheinStudioBatchRequestOptions,
): Promise<SheinStudioBatchDetail> {
  const payload = await apiRequest<unknown>(
    `/studio/batches/${batchId}/generate`,
    {
      method: "POST",
      query: buildStudioBatchQuery(options),
      body: {},
    },
  );
  return parseSheinStudioBatchDetailResponse(payload);
}

export async function retrySheinStudioBatchItems(
  batchId: string,
  itemIds: string[],
  options?: SheinStudioBatchRequestOptions,
): Promise<SheinStudioBatchDetail> {
  const payload = await apiRequest<unknown>(
    `/studio/batches/${batchId}/items/retry`,
    {
      method: "POST",
      query: buildStudioBatchQuery(options),
      body: { item_ids: itemIds },
    },
  );
  return parseSheinStudioBatchDetailResponse(payload);
}

export async function createSheinStudioBatchTasks(
  batchId: string,
  designIds: string[],
  options?: SheinStudioBatchRequestOptions,
): Promise<SheinStudioBatchTaskCreationResult> {
  const payload = await apiRequest<unknown>(
    `/studio/batches/${batchId}/tasks`,
    {
      method: "POST",
      query: buildStudioBatchQuery(options),
      body: { design_ids: designIds },
    },
  );
  return parseSheinStudioBatchTaskCreationResponse(payload);
}
