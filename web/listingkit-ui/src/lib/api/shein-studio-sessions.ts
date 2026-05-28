import { apiRequest } from "@/lib/api/client";
import { parseStudioSessionDetailResponse } from "@/lib/api/shein-studio-session-schema";
import { normalizeDraft } from "@/lib/shein-studio/storage-shared";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import {
  buildGroupedSDSSelectionID,
  type GroupedSDSSelectionEligibility,
  normalizeSDSBaselineStatus,
} from "@/lib/types/sds-baseline";
import type {
  SDSGroupedPromptHistoryEntry,
  SheinStudioArtworkModel,
  SheinStudioSavedBatch,
  SheinStudioCreatedTask,
  SheinStudioDraft,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedWorkspace,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";
import { normalizeSelectedSDSImages } from "@/lib/shein-studio/sds-selectable-images";

export type StudioSessionStatus =
  | "selecting"
  | "generating"
  | "generated"
  | "reviewing"
  | "failed"
  | "tasks_created";

export type StudioSessionDetailResponse = {
  session?: {
    id: string;
    tenant_id?: string;
    batch_name?: string;
    status?: StudioSessionStatus;
    selection?: Record<string, unknown>;
    prompt?: string;
    style_count?: string;
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
    generation_job_id?: string;
    generation_error?: string;
    approved_design_ids?: string[];
    created_tasks?: SheinStudioCreatedTask[];
    updated_at?: string;
  };
  designs?: Array<{
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
  }>;
};

type RawCreatedTask = {
  id?: string;
  title?: string;
  designId?: string;
  design_id?: string;
};

type StudioBatchListResponse = {
  items?: Array<{
    id: string;
    batch_name?: string;
    prompt?: string;
    style_count?: string;
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
    approved_design_ids?: string[];
    created_tasks?: SheinStudioCreatedTask[];
    design_count?: number;
    updated_at?: string;
  }>;
};

const sessionCache = new Map<string, string>();
const STUDIO_SESSION_TIMEOUT_MS = 15_000;

type StudioSessionRequestOptions = {
  signal?: AbortSignal;
  timeoutMs?: number;
};

export function buildStudioSessionSelectionKey(selection?: SDSProductVariantSelection) {
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

export async function ensureSheinStudioSession(
  selection?: SDSProductVariantSelection,
  options?: StudioSessionRequestOptions,
) {
  if (!selection?.variantId) {
    return null;
  }
  const detail = parseStudioSessionDetailResponse(await apiRequest<unknown>("/studio/sessions", {
    method: "POST",
    body: {
      selection: selectionToPayload(selection),
    },
    signal: options?.signal,
    timeoutMs: options?.timeoutMs ?? STUDIO_SESSION_TIMEOUT_MS,
  }));
  cacheStudioSession(detail, selection);
  return detail;
}

export async function getSheinStudioSession(
  sessionId: string,
  options?: StudioSessionRequestOptions,
) {
  return parseStudioSessionDetailResponse(
    await apiRequest<unknown>(`/studio/sessions/${sessionId}`, {
      signal: options?.signal,
      timeoutMs: options?.timeoutMs ?? STUDIO_SESSION_TIMEOUT_MS,
    }),
  );
}

export async function updateSheinStudioSession(
  sessionId: string,
  patch: {
    status?: StudioSessionStatus;
    prompt?: string;
    styleCount?: string;
    variationIntensity?: SheinStudioVariationIntensity;
    productImageCount?: string;
    productImagePrompt?: string;
    productImagePrompts?: SheinStudioProductImagePrompt[];
    artworkModel?: string;
    imageStrategy?: string;
    groupedImageMode?: SheinStudioGroupedImageMode;
    selectedSdsImages?: SheinStudioSelectedSDSImage[];
    groups?: SheinStudioGroupedWorkspace[];
    groupedSelections?: GroupedSDSSelectionEligibility[];
    transparentBackground?: boolean;
    renderSizeImagesWithSds?: boolean;
    sheinStoreId?: string;
    generationJobId?: string;
    generationError?: string;
    approvedDesignIds?: string[];  
    createdTasks?: SheinStudioCreatedTask[];
  },
  options?: StudioSessionRequestOptions,
) {
  return parseStudioSessionDetailResponse(
    await apiRequest<unknown>(`/studio/sessions/${sessionId}`, {
      method: "PATCH",
      body: {
        status: patch.status,
        prompt: patch.prompt,
        style_count: patch.styleCount,
        variation_intensity: patch.variationIntensity,
        product_image_count: patch.productImageCount,
        product_image_prompt: patch.productImagePrompt,
        product_image_prompts: patch.productImagePrompts,
        artwork_model: patch.artworkModel,
        image_strategy: patch.imageStrategy,
        grouped_image_mode: patch.groupedImageMode,
        selected_sds_images: patch.selectedSdsImages,
        grouped_selections: patch.groupedSelections?.map(groupedSelectionToPayload),
        transparent_background: patch.transparentBackground,
        render_size_images_with_sds: patch.renderSizeImagesWithSds,
        shein_store_id: patch.sheinStoreId,
        generation_job_id: patch.generationJobId,
        generation_error: patch.generationError,
        approved_design_ids: patch.approvedDesignIds,
        created_tasks: patch.createdTasks,
      },
      signal: options?.signal,
      timeoutMs: options?.timeoutMs ?? STUDIO_SESSION_TIMEOUT_MS,
    }),
  );
}

export async function replaceSheinStudioSessionDesigns(
  sessionId: string,
  input: {
    status?: StudioSessionStatus;
    approvedDesignIds: string[];
    designs: SheinStudioGeneratedDesign[];
  },
  options?: StudioSessionRequestOptions,
) {
  return parseStudioSessionDetailResponse(
    await apiRequest<unknown>(`/studio/sessions/${sessionId}/designs`, {
      method: "POST",
      body: {
        status: input.status,
        approved_design_ids: input.approvedDesignIds,
        designs: input.designs.map((design) => ({
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
      },
      signal: options?.signal,
      timeoutMs: options?.timeoutMs ?? STUDIO_SESSION_TIMEOUT_MS,
    }),
  );
}

export async function listSheinStudioSessionBatches(options?: StudioSessionRequestOptions) {
  const payload = await apiRequest<unknown>("/studio/batches", {
    signal: options?.signal,
    timeoutMs: options?.timeoutMs ?? STUDIO_SESSION_TIMEOUT_MS,
  });
  const response = payload as StudioBatchListResponse;
  return (response.items ?? []).map(mapStudioBatchListItemToBatch);
}

export async function getSheinStudioSessionBatch(
  batchId: string,
  options?: StudioSessionRequestOptions,
) {
  const detail = parseStudioSessionDetailResponse(
    await apiRequest<unknown>(`/studio/batches/${batchId}`, {
      signal: options?.signal,
      timeoutMs: options?.timeoutMs ?? STUDIO_SESSION_TIMEOUT_MS,
    }),
  );
  return mapStudioSessionDetailToBatch(detail);
}

export async function upsertSheinStudioSessionBatch(
  input: {
    id?: string;
    name?: string;
    prompt: string;
    styleCount: string;
    variationIntensity?: SheinStudioVariationIntensity;
    productImageCount?: string;
    productImagePrompt?: string;
    productImagePrompts?: SheinStudioProductImagePrompt[];
    artworkModel?: string;
    imageStrategy?: string;
    groupedImageMode?: SheinStudioGroupedImageMode;
    selectedSdsImages?: SheinStudioSelectedSDSImage[];
    transparentBackground?: boolean;
    renderSizeImagesWithSds?: boolean;
    sheinStoreId?: string;
    selection?: SDSProductVariantSelection;
    groups?: SheinStudioGroupedWorkspace[];
    groupedSelections?: GroupedSDSSelectionEligibility[];
    approvedDesignIds: string[];
    createdTasks: SheinStudioCreatedTask[];
    designs: SheinStudioGeneratedDesign[];
  },
  options?: StudioSessionRequestOptions,
) {
  const explicitBatchName = input.name?.trim() || undefined;
  const batchName =
    explicitBatchName ?? (input.id ? undefined : deriveBatchName(input.prompt));
  const detail = parseStudioSessionDetailResponse(
    await apiRequest<unknown>("/studio/batches", {
      method: "POST",
      body: {
        id: input.id,
        batch_name: batchName,
        prompt: input.prompt,
        style_count: input.styleCount,
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
        grouped_selections: input.groupedSelections?.map(groupedSelectionToPayload),
        approved_design_ids: input.approvedDesignIds,
        created_tasks: input.createdTasks,
        designs: input.designs.map((design) => ({
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
      },
      signal: options?.signal,
      timeoutMs: options?.timeoutMs ?? STUDIO_SESSION_TIMEOUT_MS,
    }),
  );
  return mapStudioSessionDetailToBatch(detail);
}

export async function deleteSheinStudioSessionBatch(
  batchId: string,
  options?: StudioSessionRequestOptions,
) {
  await apiRequest<unknown>(`/studio/batches/${batchId}`, {
    method: "DELETE",
    signal: options?.signal,
    timeoutMs: options?.timeoutMs ?? STUDIO_SESSION_TIMEOUT_MS,
  });
}

export function mapStudioSessionDetailToDraft(
  detail: StudioSessionDetailResponse | null | undefined,
): SheinStudioDraft | null {
  if (!detail?.session) {
    return null;
  }

  const selectedIds =
    detail.session.approved_design_ids ??
    detail.designs
      ?.filter((design) => design.approved)
      .map((design) => design.id) ??
    [];

  return normalizeDraft({
    prompt: detail.session.prompt ?? "",
    styleCount: detail.session.style_count ?? "1",
    variationIntensity: detail.session.variation_intensity ?? "medium",
    productImageCount: detail.session.product_image_count ?? "5",
    productImagePrompt: detail.session.product_image_prompt ?? "",
    productImagePrompts: detail.session.product_image_prompts ?? [],
    artworkModel: detail.session.artwork_model,
    transparentBackground: detail.session.transparent_background ?? false,
    sheinStoreId: detail.session.shein_store_id ?? "",
    imageStrategy: detail.session.image_strategy,
    groupedImageMode: detail.session.grouped_image_mode,
    selectedSdsImages: normalizeSelectedSDSImages(detail.session.selected_sds_images),
    renderSizeImagesWithSds: detail.session.render_size_images_with_sds ?? true,
    selectionVariantId: normalizeSelectionResponse(detail.session.selection)?.variantId,
    selection: normalizeSelectionResponse(detail.session.selection),
    groups: normalizeGroupsResponse(detail.session.groups),
    groupedSelections: normalizeGroupedSelectionsResponse(
      detail.session.grouped_selections,
    ),
    designs:
      detail.designs?.map((design) => ({
        id: design.id,
        imageUrl: design.image_url,
        prompt: design.prompt ?? detail.session?.prompt,
        revisedPrompt: design.revised_prompt,
        imageModel: design.image_model ?? detail.session?.artwork_model,
        transparentBackground:
          design.transparent_background ?? detail.session?.transparent_background,
        variationIntensity:
          design.variation_intensity ?? detail.session?.variation_intensity,
        reviewNote: design.review_note,
        role: design.role,
        roleLabel: design.role_label,
        targetGroupKey: design.target_group_key,
        targetGroupLabel: design.target_group_label,
        productImageUrls: design.product_image_urls,
      })) ?? [],
    selectedIds,
    createdTasks: normalizeCreatedTasks(
      detail.session.created_tasks,
      selectedIds,
      detail.designs,
    ),
    generationError: detail.session.generation_error ?? "",
    generationJobId: detail.session.generation_job_id ?? "",
    sessionStatus: detail.session.status ?? "",
    updatedAt: detail.session.updated_at ?? new Date().toISOString(),
  });
}

export function mapStudioSessionDetailToBatch(
  detail: StudioSessionDetailResponse | null | undefined,
): SheinStudioSavedBatch | null {
  const draft = mapStudioSessionDetailToDraft(detail);
  if (!draft || !detail?.session?.id) {
    return null;
  }
  return {
    id: detail.session.id,
    name: detail.session.batch_name ?? deriveBatchName(detail.session.prompt ?? draft.prompt),
    ...draft,
  };
}

export function getCachedStudioSessionId(selection?: SDSProductVariantSelection) {
  return sessionCache.get(buildStudioSessionSelectionKey(selection));
}

function cacheStudioSession(
  detail: StudioSessionDetailResponse,
  selection?: SDSProductVariantSelection,
) {
  const sessionId = detail.session?.id;
  if (!sessionId) {
    return;
  }
  const key = buildStudioSessionSelectionKey(
    selection ?? normalizeSelectionResponse(detail.session?.selection),
  );
  if (key) {
    sessionCache.set(key, sessionId);
  }
}

function normalizeSelectionResponse(
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
  return {
    id: item.id,
    name: item.batch_name ?? deriveBatchName(item.prompt ?? ""),
    prompt: item.prompt ?? "",
    styleCount: item.style_count ?? "1",
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
    selectionVariantId: normalizeSelectionResponse(item.selection)?.variantId,
    selection: normalizeSelectionResponse(item.selection),
    groups: normalizeGroupsResponse(item.groups),
    groupedSelections: normalizeGroupedSelectionsResponse(item.grouped_selections),
    designs: [],
    selectedIds: item.approved_design_ids ?? [],
    createdTasks: normalizeCreatedTasks(item.created_tasks, item.approved_design_ids),
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

function normalizeGroupedSelectionsResponse(
  items: Array<Record<string, unknown>> | undefined,
): GroupedSDSSelectionEligibility[] {
  if (!Array.isArray(items)) {
    return [];
  }
  return items
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
    .filter((item): item is GroupedSDSSelectionEligibility => Boolean(item));
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
      if (!id || !name) {
        return null;
      }
      return {
        id,
        name,
        currentPrompt,
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
        designs: Array.isArray(group.designs)
          ? (group.designs as Array<Record<string, unknown>>)
              .map((design) => normalizeDesignResponse(design))
              .filter((design): design is NonNullable<typeof design> => Boolean(design))
          : [],
        selectedIds: Array.isArray(group.approved_design_ids)
          ? (group.approved_design_ids as unknown[]).filter(
              (item): item is string => typeof item === "string",
            )
          : Array.isArray(group.selectedIds)
            ? (group.selectedIds as unknown[]).filter(
                (item): item is string => typeof item === "string",
              )
            : [],
        createdTasks: normalizeCreatedTasks(
          Array.isArray(group.created_tasks) ? group.created_tasks : group.createdTasks,
          Array.isArray(group.approved_design_ids)
            ? (group.approved_design_ids as unknown[]).filter(
                (item): item is string => typeof item === "string",
              )
            : undefined,
          Array.isArray(group.designs)
            ? (group.designs as Array<Record<string, unknown>>)
                .map((design) => normalizeDesignResponse(design))
                .filter((design): design is NonNullable<typeof design> => Boolean(design))
            : undefined,
        ),
        updatedAt,
      } satisfies SheinStudioGroupedWorkspace;
    })
    .filter((item): item is SheinStudioGroupedWorkspace => Boolean(item));
}
