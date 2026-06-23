import type { SheinStudioSavedBatch } from "@/lib/types/shein-studio";
import type { SheinStudioWorkbenchHydratedBatch } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";

type RecentBatchSelectionSummary = {
  id: string;
  source: "batch" | "local_draft";
};

type RecentBatchSelectionProjectionParams = {
  rawSelectedRecentBatchSummaryIds: string[];
  validRecentBatchSummaryKeys: Set<string>;
};

type FreshRecentBatchHydrationParams = {
  cachedHydratedBatch?: SheinStudioWorkbenchHydratedBatch | null;
  savedBatch: SheinStudioSavedBatch;
};

export function buildRecentBatchSummaryKey(
  summary: RecentBatchSelectionSummary,
): string {
  return `${summary.source}:${summary.id}`;
}

export function buildRecentBatchSummaryKeys(
  recentBatchSummaries: RecentBatchSelectionSummary[],
): Set<string> {
  return new Set(
    recentBatchSummaries.map((summary) => buildRecentBatchSummaryKey(summary)),
  );
}

export function removeRecentBatchSummarySelection(
  current: string[],
  summary: RecentBatchSelectionSummary,
): string[] {
  const key = buildRecentBatchSummaryKey(summary);
  return current.filter((item) => item !== key);
}

export function selectFreshRecentBatchHydration({
  cachedHydratedBatch,
  savedBatch,
}: FreshRecentBatchHydrationParams): SheinStudioWorkbenchHydratedBatch | null {
  if (
    cachedHydratedBatch &&
    Date.parse(cachedHydratedBatch.savedBatch.updatedAt) >=
      Date.parse(savedBatch.updatedAt)
  ) {
    return cachedHydratedBatch;
  }
  return null;
}

export function projectRecentBatchSelectionState({
  rawSelectedRecentBatchSummaryIds,
  validRecentBatchSummaryKeys,
}: RecentBatchSelectionProjectionParams) {
  const selectedRecentBatchSummaryIds =
    rawSelectedRecentBatchSummaryIds.filter((key) =>
      validRecentBatchSummaryKeys.has(key),
    );
  const selectedPersistedRecentBatchIds =
    selectedRecentBatchSummaryIds.flatMap((key) => {
      const batchPrefix = "batch:";
      return key.startsWith(batchPrefix)
        ? [key.slice(batchPrefix.length)]
        : [];
    });

  return {
    selectedPersistedRecentBatchIds,
    selectedRecentBatchSummaryIds,
    validRecentBatchSummaryKeys,
  };
}

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
    promptMode: overrides?.promptMode ?? batch.promptMode,
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

export function buildRecentBatchBulkStoreUpdateInputs(
  batches: SheinStudioSavedBatch[],
  storeId: string,
) {
  return batches.map((batch) => buildRecentBatchStoreUpdateInput(batch, storeId));
}

function isMissingRecentBatchDeleteError(error: unknown) {
  return error instanceof Error && /studio session not found/i.test(error.message);
}

export function selectRecentBatchBulkDeleteFailure(
  results: PromiseSettledResult<unknown>[],
): unknown | null {
  const failed = results.find(
    (result) =>
      result.status === "rejected" &&
      !isMissingRecentBatchDeleteError(result.reason),
  );
  return failed?.status === "rejected" ? failed.reason : null;
}
