import type { MutableRefObject, RefObject } from "react";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import { useSheinStudioTaskCreationAction } from "@/components/listingkit/shein-studio/shein-studio-task-creation-actions";
import type { SheinStudioWorkbenchController } from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import { generateSheinStudioDesigns } from "@/lib/api/shein-studio";
import { formatSubscriptionApiError } from "@/lib/api/subscription";
import {
  buildGroupedGenerationTargets,
  resolveDesignTargetKey,
} from "@/lib/shein-studio/grouped-image-mode";
import {
  buildHotStyleReferenceGenerationInput,
  buildSheinStudioGenerateRequest,
  buildGenerationPromptHistoryGroups,
  executeStandaloneGeneration,
  replaceRegeneratedDesign,
  resolveGenerationStartValidation,
  resolveRegenerationStartValidation,
} from "@/lib/shein-studio/generation-controller";
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
  SheinStudioGenerateRequest,
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
  startGenerationRun: (savedBatch: SheinStudioSavedBatch) => Promise<void>;
};

type WorkbenchTaskRecoveryAlertsController = Pick<
  SheinStudioWorkbenchController,
  "setField"
>;

export function clearWorkbenchTaskRecoveryAlerts(
  workbench: WorkbenchTaskRecoveryAlertsController,
) {
  workbench.setField("generationError", "");
  workbench.setField("generationWarning", "");
  workbench.setField("creatingError", "");
  workbench.setField("creatingWarning", "");
}

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
  hotStyleReferenceBrief: string;
  hotStyleReferenceImageUrls: string[];
  hotStyleReferencePrompt: string;
  prompt: string;
  promptMode?: SheinStudioGenerateRequest["promptMode"];
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
  hotStyleReferenceBrief,
  hotStyleReferenceImageUrls,
  hotStyleReferencePrompt,
  prompt,
  promptMode,
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
    promptMode,
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

    try {
      if (batchGenerationContext) {
        const savedBatch = await batchGenerationContext.ensureBatch();
        if (!savedBatch?.id) {
          throw new Error("当前批次保存失败，请稍后重试。");
        }
        writeListingKitTraceContext({ batchId: savedBatch.id });
        await batchGenerationContext.startGenerationRun(savedBatch);
        logListingKitTraceEvent("info", "studio batch generation run started", {
          batchId: savedBatch.id,
        });
        console.info("[shein-studio] batch generation run started", {
          batchId: savedBatch.id,
          selectionVariantId: generationSelection.variantId,
        });
        return;
      }

      const generationResult = await executeStandaloneGeneration({
        activeGroupId,
        activeSelection: generationSelection,
        artworkModel,
        generateDesigns: generateSheinStudioDesigns,
        generationJobs,
        groupedImageMode,
        groupedSelections,
        groups: nextGroups,
        hasLocalWorkflowStateRef,
        hotStyleReferenceBrief,
        hotStyleReferenceImageUrls,
        hotStyleReferencePrompt,
        navigateToStep,
        persistDraft,
        prompt,
        setField: (field, value) => {
          switch (field) {
            case "createdTasks":
              workbench.setField("createdTasks", value as []);
              break;
            case "designs":
              workbench.setField("designs", value as SheinStudioGeneratedDesign[]);
              break;
            case "generationJobs":
              workbench.setField("generationJobs", value as SheinStudioGenerationJob[]);
              break;
            case "generationWarning":
              workbench.setField("generationWarning", value as string);
              break;
            case "groups":
              workbench.setField("groups", value as SheinStudioGroupedWorkspace[]);
              break;
            case "selectedIds":
              workbench.setField("selectedIds", value as string[]);
              break;
          }
        },
        styleCount,
        transparentBackground,
        variationIntensity,
        formatError: formatSubscriptionApiError,
        onJobStarted: ({ jobId, target }) => {
          logListingKitTraceEvent("info", "studio async job started", {
            jobId,
            targetGroupKey: target.key,
            targetGroupLabel: target.label,
          });
        },
      });
      console.info("[shein-studio] generation succeeded", {
        designCount: generationResult.designs.length,
        draftSaveStatus: "pending",
        selectionVariantId: generationSelection.variantId,
      });
      logListingKitTraceEvent("info", "studio generation completed", {
        designCount: generationResult.designs.length,
        warningCount: generationResult.warnings.length,
        errorCount: generationResult.targetErrors.length,
      });
    } catch (error) {
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
      const generationInput = buildHotStyleReferenceGenerationInput({
        prompt,
        hotStyleReferenceBrief,
        hotStyleReferencePrompt,
        productReferenceImageUrls:
          buildSDSProductReferenceImageUrls(targetSelection),
        hotStyleReferenceImageUrls,
      });
      const response = await generateSheinStudioDesigns(
        buildSheinStudioGenerateRequest({
          prompt: generationInput.prompt,
          promptMode,
          variationIntensity,
          printableWidth: targetSelection?.printableWidth,
          printableHeight: targetSelection?.printableHeight,
          productReferenceImageUrls: generationInput.productReferenceImageUrls,
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
