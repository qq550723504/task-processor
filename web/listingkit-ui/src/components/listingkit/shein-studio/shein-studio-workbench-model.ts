import {
  evaluateSDSRatioMatch,
  type SDSRatioMatch,
} from "@/lib/shein-studio/gallery-handoff";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import {
  DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
  DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
  DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
  DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
  DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
} from "@/lib/shein-studio/storage-shared";
import { DEFAULT_SHEIN_STORE_ID } from "@/lib/shein-studio/create-review-tasks";
import type {
  SheinStudioBatchDetail,
  SheinStudioBatchItemStatus,
  SheinStudioBatchStatus,
  SheinStudioGroupedWorkspace,
  SheinStudioDraft,
  SheinStudioGeneratedDesign,
  SheinStudioGenerateRequest,
  SheinStudioArtworkModel,
  SheinStudioSavedBatch,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";

export const STUDIO_SESSION_SYNC_TIMEOUT_MS = 15_000;

export function pickActiveSheinStudioGroup(
  groups: SheinStudioGroupedWorkspace[],
  activeGroupId?: string,
) {
  if (groups.length === 0) {
    return null;
  }
  const explicit =
    activeGroupId &&
    groups.find((group) => group.id === activeGroupId);
  if (explicit) {
    return explicit;
  }
  return [...groups].sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))[0] ?? null;
}

export function projectGroupToWorkbench(group: SheinStudioGroupedWorkspace) {
  return {
    prompt: group.currentPrompt,
    styleCount: group.styleCount ?? "1",
    variationIntensity:
      group.variationIntensity ?? DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
    productImageCount:
      group.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    productImagePrompt: group.productImagePrompt ?? "",
    productImagePrompts: group.productImagePrompts ?? [],
    artworkModel: group.artworkModel ?? DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
    transparentBackground: group.transparentBackground ?? false,
    sheinStoreId: group.sheinStoreId || DEFAULT_SHEIN_STORE_ID,
    imageStrategy: group.imageStrategy ?? DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
    groupedImageMode:
      group.groupedImageMode ?? DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
    selectedSdsImages: group.selectedSdsImages ?? [],
    groupedSelections: group.groupedSelections,
    renderSizeImagesWithSds: group.renderSizeImagesWithSds ?? true,
    designs: group.designs,
    selectedIds: group.selectedIds,
    createdTasks: group.createdTasks,
    itemizedBatchDetail: null,
  };
}

export type SheinStudioWorkbenchHydratedBatch = {
  savedBatch: SheinStudioSavedBatch;
  detail: SheinStudioBatchDetail;
};

export function flattenItemizedBatchDesigns(
  detail: SheinStudioBatchDetail,
): SheinStudioGeneratedDesign[] {
  return detail.items.flatMap((entry) =>
    entry.designs.map((design) => ({
      id: design.id,
      imageUrl: design.imageUrl,
      prompt: detail.batch.prompt,
      reviewNote: design.reviewNote,
      role: design.role,
      roleLabel: design.roleLabel,
      targetGroupKey: design.targetGroupKey,
      targetGroupLabel: design.targetGroupLabel,
      productImageUrls: design.productImageUrls,
    })),
  );
}

export function getApprovedItemizedBatchDesignIDs(
  detail: SheinStudioBatchDetail,
) {
  return detail.items.flatMap((entry) =>
    entry.designs
      .filter((design) => design.reviewStatus === "approved")
      .map((design) => design.id),
  );
}

const ACTIVE_ITEMIZED_BATCH_STATUSES = new Set<SheinStudioBatchStatus>([
  "generating",
]);

const ACTIVE_ITEMIZED_BATCH_ITEM_STATUSES = new Set<SheinStudioBatchItemStatus>([
  "pending",
  "generating",
  "awaiting_materialization",
]);

export function hasInFlightItemizedBatchGeneration(
  detail?: SheinStudioBatchDetail | null,
) {
  if (!detail) {
    return false;
  }
  if (
    detail.items.some((entry) =>
      ACTIVE_ITEMIZED_BATCH_ITEM_STATUSES.has(entry.item.status),
    )
  ) {
    return true;
  }
  return ACTIVE_ITEMIZED_BATCH_STATUSES.has(detail.batch.status);
}

export function projectHydratedBatchToWorkbench(
  hydratedBatch: SheinStudioWorkbenchHydratedBatch,
) {
  const { savedBatch, detail } = hydratedBatch;

  return {
    selection: savedBatch.selection,
    prompt: savedBatch.prompt || detail.batch.prompt,
    styleCount: savedBatch.styleCount || detail.batch.styleCount || "1",
    variationIntensity:
      savedBatch.variationIntensity ??
      DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
    productImageCount:
      savedBatch.productImageCount ??
      DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    productImagePrompt: savedBatch.productImagePrompt ?? "",
    productImagePrompts: savedBatch.productImagePrompts ?? [],
    artworkModel:
      savedBatch.artworkModel ?? DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
    transparentBackground: savedBatch.transparentBackground ?? false,
    sheinStoreId:
      savedBatch.sheinStoreId ||
      (detail.batch.sheinStoreId > 0
        ? String(detail.batch.sheinStoreId)
        : DEFAULT_SHEIN_STORE_ID),
    imageStrategy:
      savedBatch.imageStrategy ?? DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
    groupedImageMode:
      savedBatch.groupedImageMode ?? DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
    selectedSdsImages: savedBatch.selectedSdsImages ?? [],
    groupedSelections: savedBatch.groupedSelections ?? [],
    renderSizeImagesWithSds: savedBatch.renderSizeImagesWithSds ?? true,
    designs: flattenItemizedBatchDesigns(detail),
    selectedIds: getApprovedItemizedBatchDesignIDs(detail),
    generationJobs: savedBatch.generationJobs ?? [],
    createdTasks: savedBatch.createdTasks,
    persistedUpdatedAt:
      detail.batch.draftUpdatedAt ||
      savedBatch.draftUpdatedAt ||
      savedBatch.updatedAt,
    itemizedBatchDetail: detail,
  };
}

export function evaluateImportedGalleryDesigns(
  designs: SheinStudioGeneratedDesign[],
  selection?: SDSProductVariantSelection,
): SDSRatioMatch | null {
  const imported = designs.find(
    (design) => design.role === "gallery" && design.sourceWidth && design.sourceHeight,
  );
  if (!imported) {
    return null;
  }
  return evaluateSDSRatioMatch({
    sourceWidth: imported.sourceWidth,
    sourceHeight: imported.sourceHeight,
    targetWidth: selection?.printableWidth,
    targetHeight: selection?.printableHeight,
  });
}

export function buildSheinStudioSelectionKey(
  selection?: SDSProductVariantSelection,
) {
  if (!selection) {
    return "none";
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

export function buildSheinStudioSelectedVariants(
  selection?: SDSProductVariantSelection,
) {
  if (selection?.variants?.length) {
    return selection.variants;
  }

  if (selection?.selectedVariantIds?.length) {
    return selection.selectedVariantIds.map((variantId) => ({
      variantId,
      size: undefined,
      color: undefined,
    }));
  }

  if (selection?.variantId) {
    return [
      {
        variantId: selection.variantId,
        size: selection.variantLabel,
        color: "默认",
      },
    ];
  }

  return [];
}

export function summarizeSheinStudioSelection(
  selection?: SDSProductVariantSelection,
) {
  const selectedVariants = buildSheinStudioSelectedVariants(selection);
  return {
    printableAreaLabel:
      selection?.printableWidth && selection?.printableHeight
        ? `${selection.printableWidth} × ${selection.printableHeight}px`
        : "自动",
    selectedVariants,
    selectedColorCount: new Set(
      selectedVariants.map((variant) => variant.color || "default"),
    ).size,
    selectedSizeCount: new Set(
      selectedVariants.map((variant) => variant.size || "One size"),
    ).size,
  };
}

export function getSheinStudioCreateActionDisabledReason({
  selection,
  galleryRatioCheck,
  selectedIds,
}: {
  selection?: SDSProductVariantSelection;
  galleryRatioCheck?: SDSRatioMatch | null;
  selectedIds: string[];
}) {
  if (!selection?.variantId) {
    return "请先选择 SDS 商品变体。生成 SHEIN 资料前需要锁定商品模板。";
  }
  if (galleryRatioCheck?.status === "blocking") {
    return galleryRatioCheck.message;
  }
  if (selectedIds.length === 0) {
    return "请至少批准 1 个款式后再生成 SHEIN 资料。";
  }
  return undefined;
}

export function mergeSheinStudioDraftState({
  draft,
  galleryDesign,
  galleryPrompt,
}: {
  draft?: SheinStudioDraft | null;
  galleryDesign?: SheinStudioGeneratedDesign | null;
  galleryPrompt?: string | null;
}) {
  const draftDesigns = draft?.designs ?? [];
  const designs =
    galleryDesign && !draftDesigns.some((design) => design.id === galleryDesign.id)
      ? [...draftDesigns, galleryDesign]
      : draftDesigns;
  const draftSelectedIds = draft?.selectedIds ?? [];
  const selectedIds =
    galleryDesign && !draftSelectedIds.includes(galleryDesign.id)
      ? [...draftSelectedIds, galleryDesign.id]
      : draftSelectedIds;
  const createdTasks = draft?.createdTasks ?? [];

  return {
    prompt: draft?.prompt || galleryPrompt || "",
    selection: draft?.selection,
    styleCount: draft?.styleCount ?? "1",
    variationIntensity:
      draft?.variationIntensity ?? DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
    productImageCount:
      draft?.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    productImagePrompt: draft?.productImagePrompt ?? "",
    productImagePrompts: draft?.productImagePrompts ?? [],
    artworkModel: draft?.artworkModel ?? DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
    transparentBackground: draft?.transparentBackground ?? false,
    sheinStoreId: draft?.sheinStoreId || DEFAULT_SHEIN_STORE_ID,
    imageStrategy: draft?.imageStrategy ?? DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
    groupedImageMode:
      draft?.groupedImageMode ?? DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
    selectedSdsImages: draft?.selectedSdsImages ?? [],
    groups: draft?.groups ?? [],
    groupedSelections: draft?.groupedSelections ?? [],
    renderSizeImagesWithSds: draft?.renderSizeImagesWithSds ?? true,
    designs,
    selectedIds,
    generationJobs: draft?.generationJobs ?? [],
    createdTasks,
    hasCustomizedSdsSelection: (draft?.selectedSdsImages?.length ?? 0) > 0,
    importedGalleryDesign: Boolean(galleryDesign),
    designCount: designs.length,
    createdTaskCount: createdTasks.length,
  };
}

export function buildSheinStudioGenerateRequest({
  artworkModel,
  prompt,
  printableHeight,
  printableWidth,
  productReferenceImageUrls,
  styleCount,
  transparentBackground,
  variationIntensity,
}: {
  artworkModel: SheinStudioArtworkModel;
  prompt: string;
  printableHeight?: number;
  printableWidth?: number;
  productReferenceImageUrls?: string[];
  styleCount: number;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
}): SheinStudioGenerateRequest {
  const trimmedModel = artworkModel.trim();
  const normalizedPrompt = normalizeSheinStudioPromptWithSDSSize({
    prompt,
    printableWidth,
    printableHeight,
  });
  return {
    prompt: normalizedPrompt,
    count: styleCount,
    variationIntensity,
    printableWidth,
    printableHeight,
    productReferenceImageUrls,
    imageModel: transparentBackground ? "gpt-image-2" : trimmedModel || undefined,
    transparentBackground,
  };
}

function normalizeSheinStudioPromptWithSDSSize({
  prompt,
  printableWidth,
  printableHeight,
}: {
  prompt: string;
  printableWidth?: number;
  printableHeight?: number;
}) {
  const trimmedPrompt = prompt.trim();
  if (!trimmedPrompt) {
    return trimmedPrompt;
  }
  if (!printableWidth || !printableHeight) {
    return trimmedPrompt;
  }
  const sizeSuffix = `printable size: ${printableWidth}x${printableHeight}px.`;
  const normalizedPrompt = trimmedPrompt.replace(/\s+/g, " ");
  const lowerPrompt = normalizedPrompt.toLowerCase();
  const lowerSuffix = sizeSuffix.toLowerCase();
  const compactSizeToken = `${printableWidth}x${printableHeight}px`.toLowerCase();
  const spacedSizeToken = `${printableWidth} × ${printableHeight}px`.toLowerCase();

  if (
    lowerPrompt.includes(lowerSuffix) ||
    lowerPrompt.includes(compactSizeToken) ||
    lowerPrompt.includes(spacedSizeToken)
  ) {
    return normalizedPrompt;
  }
  return `${normalizedPrompt}\n\n${sizeSuffix}`;
}

export function sheinStudioBusyMessage({
  isCreatingTasks,
  isGenerating,
  regeneratingId,
}: {
  isCreatingTasks: boolean;
  isGenerating: boolean;
  regeneratingId?: string;
}) {
  if (isGenerating) {
    return "正在生成款式图";
  }
  if (regeneratingId) {
    return "正在重新生成图片";
  }
  if (isCreatingTasks) {
    return "正在生成商品图和 SHEIN 资料";
  }
  return "";
}
