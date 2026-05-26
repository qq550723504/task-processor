"use client";

import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { SheinStudioBusyOverlay } from "@/components/listingkit/shein-studio/shein-studio-busy-overlay";
import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import { SheinStudioGroupedSelectionPanel } from "@/components/listingkit/shein-studio/shein-studio-grouped-selection-panel";
import { SheinStudioSelectionOverview } from "@/components/listingkit/shein-studio/shein-studio-selection-overview";
import { SheinStudioTasksStep } from "@/components/listingkit/shein-studio/shein-studio-tasks-step";
import { useSheinStudioDesignActions } from "@/components/listingkit/shein-studio/shein-studio-workbench-actions";
import {
  useHydratedSDSVariantSelection,
  useSheinStudioPendingNavigationGuard,
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
import { getSDSBaselineReadiness } from "@/lib/api/sds-baseline";
import { warmSDSBaselineForSelection } from "@/lib/api/sds-baseline";
import { getCurrentSubscription } from "@/lib/api/subscription";
import { useSDSGroupedCandidates } from "@/lib/query/use-sds-grouped-candidates";
import { useSheinStoreSelector } from "@/lib/query/use-shein-store-selector";
import {
  buildGroupedSDSSelectionID,
  type GroupedSDSSelectionEligibility,
  type SDSBaselineStatus,
} from "@/lib/types/sds-baseline";
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
      setGenerationWarningAction: (
        value: SheinStudioWorkbenchStateUpdater<"generationWarningAction">,
      ) => setWorkbenchField("generationWarningAction", value),
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
      setGroupedSelections: (
        value: SheinStudioWorkbenchStateUpdater<"groupedSelections">,
      ) => setWorkbenchField("groupedSelections", value),
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
    generationWarningAction,
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
    groupedSelections,
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
    setGroupedSelections,
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
  const groupedCandidateSelections = useSDSGroupedCandidates();
  const [isExecutingWarningAction, setIsExecutingWarningAction] = useState(false);
  const [baselineStatuses, setBaselineStatuses] = useReducer(
    (
      _current: Record<
        string,
        { status: SDSBaselineStatus; reason: string; baselineKey?: string }
      >,
      next: Record<
        string,
        { status: SDSBaselineStatus; reason: string; baselineKey?: string }
      >,
    ) => next,
    {},
  );
  const { recommendedStoreId, enabledProfiles } = useSheinStoreSelector();
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
  const activeGroupedSelectionID = buildGroupedSDSSelectionID(activeSelection);
  const groupedSelectionCandidates = useMemo(
    () =>
      groupedCandidateSelections.filter(
        (item) =>
          item.variantId !== activeSelection?.variantId &&
          Boolean(buildGroupedSDSSelectionID(item)),
      ),
    [activeSelection?.variantId, groupedCandidateSelections],
  );
  const activeSelectionBaseline = baselineStatuses[activeGroupedSelectionID] ?? {
    status: "missing" as SDSBaselineStatus,
    reason: activeSelection?.variantId ? "正在检查 baseline 状态..." : "",
  };
  const groupedCandidates = useMemo(
    () =>
      groupedSelectionCandidates.map((item) => {
        const selectionId = buildGroupedSDSSelectionID(item);
        const baseline = baselineStatuses[selectionId] ?? {
          status: "missing" as SDSBaselineStatus,
          reason: "正在检查 baseline 状态...",
        };
        const compatibility = evaluateGroupedSelectionCompatibility(
          activeSelection,
          item,
        );
        return {
          selectionId,
          selection: item,
          baselineStatus: baseline.status,
          baselineReason: baseline.reason,
          baselineKey: baseline.baselineKey,
          eligible: baseline.status === "ready" && compatibility.compatible,
          eligibilityReason:
            baseline.status !== "ready"
              ? baseline.reason || "只有 baseline ready 的 SDS 商品才能加入分组。"
              : compatibility.reason,
        };
      }),
    [activeSelection, baselineStatuses, groupedSelectionCandidates],
  );
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
    groupedSelections,
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

  const handleGenerationWarningAction = useCallback(() => {
    if (generationWarningAction?.intent !== "focus_generate") {
      return;
    }
    navigateToStep("generate");
    window.setTimeout(() => {
      const generator = document.getElementById("shein-studio-generator");
      generator?.scrollIntoView({ behavior: "smooth", block: "start" });
      promptInputRef.current?.focus();
    }, 0);
  }, [generationWarningAction?.intent, navigateToStep]);

  const handleWarmBaselineAction = useCallback(async () => {
    if (generationWarningAction?.intent !== "warm_baseline" || !activeSelection?.variantId) {
      return;
    }
    const activeSelectionId = buildGroupedSDSSelectionID(activeSelection);
    setIsExecutingWarningAction(true);
    setWorkbenchField("generationError", "");
    try {
      const readiness = await warmSDSBaselineForSelection(activeSelection);
      setBaselineStatuses({
        ...baselineStatuses,
        [activeSelectionId]: {
          status: readiness.status,
          reason: readiness.reason ?? "",
          baselineKey: readiness.baselineKey,
        },
      });
      setWorkbenchField(
        "generationWarning",
        readiness.status === "ready"
          ? "这款 SDS 商品的 baseline 已预热完成，现在可以继续加入 grouped 批量上品。"
          : readiness.reason || "baseline 预热已发起，请稍后再试。",
      );
      setWorkbenchField("generationWarningAction", null);
    } catch (error) {
      setWorkbenchField(
        "generationWarning",
        error instanceof Error ? error.message : "baseline 预热失败。",
      );
    } finally {
      setIsExecutingWarningAction(false);
    }
  }, [
    activeSelection,
    baselineStatuses,
    generationWarningAction?.intent,
    setWorkbenchField,
  ]);

  useEffect(() => {
    const selections = [
      ...(activeSelection?.variantId ? [activeSelection] : []),
      ...groupedSelectionCandidates,
    ];
    if (selections.length === 0) {
      setBaselineStatuses({});
      return;
    }

    let cancelled = false;
    void Promise.all(
      selections.map(async (item) => {
        const selectionId = buildGroupedSDSSelectionID(item);
        try {
          const readiness = await getSDSBaselineReadiness({
            parentProductId: item.parentProductId,
            prototypeGroupId: item.prototypeGroupId,
            variantId: item.variantId,
            selectedVariantIds: item.selectedVariantIds,
          });
          return [
            selectionId,
            {
              status: readiness.status,
              reason: readiness.reason ?? "",
              baselineKey: readiness.baselineKey,
            },
          ] as const;
        } catch (error) {
          return [
            selectionId,
            {
              status: "failed" as SDSBaselineStatus,
              reason:
                error instanceof Error
                  ? error.message
                  : "读取 SDS baseline 状态失败。",
            },
          ] as const;
        }
      }),
    ).then((entries) => {
      if (cancelled) {
        return;
      }
      setBaselineStatuses(Object.fromEntries(entries));
    });

    return () => {
      cancelled = true;
    };
  }, [activeSelection, groupedSelectionCandidates]);

  useEffect(() => {
    setGroupedSelections((current) =>
      current.map((item) => {
        const baseline = baselineStatuses[item.selectionId] ?? {
          status: item.baselineStatus,
          reason: item.baselineReason,
          baselineKey: item.baselineKey,
        };
        const compatibility = evaluateGroupedSelectionCompatibility(
          activeSelection,
          item.selection,
        );
        return {
          ...item,
          baselineKey: baseline.baselineKey,
          baselineStatus: baseline.status,
          baselineReason: baseline.reason,
          eligible: baseline.status === "ready" && compatibility.compatible,
          eligibilityReason:
            baseline.status !== "ready"
              ? baseline.reason || "只有 baseline ready 的 SDS 商品才能加入分组。"
              : compatibility.reason,
        };
      }),
    );
  }, [activeSelection, baselineStatuses, setGroupedSelections]);

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
      groupedSelections,
      activeSelectionBaselineStatus: activeSelectionBaseline.status,
      activeSelectionBaselineReason: activeSelectionBaseline.reason,
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
  useSheinStudioPendingNavigationGuard({
    enabled: Boolean(isGenerating || isCreatingTasks || regeneratingId),
    message:
      "当前正在生成款式图或创建 SHEIN 资料。现在离开会中断当前页面上的进度承接，确认还要离开吗？",
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
        generationWarningAction={
          generationWarningAction
            ? {
                ...generationWarningAction,
                label:
                  isExecutingWarningAction &&
                  generationWarningAction.intent === "warm_baseline"
                    ? "预热中..."
                    : generationWarningAction.label,
                onClick:
                  generationWarningAction.intent === "warm_baseline"
                    ? () => {
                        void handleWarmBaselineAction();
                      }
                    : handleGenerationWarningAction,
              }
            : null
        }
        galleryRatioCheck={galleryRatioCheck}
      />

      {effectiveStep === "generate" ? (
        <div className="space-y-4">
          <SheinStudioGroupedSelectionPanel
            activeSelection={activeSelection}
            activeSelectionBaselineReason={activeSelectionBaseline.reason}
            activeSelectionBaselineStatus={activeSelectionBaseline.status}
            candidates={groupedCandidates}
            groupedSelections={groupedSelections}
            onAddSelection={(candidate) =>
              setGroupedSelections((current) => {
                if (current.some((item) => item.selectionId === candidate.selectionId)) {
                  return current;
                }
                return [
                  ...current,
                  {
                    selectionId: candidate.selectionId,
                    selection: candidate.selection,
                    baselineKey: candidate.baselineKey,
                    baselineStatus: candidate.baselineStatus,
                    baselineReason: candidate.baselineReason,
                    sheinStoreId: sheinStoreId.trim(),
                    eligible: candidate.eligible,
                    eligibilityReason: candidate.eligibilityReason,
                  },
                ];
              })
            }
            onRemoveSelection={(selectionId) =>
              setGroupedSelections((current) =>
                current.filter((item) => item.selectionId !== selectionId),
              )
            }
            onUpdateSelectionStore={(selectionId, storeId) =>
              setGroupedSelections((current) =>
                current.map((item) =>
                  item.selectionId === selectionId
                    ? { ...item, sheinStoreId: storeId }
                    : item,
                ),
              )
            }
            storeOptions={enabledProfiles}
          />
          <SheinStudioGenerationPanel
            artworkModel={artworkModel}
            availableSdsImages={availableSdsImages}
            createTaskButtonLabel={
              groupedSelections.length > 0
                ? `为 ${groupedSelections.length + 1} 款商品生成 SHEIN 资料`
                : "生成 SHEIN 资料"
            }
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
        </div>
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

function evaluateGroupedSelectionCompatibility(
  activeSelection?: SDSProductVariantSelection,
  candidate?: SDSProductVariantSelection,
) {
  if (!activeSelection?.variantId || !candidate?.variantId) {
    return { compatible: false, reason: "缺少 SDS 选择信息，暂时无法加入分组。" };
  }
  if (activeSelection.variantId === candidate.variantId) {
    return { compatible: false, reason: "当前主商品已经在工作台中，无需重复加入。" };
  }
  if (
    activeSelection.printableWidth &&
    candidate.printableWidth &&
    activeSelection.printableWidth !== candidate.printableWidth
  ) {
    return {
      compatible: false,
      reason: "印刷宽度与当前主商品不一致，先不要混在同一批创建。",
    };
  }
  if (
    activeSelection.printableHeight &&
    candidate.printableHeight &&
    activeSelection.printableHeight !== candidate.printableHeight
  ) {
    return {
      compatible: false,
      reason: "印刷高度与当前主商品不一致，先不要混在同一批创建。",
    };
  }
  return { compatible: true, reason: "" };
}
