import { useMemo } from "react";

import {
  flattenItemizedBatchDesigns,
  getApprovedItemizedBatchDesignIDs,
  hasInFlightItemizedBatchGeneration,
  toggleItemizedBatchDesignApproval,
  toggleSelectedDesignId,
  updateFlatDesignReviewNote,
  updateItemizedBatchDesignReviewNote,
  type SheinStudioWorkbenchHydratedBatch,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import { upsertRecentSavedBatch } from "@/components/listingkit/shein-studio/shein-studio-recent-batch-controller";
import type { SheinStudioBatchTaskCreationResult } from "@/lib/api/shein-studio-batches";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioArtworkModel,
  SheinStudioBatchDetail,
  SheinStudioBatchQueueMode,
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
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

type ItemizedTaskCreationProgressInput = {
  creatingMessage: string;
  detail: SheinStudioBatchDetail;
  isCreatingTasks: boolean;
};

type ItemizedTaskRecoveryStateInput = {
  detail?: SheinStudioBatchDetail | null;
  generationInFlight: boolean;
  pendingTaskDesignIds: string[];
};

type ItemizedDesignApprovalRequestInput = {
  activeBatchId: string;
  currentActiveBatch?: Partial<SheinStudioSavedBatch> | null;
  detail?: SheinStudioBatchDetail | null;
  selectedIds: string[];
};

type ItemizedReviewNoteUpdateInput = {
  activeBatchId: string;
  designs: SheinStudioGeneratedDesign[];
  detail?: SheinStudioBatchDetail | null;
  designId: string;
  note: string;
};

type ItemizedSelectionToggleInput = {
  activeBatchId: string;
  detail?: SheinStudioBatchDetail | null;
  designId: string;
  selectedIds: string[];
};

type ItemizedDesignApprovalRunnerInput = ItemizedDesignApprovalRequestInput & {
  approveDesigns: (
    batchId: string,
    selectedIds: string[],
    options?: { tenantId?: string },
  ) => Promise<SheinStudioBatchDetail | null>;
};

type ItemizedFailedRetryRequestInput = {
  activeBatchId: string;
  currentActiveBatch?: Partial<SheinStudioSavedBatch> | null;
  detail?: SheinStudioBatchDetail | null;
  itemId: string;
};

type ItemizedFailedRetryRunnerInput = ItemizedFailedRetryRequestInput & {
  retryItems: (
    batchId: string,
    itemIds: string[],
    options?: { tenantId?: string },
  ) => Promise<SheinStudioBatchDetail>;
};

type ItemizedGenerationPollBatchInput = {
  activeBatchId: string;
  getHydratedBatch: (
    batchId: string,
  ) => Promise<SheinStudioWorkbenchHydratedBatch | null>;
};

type ItemizedTaskCreationProgress =
  | {
      completionSignature: string;
      creatingMessage?: string;
      isCreatingTasks: true;
      kind: "creating";
    }
  | {
      completionSignature: string;
      creatingMessage: string;
      creatingWarning: string;
      isCreatingTasks: false;
      kind: "completed";
      toast: {
        duration: number;
        message: string;
        title: string;
        type: "error" | "success" | "warning";
      };
    }
  | {
      kind: "unchanged";
    };

type ItemizedTaskCreationProgressEffectsInput = {
  currentCompletionSignature: string;
  progress: ItemizedTaskCreationProgress;
};

type ItemizedTaskCreationProgressEffects =
  | {
      kind: "unchanged";
    }
  | {
      completionSignature?: string;
      fields: {
        creatingMessage?: string;
        creatingWarning?: string;
        isCreatingTasks: boolean;
      };
      kind: "apply";
      toast?: Extract<ItemizedTaskCreationProgress, { kind: "completed" }>["toast"];
    };

export function projectItemizedTaskCreationProgress({
  creatingMessage,
  detail,
  isCreatingTasks,
}: ItemizedTaskCreationProgressInput): ItemizedTaskCreationProgress {
  const failedTasks = detail.failedTasks ?? [];
  const rejectedTasks = detail.rejectedTasks ?? [];
  const reusedBatchTasks = detail.reusedTasks ?? [];
  const createdBatchTasks = detail.createdTasks ?? [];
  const availableBatchTasks = [...createdBatchTasks, ...reusedBatchTasks];
  const completionSignature = `${detail.batch.id}:${detail.batch.status}:${createdBatchTasks.length}:${reusedBatchTasks.length}:${rejectedTasks.length}:${failedTasks.length}`;

  if (detail.batch.status === "tasks_creating") {
    return {
      completionSignature,
      creatingMessage: creatingMessage.trim()
        ? undefined
        : "已开始在后台创建 SHEIN 资料，可离开当前页面，结果会自动刷新。",
      isCreatingTasks: true,
      kind: "creating",
    };
  }
  if (!isCreatingTasks || detail.batch.status !== "tasks_created") {
    return { kind: "unchanged" };
  }
  if (failedTasks.length > 0 || rejectedTasks.length > 0) {
    const rejectedPreview = rejectedTasks
      .slice(0, 3)
      .map(
        (task) =>
          `${task.title?.trim() || task.designId}: ${
            task.reasonCode ? `${task.reasonCode} · ` : ""
          }${task.message ?? "候选不满足创建条件"}`,
      )
      .join("；");
    const failedPreview = failedTasks
      .slice(0, Math.max(0, 3 - rejectedTasks.length))
      .map(
        (task) =>
          `${task.title}: ${task.reasonCode ? `${task.reasonCode} · ` : ""}${
            task.message
          }`,
      )
      .join("；");
    const preview = [rejectedPreview, failedPreview].filter(Boolean).join("；");
    const blockedCount = failedTasks.length + rejectedTasks.length;
    const suffix = blockedCount > 3 ? ` 等 ${blockedCount} 个任务` : "";
    return {
      completionSignature,
      creatingMessage:
        availableBatchTasks.length > 0
          ? `后台已完成创建：可处理 ${availableBatchTasks.length} 个，拒绝 ${rejectedTasks.length} 个，失败 ${failedTasks.length} 个。`
          : "后台任务创建已结束，但本次没有成功创建任务。",
      creatingWarning: `部分任务被拒绝或创建失败：${preview}${suffix}`,
      isCreatingTasks: false,
      kind: "completed",
      toast:
        availableBatchTasks.length > 0
          ? {
              duration: 8000,
              message: `可处理 ${availableBatchTasks.length} 个，拒绝 ${rejectedTasks.length} 个，失败 ${failedTasks.length} 个。`,
              title: "SHEIN 资料已部分创建",
              type: "warning",
            }
          : {
              duration: 8000,
              message: "本次没有成功创建任务。",
              title: "SHEIN 资料创建失败",
              type: "error",
            },
    };
  }
  return {
    completionSignature,
    creatingMessage: `后台已完成创建，共生成或复用 ${availableBatchTasks.length} 个 SHEIN 任务。`,
    creatingWarning: "",
    isCreatingTasks: false,
    kind: "completed",
    toast: {
      duration: 7000,
      message: `共生成或复用 ${availableBatchTasks.length} 个任务。`,
      title: "SHEIN 资料创建完成",
      type: "success",
    },
  };
}

export function projectItemizedTaskCreationProgressEffects({
  currentCompletionSignature,
  progress,
}: ItemizedTaskCreationProgressEffectsInput): ItemizedTaskCreationProgressEffects {
  if (progress.kind === "unchanged") {
    return { kind: "unchanged" };
  }
  if (progress.kind === "creating") {
    return {
      completionSignature:
        currentCompletionSignature !== progress.completionSignature
          ? progress.completionSignature
          : undefined,
      fields: {
        ...(progress.creatingMessage
          ? { creatingMessage: progress.creatingMessage }
          : {}),
        isCreatingTasks: true,
      },
      kind: "apply",
    };
  }
  const isNewCompletion =
    currentCompletionSignature !== progress.completionSignature;
  return {
    completionSignature: isNewCompletion
      ? progress.completionSignature
      : undefined,
    fields: {
      creatingMessage: progress.creatingMessage,
      creatingWarning: progress.creatingWarning,
      isCreatingTasks: false,
    },
    kind: "apply",
    toast: isNewCompletion ? progress.toast : undefined,
  };
}

export function projectItemizedDesignApprovalRequest({
  activeBatchId,
  currentActiveBatch,
  detail,
  selectedIds,
}: ItemizedDesignApprovalRequestInput): {
  batchId: string;
  selectedIds: string[];
  tenantId?: string;
} | null {
  if (!activeBatchId || !detail) {
    return null;
  }
  const tenantId =
    detail.batch.tenantId?.trim() ?? currentActiveBatch?.tenantId?.trim();
  return {
    batchId: activeBatchId,
    selectedIds,
    tenantId: tenantId || undefined,
  };
}

export function projectItemizedReviewNoteUpdate({
  activeBatchId,
  designs,
  detail,
  designId,
  note,
}: ItemizedReviewNoteUpdateInput):
  | {
      detail: SheinStudioBatchDetail;
      kind: "itemized";
    }
  | {
      designs: SheinStudioGeneratedDesign[];
      kind: "flat";
    } {
  if (activeBatchId && detail) {
    return {
      detail: updateItemizedBatchDesignReviewNote(detail, designId, note),
      kind: "itemized",
    };
  }
  return {
    designs: updateFlatDesignReviewNote(designs, designId, note),
    kind: "flat",
  };
}

export function projectItemizedSelectionToggle({
  activeBatchId,
  detail,
  designId,
  selectedIds,
}: ItemizedSelectionToggleInput):
  | {
      detail: SheinStudioBatchDetail;
      kind: "itemized";
      selectedIds: string[];
    }
  | {
      kind: "flat";
      selectedIds: string[];
    } {
  const nextSelectedIds = toggleSelectedDesignId(selectedIds, designId);
  if (activeBatchId && detail) {
    return {
      detail: toggleItemizedBatchDesignApproval(detail, designId),
      kind: "itemized",
      selectedIds: nextSelectedIds,
    };
  }
  return {
    kind: "flat",
    selectedIds: nextSelectedIds,
  };
}

export async function runItemizedDesignApproval({
  activeBatchId,
  approveDesigns,
  currentActiveBatch,
  detail,
  selectedIds,
}: ItemizedDesignApprovalRunnerInput): Promise<SheinStudioBatchDetail | null> {
  const approvalRequest = projectItemizedDesignApprovalRequest({
    activeBatchId,
    currentActiveBatch,
    detail,
    selectedIds,
  });
  if (!approvalRequest) {
    return null;
  }
  if (approvalRequest.tenantId) {
    return approveDesigns(approvalRequest.batchId, approvalRequest.selectedIds, {
      tenantId: approvalRequest.tenantId,
    });
  }
  return approveDesigns(approvalRequest.batchId, approvalRequest.selectedIds);
}

export function projectItemizedTaskRecoveryState({
  detail,
  generationInFlight,
  pendingTaskDesignIds,
}: ItemizedTaskRecoveryStateInput): {
  dedicatedGenerateButtonLabel: string;
  hasRetryableFailedItems: boolean;
  retryableFailedItemCount: number;
  shouldPrioritizeTaskCreationRecovery: boolean;
} {
  const retryableFailedItemCount =
    detail?.items.filter((entry) => entry.item.status === "failed").length ?? 0;
  const hasRetryableFailedItems =
    retryableFailedItemCount > 0 &&
    (detail?.batch.status === "partially_failed" || detail?.batch.status === "failed");

  return {
    dedicatedGenerateButtonLabel:
      hasRetryableFailedItems && retryableFailedItemCount > 0
        ? `重试失败款式 ${retryableFailedItemCount} 个`
        : "继续生成剩余款式",
    hasRetryableFailedItems,
    retryableFailedItemCount,
    shouldPrioritizeTaskCreationRecovery:
      pendingTaskDesignIds.length > 0 && !hasRetryableFailedItems && !generationInFlight,
  };
}

export function projectItemizedFailedRetryRequest({
  activeBatchId,
  currentActiveBatch,
  detail,
  itemId,
}: ItemizedFailedRetryRequestInput): {
  batchId: string;
  itemIds: string[];
  tenantId?: string;
} | null {
  if (!activeBatchId || !detail) {
    return null;
  }
  const failedEntry = detail.items.find(
    (entry) => entry.item.id === itemId && entry.item.status === "failed",
  );
  if (!failedEntry) {
    return null;
  }
  const tenantId =
    detail.batch.tenantId?.trim() || currentActiveBatch?.tenantId?.trim();
  return {
    batchId: activeBatchId,
    itemIds: [itemId],
    tenantId: tenantId || undefined,
  };
}

export async function runItemizedFailedRetry({
  activeBatchId,
  currentActiveBatch,
  detail,
  itemId,
  retryItems,
}: ItemizedFailedRetryRunnerInput): Promise<SheinStudioBatchDetail | null> {
  const retryRequest = projectItemizedFailedRetryRequest({
    activeBatchId,
    currentActiveBatch,
    detail,
    itemId,
  });
  if (!retryRequest) {
    return null;
  }
  if (retryRequest.tenantId) {
    return retryItems(retryRequest.batchId, retryRequest.itemIds, {
      tenantId: retryRequest.tenantId,
    });
  }
  return retryItems(retryRequest.batchId, retryRequest.itemIds);
}

export async function loadItemizedGenerationPollBatch({
  activeBatchId,
  getHydratedBatch,
}: ItemizedGenerationPollBatchInput): Promise<SheinStudioWorkbenchHydratedBatch | null> {
  try {
    return await getHydratedBatch(activeBatchId);
  } catch {
    return null;
  }
}

export function projectItemizedFailedRetryStep(
  detail: SheinStudioBatchDetail,
): "generate" | null {
  if (
    detail.batch.status === "generating" ||
    hasInFlightItemizedBatchGeneration(detail)
  ) {
    return "generate";
  }
  return null;
}

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
  upsertSavedBatch?: (
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
  upsertSavedBatch = upsertRecentSavedBatch,
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
