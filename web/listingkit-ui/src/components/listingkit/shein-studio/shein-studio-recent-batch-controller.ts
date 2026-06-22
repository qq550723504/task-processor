import type { SheinStudioSavedBatch } from "@/lib/types/shein-studio";

export function buildRecentBatchSaveInput(
  batch: SheinStudioSavedBatch,
  overrides?: Partial<SheinStudioSavedBatch>,
) {
  return {
    id: overrides?.id ?? batch.id,
    updatedAt:
      overrides?.draftUpdatedAt ??
      overrides?.updatedAt ??
      batch.draftUpdatedAt ??
      batch.updatedAt,
    name: overrides?.name ?? batch.name,
    prompt: overrides?.prompt ?? batch.prompt,
    styleCount: overrides?.styleCount ?? batch.styleCount,
    variationIntensity:
      overrides?.variationIntensity ?? batch.variationIntensity,
    productImageCount: overrides?.productImageCount ?? batch.productImageCount,
    productImagePrompt:
      overrides?.productImagePrompt ?? batch.productImagePrompt,
    productImagePrompts:
      overrides?.productImagePrompts ?? batch.productImagePrompts,
    artworkModel: overrides?.artworkModel ?? batch.artworkModel,
    transparentBackground:
      overrides?.transparentBackground ?? batch.transparentBackground,
    sheinStoreId: overrides?.sheinStoreId ?? batch.sheinStoreId,
    imageStrategy: overrides?.imageStrategy ?? batch.imageStrategy,
    groupedImageMode:
      overrides?.groupedImageMode ?? batch.groupedImageMode,
    selectedSdsImages:
      overrides?.selectedSdsImages ?? batch.selectedSdsImages,
    renderSizeImagesWithSds:
      overrides?.renderSizeImagesWithSds ?? batch.renderSizeImagesWithSds,
    selection: overrides?.selection ?? batch.selection,
    groupedSelections:
      overrides?.groupedSelections ?? batch.groupedSelections,
    groups: overrides?.groups ?? batch.groups,
    designs: overrides?.designs ?? batch.designs,
    selectedIds: overrides?.selectedIds ?? batch.selectedIds,
    createdTasks: overrides?.createdTasks ?? batch.createdTasks,
    generationJobs: overrides?.generationJobs ?? batch.generationJobs,
    generationError: overrides?.generationError ?? batch.generationError,
    generationJobId: overrides?.generationJobId ?? batch.generationJobId,
  };
}

export function buildRecentBatchStoreUpdateInput(
  batch: SheinStudioSavedBatch,
  storeId: string,
) {
  return buildRecentBatchSaveInput(batch, {
    sheinStoreId: storeId,
    groupedSelections: (batch.groupedSelections ?? []).map((item) => ({
      ...item,
      sheinStoreId: storeId,
    })),
    groups: (batch.groups ?? []).map((group) => ({
      ...group,
      sheinStoreId: storeId,
      groupedSelections: group.groupedSelections.map((item) => ({
        ...item,
        sheinStoreId: storeId,
      })),
    })),
  });
}
