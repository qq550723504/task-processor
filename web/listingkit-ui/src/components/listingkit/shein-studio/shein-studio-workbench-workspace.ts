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
  saveLocalSheinStudioDraftSnapshot,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-hooks";
import type { SheinStudioWorkbenchController } from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import { resumeSheinStudioDesignGeneration } from "@/lib/api/shein-studio";
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
  persistDraft,
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
  persistDraft: (
    overrides?: Partial<{
      designs: SheinStudioGeneratedDesign[];
      groups: SheinStudioGroupedWorkspace[];
      selectedIds: string[];
      createdTasks: SheinStudioCreatedTask[];
      generationJobs: SheinStudioGenerationJob[];
    }>,
  ) => Promise<unknown>;
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
        let resumableGenerationJobs: SheinStudioGenerationJob[] = [];
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
          const runningGenerationJobs = draft?.generationJobs?.filter(
            (job) => job.status === "running" && job.jobId.trim(),
          );
          resumableGenerationJobs =
            draft?.batchStatus === "generating" ||
            (draft?.generationJobs?.length ?? 0) > 0 ||
            Boolean(draft?.generationJobId)
              ? runningGenerationJobs && runningGenerationJobs.length > 0
                ? runningGenerationJobs
                : draft?.generationJobId
                  ? [
                      {
                        jobId: draft.generationJobId,
                        status: "running" as const,
                      },
                    ]
                  : []
              : [];
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
        workbench.setField("creatingWarning", "");
        workbench.setField("creatingMessage", "");
        workbench.setField("saveMessage", "");
        workbench.setField("draftWarning", "");

        if (resumableGenerationJobs.length > 0) {
          workbench.setField("isGenerating", true);
          workbench.setField("generationJobs", resumableGenerationJobs);
          workbench.setField("generationError", "");
          workbench.setField(
            "generationWarning",
            "已恢复之前发起的生成任务，正在继续等待结果。",
          );
          workbench.setField("generationWarningAction", null);
          try {
            let nextDesigns = effectiveDraft?.designs ?? [];
            let nextSelectedIds = effectiveDraft?.selectedIds ?? [];
            let pendingJobs = [...resumableGenerationJobs];
            const warningMessages: string[] = [];

            for (const job of resumableGenerationJobs) {
              const response = await resumeSheinStudioDesignGeneration(job.jobId);
              if (cancelled) {
                return;
              }
              const images = response.images.map((image) => ({
                ...image,
                targetGroupKey: job.targetGroupKey,
                targetGroupLabel: job.targetGroupLabel,
              }));
              nextDesigns = [
                ...nextDesigns.filter(
                  (design) =>
                    !images.some((incoming) => incoming.id === design.id),
                ),
                ...images,
              ];
              nextSelectedIds = Array.from(
                new Set([...nextSelectedIds, ...images.map((image) => image.id)]),
              );
              pendingJobs = pendingJobs.filter(
                (candidate) => candidate.jobId !== job.jobId,
              );
              hasLocalWorkflowStateRef.current = true;
              workbench.setField("designs", nextDesigns);
              workbench.setField("selectedIds", nextSelectedIds);
              workbench.setField("createdTasks", []);
              workbench.setField("generationJobs", pendingJobs);
              if (effectiveDraft) {
                saveLocalSheinStudioDraftSnapshot({
                  ...effectiveDraft,
                  designs: nextDesigns,
                  selectedIds: nextSelectedIds,
                  createdTasks: [],
                  generationJobs: pendingJobs,
                  updatedAt: new Date().toISOString(),
                });
              }
              if (response.warnings?.length) {
                warningMessages.push(...response.warnings);
              }
            }
            if (nextDesigns.length === 0) {
              throw new Error(
                "款式图生成完成，但没有返回任何图片。请重试一次；如果持续出现，说明上游生成链路返回了空结果。",
              );
            }
            workbench.setField(
              "galleryRatioCheck",
              evaluateImportedGalleryDesigns(
                nextDesigns,
                activeSelectionRef.current,
              ),
            );
            workbench.setField("generationJobs", []);
            workbench.setField(
              "generationWarning",
              warningMessages.length
                ? `已恢复之前的生成任务。${warningMessages.join(" ")}`
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
  buildDraftInput: () => SheinStudioSaveInput;
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
