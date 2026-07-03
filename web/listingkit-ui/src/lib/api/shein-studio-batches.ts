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
  SheinStudioRejectedTask,
  SheinStudioTaskLifecycleStatus,
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
    itemId: z.string().optional(),
    item_id: z.string().optional(),
    selectionId: z.string().optional(),
    selection_id: z.string().optional(),
    compatibilityFingerprint: z.string().optional(),
    compatibility_fingerprint: z.string().optional(),
    status: z.string().optional(),
    submissionState: z.string().optional(),
    submission_state: z.string().optional(),
    lastSubmissionAction: z.string().optional(),
    last_submission_action: z.string().optional(),
    source: z.string().optional(),
    reasonCode: z.string().optional(),
    reason_code: z.string().optional(),
    message: z.string().optional(),
  })
  .passthrough();

const studioFailedTaskSchema = z
  .object({
    designId: z.string().optional(),
    design_id: z.string().optional(),
    itemId: z.string().optional(),
    item_id: z.string().optional(),
    selectionId: z.string().optional(),
    selection_id: z.string().optional(),
    compatibilityFingerprint: z.string().optional(),
    compatibility_fingerprint: z.string().optional(),
    title: z.string().optional(),
    status: z.string().optional(),
    submissionState: z.string().optional(),
    submission_state: z.string().optional(),
    lastSubmissionAction: z.string().optional(),
    last_submission_action: z.string().optional(),
    source: z.string().optional(),
    reasonCode: z.string().optional(),
    reason_code: z.string().optional(),
    message: z.string().optional(),
  })
  .passthrough();

const studioRejectedTaskSchema = z
  .object({
    designId: z.string().optional(),
    design_id: z.string().optional(),
    itemId: z.string().optional(),
    item_id: z.string().optional(),
    selectionId: z.string().optional(),
    selection_id: z.string().optional(),
    compatibilityFingerprint: z.string().optional(),
    compatibility_fingerprint: z.string().optional(),
    title: z.string().optional(),
    status: z.string().optional(),
    submissionState: z.string().optional(),
    submission_state: z.string().optional(),
    lastSubmissionAction: z.string().optional(),
    last_submission_action: z.string().optional(),
    source: z.string().optional(),
    reasonCode: z.string().optional(),
    reason_code: z.string().optional(),
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
    reused_tasks: z.array(studioCreatedTaskSchema).optional(),
    rejected_tasks: z.array(studioRejectedTaskSchema).optional(),
    failed_tasks: z.array(studioFailedTaskSchema).optional(),
    status_groups: studioBatchStatusGroupsSchema.optional(),
  })
  .passthrough();

const studioBatchTaskCreationResponseSchema = z
  .object({
    batch: studioBatchSchema,
    items: z.array(studioBatchDetailItemSchema).optional(),
    created_tasks: z.array(studioCreatedTaskSchema).optional(),
    reused_tasks: z.array(studioCreatedTaskSchema).optional(),
    rejected_tasks: z.array(studioRejectedTaskSchema).optional(),
    failed_tasks: z.array(studioFailedTaskSchema).optional(),
    status_groups: studioBatchStatusGroupsSchema.optional(),
  })
  .passthrough();

export type SheinStudioBatchTaskCreationResult = SheinStudioBatchDetail & {
  createdTasks: SheinStudioCreatedTask[];
  reusedTasks: SheinStudioCreatedTask[];
  rejectedTasks: SheinStudioRejectedTask[];
  failedTasks: SheinStudioFailedTask[];
};

function mapStudioBatch(
  payload: z.infer<typeof studioBatchSchema>,
): SheinStudioBatchRecord {
  const selection = normalizeSelectionResponse(
    payload.selection as Record<string, unknown> | undefined,
  );
  const batch: SheinStudioBatchRecord = {
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
  if (
    Object.prototype.hasOwnProperty.call(
      payload,
      "hot_style_reference_image_urls",
    )
  ) {
    batch.hotStyleReferenceImageUrls = Array.isArray(
      payload.hot_style_reference_image_urls,
    )
      ? payload.hot_style_reference_image_urls.filter(
          (item): item is string => typeof item === "string",
        )
      : [];
  }
  if (
    Object.prototype.hasOwnProperty.call(payload, "hot_style_reference_brief")
  ) {
    batch.hotStyleReferenceBrief =
      typeof payload.hot_style_reference_brief === "string"
        ? payload.hot_style_reference_brief
        : "";
  }
  if (
    Object.prototype.hasOwnProperty.call(payload, "hot_style_reference_prompt")
  ) {
    batch.hotStyleReferencePrompt =
      typeof payload.hot_style_reference_prompt === "string"
        ? payload.hot_style_reference_prompt
        : "";
  }
  return batch;
}

type SheinStudioBatchRequestOptions = {
  allowPartialWhileGenerating?: boolean;
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

const taskLifecycleStatuses = new Set<SheinStudioTaskLifecycleStatus>([
  "task_created",
  "needs_review",
  "ready_to_submit",
  "draft_saved",
  "published",
  "submit_failed",
  "unknown",
]);

function normalizeTaskLifecycleStatus(value: string | undefined) {
  const trimmed = value?.trim();
  if (!trimmed) {
    return undefined;
  }
  return taskLifecycleStatuses.has(trimmed as SheinStudioTaskLifecycleStatus)
    ? (trimmed as SheinStudioTaskLifecycleStatus)
    : trimmed;
}

function firstTrimmed(...values: Array<string | undefined>) {
  for (const value of values) {
    const trimmed = value?.trim();
    if (trimmed) {
      return trimmed;
    }
  }
  return undefined;
}

function mapStudioCreatedTasks(
  payload: z.infer<typeof studioCreatedTaskSchema>[],
  outcome: "created" | "reused" = "created",
): SheinStudioCreatedTask[] {
  return payload
    .map((item): SheinStudioCreatedTask | null => {
      const id = item.id?.trim();
      const title = item.title?.trim();
      if (!id || !title) {
        return null;
      }
      return {
        id,
        title,
        designId: firstTrimmed(item.designId, item.design_id) ?? "",
        itemId: firstTrimmed(item.itemId, item.item_id),
        selectionId: firstTrimmed(item.selectionId, item.selection_id),
        compatibilityFingerprint: firstTrimmed(
          item.compatibilityFingerprint,
          item.compatibility_fingerprint,
        ),
        status: normalizeTaskLifecycleStatus(item.status) ?? "task_created",
        submissionState: normalizeTaskLifecycleStatus(
          firstTrimmed(item.submissionState, item.submission_state),
        ),
        lastSubmissionAction: firstTrimmed(
          item.lastSubmissionAction,
          item.last_submission_action,
        ),
        source: firstTrimmed(item.source),
        reasonCode: firstTrimmed(item.reasonCode, item.reason_code),
        message: firstTrimmed(item.message),
        outcome,
      } satisfies SheinStudioCreatedTask;
    })
    .filter((item): item is SheinStudioCreatedTask => Boolean(item));
}

function mapStudioRejectedTasks(
  payload: z.infer<typeof studioRejectedTaskSchema>[],
): SheinStudioRejectedTask[] {
  return payload
    .map((item): SheinStudioRejectedTask | null => {
      const designId = firstTrimmed(item.designId, item.design_id);
      const message = firstTrimmed(item.message);
      const reasonCode = firstTrimmed(item.reasonCode, item.reason_code);
      if (!designId || (!message && !reasonCode)) {
        return null;
      }
      return {
        designId,
        title: firstTrimmed(item.title),
        itemId: firstTrimmed(item.itemId, item.item_id),
        selectionId: firstTrimmed(item.selectionId, item.selection_id),
        compatibilityFingerprint: firstTrimmed(
          item.compatibilityFingerprint,
          item.compatibility_fingerprint,
        ),
        status: firstTrimmed(item.status) ?? "rejected",
        submissionState: normalizeTaskLifecycleStatus(
          firstTrimmed(item.submissionState, item.submission_state),
        ),
        lastSubmissionAction: firstTrimmed(
          item.lastSubmissionAction,
          item.last_submission_action,
        ),
        source: firstTrimmed(item.source),
        reasonCode,
        message,
        outcome: "rejected",
      } satisfies SheinStudioRejectedTask;
    })
    .filter((item): item is SheinStudioRejectedTask => Boolean(item));
}

function mapStudioFailedTasks(
  payload: z.infer<typeof studioFailedTaskSchema>[],
): SheinStudioFailedTask[] {
  return payload
    .map((item): SheinStudioFailedTask | null => {
      const designId = item.designId?.trim() || item.design_id?.trim() || "";
      const title = item.title?.trim() || "";
      const message = item.message?.trim() || "";
      if (!designId || !message) {
        return null;
      }
      return {
        designId,
        title: title || designId,
        itemId: firstTrimmed(item.itemId, item.item_id),
        selectionId: firstTrimmed(item.selectionId, item.selection_id),
        compatibilityFingerprint: firstTrimmed(
          item.compatibilityFingerprint,
          item.compatibility_fingerprint,
        ),
        status: firstTrimmed(item.status) ?? "failed",
        submissionState: normalizeTaskLifecycleStatus(
          firstTrimmed(item.submissionState, item.submission_state),
        ),
        lastSubmissionAction: firstTrimmed(
          item.lastSubmissionAction,
          item.last_submission_action,
        ),
        source: firstTrimmed(item.source),
        reasonCode: firstTrimmed(item.reasonCode, item.reason_code),
        message,
        outcome: "failed",
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
    reusedTasks: mapStudioCreatedTasks(parsed.reused_tasks ?? [], "reused"),
    rejectedTasks: mapStudioRejectedTasks(parsed.rejected_tasks ?? []),
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
    reusedTasks: mapStudioCreatedTasks(parsed.reused_tasks ?? [], "reused"),
    rejectedTasks: mapStudioRejectedTasks(parsed.rejected_tasks ?? []),
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
  const body: {
    allow_partial_while_generating?: boolean;
    design_ids: string[];
  } = { design_ids: designIds };
  if (options?.allowPartialWhileGenerating) {
    body.allow_partial_while_generating = true;
  }
  const payload = await apiRequest<unknown>(
    `/studio/batches/${batchId}/tasks`,
    {
      method: "POST",
      query: buildStudioBatchQuery(options),
      body,
    },
  );
  return parseSheinStudioBatchTaskCreationResponse(payload);
}
