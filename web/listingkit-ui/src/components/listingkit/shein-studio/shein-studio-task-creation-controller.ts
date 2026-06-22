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
  return {
    detail,
    savedBatch: {
      ...(currentActiveBatch ?? {}),
      id: activeBatchId,
      tenantId:
        result.batch.tenantId ??
        currentDetail.batch.tenantId ??
        currentActiveBatch?.tenantId,
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
      createdTasks: availableTasks,
      generationJobs: [],
      draftUpdatedAt: currentActiveBatch?.draftUpdatedAt || persistedUpdatedAt,
      updatedAt:
        currentActiveBatch?.updatedAt ||
        detail.batch.updatedAt ||
        persistedUpdatedAt,
    },
  };
}
