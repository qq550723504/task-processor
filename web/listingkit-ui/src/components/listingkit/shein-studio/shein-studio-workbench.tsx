"use client";

import { useEffect, useRef, useState } from "react";

import { SheinStudioBusyOverlay } from "@/components/listingkit/shein-studio/shein-studio-busy-overlay";
import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import { SheinStudioSelectionOverview } from "@/components/listingkit/shein-studio/shein-studio-selection-overview";
import { SheinStudioTasksStep } from "@/components/listingkit/shein-studio/shein-studio-tasks-step";
import { useSheinStudioDesignActions } from "@/components/listingkit/shein-studio/shein-studio-workbench-actions";
import {
  useHydratedSDSVariantSelection,
  useSheinStudioDraftPersistence,
  useSheinStudioStepNavigation,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-hooks";
import {
  SheinStudioReviewStep,
  SheinStudioWorkbenchAlerts,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-sections";
import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import {
  buildSheinStudioSelectionKey,
  evaluateImportedGalleryDesigns,
  getSheinStudioCreateActionDisabledReason,
  mergeSheinStudioDraftState,
  sheinStudioBusyMessage,
  summarizeSheinStudioSelection,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import { DEFAULT_SHEIN_STORE_ID } from "@/lib/shein-studio/create-review-tasks";
import {
  consumeSheinStudioGalleryHandoff,
  galleryHandoffToDesign,
  type SDSRatioMatch,
} from "@/lib/shein-studio/gallery-handoff";
import {
  buildDefaultSelectedSDSImages,
  buildSelectableSDSImages,
} from "@/lib/shein-studio/sds-selectable-images";
import {
  DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
  DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
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
} from "@/lib/utils/shein-studio-batches";

export function SheinStudioWorkbench({
  activeStep = "generate",
  selection,
}: {
  activeStep?: SheinStudioStepKey;
  selection?: SDSProductVariantSelection;
}) {
  const [prompt, setPrompt] = useState("");
  const [styleCount, setStyleCount] = useState("1");
  const [variationIntensity, setVariationIntensity] =
    useState<SheinStudioVariationIntensity>(
      DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
    );
  const [productImageCount, setProductImageCount] = useState(
    DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
  );
  const [productImagePrompt, setProductImagePrompt] = useState("");
  const [productImagePrompts, setProductImagePrompts] = useState<
    SheinStudioProductImagePrompt[]
  >([]);
  const [artworkModel, setArtworkModel] = useState<SheinStudioArtworkModel>(
    DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
  );
  const [transparentBackground, setTransparentBackground] = useState(false);
  const [sheinStoreId, setSheinStoreId] = useState(DEFAULT_SHEIN_STORE_ID);
  const [imageStrategy, setImageStrategy] = useState<SheinStudioImageStrategy>(
    DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
  );
  const [selectedSdsImages, setSelectedSdsImages] = useState<SheinStudioSelectedSDSImage[]>([]);
  const [renderSizeImagesWithSds, setRenderSizeImagesWithSds] = useState(true);
  const [designs, setDesigns] = useState<SheinStudioGeneratedDesign[]>([]);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [generationError, setGenerationError] = useState<string>("");
  const [creatingError, setCreatingError] = useState<string>("");
  const [creatingMessage, setCreatingMessage] = useState<string>("");
  const [isGenerating, setIsGenerating] = useState(false);
  const [isCreatingTasks, setIsCreatingTasks] = useState(false);
  const [regeneratingId, setRegeneratingId] = useState<string>("");
  const [createdTasks, setCreatedTasks] = useState<SheinStudioCreatedTask[]>([]);
  const [galleryRatioCheck, setGalleryRatioCheck] = useState<SDSRatioMatch | null>(null);
  const [savedBatches, setSavedBatches] = useState<SheinStudioSavedBatch[]>([]);
  const [isLoadingWorkspace, setIsLoadingWorkspace] = useState(true);
  const [saveMessage, setSaveMessage] = useState("");
  const [draftWarning, setDraftWarning] = useState("");
  const hasLocalWorkflowStateRef = useRef(false);
  const hasCustomizedSdsSelectionRef = useRef(false);
  const promptInputRef = useRef<HTMLTextAreaElement>(null);
  const activeSelection = useHydratedSDSVariantSelection(selection);
  const activeSelectionRef = useRef(activeSelection);
  const {
    activeStepRef,
    effectiveStep,
    navigateToStep,
    setEffectiveStep,
  } = useSheinStudioStepNavigation(activeStep);
  const activeSelectionKey = buildSheinStudioSelectionKey(activeSelection);
  const {
    printableAreaLabel,
    selectedColorCount,
    selectedSizeCount,
    selectedVariants,
  } = summarizeSheinStudioSelection(activeSelection);
  const availableSdsImages = buildSelectableSDSImages(activeSelection);
  const createActionDisabledReason = getSheinStudioCreateActionDisabledReason({
    selection: activeSelection,
    galleryRatioCheck,
    selectedIds,
  });
  const { buildDraftInput, persistDraft } = useSheinStudioDraftPersistence({
    activeSelection,
    artworkModel,
    createdTasks,
    designs,
    imageStrategy,
    isCreatingTasks,
    isGenerating,
    isLoadingWorkspace,
    productImageCount,
    productImagePrompt,
    productImagePrompts,
    prompt,
    regeneratingId,
    renderSizeImagesWithSds,
    selectedIds,
    selectedSdsImages,
    setDraftWarning,
    sheinStoreId,
    styleCount,
    transparentBackground,
    variationIntensity,
  });

  useEffect(() => {
    activeSelectionRef.current = activeSelection;
  }, [activeSelection]);

  useEffect(() => {
    hasLocalWorkflowStateRef.current = false;
    hasCustomizedSdsSelectionRef.current = false;
  }, [selection?.variantId]);

  useEffect(() => {
    let cancelled = false;

    async function loadWorkspaceState() {
      setIsLoadingWorkspace(true);
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

          setPrompt(draftState.prompt);
          setStyleCount(draftState.styleCount);
          setVariationIntensity(draftState.variationIntensity);
          setProductImageCount(draftState.productImageCount);
          setProductImagePrompt(draftState.productImagePrompt);
          setProductImagePrompts(draftState.productImagePrompts);
          setArtworkModel(draftState.artworkModel);
          setTransparentBackground(draftState.transparentBackground);
          setSheinStoreId(draftState.sheinStoreId);
          setImageStrategy(draftState.imageStrategy);
          setSelectedSdsImages(draftState.selectedSdsImages);
          hasCustomizedSdsSelectionRef.current = draftState.hasCustomizedSdsSelection;
          setRenderSizeImagesWithSds(draftState.renderSizeImagesWithSds);
          setDesigns(draftState.designs);
          setSelectedIds(draftState.selectedIds);
          setCreatedTasks(draftState.createdTasks);
          setGalleryRatioCheck(
            evaluateImportedGalleryDesigns(draftState.designs, activeSelectionRef.current),
          );
          nextEffectiveDesignCount = draftState.designCount;
          nextEffectiveCreatedTaskCount = draftState.createdTaskCount;
          if (draftState.importedGalleryDesign) {
            hasLocalWorkflowStateRef.current = true;
            importedGalleryDesign = true;
          }
        }
        setSavedBatches(batches);
        if (draft || importedGalleryDesign) {
          setEffectiveStep(
            resolveSheinStudioEffectiveStep({
              activeStep: activeStepRef.current,
              createdTaskCount: nextEffectiveCreatedTaskCount,
              designCount: nextEffectiveDesignCount,
            }),
          );
        }
        setGenerationError("");
        setCreatingError("");
        setCreatingMessage("");
        setSaveMessage("");
        setDraftWarning("");
      } finally {
        if (!cancelled) {
          setIsLoadingWorkspace(false);
        }
      }
    }

    void loadWorkspaceState();

    return () => {
      cancelled = true;
    };
  }, [activeSelectionKey, activeStepRef, setEffectiveStep]);

  useEffect(() => {
    if (imageStrategy !== "hybrid" && imageStrategy !== "sds_official") {
      return;
    }
    if (hasCustomizedSdsSelectionRef.current) {
      return;
    }

    const nextDefaults = buildDefaultSelectedSDSImages(availableSdsImages, {
      includeSizeReferenceImages: renderSizeImagesWithSds,
    });
    const currentSelection = JSON.stringify(selectedSdsImages);
    const defaultSelection = JSON.stringify(nextDefaults);
    if (currentSelection !== defaultSelection) {
      const timer = window.setTimeout(() => {
        setSelectedSdsImages(nextDefaults);
      }, 0);
      return () => {
        window.clearTimeout(timer);
      };
    }
  }, [
    availableSdsImages,
    imageStrategy,
    renderSizeImagesWithSds,
    selectedSdsImages,
  ]);

  const { handleCreateTasks, handleGenerate, handleRegenerate } =
    useSheinStudioDesignActions({
      activeSelection,
      artworkModel,
      designs,
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
      setCreatedTasks,
      setCreatingError,
      setCreatingMessage,
      setDesigns,
      setDraftWarning,
      setGalleryRatioCheck,
      setGenerationError,
      setIsCreatingTasks,
      setIsGenerating,
      setRegeneratingId,
      setSelectedIds,
      sheinStoreId,
      styleCount,
      transparentBackground,
      variationIntensity,
      hasLocalWorkflowStateRef,
    });

  async function handleSaveBatch() {
    if (!prompt.trim()) {
      setSaveMessage("保存批次前请先填写主题提示词。");
      return;
    }

    const saved = await saveSheinStudioBatch(buildDraftInput());

    if (!saved) {
      setSaveMessage("批次保存失败。");
      return;
    }

    setSavedBatches(await listSheinStudioBatches());
    setSaveMessage(`批次已保存：${saved.name}`);
  }

  function handleLoadBatch(batch: SheinStudioSavedBatch) {
    hasLocalWorkflowStateRef.current = true;
    setPrompt(batch.prompt);
    setStyleCount(batch.styleCount);
    setVariationIntensity(
      batch.variationIntensity ?? DEFAULT_SHEIN_STUDIO_VARIATION_INTENSITY,
    );
    setProductImageCount(
      batch.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    );
    setProductImagePrompt(batch.productImagePrompt ?? "");
    setProductImagePrompts(batch.productImagePrompts ?? []);
    setArtworkModel(batch.artworkModel ?? DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL);
    setTransparentBackground(batch.transparentBackground ?? false);
    setSheinStoreId(batch.sheinStoreId);
    setImageStrategy(batch.imageStrategy ?? "sds_official");
    setSelectedSdsImages(batch.selectedSdsImages ?? []);
    hasCustomizedSdsSelectionRef.current =
      (batch.selectedSdsImages?.length ?? 0) > 0;
    setRenderSizeImagesWithSds(batch.renderSizeImagesWithSds ?? true);
    setDesigns(batch.designs);
    setSelectedIds(batch.selectedIds);
    setCreatedTasks(batch.createdTasks);
    setEffectiveStep(
      resolveSheinStudioEffectiveStep({
        activeStep,
        createdTaskCount: batch.createdTasks.length,
        designCount: batch.designs.length,
      }),
    );
    setSaveMessage(`已载入批次：${batch.name}`);
  }

  async function handleDeleteBatch(batchID: string) {
    await deleteSheinStudioBatch(batchID);
    setSavedBatches(await listSheinStudioBatches());
  }

  function toggleSelection(designId: string) {
    setSelectedIds((current) =>
      current.includes(designId)
        ? current.filter((item) => item !== designId)
        : [...current, designId],
    );
  }

  function handleNoteChange(designId: string, note: string) {
    setDesigns((current) =>
      current.map((design) =>
        design.id === designId ? { ...design, reviewNote: note } : design,
      ),
    );
  }

  const busyMessage = sheinStudioBusyMessage({
    isCreatingTasks,
    isGenerating,
    regeneratingId,
  });

  return (
    <section className="relative space-y-6">
      {busyMessage ? <SheinStudioBusyOverlay message={busyMessage} /> : null}

      <SheinStudioSelectionOverview
        printableAreaLabel={printableAreaLabel}
        selectedColorCount={selectedColorCount}
        selectedSizeCount={selectedSizeCount}
        selectedVariantCount={selectedVariants.length}
        selection={activeSelection}
      />

      <SheinStudioWorkbenchAlerts
        draftWarning={draftWarning}
        galleryRatioCheck={galleryRatioCheck}
      />

      {effectiveStep === "generate" ? (
        <SheinStudioGenerationPanel
          artworkModel={artworkModel}
          availableSdsImages={availableSdsImages}
          createdTasks={createdTasks}
          creatingError={creatingError}
          creatingMessage={creatingMessage}
          generationError={generationError}
          imageStrategy={imageStrategy}
          isCreatingTasks={isCreatingTasks}
          isGenerating={isGenerating}
          onCreateTasks={handleCreateTasks}
          onDeleteBatch={handleDeleteBatch}
          onGenerate={handleGenerate}
          onLoadBatch={handleLoadBatch}
          onSaveBatch={handleSaveBatch}
          productImageCount={productImageCount}
          productImagePrompt={productImagePrompt}
          productImagePrompts={productImagePrompts}
          prompt={prompt}
          promptInputRef={promptInputRef}
          renderSizeImagesWithSds={renderSizeImagesWithSds}
          saveMessage={saveMessage}
          savedBatches={savedBatches}
          selectedSdsImages={selectedSdsImages}
          selectedStyleCount={selectedIds.length}
          selectionReady={Boolean(activeSelection?.variantId)}
          setArtworkModel={setArtworkModel}
          setImageStrategy={setImageStrategy}
          setProductImageCount={setProductImageCount}
          setProductImagePrompt={setProductImagePrompt}
          setProductImagePrompts={setProductImagePrompts}
          setPrompt={setPrompt}
          setRenderSizeImagesWithSds={setRenderSizeImagesWithSds}
          setSelectedSdsImages={(value) => {
            hasCustomizedSdsSelectionRef.current = true;
            setSelectedSdsImages(value);
          }}
          setSheinStoreId={setSheinStoreId}
          setStyleCount={setStyleCount}
          setVariationIntensity={setVariationIntensity}
          setTransparentBackground={setTransparentBackground}
          sheinStoreId={sheinStoreId}
          styleCount={styleCount}
          variationIntensity={variationIntensity}
          transparentBackground={transparentBackground}
        />
      ) : null}

      {effectiveStep === "review" ? (
        <SheinStudioReviewStep
          createdTaskCount={createdTasks.length}
          createActionDisabledReason={createActionDisabledReason}
          designs={designs}
          imageStrategy={imageStrategy}
          isCreatingTasks={isCreatingTasks}
          onBackToGenerate={() => navigateToStep("generate")}
          onCreateReviewTasks={handleCreateTasks}
          onNoteChange={handleNoteChange}
          onRegenerate={handleRegenerate}
          onToggle={toggleSelection}
          productImageCount={productImageCount}
          regeneratingId={regeneratingId || undefined}
          renderSizeImagesWithSds={renderSizeImagesWithSds}
          selectedIds={selectedIds}
          selection={activeSelection}
        />
      ) : null}

      {effectiveStep === "tasks" ? (
        <SheinStudioTasksStep createdTasks={createdTasks} />
      ) : null}
    </section>
  );
}
