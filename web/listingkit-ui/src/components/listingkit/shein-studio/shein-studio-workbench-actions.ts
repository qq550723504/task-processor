import type { MutableRefObject, RefObject } from "react";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import { useSheinStudioTaskCreationAction } from "@/components/listingkit/shein-studio/shein-studio-task-creation-actions";
import type { SheinStudioWorkbenchController } from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import {
  buildSheinStudioGenerateRequest,
  hasInFlightItemizedBatchGeneration,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import { generateSheinStudioDesigns } from "@/lib/api/shein-studio";
import {
  generateSheinStudioBatch,
  retrySheinStudioBatchItems,
} from "@/lib/api/shein-studio-batches";
import { formatSubscriptionApiError } from "@/lib/api/subscription";
import {
  buildGroupedGenerationTargets,
  resolveDesignTargetKey,
} from "@/lib/shein-studio/grouped-image-mode";
import {
  buildGenerationPromptHistoryGroups,
  mergeGeneratedDesignCollections,
  mergeGeneratedSelectedIds,
  replaceRegeneratedDesign,
  resolveGenerationStartValidation,
  resolveRegenerationStartValidation,
  withGenerationTargetMetadata,
} from "@/lib/shein-studio/generation-controller";
import { parsePositiveInt } from "@/lib/shein-studio/create-review-tasks";
import { buildSDSProductReferenceImageUrls } from "@/lib/shein-studio/sds-reference-images";
import {
  beginListingKitTraceRun,
  logListingKitTraceEvent,
  writeListingKitTraceContext,
} from "@/lib/listingkit/request-trace";
import type {
  GroupedSDSSelectionEligibility,
  SDSBaselineStatus,
} from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioArtworkModel,
  SheinStudioBatchDetail,
  SheinStudioBatchQueueMode,
  SheinStudioCreatedTask,
  SheinStudioGenerationJob,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioGroupedWorkspace,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioSavedBatch,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";
import type { SheinStudioBatchTaskCreationResult } from "@/lib/api/shein-studio-batches";

type PersistDraft = (
  overrides?: Partial<{
    designs: SheinStudioGeneratedDesign[];
    groups: SheinStudioGroupedWorkspace[];
    selectedIds: string[];
    createdTasks: SheinStudioCreatedTask[];
    generationJobs: SheinStudioGenerationJob[];
  }>,
  options?: {
    navigationTriggered?: boolean;
    source?: string;
    signal?: AbortSignal;
    warnOnFailure?: boolean;
  },
) => Promise<unknown>;

type BatchGenerationContext = {
  ensureBatch: () => Promise<SheinStudioSavedBatch | null>;
  detail?: SheinStudioBatchDetail | null;
  onGenerated: (result: {
    savedBatch: SheinStudioSavedBatch;
    detail: SheinStudioBatchDetail;
  }) => void;
  recoverInFlightGeneration?: (input: {
    batchId: string;
    error: unknown;
  }) => Promise<boolean>;
};

type UseSheinStudioDesignActionsParams = {
  activeGroupId: string;
  activeSelection?: SDSProductVariantSelection;
  artworkModel: SheinStudioArtworkModel;
  designs: SheinStudioGeneratedDesign[];
  groups: SheinStudioGroupedWorkspace[];
  groupedImageMode: SheinStudioGroupedImageMode;
  imageStrategy: SheinStudioImageStrategy;
  navigateToStep: (step: SheinStudioStepKey) => void;
  persistDraft: PersistDraft;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  prompt: string;
  promptInputRef: RefObject<HTMLTextAreaElement | null>;
  renderSizeImagesWithSds: boolean;
  selectedIds: string[];
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  groupedSelections: GroupedSDSSelectionEligibility[];
  generationJobs: SheinStudioGenerationJob[];
  activeSelectionBaselineStatus: SDSBaselineStatus;
  activeSelectionBaselineReason: string;
  workbench: Pick<SheinStudioWorkbenchController, "setField">;
  batchGenerationContext?: BatchGenerationContext;
  sheinStoreId: string;
  styleCount: string;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
  hasLocalWorkflowStateRef: MutableRefObject<boolean>;
  batchTraceContext: {
    batchId?: string;
    queueMode?: SheinStudioBatchQueueMode | null;
    queueIndex?: number;
    queueTotal?: number;
  };
  itemizedBatchContext?: {
    batchId: string;
    tenantId?: string;
    detail: SheinStudioBatchDetail;
    onCreated: (result: SheinStudioBatchTaskCreationResult) => void;
  };
};

export function useSheinStudioDesignActions({
  activeGroupId,
  activeSelection,
  artworkModel,
  designs,
  groups,
  groupedImageMode,
  imageStrategy,
  navigateToStep,
  persistDraft,
  productImageCount,
  productImagePrompt,
  productImagePrompts,
  prompt,
  promptInputRef,
  renderSizeImagesWithSds,
  selectedIds,
  selectedSdsImages,
  groupedSelections,
  generationJobs,
  activeSelectionBaselineStatus,
  activeSelectionBaselineReason,
  workbench,
  batchGenerationContext,
  sheinStoreId,
  styleCount,
  transparentBackground,
  variationIntensity,
  hasLocalWorkflowStateRef,
  batchTraceContext,
  itemizedBatchContext,
}: UseSheinStudioDesignActionsParams) {
  const { handleCreateTasks } = useSheinStudioTaskCreationAction({
    activeSelection,
    designs,
    imageStrategy,
    groupedImageMode,
    navigateToStep,
    persistDraft,
    productImageCount,
    productImagePrompt,
    productImagePrompts,
    prompt,
    renderSizeImagesWithSds,
    selectedIds,
    selectedSdsImages,
    groupedSelections,
    activeSelectionBaselineStatus,
    activeSelectionBaselineReason,
    setCreatedTasks: (value) => workbench.setField("createdTasks", value),
    setCreatingError: (value) => workbench.setField("creatingError", value),
    setCreatingMessage: (value) => workbench.setField("creatingMessage", value),
    setCreatingWarning: (value) => workbench.setField("creatingWarning", value),
    setGalleryRatioCheck: (value) =>
      workbench.setField("galleryRatioCheck", value),
    setIsCreatingTasks: (value) => workbench.setField("isCreatingTasks", value),
    sheinStoreId,
    hasLocalWorkflowStateRef,
    itemizedBatchContext,
  });

  async function handleGenerate() {
    const startValidation = resolveGenerationStartValidation({
      activeSelection,
      prompt,
      sheinStoreId,
    });
    if (startValidation) {
      workbench.setField("generationError", startValidation.error);
      if (startValidation.focusPrompt) {
        promptInputRef.current?.scrollIntoView({
          behavior: "smooth",
          block: "center",
        });
        promptInputRef.current?.focus();
      }
      return;
    }
    const generationSelection = activeSelection;
    if (!generationSelection) {
      return;
    }

    workbench.setField("generationError", "");
    workbench.setField("generationWarning", "");
    workbench.setField("creatingError", "");
    workbench.setField("creatingWarning", "");
    workbench.setField("creatingMessage", "");
    workbench.setField("createdTasks", []);
    workbench.setField("generationJobs", []);
    workbench.setField("draftWarning", "");
    workbench.setField("isGenerating", true);
    const traceContext = beginListingKitTraceRun({
      batchId: batchTraceContext.batchId,
      queueMode: batchTraceContext.queueMode ?? undefined,
      queueIndex: batchTraceContext.queueIndex,
      queueTotal: batchTraceContext.queueTotal,
    });
    logListingKitTraceEvent("info", "studio generation started", {
      promptLength: prompt.trim().length,
      selectionVariantId: generationSelection.variantId,
      styleCount,
      traceContext,
    });
    const nextGroups = buildGenerationPromptHistoryGroups({
      activeGroupId,
      groupedImageMode,
      groups,
      prompt,
    });
    if (nextGroups !== groups) {
      workbench.setField("groups", nextGroups);
    }

    let generatedBatchID = "";
    try {
      if (batchGenerationContext) {
        const savedBatch = await batchGenerationContext.ensureBatch();
        if (!savedBatch?.id) {
          throw new Error("当前批次保存失败，请稍后重试。");
        }
        generatedBatchID = savedBatch.id;
        writeListingKitTraceContext({ batchId: savedBatch.id });
        const failedItemIDs =
          batchGenerationContext.detail?.items
            .filter((entry) => entry.item.status === "failed")
            .map((entry) => entry.item.id) ?? [];
        const detail =
          savedBatch.tenantId?.trim()
            ? failedItemIDs.length > 0
              ? await retrySheinStudioBatchItems(savedBatch.id, failedItemIDs, {
                  tenantId: savedBatch.tenantId,
                })
              : await generateSheinStudioBatch(savedBatch.id, {
                  tenantId: savedBatch.tenantId,
                })
            :
          failedItemIDs.length > 0
            ? await retrySheinStudioBatchItems(savedBatch.id, failedItemIDs)
            : await generateSheinStudioBatch(savedBatch.id);
        const generatedDesignCount = detail.items.reduce(
          (count, entry) => count + entry.designs.length,
          0,
        );
        if (
          generatedDesignCount === 0 &&
          !hasInFlightItemizedBatchGeneration(detail)
        ) {
          throw new Error(
            "款式图生成完成，但没有返回任何图片。请重试一次；如果持续出现，说明上游生成链路返回了空结果。",
          );
        }
        logListingKitTraceEvent("info", "studio batch generation completed", {
          batchId: savedBatch.id,
          designCount: generatedDesignCount,
        });
        console.info("[shein-studio] generation succeeded", {
          batchId: savedBatch.id,
          designCount: generatedDesignCount,
          draftSaveStatus: "succeeded",
          selectionVariantId: generationSelection.variantId,
        });
        hasLocalWorkflowStateRef.current = true;
        batchGenerationContext.onGenerated({
          savedBatch,
          detail,
        });
        navigateToStep("review");
        return;
      }

      if (!batchGenerationContext) {
        throw new Error("当前工作台尚未连接到批次生成链路，请刷新后重试。");
      }
      const targets = buildGroupedGenerationTargets({
        activeSelection: generationSelection,
        groupedSelections: groupedSelections
          .filter((item) => item.eligible)
          .map((item) => item.selection),
        groupedImageMode,
      });
      let nextGenerationJobs = generationJobs.filter(
        (job) => job.status === "running" && job.jobId.trim(),
      );
      let accumulatedDesigns: SheinStudioGeneratedDesign[] = [];
      let accumulatedSelectedIDs: string[] = [];
      const aggregatedWarnings: string[] = [];
      const targetErrors: string[] = [];

      const syncGenerationJobs = (jobs: SheinStudioGenerationJob[]) => {
        nextGenerationJobs = jobs;
        workbench.setField("generationJobs", jobs);
      };

      const persistProgress = async (
        incomingDesigns: SheinStudioGeneratedDesign[],
        nextSelectedIds: string[],
        jobs: SheinStudioGenerationJob[],
      ) => {
        await persistDraft(
          {
            designs: accumulatedDesigns,
            groups: nextGroups,
            selectedIds: nextSelectedIds,
            createdTasks: [],
            generationJobs: jobs,
          },
          {
            source: "generate_progress",
            warnOnFailure: false,
          },
        )
          .catch(() => undefined);
      };

      syncGenerationJobs([]);

      const settled = await Promise.allSettled(
        targets.map(async (target) => {
          const response = await generateSheinStudioDesigns(
            buildSheinStudioGenerateRequest({
              prompt: prompt.trim(),
              variationIntensity,
              printableWidth: target.selection.printableWidth,
              printableHeight: target.selection.printableHeight,
              productReferenceImageUrls:
                buildSDSProductReferenceImageUrls(target.selection),
              styleCount: parsePositiveInt(styleCount) ?? 1,
              artworkModel,
              transparentBackground,
            }),
            {
              onJobStarted: (jobId) => {
                const existingIndex = nextGenerationJobs.findIndex(
                  (job) => job.jobId === jobId,
                );
                const nextJob: SheinStudioGenerationJob = {
                  jobId,
                  targetGroupKey: target.key,
                  targetGroupLabel: target.label,
                  status: "running",
                };
                const jobs =
                  existingIndex >= 0
                    ? nextGenerationJobs.map((job, index) =>
                        index === existingIndex ? nextJob : job,
                      )
                    : [...nextGenerationJobs, nextJob];
                logListingKitTraceEvent("info", "studio async job started", {
                  jobId,
                  targetGroupKey: target.key,
                  targetGroupLabel: target.label,
                });
                syncGenerationJobs(jobs);
              },
            },
          );
          const nextImages = withGenerationTargetMetadata(response.images, target);
          const targetJobIndex = nextGenerationJobs.findIndex(
            (job) => job.targetGroupKey === target.key,
          );
          if (targetJobIndex >= 0) {
            const jobs: SheinStudioGenerationJob[] = nextGenerationJobs.map(
              (job, index) =>
                index === targetJobIndex
                  ? { ...job, status: "succeeded" as const }
                  : job,
            );
            syncGenerationJobs(jobs);
          }
          if (response.warnings?.length) {
            aggregatedWarnings.push(...response.warnings);
          }
          if (nextImages.length > 0) {
            accumulatedDesigns = mergeGeneratedDesignCollections(
              accumulatedDesigns,
              nextImages,
            );
            accumulatedSelectedIDs = mergeGeneratedSelectedIds(
              accumulatedSelectedIDs,
              nextImages,
            );
            hasLocalWorkflowStateRef.current = true;
            workbench.setField("designs", accumulatedDesigns);
            workbench.setField("selectedIds", accumulatedSelectedIDs);
            workbench.setField("createdTasks", []);
            navigateToStep("review");
            await persistProgress(
              nextImages,
              accumulatedSelectedIDs,
              nextGenerationJobs.filter((job) => job.status === "running"),
            );
          }
          return nextImages;
        }),
      );

      for (let index = 0; index < settled.length; index += 1) {
        const result = settled[index];
        if (result.status === "fulfilled") {
          continue;
        }
        const target = targets[index];
        const message = formatSubscriptionApiError(result.reason);
        targetErrors.push(target.label ? `${target.label}: ${message}` : message);
        const targetJobIndex = nextGenerationJobs.findIndex(
          (job) => job.targetGroupKey === target.key,
        );
        if (targetJobIndex >= 0) {
          const jobs: SheinStudioGenerationJob[] = nextGenerationJobs.map(
            (job, jobIndex) =>
              jobIndex === targetJobIndex
                ? { ...job, status: "failed" as const }
                : job,
          );
          syncGenerationJobs(jobs);
        }
      }

      if (!accumulatedDesigns.length) {
        throw new Error(
          "款式图生成完成，但没有返回任何图片。请重试一次；如果持续出现，说明上游生成链路返回了空结果。",
        );
      }
      console.info("[shein-studio] generation succeeded", {
        designCount: accumulatedDesigns.length,
        draftSaveStatus: "pending",
        selectionVariantId: generationSelection.variantId,
      });
      logListingKitTraceEvent("info", "studio generation completed", {
        designCount: accumulatedDesigns.length,
        warningCount: aggregatedWarnings.length,
        errorCount: targetErrors.length,
      });
      if (aggregatedWarnings.length > 0 || targetErrors.length > 0) {
        workbench.setField(
          "generationWarning",
          [...aggregatedWarnings, ...targetErrors].join(" "),
        );
      }
      hasLocalWorkflowStateRef.current = true;
      workbench.setField("designs", accumulatedDesigns);
      workbench.setField("selectedIds", accumulatedSelectedIDs);
      workbench.setField("generationJobs", []);
      navigateToStep("review");
      void persistDraft(
        {
          designs: accumulatedDesigns,
          groups: nextGroups,
          selectedIds: accumulatedSelectedIDs,
          createdTasks: [],
          generationJobs: [],
        },
        {
          navigationTriggered: true,
          source: "generate_success",
          warnOnFailure: false,
        },
      ).catch(() => undefined);
    } catch (error) {
      if (
        batchGenerationContext?.recoverInFlightGeneration &&
        generatedBatchID &&
        (await batchGenerationContext.recoverInFlightGeneration({
          batchId: generatedBatchID,
          error,
        }))
      ) {
        return;
      }
      const message = formatSubscriptionApiError(error);
      logListingKitTraceEvent("warn", "studio generation failed", {
        error: message,
      });
      workbench.setField("designs", []);
      workbench.setField("selectedIds", []);
      workbench.setField("generationJobs", []);
      workbench.setField("generationWarning", "");
      workbench.setField("generationError", message);
    } finally {
      workbench.setField("isGenerating", false);
    }
  }

  async function handleRegenerate(designId: string) {
    const startValidation = resolveRegenerationStartValidation({
      activeSelection,
      prompt,
    });
    if (startValidation) {
      workbench.setField("generationError", startValidation.error);
      if (startValidation.focusPrompt) {
        promptInputRef.current?.scrollIntoView({
          behavior: "smooth",
          block: "center",
        });
        promptInputRef.current?.focus();
      }
      return;
    }
    const regenerationSelection = activeSelection;
    if (!regenerationSelection) {
      return;
    }

    workbench.setField("generationError", "");
    workbench.setField("regeneratingId", designId);
    beginListingKitTraceRun({
      batchId: batchTraceContext.batchId,
      queueMode: batchTraceContext.queueMode ?? undefined,
      queueIndex: batchTraceContext.queueIndex,
      queueTotal: batchTraceContext.queueTotal,
    });
    logListingKitTraceEvent("info", "studio regenerate started", {
      designId,
      selectionVariantId: regenerationSelection.variantId,
    });
    const nextGroups = buildGenerationPromptHistoryGroups({
      activeGroupId,
      groupedImageMode,
      groups,
      prompt,
    });
    if (nextGroups !== groups) {
      workbench.setField("groups", nextGroups);
    }

    try {
      const targets = buildGroupedGenerationTargets({
        activeSelection: regenerationSelection,
        groupedSelections: groupedSelections
          .filter((item) => item.eligible)
          .map((item) => item.selection),
        groupedImageMode,
      });
      const currentDesign = designs.find((design) => design.id === designId);
      const target = targets.find(
        (item) =>
          currentDesign &&
          resolveDesignTargetKey(currentDesign, item.selection, groupedImageMode) ===
            item.key,
      );
      const targetSelection = target?.selection ?? regenerationSelection;
      const response = await generateSheinStudioDesigns(
        buildSheinStudioGenerateRequest({
          prompt: prompt.trim(),
          variationIntensity,
          printableWidth: targetSelection?.printableWidth,
          printableHeight: targetSelection?.printableHeight,
          productReferenceImageUrls:
            buildSDSProductReferenceImageUrls(targetSelection),
          styleCount: 1,
          artworkModel,
          transparentBackground,
        }),
      );
      const replacement = response.images[0];
      if (!replacement) {
        throw new Error("重新生成已完成，但没有返回任何图片。");
      }

      hasLocalWorkflowStateRef.current = true;
      const nextDesigns = replaceRegeneratedDesign({
        designId,
        designs,
        replacement,
      });
      workbench.setField("designs", (current) =>
        replaceRegeneratedDesign({
          designId,
          designs: current,
          replacement,
        }),
      );
      const nextSelectedIds = selectedIds.includes(designId)
        ? selectedIds
        : [...selectedIds, designId];
      workbench.setField("selectedIds", nextSelectedIds);
      logListingKitTraceEvent("info", "studio regenerate completed", {
        designId,
      });
      void persistDraft(
        {
          designs: nextDesigns,
          groups: nextGroups,
          selectedIds: nextSelectedIds,
        },
        {
          source: "regenerate_success",
        },
      ).catch(() => undefined);
    } catch (error) {
      const message = formatSubscriptionApiError(error);
      logListingKitTraceEvent("warn", "studio regenerate failed", {
        designId,
        error: message,
      });
      workbench.setField("generationError", message);
    } finally {
      workbench.setField("regeneratingId", "");
    }
  }

  return {
    handleCreateTasks,
    handleGenerate,
    handleRegenerate,
  };
}
