import { apiRequest } from "@/lib/api/client";
import { normalizeDraft } from "@/lib/shein-studio/storage-shared";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioDraft,
  SheinStudioGeneratedDesign,
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

type StudioSessionDetailResponse = {
  session?: {
    id: string;
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
    selected_sds_images?: SheinStudioSelectedSDSImage[];
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
    image_url?: string;
    revised_prompt?: string;
    review_note?: string;
    role?: string;
    role_label?: string;
    product_image_urls?: string[];
    approved?: boolean;
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
  const detail = await apiRequest<StudioSessionDetailResponse>("/studio/sessions", {
    method: "POST",
    body: {
      selection: selectionToPayload(selection),
    },
    signal: options?.signal,
    timeoutMs: options?.timeoutMs ?? STUDIO_SESSION_TIMEOUT_MS,
  });
  cacheStudioSession(detail, selection);
  return detail;
}

export async function getSheinStudioSession(
  sessionId: string,
  options?: StudioSessionRequestOptions,
) {
  return apiRequest<StudioSessionDetailResponse>(`/studio/sessions/${sessionId}`, {
    signal: options?.signal,
    timeoutMs: options?.timeoutMs ?? STUDIO_SESSION_TIMEOUT_MS,
  });
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
    selectedSdsImages?: SheinStudioSelectedSDSImage[];
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
  return apiRequest<StudioSessionDetailResponse>(`/studio/sessions/${sessionId}`, {
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
      selected_sds_images: patch.selectedSdsImages,
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
  });
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
  return apiRequest<StudioSessionDetailResponse>(`/studio/sessions/${sessionId}/designs`, {
    method: "POST",
    body: {
      status: input.status,
      approved_design_ids: input.approvedDesignIds,
      designs: input.designs.map((design) => ({
        id: design.id,
        image_url: design.imageUrl ?? design.dataUrl,
        revised_prompt: design.revisedPrompt,
        review_note: design.reviewNote,
        role: design.role,
        role_label: design.roleLabel,
        product_image_urls: design.productImageUrls,
      })),
    },
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
    selectedSdsImages: normalizeSelectedSDSImages(detail.session.selected_sds_images),
    renderSizeImagesWithSds: detail.session.render_size_images_with_sds ?? true,
    selectionVariantId: normalizeSelectionResponse(detail.session.selection)?.variantId,
    selection: normalizeSelectionResponse(detail.session.selection),
    designs:
      detail.designs?.map((design) => ({
        id: design.id,
        imageUrl: design.image_url,
        revisedPrompt: design.revised_prompt,
        reviewNote: design.review_note,
        role: design.role,
        roleLabel: design.role_label,
        productImageUrls: design.product_image_urls,
      })) ?? [],
    selectedIds,
    createdTasks: detail.session.created_tasks ?? [],
    updatedAt: detail.session.updated_at ?? new Date().toISOString(),
  });
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

function asString(value: unknown) {
  return typeof value === "string" && value.trim() ? value : undefined;
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
