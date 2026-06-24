import {
  evaluateSDSRatioMatch,
  type SDSRatioMatch,
} from "@/lib/shein-studio/gallery-handoff";
import { formatSheinStoreOptionLabel } from "@/lib/shein-studio/store-option-label";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import {
  DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
  DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
  DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
  DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
  DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
} from "@/lib/shein-studio/storage-shared";
import { DEFAULT_SHEIN_STORE_ID } from "@/lib/shein-studio/create-review-tasks";
export { buildSheinStudioGenerateRequest } from "@/lib/shein-studio/generation-controller";
import type {
  SheinStudioBatchDetail,
  SheinStudioBatchItemStatus,
  SheinStudioBatchStatus,
  SheinStudioCreatedTask,
  SheinStudioGroupedImageMode,
  SheinStudioGroupedWorkspace,
  SheinStudioDraft,
  SheinStudioGeneratedDesign,
  SheinStudioGenerationJob,
  SheinStudioImageStrategy,
  SheinStudioArtworkModel,
  SheinStudioBatchQueueMode,
  SheinStudioProductImagePrompt,
  SheinStudioSavedBatch,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";
import {
  buildDefaultSelectedSDSImages,
  type SheinStudioSelectableSDSImage,
} from "@/lib/shein-studio/sds-selectable-images";
import type { SheinStudioWorkbenchState } from "@/components/listingkit/shein-studio/shein-studio-workbench-state";

export const STUDIO_SESSION_SYNC_TIMEOUT_MS = 15_000;

type SheinStoreOptionProjectionInput = {
  currentStoreId?: string | null;
  enabledProfiles: Array<Parameters<typeof formatSheinStoreOptionLabel>[0]>;
};

export function projectSheinStudioStoreSelectionState({
  currentStoreId,
  enabledProfiles,
}: SheinStoreOptionProjectionInput) {
  const effectiveCurrentStoreId = (currentStoreId ?? "").trim();
  const matched = enabledProfiles.find(
    (item) => String(item.store_id) === effectiveCurrentStoreId,
  );
  return {
    currentStoreLabel: matched ? formatSheinStoreOptionLabel(matched) : "",
    effectiveCurrentStoreId,
    recentBatchStoreOptions: enabledProfiles.map((profile) => ({
      id: String(profile.store_id),
      label: formatSheinStoreOptionLabel(profile),
    })),
  };
}

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

export function selectActiveGroupPromptHistory({
  activeGroupId,
  groups,
}: {
  activeGroupId: string;
  groups: SheinStudioGroupedWorkspace[];
}) {
  return groups.find((group) => group.id === activeGroupId)?.promptHistory ?? [];
}

export function projectGroupToWorkbench(group: SheinStudioGroupedWorkspace) {
  return {
    prompt: group.currentPrompt,
    promptMode: group.promptMode ?? "managed",
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

function projectSavedBatchCompatibilityFields(savedBatch: SheinStudioSavedBatch) {
  const compatibility = savedBatch.legacyCompatibilitySnapshot;
  return {
    designs:
      savedBatch.designs.length > 0
        ? savedBatch.designs
        : (compatibility?.designs ?? []),
    selectedIds:
      savedBatch.selectedIds.length > 0
        ? savedBatch.selectedIds
        : (compatibility?.selectedIds ?? []),
    createdTasks:
      savedBatch.createdTasks.length > 0
        ? savedBatch.createdTasks
        : (compatibility?.createdTasks ?? []),
    generationJobs:
      (savedBatch.generationJobs?.length ?? 0) > 0
        ? (savedBatch.generationJobs ?? [])
        : (compatibility?.generationJobs ?? []),
    generationError:
      savedBatch.generationError !== undefined
        ? savedBatch.generationError
        : (compatibility?.generationError ?? ""),
  };
}

function projectItemizedBatchCompatibilityFields(
  detail: SheinStudioBatchDetail,
) {
  return {
    designs: flattenItemizedBatchDesigns(detail),
    selectedIds: getApprovedItemizedBatchDesignIDs(detail),
    selection: detail.batch.selection,
    prompt: detail.batch.prompt,
    promptMode: detail.batch.promptMode ?? "managed",
    styleCount: detail.batch.styleCount || "1",
    variationIntensity: detail.batch.variationIntensity,
    artworkModel: detail.batch.artworkModel,
    transparentBackground: detail.batch.transparentBackground,
    sheinStoreId:
      detail.batch.sheinStoreId > 0
        ? String(detail.batch.sheinStoreId)
        : undefined,
    groupedImageMode: detail.batch.groupedImageMode,
    selectedSdsImages: detail.batch.selectedSdsImages ?? [],
    groupedSelections: detail.batch.groupedSelections ?? [],
  };
}

function resolveDraftComparableTimestamp(value?: string) {
  return value?.trim() || "";
}

function preferSavedBatchDraftValue(
  savedValue: string | undefined,
  itemizedValue: string | undefined,
  savedUpdatedAt: string | undefined,
  itemizedUpdatedAt: string | undefined,
) {
  const normalizedSaved = savedValue?.trim() ?? "";
  const normalizedItemized = itemizedValue?.trim() ?? "";
  if (!normalizedSaved) {
    return normalizedItemized;
  }
  if (!normalizedItemized) {
    return normalizedSaved;
  }
  return resolveDraftComparableTimestamp(savedUpdatedAt) >=
    resolveDraftComparableTimestamp(itemizedUpdatedAt)
    ? normalizedSaved
    : normalizedItemized;
}

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

export function toggleItemizedBatchDesignApproval(
  detail: SheinStudioBatchDetail,
  designId: string,
): SheinStudioBatchDetail {
  return {
    ...detail,
    items: detail.items.map((entry) => ({
      ...entry,
      designs: entry.designs.map((design) =>
        design.id !== designId
          ? design
          : {
              ...design,
              reviewStatus:
                design.reviewStatus === "approved" ? "unreviewed" : "approved",
            },
      ),
    })),
  };
}

export function updateItemizedBatchDesignReviewNote(
  detail: SheinStudioBatchDetail,
  designId: string,
  note: string,
): SheinStudioBatchDetail {
  return {
    ...detail,
    items: detail.items.map((entry) => ({
      ...entry,
      designs: entry.designs.map((design) =>
        design.id === designId ? { ...design, reviewNote: note } : design,
      ),
    })),
  };
}

export function updateFlatDesignReviewNote(
  designs: SheinStudioGeneratedDesign[],
  designId: string,
  note: string,
) {
  return designs.map((design) =>
    design.id === designId ? { ...design, reviewNote: note } : design,
  );
}

export function toggleSelectedDesignId(selectedIds: string[], designId: string) {
  return selectedIds.includes(designId)
    ? selectedIds.filter((item) => item !== designId)
    : [...selectedIds, designId];
}

export function getItemizedBatchPendingTaskDesignIDs(
  detail?: SheinStudioBatchDetail | null,
) {
  if (!detail) {
    return [];
  }
  const taskDesignIDs = new Set(
    [...(detail.createdTasks ?? []), ...(detail.reusedTasks ?? [])]
      .map((task) => task.designId?.trim())
      .filter((designId): designId is string => Boolean(designId)),
  );
  return getApprovedItemizedBatchDesignIDs(detail).filter(
    (designId) => !taskDesignIDs.has(designId),
  );
}

const ACTIVE_ITEMIZED_BATCH_STATUSES = new Set<SheinStudioBatchStatus>([
  "generating",
  "tasks_creating",
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

export function projectSavedBatchToWorkbench(savedBatch: SheinStudioSavedBatch) {
  const compatibility = projectSavedBatchCompatibilityFields(savedBatch);

  return {
    selection: savedBatch.selection,
    prompt: savedBatch.prompt,
    promptMode: savedBatch.promptMode ?? "managed",
    styleCount: savedBatch.styleCount || "1",
    variationIntensity:
      savedBatch.variationIntensity ?? DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
    productImageCount:
      savedBatch.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    productImagePrompt: savedBatch.productImagePrompt ?? "",
    productImagePrompts: savedBatch.productImagePrompts ?? [],
    artworkModel:
      savedBatch.artworkModel ?? DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
    transparentBackground: savedBatch.transparentBackground ?? false,
    sheinStoreId: savedBatch.sheinStoreId || DEFAULT_SHEIN_STORE_ID,
    imageStrategy:
      savedBatch.imageStrategy ?? DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
    groupedImageMode:
      savedBatch.groupedImageMode ?? DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
    selectedSdsImages: savedBatch.selectedSdsImages ?? [],
    groupedSelections: savedBatch.groupedSelections ?? [],
    renderSizeImagesWithSds: savedBatch.renderSizeImagesWithSds ?? true,
    designs: compatibility.designs,
    selectedIds: compatibility.selectedIds,
    generationJobs: compatibility.generationJobs,
    generationError: compatibility.generationError,
    createdTasks: compatibility.createdTasks,
    persistedUpdatedAt: savedBatch.draftUpdatedAt ?? savedBatch.updatedAt,
    itemizedBatchDetail: null,
  };
}

export function projectHydratedBatchToWorkbench(
  hydratedBatch: SheinStudioWorkbenchHydratedBatch,
) {
  const { savedBatch, detail } = hydratedBatch;
  const saved = projectSavedBatchToWorkbench(savedBatch);
  const itemized = projectItemizedBatchCompatibilityFields(detail);
  const detailCreatedTasks = detail.createdTasks ?? [];
  const savedDraftUpdatedAt = savedBatch.draftUpdatedAt || savedBatch.updatedAt;
  const itemizedDraftUpdatedAt =
    detail.batch.draftUpdatedAt || detail.batch.updatedAt;
  const hasInFlightGeneration = hasInFlightItemizedBatchGeneration(detail);
  const shouldPreserveSavedGenerationError =
    detail.batch.status === "failed" || detail.batch.status === "partially_failed";

  return {
    selection: itemized.selection ?? saved.selection,
    prompt: preferSavedBatchDraftValue(
      saved.prompt,
      itemized.prompt,
      savedDraftUpdatedAt,
      itemizedDraftUpdatedAt,
    ),
    styleCount: itemized.styleCount || saved.styleCount || "1",
    variationIntensity:
      itemized.variationIntensity ??
      saved.variationIntensity ??
      DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
    productImageCount:
      saved.productImageCount ??
      DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    productImagePrompt: saved.productImagePrompt ?? "",
    productImagePrompts: saved.productImagePrompts ?? [],
    artworkModel:
      itemized.artworkModel ??
      saved.artworkModel ??
      DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
    transparentBackground:
      itemized.transparentBackground ??
      saved.transparentBackground ??
      false,
    sheinStoreId:
      itemized.sheinStoreId ||
      saved.sheinStoreId ||
      DEFAULT_SHEIN_STORE_ID,
    imageStrategy:
      saved.imageStrategy ?? DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
    groupedImageMode:
      itemized.groupedImageMode ??
      saved.groupedImageMode ??
      DEFAULT_SHEIN_STUDIO_GROUPED_IMAGE_MODE,
    selectedSdsImages:
      itemized.selectedSdsImages.length > 0
        ? itemized.selectedSdsImages
        : saved.selectedSdsImages,
    groupedSelections:
      itemized.groupedSelections.length > 0
        ? itemized.groupedSelections
        : saved.groupedSelections,
    renderSizeImagesWithSds: saved.renderSizeImagesWithSds ?? true,
    designs: itemized.designs,
    selectedIds: itemized.selectedIds,
    generationJobs: hasInFlightGeneration ? saved.generationJobs : [],
    generationError:
      hasInFlightGeneration || shouldPreserveSavedGenerationError
        ? saved.generationError
        : "",
    createdTasks:
      detailCreatedTasks.length > 0 ? detailCreatedTasks : saved.createdTasks,
    persistedUpdatedAt:
      itemizedDraftUpdatedAt ||
      savedDraftUpdatedAt ||
      savedBatch.updatedAt,
    itemizedBatchDetail: detail,
  };
}

type WorkbenchSavedBatchProjectionInput = {
  id: string;
  prompt: string;
  promptMode?: "managed" | "raw";
  styleCount: string;
  variationIntensity: SheinStudioVariationIntensity;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  artworkModel: SheinStudioArtworkModel;
  transparentBackground: boolean;
  sheinStoreId: string;
  imageStrategy: SheinStudioImageStrategy;
  groupedImageMode: SheinStudioGroupedImageMode;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  renderSizeImagesWithSds: boolean;
  selection?: SDSProductVariantSelection;
  groupedSelections: GroupedSDSSelectionEligibility[];
  groups: SheinStudioGroupedWorkspace[];
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
  generationJobs: SheinStudioGenerationJob[];
  updatedAt: string;
  name?: string;
};

export function projectWorkbenchStateToSavedBatch({
  id,
  prompt,
  styleCount,
  variationIntensity,
  productImageCount,
  productImagePrompt,
  productImagePrompts,
  artworkModel,
  transparentBackground,
  sheinStoreId,
  imageStrategy,
  groupedImageMode,
  selectedSdsImages,
  renderSizeImagesWithSds,
  selection,
  groupedSelections,
  groups,
  designs,
  selectedIds,
  createdTasks,
  generationJobs,
  updatedAt,
  name = "",
  promptMode,
}: WorkbenchSavedBatchProjectionInput): SheinStudioSavedBatch {
  const normalizedPromptMode = promptMode ?? "managed";
  return {
    id,
    name,
    prompt,
    promptMode: normalizedPromptMode,
    styleCount,
    variationIntensity,
    productImageCount,
    productImagePrompt,
    productImagePrompts,
    artworkModel,
    transparentBackground,
    sheinStoreId,
    imageStrategy,
    groupedImageMode,
    selectedSdsImages,
    renderSizeImagesWithSds,
    selection,
    groupedSelections,
    groups,
    designs,
    selectedIds,
    createdTasks,
    generationJobs,
    updatedAt,
  };
}

export function projectWorkbenchStateFallback(
  state: SheinStudioWorkbenchState,
): Omit<WorkbenchSavedBatchProjectionInput, "id"> {
  return {
    artworkModel: state.artworkModel,
    createdTasks: state.createdTasks,
    designs: state.designs,
    generationJobs: state.generationJobs,
    groupedImageMode: state.groupedImageMode,
    groupedSelections: state.groupedSelections,
    groups: state.groups,
    imageStrategy: state.imageStrategy,
    productImageCount: state.productImageCount,
    productImagePrompt: state.productImagePrompt,
    productImagePrompts: state.productImagePrompts,
    prompt: state.prompt,
    promptMode: state.promptMode,
    renderSizeImagesWithSds: state.renderSizeImagesWithSds,
    selectedIds: state.selectedIds,
    selectedSdsImages: state.selectedSdsImages,
    selection: state.selection,
    sheinStoreId: state.sheinStoreId,
    styleCount: state.styleCount,
    transparentBackground: state.transparentBackground,
    updatedAt: state.persistedUpdatedAt,
    variationIntensity: state.variationIntensity,
  };
}

export function resolveCurrentSheinStudioSavedBatch({
  activeBatchId,
  fallback,
  initialBatchId,
  savedBatches,
}: {
  activeBatchId: string;
  fallback: Omit<WorkbenchSavedBatchProjectionInput, "id">;
  initialBatchId?: string;
  savedBatches: SheinStudioSavedBatch[];
}): SheinStudioSavedBatch | null {
  const resolvedBatchId = activeBatchId || initialBatchId || "";
  if (!resolvedBatchId) {
    return null;
  }
  return (
    savedBatches.find((item) => item.id === resolvedBatchId) ??
    projectWorkbenchStateToSavedBatch({
      ...fallback,
      id: resolvedBatchId,
    })
  );
}

export function selectCurrentDedicatedBatch({
  currentActiveBatch,
  initialBatchId,
}: {
  currentActiveBatch: SheinStudioSavedBatch | null;
  initialBatchId?: string;
}) {
  return initialBatchId && currentActiveBatch?.id === initialBatchId
    ? currentActiveBatch
    : null;
}

export function projectDefaultSelectedSDSImages({
  availableSdsImages,
  currentSelectedSdsImages,
  hasCustomizedSdsSelection,
  imageStrategy,
  renderSizeImagesWithSds,
}: {
  availableSdsImages: SheinStudioSelectableSDSImage[];
  currentSelectedSdsImages: SheinStudioSelectedSDSImage[];
  hasCustomizedSdsSelection: boolean;
  imageStrategy: SheinStudioImageStrategy;
  renderSizeImagesWithSds: boolean;
}): SheinStudioSelectedSDSImage[] | null {
  if (imageStrategy !== "hybrid" && imageStrategy !== "sds_official") {
    return null;
  }
  if (hasCustomizedSdsSelection) {
    return null;
  }
  const nextDefaults = buildDefaultSelectedSDSImages(availableSdsImages, {
    includeSizeReferenceImages: renderSizeImagesWithSds,
  });
  return JSON.stringify(currentSelectedSdsImages) === JSON.stringify(nextDefaults)
    ? null
    : nextDefaults;
}

export function projectWorkbenchTraceContext({
  batchQueueMode,
  queuedBatchIds,
  queuedBatchIndex,
  traceBatchId,
}: {
  batchQueueMode: SheinStudioBatchQueueMode | null;
  queuedBatchIds: string[];
  queuedBatchIndex: number;
  traceBatchId: string;
}) {
  return {
    batchId: traceBatchId || undefined,
    queueMode: batchQueueMode ?? undefined,
    queueIndex: batchQueueMode ? queuedBatchIndex + 1 : undefined,
    queueTotal: batchQueueMode ? queuedBatchIds.length : undefined,
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
    promptMode: draft?.promptMode ?? "managed",
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
    generationError: draft?.generationError ?? "",
    createdTasks,
    hasCustomizedSdsSelection: (draft?.selectedSdsImages?.length ?? 0) > 0,
    importedGalleryDesign: Boolean(galleryDesign),
    designCount: designs.length,
    createdTaskCount: createdTasks.length,
  };
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
  return "";
}
