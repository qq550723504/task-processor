import { useMemo } from "react";

import {
  flattenItemizedBatchDesigns,
  getApprovedItemizedBatchDesignIDs,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import type { SheinStudioBatchTaskCreationResult } from "@/lib/api/shein-studio-batches";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioArtworkModel,
  SheinStudioBatchDetail,
  SheinStudioBatchQueueMode,
  SheinStudioCreatedTask,
  SheinStudioGenerationJob,
  SheinStudioGroupedImageMode,
  SheinStudioGroupedWorkspace,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSavedBatch,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";

type ItemizedTaskCreationProjectionInput = {
  activeBatchId: string;
  activeSelection?: SDSProductVariantSelection;
  artworkModel: SheinStudioArtworkModel;
  currentActiveBatch?: Partial<SheinStudioSavedBatch> | null;
  currentDetail: SheinStudioBatchDetail;
  generationJobs: SheinStudioGenerationJob[];
  groupedImageMode: SheinStudioGroupedImageMode;
  groupedSelections: GroupedSDSSelectionEligibility[];
  groups: SheinStudioGroupedWorkspace[];
  imageStrategy: SheinStudioImageStrategy;
  persistedUpdatedAt: string;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  prompt: string;
  renderSizeImagesWithSds: boolean;
  result: SheinStudioBatchTaskCreationResult;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  sheinStoreId: string;
  styleCount: string;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
};

type ItemizedBatchTaskContext = {
  batchId: string;
  tenantId?: string;
  detail: SheinStudioBatchDetail;
  onCreated: (result: SheinStudioBatchTaskCreationResult) => void;
};

type ItemizedBatchDetailProjectionInput = Omit<
  ItemizedTaskCreationProjectionInput,
  "currentDetail" | "result"
> & {
  createdTasks: SheinStudioCreatedTask[];
  detail: SheinStudioBatchDetail;
};

export function projectItemizedBatchDetail({
  activeBatchId,
  activeSelection,
  artworkModel,
  createdTasks,
  currentActiveBatch,
  detail,
  generationJobs,
  groupedImageMode,
  groupedSelections,
  groups,
  imageStrategy,
  persistedUpdatedAt,
  productImageCount,
  productImagePrompt,
  productImagePrompts,
  prompt,
  renderSizeImagesWithSds,
  selectedSdsImages,
  sheinStoreId,
  styleCount,
  transparentBackground,
  variationIntensity,
}: ItemizedBatchDetailProjectionInput): {
  detail: SheinStudioBatchDetail;
  savedBatch: SheinStudioSavedBatch;
} {
  return {
    detail,
    savedBatch: {
      ...(currentActiveBatch ?? {}),
      id: activeBatchId,
      name: currentActiveBatch?.name ?? "未命名批次",
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
      selection: activeSelection,
      groupedSelections,
      groups,
      designs: flattenItemizedBatchDesigns(detail),
      selectedIds: getApprovedItemizedBatchDesignIDs(detail),
      createdTasks,
      generationJobs,
      draftUpdatedAt: currentActiveBatch?.draftUpdatedAt || persistedUpdatedAt,
      updatedAt:
        detail.batch.updatedAt ||
        currentActiveBatch?.updatedAt ||
        persistedUpdatedAt,
    },
  };
}

export function projectItemizedTaskCreationResult({
  activeBatchId,
  activeSelection,
  artworkModel,
  currentActiveBatch,
  currentDetail,
  generationJobs,
  groupedImageMode,
  groupedSelections,
  groups,
  imageStrategy,
  persistedUpdatedAt,
  productImageCount,
  productImagePrompt,
  productImagePrompts,
  prompt,
  renderSizeImagesWithSds,
  result,
  selectedSdsImages,
  sheinStoreId,
  styleCount,
  transparentBackground,
  variationIntensity,
}: ItemizedTaskCreationProjectionInput): {
  detail: SheinStudioBatchDetail;
  savedBatch: SheinStudioSavedBatch;
} {
  const detail: SheinStudioBatchDetail = {
    batch: result.batch,
    items: result.items,
    createdTasks: result.createdTasks,
    reusedTasks: result.reusedTasks,
    rejectedTasks: result.rejectedTasks,
    failedTasks: result.failedTasks,
    statusGroups: result.statusGroups,
  };
  const availableTasks: SheinStudioCreatedTask[] = [
    ...result.createdTasks,
    ...(result.reusedTasks ?? []),
  ];
  const projected = projectItemizedBatchDetail({
    activeBatchId,
    activeSelection,
    artworkModel,
    createdTasks: availableTasks,
    currentActiveBatch,
    detail,
    generationJobs: [],
    groupedImageMode,
    groupedSelections,
    groups,
    imageStrategy,
    persistedUpdatedAt,
    productImageCount,
    productImagePrompt,
    productImagePrompts,
    prompt,
    renderSizeImagesWithSds,
    selectedSdsImages,
    sheinStoreId,
    styleCount,
    transparentBackground,
    variationIntensity,
  });
  return {
    detail,
    savedBatch: {
      ...projected.savedBatch,
      tenantId:
        result.batch.tenantId ??
        projected.savedBatch.tenantId ??
        currentDetail.batch.tenantId ??
        currentActiveBatch?.tenantId,
      updatedAt:
        currentActiveBatch?.updatedAt ||
        projected.savedBatch.updatedAt,
    },
  };
}

type ItemizedBatchContextParams = Omit<
  ItemizedTaskCreationProjectionInput,
  "currentDetail" | "result"
> & {
  applyHydratedBatch: (batch: {
    detail: SheinStudioBatchDetail;
    savedBatch: SheinStudioSavedBatch;
  }) => void;
  itemizedBatchDetail?: SheinStudioBatchDetail | null;
  setSavedBatches: (
    updater: (current: SheinStudioSavedBatch[]) => SheinStudioSavedBatch[],
  ) => void;
  upsertSavedBatch: (
    current: SheinStudioSavedBatch[],
    savedBatch: SheinStudioSavedBatch,
  ) => SheinStudioSavedBatch[];
};

export function useSheinStudioItemizedBatchContext({
  activeBatchId,
  activeSelection,
  applyHydratedBatch,
  artworkModel,
  currentActiveBatch,
  generationJobs,
  groupedImageMode,
  groupedSelections,
  groups,
  imageStrategy,
  itemizedBatchDetail,
  persistedUpdatedAt,
  productImageCount,
  productImagePrompt,
  productImagePrompts,
  prompt,
  renderSizeImagesWithSds,
  selectedSdsImages,
  setSavedBatches,
  sheinStoreId,
  styleCount,
  transparentBackground,
  upsertSavedBatch,
  variationIntensity,
}: ItemizedBatchContextParams): {
  itemizedBatchContext?: ItemizedBatchTaskContext;
} {
  const itemizedBatchContext = useMemo<ItemizedBatchTaskContext | undefined>(() => {
    if (!activeBatchId || !itemizedBatchDetail) {
      return undefined;
    }
    return {
      batchId: activeBatchId,
      tenantId: itemizedBatchDetail.batch.tenantId ?? currentActiveBatch?.tenantId,
      detail: itemizedBatchDetail,
      onCreated: (result) => {
        const { detail, savedBatch } = projectItemizedTaskCreationResult({
          activeBatchId,
          activeSelection,
          artworkModel,
          currentActiveBatch,
          currentDetail: itemizedBatchDetail,
          generationJobs,
          groupedImageMode,
          groupedSelections,
          groups,
          imageStrategy,
          persistedUpdatedAt,
          productImageCount,
          productImagePrompt,
          productImagePrompts,
          prompt,
          renderSizeImagesWithSds,
          result,
          selectedSdsImages,
          sheinStoreId,
          styleCount,
          transparentBackground,
          variationIntensity,
        });
        setSavedBatches((current) => upsertSavedBatch(current, savedBatch));
        applyHydratedBatch({
          savedBatch,
          detail,
        });
      },
    };
  }, [
    activeBatchId,
    activeSelection,
    applyHydratedBatch,
    artworkModel,
    currentActiveBatch,
    generationJobs,
    groupedImageMode,
    groupedSelections,
    groups,
    imageStrategy,
    itemizedBatchDetail,
    persistedUpdatedAt,
    productImageCount,
    productImagePrompt,
    productImagePrompts,
    prompt,
    renderSizeImagesWithSds,
    selectedSdsImages,
    setSavedBatches,
    sheinStoreId,
    styleCount,
    transparentBackground,
    upsertSavedBatch,
    variationIntensity,
  ]);

  return {
    itemizedBatchContext,
  };
}
