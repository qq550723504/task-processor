import { useEffect, useRef } from "react";
import type { Dispatch, MutableRefObject, SetStateAction } from "react";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import {
  evaluateImportedGalleryDesigns,
  mergeSheinStudioDraftState,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import {
  consumeSheinStudioGalleryHandoff,
  galleryHandoffToDesign,
  type SDSRatioMatch,
} from "@/lib/shein-studio/gallery-handoff";
import {
  DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
  DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
  DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
} from "@/lib/shein-studio/storage-shared";
import { resolveSheinStudioEffectiveStep } from "@/lib/shein-studio/workbench-step";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSavedBatch,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";
import {
  deleteSheinStudioBatch,
  listSheinStudioBatches,
  loadSheinStudioDraft,
  saveSheinStudioBatch,
  type SheinStudioSaveInput,
} from "@/lib/utils/shein-studio-batches";

type WorkbenchSetters = {
  setArtworkModel: (value: SheinStudioArtworkModel) => void;
  setCreatedTasks: Dispatch<SetStateAction<SheinStudioCreatedTask[]>>;
  setCreatingError: (value: string) => void;
  setCreatingMessage: (value: string) => void;
  setDesigns: Dispatch<SetStateAction<SheinStudioGeneratedDesign[]>>;
  setDraftWarning: (value: string) => void;
  setGalleryRatioCheck: Dispatch<SetStateAction<SDSRatioMatch | null>>;
  setGenerationError: (value: string) => void;
  setImageStrategy: (value: SheinStudioImageStrategy) => void;
  setIsLoadingWorkspace: (value: boolean) => void;
  setProductImageCount: (value: string) => void;
  setProductImagePrompt: (value: string) => void;
  setProductImagePrompts: (
    value: SheinStudioProductImagePrompt[],
  ) => void;
  setPrompt: (value: string) => void;
  setRenderSizeImagesWithSds: (value: boolean) => void;
  setSavedBatches: Dispatch<SetStateAction<SheinStudioSavedBatch[]>>;
  setSaveMessage: (value: string) => void;
  setSelectedIds: Dispatch<SetStateAction<string[]>>;
  setSelectedSdsImages: (
    value: SheinStudioSelectedSDSImage[],
  ) => void;
  setSheinStoreId: (value: string) => void;
  setStyleCount: (value: string) => void;
  setTransparentBackground: (value: boolean) => void;
  setVariationIntensity: (value: SheinStudioVariationIntensity) => void;
};

export function useSheinStudioWorkspaceLoader({
  activeSelection,
  activeSelectionKey,
  activeStepRef,
  hasCustomizedSdsSelectionRef,
  hasLocalWorkflowStateRef,
  setEffectiveStep,
  setters,
}: {
  activeSelection?: SDSProductVariantSelection;
  activeSelectionKey: string;
  activeStepRef: MutableRefObject<SheinStudioStepKey>;
  hasCustomizedSdsSelectionRef: MutableRefObject<boolean>;
  hasLocalWorkflowStateRef: MutableRefObject<boolean>;
  setEffectiveStep: (step: SheinStudioStepKey) => void;
  setters: WorkbenchSetters;
}) {
  const activeSelectionRef = useRef(activeSelection);

  useEffect(() => {
    activeSelectionRef.current = activeSelection;
  }, [activeSelection]);

  useEffect(() => {
    let cancelled = false;

    async function loadWorkspaceState() {
      setters.setIsLoadingWorkspace(true);
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

          setters.setPrompt(draftState.prompt);
          setters.setStyleCount(draftState.styleCount);
          setters.setVariationIntensity(draftState.variationIntensity);
          setters.setProductImageCount(draftState.productImageCount);
          setters.setProductImagePrompt(draftState.productImagePrompt);
          setters.setProductImagePrompts(draftState.productImagePrompts);
          setters.setArtworkModel(draftState.artworkModel);
          setters.setTransparentBackground(draftState.transparentBackground);
          setters.setSheinStoreId(draftState.sheinStoreId);
          setters.setImageStrategy(draftState.imageStrategy);
          setters.setSelectedSdsImages(draftState.selectedSdsImages);
          hasCustomizedSdsSelectionRef.current =
            draftState.hasCustomizedSdsSelection;
          setters.setRenderSizeImagesWithSds(
            draftState.renderSizeImagesWithSds,
          );
          setters.setDesigns(draftState.designs);
          setters.setSelectedIds(draftState.selectedIds);
          setters.setCreatedTasks(draftState.createdTasks);
          setters.setGalleryRatioCheck(
            evaluateImportedGalleryDesigns(
              draftState.designs,
              activeSelectionRef.current,
            ),
          );
          nextEffectiveDesignCount = draftState.designCount;
          nextEffectiveCreatedTaskCount = draftState.createdTaskCount;
          if (draftState.importedGalleryDesign) {
            hasLocalWorkflowStateRef.current = true;
            importedGalleryDesign = true;
          }
        }
        setters.setSavedBatches(batches);
        if (draft || importedGalleryDesign) {
          setEffectiveStep(
            resolveSheinStudioEffectiveStep({
              activeStep: activeStepRef.current,
              createdTaskCount: nextEffectiveCreatedTaskCount,
              designCount: nextEffectiveDesignCount,
            }),
          );
        }
        setters.setGenerationError("");
        setters.setCreatingError("");
        setters.setCreatingMessage("");
        setters.setSaveMessage("");
        setters.setDraftWarning("");
      } finally {
        if (!cancelled) {
          setters.setIsLoadingWorkspace(false);
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
    setters,
  ]);
}

export function useSheinStudioBatchActions({
  activeStep,
  buildDraftInput,
  hasCustomizedSdsSelectionRef,
  hasLocalWorkflowStateRef,
  setEffectiveStep,
  setters,
}: {
  activeStep: SheinStudioStepKey;
  buildDraftInput: () => SheinStudioSaveInput;
  hasCustomizedSdsSelectionRef: MutableRefObject<boolean>;
  hasLocalWorkflowStateRef: MutableRefObject<boolean>;
  setEffectiveStep: (step: SheinStudioStepKey) => void;
  setters: WorkbenchSetters;
}) {
  async function handleSaveBatch() {
    const draftInput = buildDraftInput();
    if (!draftInput.prompt?.trim()) {
      setters.setSaveMessage("保存批次前请先填写主题提示词。");
      return;
    }

    const saved = await saveSheinStudioBatch(draftInput);

    if (!saved) {
      setters.setSaveMessage("批次保存失败。");
      return;
    }

    setters.setSavedBatches(await listSheinStudioBatches());
    setters.setSaveMessage(`批次已保存：${saved.name}`);
  }

  function handleLoadBatch(batch: SheinStudioSavedBatch) {
    hasLocalWorkflowStateRef.current = true;
    setters.setPrompt(batch.prompt);
    setters.setStyleCount(batch.styleCount);
    setters.setVariationIntensity(
      batch.variationIntensity ?? DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
    );
    setters.setProductImageCount(
      batch.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    );
    setters.setProductImagePrompt(batch.productImagePrompt ?? "");
    setters.setProductImagePrompts(batch.productImagePrompts ?? []);
    setters.setArtworkModel(
      batch.artworkModel ?? DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
    );
    setters.setTransparentBackground(batch.transparentBackground ?? false);
    setters.setSheinStoreId(batch.sheinStoreId);
    setters.setImageStrategy(batch.imageStrategy ?? "sds_official");
    setters.setSelectedSdsImages(batch.selectedSdsImages ?? []);
    hasCustomizedSdsSelectionRef.current =
      (batch.selectedSdsImages?.length ?? 0) > 0;
    setters.setRenderSizeImagesWithSds(
      batch.renderSizeImagesWithSds ?? true,
    );
    setters.setDesigns(batch.designs);
    setters.setSelectedIds(batch.selectedIds);
    setters.setCreatedTasks(batch.createdTasks);
    setEffectiveStep(
      resolveSheinStudioEffectiveStep({
        activeStep,
        createdTaskCount: batch.createdTasks.length,
        designCount: batch.designs.length,
      }),
    );
    setters.setSaveMessage(`已载入批次：${batch.name}`);
  }

  async function handleDeleteBatch(batchID: string) {
    await deleteSheinStudioBatch(batchID);
    setters.setSavedBatches(await listSheinStudioBatches());
  }

  return {
    handleDeleteBatch,
    handleLoadBatch,
    handleSaveBatch,
  };
}
