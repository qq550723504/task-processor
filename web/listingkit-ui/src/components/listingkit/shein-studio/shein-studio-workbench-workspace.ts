import { useEffect, useRef } from "react";
import type { MutableRefObject } from "react";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import {
  evaluateImportedGalleryDesigns,
  mergeSheinStudioDraftState,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import type { SheinStudioWorkbenchController } from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import { resumeSheinStudioDesignGeneration } from "@/lib/api/shein-studio";
import {
  consumeSheinStudioGalleryHandoff,
  galleryHandoffToDesign,
} from "@/lib/shein-studio/gallery-handoff";
import { resolveSheinStudioEffectiveStep } from "@/lib/shein-studio/workbench-step";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { SheinStudioSavedBatch } from "@/lib/types/shein-studio";
import { consumeSDSGroupedCandidateHandoff } from "@/lib/utils/sds-grouped-candidate-handoff";
import {
  deleteSheinStudioBatch,
  listSheinStudioBatches,
  loadSheinStudioDraft,
  saveSheinStudioBatch,
  type SheinStudioSaveInput,
} from "@/lib/utils/shein-studio-batches";

export function useSheinStudioWorkspaceLoader({
  activeSelection,
  activeSelectionKey,
  activeStepRef,
  hasCustomizedSdsSelectionRef,
  hasLocalWorkflowStateRef,
  setEffectiveStep,
  workbench,
}: {
  activeSelection?: SDSProductVariantSelection;
  activeSelectionKey: string;
  activeStepRef: MutableRefObject<SheinStudioStepKey>;
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
        const [draft, batches] = await Promise.all([
          loadSheinStudioDraft(activeSelectionRef.current),
          listSheinStudioBatches(),
        ]);

        if (cancelled) {
          return;
        }

        let nextEffectiveDesignCount = 0;
        let nextEffectiveCreatedTaskCount = 0;
        let importedGalleryDesign = false;
        let resumableGenerationJobId = "";
        const groupedCandidateHandoff = consumeSDSGroupedCandidateHandoff();

        if (draft || !hasLocalWorkflowStateRef.current) {
          const galleryHandoff = activeSelectionRef.current
            ? consumeSheinStudioGalleryHandoff()
            : null;
          const galleryDesign = galleryHandoff
            ? galleryHandoffToDesign(galleryHandoff)
            : null;
          const draftState = mergeSheinStudioDraftState({
            draft,
            galleryDesign,
            galleryPrompt: galleryHandoff?.prompt || galleryHandoff?.title,
          });

          hasCustomizedSdsSelectionRef.current =
            draftState.hasCustomizedSdsSelection;
          workbench.applyDraft({
            prompt: draftState.prompt,
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
            groupedSelections: draftState.groupedSelections,
            renderSizeImagesWithSds: draftState.renderSizeImagesWithSds,
            designs: draftState.designs,
            selectedIds: draftState.selectedIds,
            createdTasks: draftState.createdTasks,
            galleryRatioCheck: evaluateImportedGalleryDesigns(
              draftState.designs,
              activeSelectionRef.current,
            ),
          });
          nextEffectiveDesignCount = draftState.designCount;
          nextEffectiveCreatedTaskCount = draftState.createdTaskCount;
          resumableGenerationJobId =
            draft?.sessionStatus === "generating" &&
            draftState.designCount === 0 &&
            draftState.createdTaskCount === 0
              ? draft?.generationJobId ?? ""
              : "";
          if (draftState.importedGalleryDesign) {
            hasLocalWorkflowStateRef.current = true;
            importedGalleryDesign = true;
          }
        }
        workbench.setField("savedBatches", batches);
        if (draft || importedGalleryDesign) {
          setEffectiveStep(
            resolveSheinStudioEffectiveStep({
              activeStep: activeStepRef.current,
              createdTaskCount: nextEffectiveCreatedTaskCount,
              designCount: nextEffectiveDesignCount,
            }),
          );
        }
        workbench.setField("generationError", "");
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
        workbench.setField("creatingMessage", "");
        workbench.setField("saveMessage", "");
        workbench.setField("draftWarning", "");

        if (resumableGenerationJobId) {
          workbench.setField("isGenerating", true);
          workbench.setField("generationError", "");
          workbench.setField(
            "generationWarning",
            "已恢复之前发起的生成任务，正在继续等待结果。",
          );
          workbench.setField("generationWarningAction", null);
          try {
            const response =
              await resumeSheinStudioDesignGeneration(resumableGenerationJobId);
            if (cancelled) {
              return;
            }
            if (!response.images.length) {
              throw new Error(
                "款式图生成完成，但没有返回任何图片。请重试一次；如果持续出现，说明上游生成链路返回了空结果。",
              );
            }
            const nextSelectedIds = response.images.map((item) => item.id);
            hasLocalWorkflowStateRef.current = true;
            workbench.setField("designs", response.images);
            workbench.setField("selectedIds", nextSelectedIds);
            workbench.setField("createdTasks", []);
            workbench.setField(
              "galleryRatioCheck",
              evaluateImportedGalleryDesigns(
                response.images,
                activeSelectionRef.current,
              ),
            );
            workbench.setField(
              "generationWarning",
              response.warnings?.length
                ? `已恢复之前的生成任务。${response.warnings.join(" ")}`
                : "",
            );
            workbench.setField("generationWarningAction", null);
            setEffectiveStep("review");
          } catch (error) {
            if (cancelled) {
              return;
            }
            workbench.setField(
              "generationError",
              error instanceof Error ? error.message : String(error),
            );
            workbench.setField("generationWarning", "");
            workbench.setField("generationWarningAction", null);
          } finally {
            if (!cancelled) {
              workbench.setField("isGenerating", false);
            }
          }
        }
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
    hasCustomizedSdsSelectionRef,
    hasLocalWorkflowStateRef,
    setEffectiveStep,
    workbench,
  ]);
}

export function useSheinStudioBatchActions({
  activeStep,
  buildDraftInput,
  hasCustomizedSdsSelectionRef,
  hasLocalWorkflowStateRef,
  setEffectiveStep,
  workbench,
}: {
  activeStep: SheinStudioStepKey;
  buildDraftInput: () => SheinStudioSaveInput;
  hasCustomizedSdsSelectionRef: MutableRefObject<boolean>;
  hasLocalWorkflowStateRef: MutableRefObject<boolean>;
  setEffectiveStep: (step: SheinStudioStepKey) => void;
  workbench: SheinStudioWorkbenchController;
}) {
  async function handleSaveBatch() {
    const draftInput = buildDraftInput();
    if (!draftInput.prompt?.trim()) {
      workbench.setField("saveMessage", "保存批次前请先填写主题提示词。");
      return;
    }

    const saved = await saveSheinStudioBatch(draftInput);

    if (!saved) {
      workbench.setField("saveMessage", "批次保存失败。");
      return;
    }

    workbench.setField("savedBatches", await listSheinStudioBatches());
    workbench.setField("saveMessage", `批次已保存：${saved.name}`);
  }

  function handleLoadBatch(batch: SheinStudioSavedBatch) {
    hasLocalWorkflowStateRef.current = true;
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
  }

  async function handleDeleteBatch(batchID: string) {
    await deleteSheinStudioBatch(batchID);
    workbench.setField("savedBatches", await listSheinStudioBatches());
  }

  return {
    handleDeleteBatch,
    handleLoadBatch,
    handleSaveBatch,
  };
}
