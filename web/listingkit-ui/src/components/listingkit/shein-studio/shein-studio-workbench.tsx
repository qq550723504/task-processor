"use client";

import { useCallback, useEffect, useMemo, useReducer, useRef } from "react";
import { useQuery } from "@tanstack/react-query";

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
  useSheinStudioBatchActions,
  useSheinStudioWorkspaceLoader,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-workspace";
import {
  SheinStudioReviewStep,
  SheinStudioWorkbenchAlerts,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-sections";
import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import {
  buildSheinStudioSelectionKey,
  getSheinStudioCreateActionDisabledReason,
  sheinStudioBusyMessage,
  summarizeSheinStudioSelection,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import {
  applySheinStudioWorkbenchBatch,
  applySheinStudioWorkbenchDraft,
  buildInitialSheinStudioWorkbenchState,
  setSheinStudioWorkbenchField,
  sheinStudioWorkbenchReducer,
  type SheinStudioWorkbenchState,
  type SheinStudioWorkbenchStateUpdater,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import {
  buildDefaultSelectedSDSImages,
  buildSelectableSDSImages,
} from "@/lib/shein-studio/sds-selectable-images";
import { getCurrentSubscription } from "@/lib/api/subscription";
import { useSheinStoreSelector } from "@/lib/query/use-shein-store-selector";
import type { SDSProductVariantSelection } from "@/lib/types/sds";

export function SheinStudioWorkbench({
  activeStep = "generate",
  selection,
}: {
  activeStep?: SheinStudioStepKey;
  selection?: SDSProductVariantSelection;
}) {
  const [workbenchState, dispatchWorkbenchState] = useReducer(
    sheinStudioWorkbenchReducer,
    undefined,
    buildInitialSheinStudioWorkbenchState,
  );
  const setWorkbenchField = useCallback(
    <K extends keyof SheinStudioWorkbenchState>(
      field: K,
      value: SheinStudioWorkbenchStateUpdater<K>,
    ) => {
      dispatchWorkbenchState(setSheinStudioWorkbenchField(field, value));
    },
    [],
  );
  const workbenchController = useMemo(
    () => ({
      applyBatch: (batch: Parameters<typeof applySheinStudioWorkbenchBatch>[0]) =>
        dispatchWorkbenchState(applySheinStudioWorkbenchBatch(batch)),
      applyDraft: (draft: Parameters<typeof applySheinStudioWorkbenchDraft>[0]) =>
        dispatchWorkbenchState(applySheinStudioWorkbenchDraft(draft)),
      setField: setWorkbenchField,
    }),
    [setWorkbenchField],
  );
  const workbenchSetters = useMemo(
    () => ({
      setArtworkModel: (value: SheinStudioWorkbenchStateUpdater<"artworkModel">) =>
        setWorkbenchField("artworkModel", value),
      setCreatedTasks: (value: SheinStudioWorkbenchStateUpdater<"createdTasks">) =>
        setWorkbenchField("createdTasks", value),
      setCreatingError: (value: SheinStudioWorkbenchStateUpdater<"creatingError">) =>
        setWorkbenchField("creatingError", value),
      setCreatingMessage: (
        value: SheinStudioWorkbenchStateUpdater<"creatingMessage">,
      ) => setWorkbenchField("creatingMessage", value),
      setDesigns: (value: SheinStudioWorkbenchStateUpdater<"designs">) =>
        setWorkbenchField("designs", value),
      setDraftWarning: (value: SheinStudioWorkbenchStateUpdater<"draftWarning">) =>
        setWorkbenchField("draftWarning", value),
      setGalleryRatioCheck: (
        value: SheinStudioWorkbenchStateUpdater<"galleryRatioCheck">,
      ) => setWorkbenchField("galleryRatioCheck", value),
      setGenerationError: (
        value: SheinStudioWorkbenchStateUpdater<"generationError">,
      ) => setWorkbenchField("generationError", value),
      setGenerationWarning: (
        value: SheinStudioWorkbenchStateUpdater<"generationWarning">,
      ) => setWorkbenchField("generationWarning", value),
      setImageStrategy: (value: SheinStudioWorkbenchStateUpdater<"imageStrategy">) =>
        setWorkbenchField("imageStrategy", value),
      setIsCreatingTasks: (
        value: SheinStudioWorkbenchStateUpdater<"isCreatingTasks">,
      ) => setWorkbenchField("isCreatingTasks", value),
      setIsGenerating: (value: SheinStudioWorkbenchStateUpdater<"isGenerating">) =>
        setWorkbenchField("isGenerating", value),
      setIsLoadingWorkspace: (
        value: SheinStudioWorkbenchStateUpdater<"isLoadingWorkspace">,
      ) => setWorkbenchField("isLoadingWorkspace", value),
      setProductImageCount: (
        value: SheinStudioWorkbenchStateUpdater<"productImageCount">,
      ) => setWorkbenchField("productImageCount", value),
      setProductImagePrompt: (
        value: SheinStudioWorkbenchStateUpdater<"productImagePrompt">,
      ) => setWorkbenchField("productImagePrompt", value),
      setProductImagePrompts: (
        value: SheinStudioWorkbenchStateUpdater<"productImagePrompts">,
      ) => setWorkbenchField("productImagePrompts", value),
      setPrompt: (value: SheinStudioWorkbenchStateUpdater<"prompt">) =>
        setWorkbenchField("prompt", value),
      setRegeneratingId: (
        value: SheinStudioWorkbenchStateUpdater<"regeneratingId">,
      ) => setWorkbenchField("regeneratingId", value),
      setRenderSizeImagesWithSds: (
        value: SheinStudioWorkbenchStateUpdater<"renderSizeImagesWithSds">,
      ) => setWorkbenchField("renderSizeImagesWithSds", value),
      setSavedBatches: (value: SheinStudioWorkbenchStateUpdater<"savedBatches">) =>
        setWorkbenchField("savedBatches", value),
      setSaveMessage: (value: SheinStudioWorkbenchStateUpdater<"saveMessage">) =>
        setWorkbenchField("saveMessage", value),
      setSelectedIds: (value: SheinStudioWorkbenchStateUpdater<"selectedIds">) =>
        setWorkbenchField("selectedIds", value),
      setSelectedSdsImages: (
        value: SheinStudioWorkbenchStateUpdater<"selectedSdsImages">,
      ) => setWorkbenchField("selectedSdsImages", value),
      setSheinStoreId: (value: SheinStudioWorkbenchStateUpdater<"sheinStoreId">) =>
        setWorkbenchField("sheinStoreId", value),
      setStyleCount: (value: SheinStudioWorkbenchStateUpdater<"styleCount">) =>
        setWorkbenchField("styleCount", value),
      setTransparentBackground: (
        value: SheinStudioWorkbenchStateUpdater<"transparentBackground">,
      ) => setWorkbenchField("transparentBackground", value),
      setVariationIntensity: (
        value: SheinStudioWorkbenchStateUpdater<"variationIntensity">,
      ) => setWorkbenchField("variationIntensity", value),
    }),
    [setWorkbenchField],
  );
  const {
    artworkModel,
    createdTasks,
    creatingError,
    creatingMessage,
    designs,
    draftWarning,
    galleryRatioCheck,
    generationError,
    generationWarning,
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
    savedBatches,
    saveMessage,
    selectedIds,
    selectedSdsImages,
    sheinStoreId,
    styleCount,
    transparentBackground,
    variationIntensity,
  } = workbenchState;
  const {
    setArtworkModel,
    setDesigns,
    setDraftWarning,
    setImageStrategy,
    setProductImageCount,
    setProductImagePrompt,
    setProductImagePrompts,
    setPrompt,
    setRenderSizeImagesWithSds,
    setSelectedIds,
    setSelectedSdsImages,
    setSheinStoreId,
    setStyleCount,
    setTransparentBackground,
    setVariationIntensity,
  } = workbenchSetters;
  const hasLocalWorkflowStateRef = useRef(false);
  const hasCustomizedSdsSelectionRef = useRef(false);
  const promptInputRef = useRef<HTMLTextAreaElement>(null);
  const activeSelection = useHydratedSDSVariantSelection(selection);
  const { recommendedStoreId } = useSheinStoreSelector();
  const subscriptionQuery = useQuery({
    queryKey: ["listingkit-subscription"],
    queryFn: getCurrentSubscription,
  });
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
  const studioAccessAllowed =
    subscriptionQuery.data?.entitlements?.find(
      (view) => view.module.code === "studio",
    )?.allowed ?? true;
  const subscriptionBlockedMessage =
    subscriptionQuery.data && !studioAccessAllowed
      ? "当前租户未开通 Studio 模块。请在“当前租户订阅”里开通 Studio，或切换到已开通的租户后再生成款式图。"
      : "";
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
    hasLocalWorkflowStateRef.current = false;
    hasCustomizedSdsSelectionRef.current = false;
  }, [selection?.variantId]);

  useEffect(() => {
    if ((sheinStoreId ?? "").trim()) {
      return;
    }
    if (!recommendedStoreId) {
      return;
    }
    setSheinStoreId(recommendedStoreId);
  }, [recommendedStoreId, setSheinStoreId, sheinStoreId]);

  useSheinStudioWorkspaceLoader({
    activeSelection,
    activeSelectionKey,
    activeStepRef,
    hasCustomizedSdsSelectionRef,
    hasLocalWorkflowStateRef,
    setEffectiveStep,
    workbench: workbenchController,
  });

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
    setSelectedSdsImages,
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
      workbench: workbenchController,
      sheinStoreId,
      styleCount,
      transparentBackground,
      variationIntensity,
      hasLocalWorkflowStateRef,
    });

  const { handleDeleteBatch, handleLoadBatch, handleSaveBatch } =
    useSheinStudioBatchActions({
      activeStep,
      buildDraftInput,
      hasCustomizedSdsSelectionRef,
      hasLocalWorkflowStateRef,
      setEffectiveStep,
      workbench: workbenchController,
    });

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
        generationWarning={generationWarning}
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
          subscriptionBlockedMessage={subscriptionBlockedMessage}
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
