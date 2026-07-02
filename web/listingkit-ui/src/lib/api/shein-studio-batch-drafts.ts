import { apiRequest } from "@/lib/api/client";
import { parseStudioBatchDraftDetailResponse } from "@/lib/api/shein-studio-batch-draft-schema";
import { normalizeDraft } from "@/lib/shein-studio/storage-shared";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import {
  buildGroupedSDSSelectionID,
  removePrimarySelectionFromGroupedSelections,
  type GroupedSDSSelectionEligibility,
  normalizeSDSBaselineStatus,
} from "@/lib/types/sds-baseline";
import type {
  SDSGroupedPromptHistoryEntry,
  SheinStudioArtworkModel,
  SheinStudioLegacyCompatibilitySnapshot,
  SheinStudioPersistedBatchView,
  SheinStudioPersistedGroupedWorkspace,
  SheinStudioSavedBatch,
  SheinStudioCreatedTask,
  SheinStudioDraft,
  SheinStudioGenerationJob,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedWorkspace,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";
import { normalizeSelectedSDSImages } from "@/lib/shein-studio/sds-selectable-images";

export type StudioBatchDraftStatus =
  | "selecting"
  | "generating"
  | "generated"
  | "reviewing"
  | "failed"
  | "tasks_created";

type StudioBatchDraftRecordResponse = {
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

type StudioBatchDraftDesignResponse = {
  id: string;
  tenant_id?: string;
  image_url?: string;
  prompt?: string;
  revised_prompt?: string;
  image_model?: string;
  transparent_background?: boolean;
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

type RawCreatedTask = {
  id?: string;
  title?: string;
  designId?: string;
  design_id?: string;
};

type RawStudioGenerationJob = StudioBatchDraftRecordResponse["generation_jobs"];

type StudioBatchListResponse = {
  items?: Array<{
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
  }>;
};

const STUDIO_BATCH_DRAFT_TIMEOUT_MS = 60_000;

type StudioBatchDraftRequestOptions = {
  signal?: AbortSignal;
  timeoutMs?: number;
  limit?: number;
};

export function buildStudioBatchDraftSelectionKey(selection?: SDSProductVariantSelection) {
  if (!selection) {
    return "";
  }
  return JSON.stringify({
    productId: selection.productId,
    parentProductId: selection.parentProductId,
    variantId: selection.variantId,
    prototypeGroupId: selection.prototypeGroupId,
    layerId: selection.layerId,
    printableWidth: selection.printableWidth ?? null,
    printableHeight: selection.printableHeight ?? null,
    selectedVariantIds: selection.selectedVariantIds ?? [],
  });
}

export async function listSheinStudioBatchDrafts(options?: StudioBatchDraftRequestOptions) {
  const payload = await apiRequest<unknown>("/studio/batches", {
    query:
      typeof options?.limit === "number" && options.limit > 0
        ? { limit: options.limit }
        : undefined,
    signal: options?.signal,
    timeoutMs: options?.timeoutMs ?? STUDIO_BATCH_DRAFT_TIMEOUT_MS,
  });
  const response = payload as StudioBatchListResponse;
  return (response.items ?? []).map(mapStudioBatchListItemToBatch);
}

export type UpsertSheinStudioBatchDraftInput = Omit<
  SheinStudioPersistedBatchView,
  "updatedAt" | "sheinStoreId"
> & {
  id?: string;
  expectedUpdatedAt?: string;
  name?: string;
  sheinStoreId?: string;
};

export async function upsertSheinStudioBatchDraft(
  input: UpsertSheinStudioBatchDraftInput,
  options?: StudioBatchDraftRequestOptions,
) {
  const explicitBatchName = input.name?.trim() || undefined;
  const batchName =
    explicitBatchName ?? (input.id ? undefined : deriveBatchName(input.prompt));
  const detail = parseStudioBatchDraftDetailResponse(
    await apiRequest<unknown>("/studio/batches", {
      method: "POST",
      body: {
        id: input.id,
        expected_updated_at: input.expectedUpdatedAt,
        batch_name: batchName,
        prompt: input.prompt,
        prompt_mode: input.promptMode,
        style_count: input.styleCount,
        hot_style_reference_image_urls: input.hotStyleReferenceImageUrls,
        hot_style_reference_brief: input.hotStyleReferenceBrief,
        hot_style_reference_prompt: input.hotStyleReferencePrompt,
        variation_intensity: input.variationIntensity,
        product_image_count: input.productImageCount,
        product_image_prompt: input.productImagePrompt,
        product_image_prompts: input.productImagePrompts,
        artwork_model: input.artworkModel,
        image_strategy: input.imageStrategy,
        grouped_image_mode: input.groupedImageMode,
        selected_sds_images: input.selectedSdsImages,
        transparent_background: input.transparentBackground,
        render_size_images_with_sds: input.renderSizeImagesWithSds,
        shein_store_id: input.sheinStoreId,
        selection: input.selection ? selectionToPayload(input.selection) : undefined,
        legacy_compatibility_snapshot: legacyCompatibilitySnapshotToPayload(
          input.legacyCompatibilitySnapshot,
        ),
        groups: input.groups?.map(groupedWorkspaceToPayload),
        grouped_selections: input.groupedSelections?.map(groupedSelectionToPayload),
      },
      signal: options?.signal,
      timeoutMs: options?.timeoutMs ?? STUDIO_BATCH_DRAFT_TIMEOUT_MS,
    }),
  );
  return mapStudioBatchDraftDetailToBatch(detail);
}

export async function deleteSheinStudioBatchDraft(
  batchId: string,
  options?: StudioBatchDraftRequestOptions,
) {
  await apiRequest<unknown>(`/studio/batches/${batchId}`, {
    method: "DELETE",
    signal: options?.signal,
    timeoutMs: options?.timeoutMs ?? STUDIO_BATCH_DRAFT_TIMEOUT_MS,
  });
}

export function mapStudioBatchDraftDetailToDraft(
  detail: StudioBatchDraftDetailResponse | null | undefined,
): SheinStudioDraft | null {
  if (!detail?.batch) {
    return null;
  }
  const primarySelection = normalizeSelectionResponse(detail.batch.selection);
  const rawBatchLegacyCompatibilitySnapshot =
    detail.batch.legacy_compatibility_snapshot &&
    typeof detail.batch.legacy_compatibility_snapshot === "object"
      ? detail.batch.legacy_compatibility_snapshot
      : undefined;

  const selectedIds =
    detail.batch.approved_design_ids ??
    detail.designs
      ?.filter((design) => design.approved)
      .map((design) => design.id) ??
    [];

  const generationJobs = normalizeGenerationJobs(detail.batch.generation_jobs);
  const legacyCompatibilitySnapshot = normalizeLegacyCompatibilitySnapshotResponse({
    ...(rawBatchLegacyCompatibilitySnapshot ?? {}),
    approved_design_ids:
      selectedIds.length > 0
        ? selectedIds
        : rawBatchLegacyCompatibilitySnapshot?.approved_design_ids,
    created_tasks:
      detail.batch.created_tasks ?? rawBatchLegacyCompatibilitySnapshot?.created_tasks,
    generation_jobs:
      generationJobs.length > 0
        ? detail.batch.generation_jobs
        : detail.batch.generation_job_id
          ? [{ job_id: detail.batch.generation_job_id, status: "running" }]
          : rawBatchLegacyCompatibilitySnapshot?.generation_jobs,
    generation_error:
      detail.batch.generation_error ?? rawBatchLegacyCompatibilitySnapshot?.generation_error,
    generation_job_id:
      detail.batch.generation_job_id ?? rawBatchLegacyCompatibilitySnapshot?.generation_job_id,
    designs:
      detail.designs && detail.designs.length > 0
        ? detail.designs
        : rawBatchLegacyCompatibilitySnapshot?.designs,
  });
  const normalizedDesigns =
    detail.designs && detail.designs.length > 0
      ? detail.designs.map((design) => ({
          id: design.id,
          imageUrl: design.image_url,
          prompt: design.prompt ?? detail.batch?.prompt,
          revisedPrompt: design.revised_prompt,
          imageModel: design.image_model ?? detail.batch?.artwork_model,
          transparentBackground:
            design.transparent_background ?? detail.batch?.transparent_background,
          variationIntensity:
            design.variation_intensity ?? detail.batch?.variation_intensity,
          reviewNote: design.review_note,
          role: design.role,
          roleLabel: design.role_label,
          targetGroupKey: design.target_group_key,
          targetGroupLabel: design.target_group_label,
          productImageUrls: design.product_image_urls,
        }))
      : (legacyCompatibilitySnapshot?.designs ?? []);
  const normalizedSelectedIds =
    selectedIds.length > 0 ? selectedIds : (legacyCompatibilitySnapshot?.selectedIds ?? []);
  const normalizedGenerationJobs =
    generationJobs.length > 0
      ? generationJobs
      : detail.batch.generation_job_id
        ? [{ jobId: detail.batch.generation_job_id, status: "running" as const }]
        : (legacyCompatibilitySnapshot?.generationJobs ?? []);

  return normalizeDraft({
    prompt: detail.batch.prompt ?? "",
    promptMode: detail.batch.prompt_mode ?? "managed",
    styleCount: detail.batch.style_count ?? "1",
    hotStyleReferenceImageUrls: detail.batch.hot_style_reference_image_urls ?? [],
    hotStyleReferenceBrief: detail.batch.hot_style_reference_brief ?? "",
    hotStyleReferencePrompt: detail.batch.hot_style_reference_prompt ?? "",
    variationIntensity: detail.batch.variation_intensity ?? "medium",
    productImageCount: detail.batch.product_image_count ?? "5",
    productImagePrompt: detail.batch.product_image_prompt ?? "",
    productImagePrompts: detail.batch.product_image_prompts ?? [],
    artworkModel: detail.batch.artwork_model,
    transparentBackground: detail.batch.transparent_background ?? false,
    sheinStoreId: detail.batch.shein_store_id ?? "",
    imageStrategy: detail.batch.image_strategy,
    groupedImageMode: detail.batch.grouped_image_mode,
    selectedSdsImages: normalizeSelectedSDSImages(detail.batch.selected_sds_images),
    renderSizeImagesWithSds: detail.batch.render_size_images_with_sds ?? true,
    selectionVariantId: primarySelection?.variantId,
    selection: primarySelection,
    groups: normalizeGroupsResponse(detail.batch.groups),
    groupedSelections: normalizeGroupedSelectionsResponse(
      detail.batch.grouped_selections,
      primarySelection,
    ),
    designs: normalizedDesigns,
    selectedIds: normalizedSelectedIds,
    createdTasks: normalizeCreatedTasks(
      detail.batch.created_tasks ?? rawBatchLegacyCompatibilitySnapshot?.created_tasks,
      normalizedSelectedIds,
      normalizedDesigns,
    ),
    legacyCompatibilitySnapshot,
    generationError:
      detail.batch.generation_error ?? legacyCompatibilitySnapshot?.generationError ?? "",
    generationJobId:
      detail.batch.generation_job_id ?? legacyCompatibilitySnapshot?.generationJobId ?? "",
    generationJobs: normalizedGenerationJobs,
    batchStatus: detail.batch.status ?? "",
    draftUpdatedAt: detail.batch.updated_at ?? new Date().toISOString(),
    updatedAt: detail.batch.updated_at ?? new Date().toISOString(),
  });
}

export function mapStudioBatchDraftDetailToBatch(
  detail: StudioBatchDraftDetailResponse | null | undefined,
): SheinStudioSavedBatch | null {
  const draft = mapStudioBatchDraftDetailToDraft(detail);
  if (!draft || !detail?.batch?.id) {
    return null;
  }
  return {
    id: detail.batch.id,
    name: detail.batch.batch_name ?? deriveBatchName(detail.batch.prompt ?? draft.prompt),
    ...draft,
  };
}

export function normalizeSelectionResponse(
  selection: Record<string, unknown> | undefined,
): SDSProductVariantSelection | undefined {
  if (!selection) {
    return undefined;
  }

  const variants = Array.isArray(selection.variants)
    ? selection.variants
        .map((variant) => {
          if (!variant || typeof variant !== "object") {
            return null;
          }
          const item = variant as Record<string, unknown>;
          return {
            variantId: Number(item.variant_id ?? item.variantId ?? 0) || 0,
            variantSku: asString(item.variant_sku ?? item.variantSku),
            size: asString(item.size),
            color: asString(item.color),
            price: asNumber(item.price),
            weight: asNumber(item.weight),
            boxLength: asNumber(item.box_length ?? item.boxLength),
            boxWidth: asNumber(item.box_width ?? item.boxWidth),
            boxHeight: asNumber(item.box_height ?? item.boxHeight),
            productionCycle: asNumber(item.production_cycle ?? item.productionCycle),
            prototypeGroupId: asNumber(item.prototype_group_id ?? item.prototypeGroupID),
            layerId: asString(item.layer_id ?? item.layerId),
            templateImageUrl: asString(item.template_image_url ?? item.templateImageURL),
            maskImageUrl: asString(item.mask_image_url ?? item.maskImageURL),
            blankDesignUrl: asString(item.blank_design_url ?? item.blankDesignURL),
            mockupImageUrl: asString(item.mockup_image_url ?? item.mockupImageURL),
            mockupImageUrls: asStringArray(item.mockup_image_urls ?? item.mockupImageURLs),
            sizeReferenceImageUrls: asStringArray(
              item.size_reference_image_urls ?? item.sizeReferenceImageURLs,
            ),
          };
        })
        .filter((item): item is NonNullable<typeof item> => Boolean(item))
    : undefined;

  return {
    productId: Number(selection.product_id ?? selection.productId ?? 0) || 0,
    parentProductId:
      Number(selection.parent_product_id ?? selection.parentProductId ?? 0) || 0,
    variantId: Number(selection.variant_id ?? selection.variantId ?? 0) || 0,
    prototypeGroupId:
      Number(selection.prototype_group_id ?? selection.prototypeGroupId ?? 0) || 0,
    layerId: asString(selection.layer_id ?? selection.layerId) ?? "",
    productSize: asString(selection.product_size ?? selection.productSize),
    packagingSpecification: asString(
      selection.packaging_specification ?? selection.packagingSpecification,
    ),
    productName: asString(selection.product_name ?? selection.productName) ?? "",
    variantLabel: asString(selection.variant_label ?? selection.variantLabel) ?? "",
    printableWidth: asNumber(selection.printable_width ?? selection.printableWidth),
    printableHeight: asNumber(selection.printable_height ?? selection.printableHeight),
    templateImageUrl: asString(selection.template_image_url ?? selection.templateImageUrl),
    maskImageUrl: asString(selection.mask_image_url ?? selection.maskImageUrl),
    blankDesignUrl: asString(selection.blank_design_url ?? selection.blankDesignUrl),
    mockupImageUrl: asString(selection.mockup_image_url ?? selection.mockupImageUrl),
    mockupImageUrls: asStringArray(selection.mockup_image_urls ?? selection.mockupImageUrls),
    sizeReferenceImageUrls: asStringArray(
      selection.size_reference_image_urls ?? selection.sizeReferenceImageUrls,
    ),
    selectedVariantIds: asNumberArray(
      selection.selected_variant_ids ?? selection.selectedVariantIds,
    ),
    variants,
  };
}

function mapStudioBatchListItemToBatch(item: NonNullable<StudioBatchListResponse["items"]>[number]) {
  const primarySelection = normalizeSelectionResponse(item.selection);
  const legacyCompatibilitySnapshot = normalizeLegacyCompatibilitySnapshotResponse(
    item.legacy_compatibility_snapshot,
  );
  const normalizedSelectedIds =
    item.approved_design_ids ?? legacyCompatibilitySnapshot?.selectedIds ?? [];
  const normalizedDesigns = legacyCompatibilitySnapshot?.designs ?? [];
  return {
    id: item.id,
    tenantId: item.tenant_id?.trim() || undefined,
    name: item.batch_name ?? deriveBatchName(item.prompt ?? ""),
    prompt: item.prompt ?? "",
    promptMode: item.prompt_mode ?? "managed",
    styleCount: item.style_count ?? "1",
    hotStyleReferenceImageUrls: item.hot_style_reference_image_urls ?? [],
    hotStyleReferenceBrief: item.hot_style_reference_brief ?? "",
    hotStyleReferencePrompt: item.hot_style_reference_prompt ?? "",
    variationIntensity: item.variation_intensity ?? "medium",
    productImageCount: item.product_image_count ?? "5",
    productImagePrompt: item.product_image_prompt ?? "",
    productImagePrompts: item.product_image_prompts ?? [],
    artworkModel: item.artwork_model ?? "",
    transparentBackground: item.transparent_background ?? false,
    sheinStoreId: item.shein_store_id ?? "",
    imageStrategy: item.image_strategy,
    groupedImageMode: item.grouped_image_mode,
    selectedSdsImages: normalizeSelectedSDSImages(undefined),
    renderSizeImagesWithSds: item.render_size_images_with_sds ?? true,
    selectionVariantId: primarySelection?.variantId,
    selection: primarySelection,
    groups: normalizeGroupsResponse(item.groups),
    groupedSelections: normalizeGroupedSelectionsResponse(
      item.grouped_selections,
      primarySelection,
    ),
    designs: normalizedDesigns,
    persistedDesignCount: item.design_count ?? normalizedDesigns.length,
    selectedIds: normalizedSelectedIds,
    createdTasks: normalizeCreatedTasks(
      item.created_tasks ?? legacyCompatibilitySnapshot?.createdTasks,
      normalizedSelectedIds,
      normalizedDesigns,
    ),
    batchStatus: item.status,
    legacyCompatibilitySnapshot,
    generationError: legacyCompatibilitySnapshot?.generationError ?? "",
    generationJobId: legacyCompatibilitySnapshot?.generationJobId ?? "",
    generationJobs: legacyCompatibilitySnapshot?.generationJobs ?? [],
    draftUpdatedAt: item.updated_at ?? new Date().toISOString(),
    updatedAt: item.updated_at ?? new Date().toISOString(),
  } satisfies SheinStudioSavedBatch;
}

function deriveBatchName(prompt: string) {
  const trimmed = prompt.trim();
  if (!trimmed) {
    return "未命名批次";
  }
  return trimmed.length > 36 ? `${trimmed.slice(0, 36)}...` : trimmed;
}

function asString(value: unknown) {
  return typeof value === "string" && value.trim() ? value : undefined;
}

function normalizeCreatedTasks(
  input: unknown,
  fallbackDesignIds?: string[],
  fallbackDesigns?: Array<{ id: string } | undefined>,
): SheinStudioCreatedTask[] {
  if (!Array.isArray(input)) {
    return [];
  }

  return input
    .map((item, index) => {
      if (!item || typeof item !== "object") {
        return null;
      }
      const raw = item as RawCreatedTask;
      const id = asString(raw.id);
      const title = asString(raw.title);
      if (!id || !title) {
        return null;
      }
      return {
        id,
        title,
        designId:
          asString(raw.designId ?? raw.design_id) ??
          fallbackDesignIds?.[index] ??
          fallbackDesigns?.[index]?.id ??
          "",
      } satisfies SheinStudioCreatedTask;
    })
    .filter((item): item is SheinStudioCreatedTask => Boolean(item));
}

function normalizeGenerationJobs(
  input: RawStudioGenerationJob | undefined,
): SheinStudioGenerationJob[] {
  if (!Array.isArray(input)) {
    return [];
  }
  return input.reduce<SheinStudioGenerationJob[]>((jobs, job) => {
      const jobId = typeof job?.job_id === "string" ? job.job_id.trim() : "";
      if (!jobId) {
        return jobs;
      }
      jobs.push({
        jobId,
        targetGroupKey:
          typeof job.target_group_key === "string"
            ? job.target_group_key
            : undefined,
        targetGroupLabel:
          typeof job.target_group_label === "string"
            ? job.target_group_label
            : undefined,
        status:
          job.status === "succeeded" || job.status === "failed"
            ? job.status
            : "running",
      } satisfies SheinStudioGenerationJob);
      return jobs;
    }, []);
}

function asStringArray(value: unknown) {
  return Array.isArray(value)
    ? value.filter((item): item is string => typeof item === "string" && item.trim().length > 0)
    : undefined;
}

function asNumber(value: unknown) {
  return typeof value === "number" && Number.isFinite(value)
    ? value
    : typeof value === "string" && value.trim() && Number.isFinite(Number(value))
      ? Number(value)
      : undefined;
}

function asNumberArray(value: unknown) {
  return Array.isArray(value)
    ? value
        .map((item) => asNumber(item))
        .filter((item): item is number => typeof item === "number" && item > 0)
    : undefined;
}

function selectionToPayload(selection: SDSProductVariantSelection) {
  return {
    product_id: selection.productId,
    parent_product_id: selection.parentProductId,
    variant_id: selection.variantId,
    prototype_group_id: selection.prototypeGroupId,
    layer_id: selection.layerId,
    product_size: selection.productSize,
    packaging_specification: selection.packagingSpecification,
    product_name: selection.productName,
    variant_label: selection.variantLabel,
    printable_width: selection.printableWidth,
    printable_height: selection.printableHeight,
    template_image_url: selection.templateImageUrl,
    mask_image_url: selection.maskImageUrl,
    blank_design_url: selection.blankDesignUrl,
    mockup_image_url: selection.mockupImageUrl,
    mockup_image_urls: selection.mockupImageUrls,
    size_reference_image_urls: selection.sizeReferenceImageUrls,
    selected_variant_ids: selection.selectedVariantIds,
    variants: selection.variants?.map((variant) => ({
      variant_id: variant.variantId,
      variant_sku: variant.variantSku,
      size: variant.size,
      color: variant.color,
      price: variant.price,
      weight: variant.weight,
      box_length: variant.boxLength,
      box_width: variant.boxWidth,
      box_height: variant.boxHeight,
      production_cycle: variant.productionCycle,
      prototype_group_id: variant.prototypeGroupId,
      layer_id: variant.layerId,
      template_image_url: variant.templateImageUrl,
      mask_image_url: variant.maskImageUrl,
      blank_design_url: variant.blankDesignUrl,
      mockup_image_url: variant.mockupImageUrl,
      mockup_image_urls: variant.mockupImageUrls,
      size_reference_image_urls: variant.sizeReferenceImageUrls,
    })),
  };
}

function groupedSelectionToPayload(selection: GroupedSDSSelectionEligibility) {
  return {
    selection_id: selection.selectionId,
    selection: selectionToPayload(selection.selection),
    baseline_key: selection.baselineKey,
    baseline_status: selection.baselineStatus,
    baseline_reason: selection.baselineReason,
    baseline_reason_code: selection.baselineReasonCode,
    shein_store_id: selection.sheinStoreId,
    eligible: selection.eligible,
    eligibility_reason: selection.eligibilityReason,
  };
}

function groupedWorkspaceToPayload(group: SheinStudioPersistedGroupedWorkspace) {
  return {
    id: group.id,
    name: group.name,
    primary_selection: selectionToPayload(group.primarySelection),
    grouped_selections: group.groupedSelections.map(groupedSelectionToPayload),
    style_count: group.styleCount,
    shein_store_id: group.sheinStoreId,
    image_strategy: group.imageStrategy,
    grouped_image_mode: group.groupedImageMode,
    selected_sds_images: group.selectedSdsImages,
    render_size_images_with_sds: group.renderSizeImagesWithSds,
    current_prompt: group.currentPrompt,
    prompt_mode: group.promptMode,
    prompt_history: group.promptHistory.map((entry) => ({
      prompt: entry.prompt,
      grouped_image_mode: entry.groupedImageMode,
      created_at: entry.createdAt,
    })),
    product_image_count: group.productImageCount,
    product_image_prompt: group.productImagePrompt,
    product_image_prompts: group.productImagePrompts,
    artwork_model: group.artworkModel,
    transparent_background: group.transparentBackground,
    variation_intensity: group.variationIntensity,
    legacy_compatibility_snapshot: legacyCompatibilitySnapshotToPayload(
      group.legacyCompatibilitySnapshot,
    ),
    updated_at: group.updatedAt,
  };
}

function legacyCompatibilitySnapshotToPayload(
  snapshot: SheinStudioLegacyCompatibilitySnapshot | undefined,
) {
  if (!snapshot) {
    return undefined;
  }

  const hasDesigns = (snapshot.designs?.length ?? 0) > 0;
  const hasSelectedIds = (snapshot.selectedIds?.length ?? 0) > 0;
  const hasCreatedTasks = (snapshot.createdTasks?.length ?? 0) > 0;
  const hasGenerationJobs = (snapshot.generationJobs?.length ?? 0) > 0;
  if (
    !hasDesigns &&
    !hasSelectedIds &&
    !hasCreatedTasks &&
    !hasGenerationJobs &&
    !snapshot.generationError &&
    !snapshot.generationJobId
  ) {
    return undefined;
  }

  return {
    approved_design_ids: snapshot.selectedIds,
    created_tasks: snapshot.createdTasks,
    generation_jobs: snapshot.generationJobs?.map((job) => ({
      job_id: job.jobId,
      target_group_key: job.targetGroupKey,
      target_group_label: job.targetGroupLabel,
      status: job.status,
    })),
    generation_error: snapshot.generationError,
    generation_job_id: snapshot.generationJobId,
    designs: (snapshot.designs ?? []).map((design) => ({
      id: design.id,
      image_url: design.imageUrl ?? design.dataUrl,
      prompt: design.prompt,
      revised_prompt: design.revisedPrompt,
      image_model: design.imageModel,
      transparent_background: design.transparentBackground,
      variation_intensity: design.variationIntensity,
      review_note: design.reviewNote,
      role: design.role,
      role_label: design.roleLabel,
      target_group_key: design.targetGroupKey,
      target_group_label: design.targetGroupLabel,
      product_image_urls: design.productImageUrls,
    })),
  };
}

function normalizeLegacyCompatibilitySnapshotResponse(
  value: Record<string, unknown> | undefined,
): SheinStudioLegacyCompatibilitySnapshot | undefined {
  if (!value) {
    return undefined;
  }

  const selectedIds = Array.isArray(value.approved_design_ids)
    ? (value.approved_design_ids as unknown[]).filter(
        (item): item is string => typeof item === "string",
      )
    : [];
  const designs = Array.isArray(value.designs)
    ? (value.designs as Array<Record<string, unknown>>)
        .map((design) => normalizeDesignResponse(design))
        .filter((design): design is NonNullable<typeof design> => Boolean(design))
    : [];
  const createdTasks = normalizeCreatedTasks(
    Array.isArray(value.created_tasks) ? value.created_tasks : undefined,
    selectedIds,
    designs,
  );
  const generationJobs = normalizeGenerationJobs(
    Array.isArray(value.generation_jobs) ? value.generation_jobs : undefined,
  );
  const generationError =
    typeof value.generation_error === "string" ? value.generation_error : undefined;
  const generationJobId =
    typeof value.generation_job_id === "string" ? value.generation_job_id : undefined;

  if (
    designs.length === 0 &&
    selectedIds.length === 0 &&
    createdTasks.length === 0 &&
    generationJobs.length === 0 &&
    !generationError &&
    !generationJobId
  ) {
    return undefined;
  }

  return {
    designs,
    selectedIds,
    createdTasks,
    generationJobs,
    generationError,
    generationJobId,
  };
}

export function normalizeGroupedSelectionsResponse(
  items: Array<Record<string, unknown>> | undefined,
  primarySelection?: SDSProductVariantSelection,
): GroupedSDSSelectionEligibility[] {
  if (!Array.isArray(items)) {
    return [];
  }
  return removePrimarySelectionFromGroupedSelections(
    items
      .map<GroupedSDSSelectionEligibility | null>((item) => {
      if (!item || typeof item !== "object") {
        return null;
      }
      const entry = item as unknown as {
        selectionId?: string;
        selection_id?: string;
        selection?: Record<string, unknown>;
        baselineKey?: string;
        baseline_key?: string;
        baselineStatus?: string;
        baseline_status?: string;
        baselineReason?: string;
        baseline_reason?: string;
        baselineReasonCode?: string;
        baseline_reason_code?: string;
        sheinStoreId?: string;
        shein_store_id?: string;
        eligible?: boolean;
        eligibilityReason?: string;
        eligibility_reason?: string;
      };
      const selection = normalizeSelectionResponse(entry.selection);
      if (!selection) {
        return null;
      }
      const selectionId =
        entry.selectionId ?? entry.selection_id ?? buildGroupedSDSSelectionID(selection);
      return {
        selectionId,
        selection,
        baselineKey: entry.baselineKey ?? entry.baseline_key,
        baselineStatus: normalizeSDSBaselineStatus(
          entry.baselineStatus ?? entry.baseline_status,
        ),
        baselineReason: entry.baselineReason ?? entry.baseline_reason ?? "",
        baselineReasonCode:
          entry.baselineReasonCode ?? entry.baseline_reason_code,
        sheinStoreId: entry.sheinStoreId ?? entry.shein_store_id ?? "",
        eligible: entry.eligible !== false,
        eligibilityReason: entry.eligibilityReason ?? entry.eligibility_reason,
      };
    })
      .filter((item): item is GroupedSDSSelectionEligibility => Boolean(item)),
    primarySelection,
  );
}

function normalizePromptHistoryResponse(
  items: Array<Record<string, unknown>> | undefined,
): SDSGroupedPromptHistoryEntry[] {
  if (!Array.isArray(items)) {
    return [];
  }
  return items
    .map<SDSGroupedPromptHistoryEntry | null>((item) => {
      if (!item || typeof item !== "object") {
        return null;
      }
      const entry = item as {
        prompt?: unknown;
        groupedImageMode?: unknown;
        grouped_image_mode?: unknown;
        createdAt?: unknown;
        created_at?: unknown;
      };
      if (typeof entry.prompt !== "string") {
        return null;
      }
      const createdAt =
        typeof entry.createdAt === "string"
          ? entry.createdAt
          : typeof entry.created_at === "string"
            ? entry.created_at
            : null;
      if (!createdAt) {
        return null;
      }
      return {
        prompt: entry.prompt,
        groupedImageMode:
          entry.groupedImageMode === "per_product" || entry.groupedImageMode === "shared_by_size"
            ? entry.groupedImageMode
            : entry.grouped_image_mode === "per_product" || entry.grouped_image_mode === "shared_by_size"
              ? entry.grouped_image_mode
              : "shared_by_size",
        createdAt,
      };
    })
    .filter((item): item is SDSGroupedPromptHistoryEntry => Boolean(item));
}

function normalizeDesignResponse(
  design: Record<string, unknown>,
): SheinStudioGeneratedDesign | null {
  if (!design || typeof design !== "object" || typeof design.id !== "string") {
    return null;
  }
  return {
    id: design.id,
    imageUrl:
      typeof design.image_url === "string"
        ? design.image_url
        : typeof design.imageUrl === "string"
          ? design.imageUrl
          : undefined,
    prompt: typeof design.prompt === "string" ? design.prompt : undefined,
    revisedPrompt:
      typeof design.revised_prompt === "string"
        ? design.revised_prompt
        : typeof design.revisedPrompt === "string"
          ? design.revisedPrompt
          : undefined,
    imageModel:
      typeof design.image_model === "string"
        ? design.image_model
        : typeof design.imageModel === "string"
          ? design.imageModel
          : undefined,
    transparentBackground:
      typeof design.transparent_background === "boolean"
        ? design.transparent_background
        : typeof design.transparentBackground === "boolean"
          ? design.transparentBackground
          : undefined,
    variationIntensity:
      design.variation_intensity === "light" ||
      design.variation_intensity === "medium" ||
      design.variation_intensity === "strong"
        ? design.variation_intensity
        : design.variationIntensity === "light" ||
            design.variationIntensity === "medium" ||
            design.variationIntensity === "strong"
          ? design.variationIntensity
          : undefined,
    reviewNote:
      typeof design.review_note === "string"
        ? design.review_note
        : typeof design.reviewNote === "string"
          ? design.reviewNote
          : undefined,
    role: typeof design.role === "string" ? design.role : undefined,
    roleLabel:
      typeof design.role_label === "string"
        ? design.role_label
        : typeof design.roleLabel === "string"
          ? design.roleLabel
          : undefined,
    targetGroupKey:
      typeof design.target_group_key === "string"
        ? design.target_group_key
        : typeof design.targetGroupKey === "string"
          ? design.targetGroupKey
          : undefined,
    targetGroupLabel:
      typeof design.target_group_label === "string"
        ? design.target_group_label
        : typeof design.targetGroupLabel === "string"
          ? design.targetGroupLabel
          : undefined,
    productImageUrls: Array.isArray(design.product_image_urls)
      ? (design.product_image_urls as string[])
      : Array.isArray(design.productImageUrls)
        ? (design.productImageUrls as string[])
        : undefined,
  } satisfies SheinStudioGeneratedDesign;
}

function normalizeGroupsResponse(
  items: Array<Record<string, unknown>> | undefined,
): SheinStudioGroupedWorkspace[] {
  if (!Array.isArray(items)) {
    return [];
  }
  return items
    .map<SheinStudioGroupedWorkspace | null>((item) => {
      if (!item || typeof item !== "object") {
        return null;
      }
      const group = item as Record<string, unknown>;
      const primarySelection = normalizeSelectionResponse(
        (group.primary_selection ?? group.primarySelection) as Record<string, unknown> | undefined,
      );
      if (!primarySelection) {
        return null;
      }
      const id =
        typeof group.id === "string" && group.id.trim().length > 0 ? group.id : null;
      const name =
        typeof group.name === "string" && group.name.trim().length > 0 ? group.name : null;
      const currentPrompt =
        typeof group.current_prompt === "string"
          ? group.current_prompt
          : typeof group.currentPrompt === "string"
            ? group.currentPrompt
            : "";
      const updatedAt =
        typeof group.updated_at === "string"
          ? group.updated_at
          : typeof group.updatedAt === "string"
            ? group.updatedAt
            : new Date().toISOString();
      const legacyCompatibilitySnapshot = normalizeLegacyCompatibilitySnapshotResponse(
        (group.legacy_compatibility_snapshot ??
          group.legacyCompatibilitySnapshot) as Record<string, unknown> | undefined,
      );
      if (!id || !name) {
        return null;
      }
      const normalizedDesigns = Array.isArray(group.designs)
        ? (group.designs as Array<Record<string, unknown>>)
            .map((design) => normalizeDesignResponse(design))
            .filter((design): design is NonNullable<typeof design> => Boolean(design))
        : legacyCompatibilitySnapshot?.designs ?? [];
      const normalizedSelectedIds = Array.isArray(group.approved_design_ids)
        ? (group.approved_design_ids as unknown[]).filter(
            (item): item is string => typeof item === "string",
          )
        : Array.isArray(group.selectedIds)
          ? (group.selectedIds as unknown[]).filter(
              (item): item is string => typeof item === "string",
            )
          : legacyCompatibilitySnapshot?.selectedIds ?? [];
      return {
        id,
        name,
        currentPrompt,
        promptMode:
          group.prompt_mode === "raw" || group.promptMode === "raw"
            ? "raw"
            : "managed",
        promptHistory: normalizePromptHistoryResponse(
          (group.prompt_history ?? group.promptHistory) as Array<Record<string, unknown>> | undefined,
        ),
        primarySelection,
        groupedSelections: normalizeGroupedSelectionsResponse(
          (group.grouped_selections ?? group.groupedSelections) as Array<Record<string, unknown>> | undefined,
        ),
        styleCount:
          typeof group.style_count === "string"
            ? group.style_count
            : typeof group.styleCount === "string"
              ? group.styleCount
              : "1",
        sheinStoreId:
          typeof group.shein_store_id === "string"
            ? group.shein_store_id
            : typeof group.sheinStoreId === "string"
              ? group.sheinStoreId
              : "",
        imageStrategy:
          group.image_strategy === "ai_generated" ||
          group.image_strategy === "sds_official" ||
          group.image_strategy === "hybrid"
            ? group.image_strategy
            : group.imageStrategy === "ai_generated" ||
                group.imageStrategy === "sds_official" ||
                group.imageStrategy === "hybrid"
              ? group.imageStrategy
              : "sds_official",
        groupedImageMode:
          group.grouped_image_mode === "per_product" ||
          group.grouped_image_mode === "shared_by_size"
            ? group.grouped_image_mode
            : group.groupedImageMode === "per_product" ||
                group.groupedImageMode === "shared_by_size"
              ? group.groupedImageMode
              : "shared_by_size",
        selectedSdsImages: normalizeSelectedSDSImages(
          group.selected_sds_images ?? group.selectedSdsImages,
        ),
        renderSizeImagesWithSds:
          typeof group.render_size_images_with_sds === "boolean"
            ? group.render_size_images_with_sds
            : typeof group.renderSizeImagesWithSds === "boolean"
              ? group.renderSizeImagesWithSds
              : true,
        productImageCount:
          typeof group.product_image_count === "string"
            ? group.product_image_count
            : typeof group.productImageCount === "string"
              ? group.productImageCount
              : "5",
        productImagePrompt:
          typeof group.product_image_prompt === "string"
            ? group.product_image_prompt
            : typeof group.productImagePrompt === "string"
              ? group.productImagePrompt
              : "",
        productImagePrompts:
          Array.isArray(group.product_image_prompts)
            ? (group.product_image_prompts as SheinStudioProductImagePrompt[])
            : Array.isArray(group.productImagePrompts)
              ? (group.productImagePrompts as SheinStudioProductImagePrompt[])
              : [],
        artworkModel:
          typeof group.artwork_model === "string"
            ? group.artwork_model
            : typeof group.artworkModel === "string"
              ? group.artworkModel
              : "",
        transparentBackground:
          typeof group.transparent_background === "boolean"
            ? group.transparent_background
            : typeof group.transparentBackground === "boolean"
              ? group.transparentBackground
              : false,
        variationIntensity:
          group.variation_intensity === "light" ||
          group.variation_intensity === "medium" ||
          group.variation_intensity === "strong"
            ? group.variation_intensity
            : group.variationIntensity === "light" ||
                group.variationIntensity === "medium" ||
                group.variationIntensity === "strong"
              ? group.variationIntensity
              : "medium",
        designs: normalizedDesigns,
        selectedIds: normalizedSelectedIds,
        createdTasks: normalizeCreatedTasks(
          Array.isArray(group.created_tasks) ? group.created_tasks : group.createdTasks,
          normalizedSelectedIds,
          normalizedDesigns,
        ),
        legacyCompatibilitySnapshot,
        updatedAt,
      } satisfies SheinStudioGroupedWorkspace;
    })
    .filter((item): item is SheinStudioGroupedWorkspace => Boolean(item));
}
