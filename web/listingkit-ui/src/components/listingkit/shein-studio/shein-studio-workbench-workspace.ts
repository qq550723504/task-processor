import { useCallback, useEffect, useRef } from "react";
import type { MutableRefObject } from "react";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import {
  evaluateImportedGalleryDesigns,
  mergeSheinStudioDraftState,
  flattenItemizedBatchDesigns,
  type SheinStudioWorkbenchHydratedBatch,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import {
  loadLocalSheinStudioDraftSnapshot,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-hooks";
import type { SheinStudioWorkbenchController } from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import {
  consumeSheinStudioGalleryHandoff,
  galleryHandoffToDesign,
} from "@/lib/shein-studio/gallery-handoff";
import { resolveSheinStudioEffectiveStep } from "@/lib/shein-studio/workbench-step";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioGenerationJob,
  SheinStudioGroupedWorkspace,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";
import { consumeSDSGroupedCandidateHandoff } from "@/lib/utils/sds-grouped-candidate-handoff";
import {
  deleteSheinStudioBatch,
  listSheinStudioBatches,
  loadSheinStudioDraft,
  saveSheinStudioBatch,
  setActiveSheinStudioBatchId,
  type SheinStudioSaveInput,
} from "@/lib/utils/shein-studio-batches";

function pickRecentGroupedBatch(batches: SheinStudioSavedBatch[]) {
  return [...batches]
    .filter((batch) => (batch.groups?.length ?? 0) > 0)
    .sort((left, right) => right.updatedAt.localeCompare(left.updatedAt))[0] ?? null;
}

export function useSheinStudioWorkspaceLoader({
  activeSelection,
  activeSelectionKey,
  activeStepRef,
  hasDedicatedBatchContext,
  hasExplicitSelection,
  hasCustomizedSdsSelectionRef,
  hasLocalWorkflowStateRef,
  setEffectiveStep,
  workbench,
}: {
  activeSelection?: SDSProductVariantSelection;
  activeSelectionKey: string;
  activeStepRef: MutableRefObject<SheinStudioStepKey>;
  hasDedicatedBatchContext: boolean;
  hasExplicitSelection: boolean;
  hasCustomizedSdsSelectionRef: MutableRefObject<boolean>;
  hasLocalWorkflowStateRef: MutableRefObject<boolean>;
  setEffectiveStep: (step: SheinStudioStepKey) => void;
  workbench: SheinStudioWorkbenchController;
}) {
  const activeSelectionRef = useRef(activeSelection);

  useEffect(() => {
    activeSelectionRef.current = activeSelection;
  }, [activeSelection]);

  useEffect(() => {
    let cancelled = false;

    async function loadWorkspaceState() {
        workbench.setField("isLoadingWorkspace", true);
      try {
        const localDraft = !hasDedicatedBatchContext && !hasExplicitSelection
          ? loadLocalSheinStudioDraftSnapshot()
          : null;
        const [draft, batches] = await Promise.all([
          !hasDedicatedBatchContext && hasExplicitSelection
            ? loadSheinStudioDraft(activeSelectionRef.current)
            : Promise.resolve(null),
          listSheinStudioBatches(),
        ]);

        if (cancelled) {
          return;
        }

        let nextEffectiveDesignCount = 0;
        let nextEffectiveCreatedTaskCount = 0;
        let importedGalleryDesign = false;
        let restoredGenerationError = "";
        const effectiveDraft =
          hasDedicatedBatchContext
            ? null
            : localDraft ??
              draft ??
              (!activeSelectionRef.current
                ? pickRecentGroupedBatch(batches)
                : null);
        const groupedCandidateHandoff = consumeSDSGroupedCandidateHandoff();

        if (effectiveDraft || !hasLocalWorkflowStateRef.current) {
          const galleryHandoff = activeSelectionRef.current
            ? consumeSheinStudioGalleryHandoff()
            : null;
          const galleryDesign = galleryHandoff
            ? galleryHandoffToDesign(galleryHandoff)
            : null;
          const draftState = mergeSheinStudioDraftState({
            draft: effectiveDraft,
            galleryDesign,
            galleryPrompt: galleryHandoff?.prompt || galleryHandoff?.title,
          });

          hasCustomizedSdsSelectionRef.current =
            draftState.hasCustomizedSdsSelection;
          workbench.applyDraft({
            prompt: draftState.prompt,
            promptMode: draftState.promptMode,
            selection: draftState.selection,
            styleCount: draftState.styleCount,
            variationIntensity: draftState.variationIntensity,
            productImageCount: draftState.productImageCount,
            productImagePrompt: draftState.productImagePrompt,
            productImagePrompts: draftState.productImagePrompts,
            artworkModel: draftState.artworkModel,
            transparentBackground: draftState.transparentBackground,
            sheinStoreId: draftState.sheinStoreId,
            imageStrategy: draftState.imageStrategy,
            groupedImageMode: draftState.groupedImageMode,
            selectedSdsImages: draftState.selectedSdsImages,
            groups: draftState.groups,
            groupedSelections: draftState.groupedSelections,
            renderSizeImagesWithSds: draftState.renderSizeImagesWithSds,
            galleryRatioCheck: evaluateImportedGalleryDesigns(
              draftState.designs,
              activeSelectionRef.current,
            ),
          });
          workbench.setField("designs", draftState.designs);
          workbench.setField("selectedIds", draftState.selectedIds);
          workbench.setField("createdTasks", draftState.createdTasks);
          workbench.setField("generationJobs", draftState.generationJobs);
          workbench.setField("generationError", draftState.generationError);
          restoredGenerationError = draftState.generationError;
          nextEffectiveDesignCount = draftState.designCount;
          nextEffectiveCreatedTaskCount = draftState.createdTaskCount;
          if (draftState.importedGalleryDesign) {
            hasLocalWorkflowStateRef.current = true;
            importedGalleryDesign = true;
          }
        }
        workbench.setField("savedBatches", batches);
        if (effectiveDraft || importedGalleryDesign) {
          setEffectiveStep(
            resolveSheinStudioEffectiveStep({
              activeStep: activeStepRef.current,
              createdTaskCount: nextEffectiveCreatedTaskCount,
              designCount: nextEffectiveDesignCount,
            }),
          );
        }
        workbench.setField("generationError", restoredGenerationError);
        workbench.setField(
          "generationWarning",
          groupedCandidateHandoff?.message ?? "",
        );
        workbench.setField(
          "generationWarningAction",
          groupedCandidateHandoff?.action && groupedCandidateHandoff.actionLabel
            ? {
                intent: groupedCandidateHandoff.action,
                label: groupedCandidateHandoff.actionLabel,
              }
            : null,
        );
        workbench.setField("creatingError", "");
        workbench.setField("creatingWarning", "");
        workbench.setField("creatingMessage", "");
        workbench.setField("saveMessage", "");
        workbench.setField("draftWarning", "");

      } finally {
        if (!cancelled) {
          workbench.setField("isLoadingWorkspace", false);
        }
      }
    }

    void loadWorkspaceState();

    return () => {
      cancelled = true;
    };
  }, [
    activeSelectionKey,
    activeStepRef,
    hasDedicatedBatchContext,
    hasExplicitSelection,
    hasCustomizedSdsSelectionRef,
    hasLocalWorkflowStateRef,
    setEffectiveStep,
    workbench,
  ]);
}

export function useSheinStudioBatchActions({
  activeBatchId,
  activeStep,
  buildDraftInput,
  hasCustomizedSdsSelectionRef,
  hasLocalWorkflowStateRef,
  setActiveBatchId,
  setEffectiveStep,
  workbench,
}: {
  activeBatchId?: string;
  activeStep: SheinStudioStepKey;
  buildDraftInput: (
    overrides?: Partial<{
      designs: SheinStudioGeneratedDesign[];
      groups: SheinStudioGroupedWorkspace[];
      selectedIds: string[];
      createdTasks: SheinStudioCreatedTask[];
      generationJobs: SheinStudioGenerationJob[];
      generationError: string;
      generationJobId: string;
    }>,
  ) => SheinStudioSaveInput;
  hasCustomizedSdsSelectionRef: MutableRefObject<boolean>;
  hasLocalWorkflowStateRef: MutableRefObject<boolean>;
  setActiveBatchId: (batchId: string) => void;
  setEffectiveStep: (step: SheinStudioStepKey) => void;
  workbench: SheinStudioWorkbenchController;
}) {
  const handleSaveBatch = useCallback(async () => {
    const currentBatchId = activeBatchId?.trim() || "";
    const draftInput = {
      ...buildDraftInput(),
      id: currentBatchId || undefined,
    };
    if (!draftInput.prompt?.trim()) {
      workbench.setField("saveMessage", "保存批次前请先填写主题提示词。");
      return;
    }

    const saved = await saveSheinStudioBatch(draftInput, {
      makeActive: currentBatchId ? false : undefined,
    });

    if (!saved) {
      workbench.setField("saveMessage", "批次保存失败。");
      return;
    }

    workbench.setField("savedBatches", await listSheinStudioBatches());
    workbench.setField("saveMessage", `批次已保存：${saved.name}`);
    setActiveBatchId(saved.id);
  }, [activeBatchId, buildDraftInput, setActiveBatchId, workbench]);

  const handleLoadBatch = useCallback((batch: SheinStudioSavedBatch) => {
    hasLocalWorkflowStateRef.current = true;
    setActiveBatchId(batch.id);
    setActiveSheinStudioBatchId(batch.id);
    workbench.applyBatch(batch);
    hasCustomizedSdsSelectionRef.current =
      (batch.selectedSdsImages?.length ?? 0) > 0;
    setEffectiveStep(
      resolveSheinStudioEffectiveStep({
        activeStep,
        createdTaskCount: batch.createdTasks.length,
        designCount: batch.designs.length,
      }),
    );
    workbench.setField("saveMessage", `已载入批次：${batch.name}`);
  }, [
    activeStep,
    hasCustomizedSdsSelectionRef,
    hasLocalWorkflowStateRef,
    setActiveBatchId,
    setEffectiveStep,
    workbench,
  ]);

  const handleLoadHydratedBatch = useCallback((
    batch: SheinStudioWorkbenchHydratedBatch,
  ) => {
    const flattenedDesigns = flattenItemizedBatchDesigns(batch.detail);
    hasLocalWorkflowStateRef.current = true;
    setActiveBatchId(batch.savedBatch.id);
    setActiveSheinStudioBatchId(batch.savedBatch.id);
    workbench.applyHydratedBatch(batch);
    hasCustomizedSdsSelectionRef.current =
      (batch.savedBatch.selectedSdsImages?.length ?? 0) > 0;
    setEffectiveStep(
      resolveSheinStudioEffectiveStep({
        activeStep,
        createdTaskCount: batch.savedBatch.createdTasks.length,
        designCount: flattenedDesigns.length,
      }),
    );
    workbench.setField("saveMessage", `已载入批次：${batch.savedBatch.name}`);
  }, [
    activeStep,
    hasCustomizedSdsSelectionRef,
    hasLocalWorkflowStateRef,
    setActiveBatchId,
    setEffectiveStep,
    workbench,
  ]);

  const handleDeleteBatch = useCallback(async (batchID: string) => {
    await deleteSheinStudioBatch(batchID);
    workbench.setField("savedBatches", await listSheinStudioBatches());
  }, [workbench]);

  return {
    handleDeleteBatch,
    handleLoadBatch,
    handleLoadHydratedBatch,
    handleSaveBatch,
  };
}
