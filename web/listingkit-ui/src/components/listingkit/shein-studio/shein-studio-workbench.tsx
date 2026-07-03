"use client";

import {
  useCallback,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  useState,
} from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { SheinStudioBatchQueueBanner } from "@/components/listingkit/shein-studio/shein-studio-batch-queue-banner";
import { SheinStudioBatchRunProgress } from "@/components/listingkit/shein-studio/shein-studio-batch-run-progress";
import { SheinStudioBusyOverlay } from "@/components/listingkit/shein-studio/shein-studio-busy-overlay";
import { BatchStoreSettings } from "@/components/listingkit/shein-studio/shein-studio-generation-form-sections";
import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import {
  projectGroupedSelectionBaselineEligibility,
  SheinStudioGroupedSelectionPanel,
} from "@/components/listingkit/shein-studio/shein-studio-grouped-selection-panel";
import { SheinStudioRecentBatchesDashboard } from "@/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard";
import { SheinStudioTasksStep } from "@/components/listingkit/shein-studio/shein-studio-tasks-step";
import { useSheinStudioDedicatedBatchRunController } from "@/components/listingkit/shein-studio/shein-studio-dedicated-batch-run-controller";
import {
  projectActiveSelectionBaselineState,
  useActiveSelectionBaselineStatuses,
  useBaselineWarmupAction,
  useSheinStudioBatchGenerationContext,
} from "@/components/listingkit/shein-studio/shein-studio-generation-controller";
import { useSheinStudioInitialBatchHydration } from "@/components/listingkit/shein-studio/shein-studio-hydration-controller";
import {
  applyLocalDraftRecoveryToWorkbench,
  buildSheinStudioDraftPersistenceState,
  projectLocalDraftRecovery,
  resetDedicatedBatchPromptOverrides,
  useSheinStudioDedicatedDraftPersistence,
} from "@/components/listingkit/shein-studio/shein-studio-persistence-controller";
import {
  getBatchRunStartErrorMessage,
  useSheinStudioQueueController,
} from "@/components/listingkit/shein-studio/shein-studio-queue-controller";
import {
  deleteRecentBatchSummary,
  duplicateRecentBatchSummary,
  mergeRecentBatchHydrations,
  refreshRecentSavedBatches,
  renameRecentBatchSummary,
  removeRecentBatchSummarySelection,
  resolveRecentBatchHydrationEntries,
  runRecentBatchBulkDelete,
  runRecentBatchBulkStoreUpdate,
  runRecentBatchSummarySelection,
  resolveRecentBatchForMutation as resolveRecentBatchForMutationTarget,
  upsertRecentSavedBatch,
  useRecentBatchSummaries,
  useRecentBatchSummarySelection,
} from "@/components/listingkit/shein-studio/shein-studio-recent-batch-controller";
import {
  projectItemizedBatchDetail,
  projectItemizedFailedRetryRequest,
  projectItemizedFailedRetryStep,
  projectItemizedReviewNoteUpdate,
  projectItemizedSelectionToggle,
  projectItemizedTaskRecoveryState,
  projectItemizedTaskCreationProgressEffects,
  projectItemizedTaskCreationProgress,
  loadItemizedGenerationPollBatch,
  runItemizedDesignApproval,
  runItemizedFailedRetry,
  usePendingItemizedTaskDesignIds,
  useSheinStudioItemizedBatchContext,
} from "@/components/listingkit/shein-studio/shein-studio-task-creation-controller";
import {
  clearWorkbenchTaskRecoveryAlerts,
  useSheinStudioDesignActions,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-actions";
import {
  clearLocalSheinStudioDraftSnapshot,
  useHydratedSDSVariantSelection,
  loadLocalSheinStudioDraftSnapshotDetail,
  useSheinStudioActiveBatchScope,
  useSheinStudioActiveGroupPrimarySelection,
  useSheinStudioActiveGroupPromptHistory,
  useSheinStudioActiveSelectionSummary,
  useSheinStudioBusyMessage,
  useSheinStudioCreateActionDisabledReason,
  useSheinStudioCurrentBatchSelection,
  useSheinStudioItemizedGenerationInFlight,
  useSheinStudioQueueState,
  useSheinStudioStoreSelection,
  useSheinStudioSubscriptionGate,
  useSheinStudioWorkbenchTraceContext,
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
  getApprovedItemizedBatchDesignIDs,
  projectDefaultSelectedSDSImages,
  type SheinStudioWorkbenchHydratedBatch,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import {
  buildInitialSheinStudioWorkbenchState,
  buildSheinStudioWorkbenchController,
  buildSheinStudioWorkbenchSetters,
  sheinStudioWorkbenchReducer,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import { buildSelectableSDSImages } from "@/lib/shein-studio/sds-selectable-images";
import { type SheinStudioBatchQueueResumeState } from "@/lib/shein-studio/batch-queue";
import {
  clearListingKitTraceContext,
  writeListingKitTraceContext,
} from "@/lib/listingkit/request-trace";
import { getSDSBaselineReadiness } from "@/lib/api/sds-baseline";
import { warmSDSBaselineForSelection } from "@/lib/api/sds-baseline";
import { analyzeSheinStudioReferenceStyle } from "@/lib/api/shein-studio";
import { startSheinStudioBatchRun } from "@/lib/api/shein-studio-batch-runs";
import { getCurrentSubscription } from "@/lib/api/subscription";
import { formatSubscriptionApiError } from "@/lib/api/subscription";
import { useSheinStoreSelector } from "@/lib/query/use-shein-store-selector";
import {
  approveSheinStudioBatchDesigns,
  retrySheinStudioBatchItems,
} from "@/lib/api/shein-studio-batches";
import { useToast } from "@/components/providers/toast-provider";
import {
  getSheinStudioHydratedBatch,
  listSheinStudioBatches,
  saveSheinStudioBatch,
  setActiveSheinStudioBatchId,
} from "@/lib/utils/shein-studio-batches";
import {
  buildGroupedSDSSelectionID,
  countSelectionsWithPrimary,
} from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";

type SheinStudioWorkbenchProps = {
  activeStep?: SheinStudioStepKey;
  initialBatchId?: string;
  selection?: SDSProductVariantSelection;
};

export { resetDedicatedBatchPromptOverrides };

export function SheinStudioWorkbench({
  activeStep = "generate",
  initialBatchId,
  selection,
}: SheinStudioWorkbenchProps) {
  const toast = useToast();
  const router = useRouter();
  const selectionVariantId = selection?.variantId ?? null;
  const { activeBatchId, setActiveBatchId } = useSheinStudioActiveBatchScope({
    initialBatchId,
    selectionVariantId,
  });
  const [isDedicatedBatchLoaded, setIsDedicatedBatchLoaded] = useState(
    () => !initialBatchId,
  );
  const [localDraftSnapshotDetail, setLocalDraftSnapshotDetail] = useState(() =>
    loadLocalSheinStudioDraftSnapshotDetail(),
  );
  const [workbenchState, dispatchWorkbenchState] = useReducer(
    sheinStudioWorkbenchReducer,
    undefined,
    buildInitialSheinStudioWorkbenchState,
  );
  const workbenchController = useMemo(
    () => buildSheinStudioWorkbenchController(dispatchWorkbenchState),
    [],
  );
  const workbenchSetters = useMemo(
    () => buildSheinStudioWorkbenchSetters(workbenchController.setField),
    [workbenchController],
  );
  const {
    activeGroupId,
    artworkModel,
    createdTasks,
    creatingError,
    creatingWarning,
    creatingMessage,
    designs,
    draftWarning,
    galleryRatioCheck,
    generationError,
    generationWarning,
    generationWarningAction,
    groups,
    groupedImageMode,
    hotStyleReferenceBrief,
    hotStyleReferenceImageUrls,
    hotStyleReferencePrompt,
    imageStrategy,
    isCreatingTasks,
    isGenerating,
    isLoadingWorkspace,
    itemizedBatchDetail,
    productImageCount,
    productImagePrompt,
    productImagePrompts,
    persistedUpdatedAt,
    prompt,
    promptMode,
    queueMessage,
    regeneratingId,
    renderSizeImagesWithSds,
    selection: loadedSelection,
    groupedSelections,
    generationJobs,
    batchQueueMode,
    queuedBatchIds,
    queuedBatchIndex,
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
    setBatchQueueMode,
    setDesigns,
    setDraftWarning,
    setGroupedImageMode,
    setHotStyleReferenceBrief,
    setHotStyleReferenceImageUrls,
    setHotStyleReferencePrompt,
    setImageStrategy,
    setGroupedSelections,
    setPersistedUpdatedAt,
    setProductImageCount,
    setProductImagePrompt,
    setProductImagePrompts,
    setPrompt,
    setPromptMode,
    setQueueMessage,
    setQueuedBatchIds,
    setQueuedBatchIndex,
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
  const directSelection = useHydratedSDSVariantSelection(selection);
  const loadedBatchSelection = useHydratedSDSVariantSelection(loadedSelection);
  const activeGroupPrimarySelection = useSheinStudioActiveGroupPrimarySelection(
    {
      activeGroupId,
      groups,
    },
  );
  const activeGroupSelection = useHydratedSDSVariantSelection(
    activeGroupPrimarySelection,
  );
  const activeSelection =
    directSelection ?? activeGroupSelection ?? loadedBatchSelection;
  const [retryingFailedItemId, setRetryingFailedItemId] = useState("");
  const [
    rawSelectedRecentBatchSummaryIds,
    setRawSelectedRecentBatchSummaryIds,
  ] = useState<string[]>([]);
  const [queueResumeState, setQueueResumeState] =
    useState<SheinStudioBatchQueueResumeState | null>(null);
  const [activeBatchRunId, setActiveBatchRunId] = useState("");
  const [batchRunError, setBatchRunError] = useState("");
  const [selectedRecentBatchHydrations, setSelectedRecentBatchHydrations] =
    useState<Record<string, SheinStudioWorkbenchHydratedBatch>>({});
  const selectedRecentBatchHydrationRequestsRef = useRef(
    new Map<string, Promise<SheinStudioWorkbenchHydratedBatch | null>>(),
  );
  const recentBatchOpenRequestVersionRef = useRef(0);
  const batchQueueRequestVersionRef = useRef(0);
  const taskCreationToastSignatureRef = useRef("");
  const { baselineStatuses, setBaselineStatuses } =
    useActiveSelectionBaselineStatuses({
      activeSelection,
      getReadiness: getSDSBaselineReadiness,
    });
  useEffect(() => {
    if (!initialBatchId) {
      return;
    }
    setActiveSheinStudioBatchId(initialBatchId);
  }, [initialBatchId]);
  const { enabledProfiles } = useSheinStoreSelector();
  const subscriptionQuery = useQuery({
    queryKey: ["listingkit-subscription"],
    queryFn: getCurrentSubscription,
  });
  const { activeStepRef, effectiveStep, navigateToStep, setEffectiveStep } =
    useSheinStudioStepNavigation(activeStep);
  const {
    activeSelectionKey,
    printableAreaLabel,
    selectedColorCount,
    selectedSizeCount,
    selectedVariants,
  } = useSheinStudioActiveSelectionSummary(activeSelection);
  const availableSdsImages = buildSelectableSDSImages(activeSelection);
  const activeGroupedSelectionID = buildGroupedSDSSelectionID(activeSelection);
  const {
    baseline: activeSelectionBaseline,
    handoff: activeSelectionBaselineHandoff,
    reason: activeSelectionBaselineReason,
  } = useMemo(
    () =>
      projectActiveSelectionBaselineState({
        activeGroupedSelectionID,
        baselineStatuses,
        hasActiveSelection: Boolean(activeSelection?.variantId),
      }),
    [activeGroupedSelectionID, activeSelection?.variantId, baselineStatuses],
  );
  const { subscriptionBlockedMessage } = useSheinStudioSubscriptionGate(
    subscriptionQuery.data,
  );
  const {
    currentStoreLabel,
    effectiveCurrentStoreId,
    recentBatchStoreOptions,
    storeRequiredMessage,
  } = useSheinStudioStoreSelection({
    currentStoreId: sheinStoreId,
    enabledProfiles,
  });
  const activeGroupPromptHistory = useSheinStudioActiveGroupPromptHistory({
    activeGroupId,
    groups,
  });
  const localDraftSnapshot = localDraftSnapshotDetail?.draft ?? null;
  const recentBatchSummaries = useRecentBatchSummaries({
    localDraftSnapshotDetail,
    savedBatches,
    selectedRecentBatchHydrations,
  });
  const {
    selectedPersistedRecentBatchIds,
    selectedRecentBatchSummaryIds,
    setSelectedRecentBatchSummaryIds,
  } = useRecentBatchSummarySelection({
    rawSelectedRecentBatchSummaryIds,
    recentBatchSummaries,
    setRawSelectedRecentBatchSummaryIds,
  });
  const isRecentBatchesHomepage = effectiveStep === "select";
  const {
    batchQueueGuidance,
    currentQueuedBatch,
    currentQueuedBatchId,
    resumableQueueBatchIds,
  } = useSheinStudioQueueState({
    batchQueueMode,
    effectiveStep,
    queueResumeState,
    queuedBatchIds,
    queuedBatchIndex,
    savedBatches,
  });
  const batchTraceContext = useSheinStudioWorkbenchTraceContext({
    activeBatchId,
    batchQueueMode,
    currentQueuedBatchId,
    initialBatchId,
    queuedBatchIds,
    queuedBatchIndex,
  });
  const { currentActiveBatch, currentDedicatedBatch } =
    useSheinStudioCurrentBatchSelection({
      activeBatchId,
      initialBatchId,
      savedBatches,
      workbenchState,
    });
  const hydrateRecentBatchSelection = useCallback(
    async (batchIds: string[]) => {
      const hydratedEntries = await resolveRecentBatchHydrationEntries({
        batchIds,
        loadHydratedBatch: getSheinStudioHydratedBatch,
        pendingHydrationRequests:
          selectedRecentBatchHydrationRequestsRef.current,
        savedBatches,
        selectedRecentBatchHydrations,
      });
      if (hydratedEntries.length > 0) {
        setSelectedRecentBatchHydrations((current) =>
          mergeRecentBatchHydrations(current, hydratedEntries),
        );
      }
      return Object.fromEntries(hydratedEntries) as Record<
        string,
        SheinStudioWorkbenchHydratedBatch
      >;
    },
    [savedBatches, selectedRecentBatchHydrations],
  );
  useEffect(() => {
    let cancelled = false;
    if (selectedPersistedRecentBatchIds.length === 0) {
      return;
    }

    void hydrateRecentBatchSelection(selectedPersistedRecentBatchIds).then(
      () => {
        if (cancelled) {
          return;
        }
      },
    );

    return () => {
      cancelled = true;
    };
  }, [hydrateRecentBatchSelection, selectedPersistedRecentBatchIds]);
  const draftPersistenceState = useMemo(
    () =>
      buildSheinStudioDraftPersistenceState({
        activeSelection,
        artworkModel,
        createdTasks,
        currentGenerationJobId: currentActiveBatch?.generationJobId,
        designs,
        generationError,
        generationJobs,
        groups,
        groupedImageMode,
        groupedSelections,
        hotStyleReferenceBrief,
        hotStyleReferenceImageUrls,
        hotStyleReferencePrompt,
        imageStrategy,
        isCreatingTasks,
        isGenerating,
        isLoadingWorkspace,
        persistedUpdatedAt,
        productImageCount,
        productImagePrompt,
        productImagePrompts,
        prompt,
        promptMode,
        regeneratingId,
        renderSizeImagesWithSds,
        selectedIds,
        selectedSdsImages,
        setDraftWarning,
        setPersistedUpdatedAt,
        sheinStoreId,
        styleCount,
        transparentBackground,
        variationIntensity,
      }),
    [
      activeSelection,
      artworkModel,
      createdTasks,
      currentActiveBatch?.generationJobId,
      designs,
      generationError,
      generationJobs,
      groups,
      groupedImageMode,
      hotStyleReferenceBrief,
      hotStyleReferenceImageUrls,
      hotStyleReferencePrompt,
      imageStrategy,
      isCreatingTasks,
      isGenerating,
      isLoadingWorkspace,
      productImageCount,
      productImagePrompt,
      productImagePrompts,
      persistedUpdatedAt,
      prompt,
      promptMode,
      regeneratingId,
      renderSizeImagesWithSds,
      groupedSelections,
      selectedIds,
      selectedSdsImages,
      setDraftWarning,
      setPersistedUpdatedAt,
      sheinStoreId,
      styleCount,
      transparentBackground,
      variationIntensity,
    ],
  );
  const { buildDraftInput, persistDraft } = useSheinStudioDraftPersistence(
    draftPersistenceState,
    {
      activeBatchId,
      persistenceEnabled: !initialBatchId || isDedicatedBatchLoaded,
    },
  );
  const {
    buildResultBackedDraftInput,
    promptOverride: dedicatedBatchPromptOverride,
    saveDedicatedBatchDraftSnapshot,
  } = useSheinStudioDedicatedDraftPersistence({
    buildDraftInput,
    createdTasks,
    currentGenerationJobId: currentActiveBatch?.generationJobId ?? "",
    designs,
    generationError,
    generationJobs,
    initialBatchId,
    selectedIds,
  });
  const { batchGenerationContext } = useSheinStudioBatchGenerationContext({
    activeBatchId,
    buildDraftInput,
    createdTasks,
    currentGenerationJobId: currentActiveBatch?.generationJobId ?? "",
    designs,
    enabled: Boolean(activeSelection?.variantId),
    generationError,
    generationJobs,
    getHydratedBatch: getSheinStudioHydratedBatch,
    initialBatchId,
    saveBatch: saveSheinStudioBatch,
    selectedIds,
    setActiveBatchId,
    setActiveBatchRunId,
    setActiveSavedBatchId: setActiveSheinStudioBatchId,
    setBatchRunError,
    setSavedBatches: (updater) =>
      workbenchController.setField("savedBatches", updater),
    startBatchRun: startSheinStudioBatchRun,
    upsertSavedBatch: upsertRecentSavedBatch,
  });
  const { itemizedBatchContext } = useSheinStudioItemizedBatchContext({
    activeBatchId,
    activeSelection,
    applyHydratedBatch: workbenchController.applyHydratedBatch,
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
    hotStyleReferenceImageUrls,
    hotStyleReferenceBrief,
    hotStyleReferencePrompt,
    prompt,
    renderSizeImagesWithSds,
    selectedSdsImages,
    setSavedBatches: (updater) =>
      workbenchController.setField("savedBatches", updater),
    sheinStoreId,
    styleCount,
    transparentBackground,
    variationIntensity,
  });
  const createActionDisabledReason = useSheinStudioCreateActionDisabledReason({
    galleryRatioCheck,
    hasItemizedBatchContext: Boolean(itemizedBatchContext),
    itemizedApprovedCount: itemizedBatchContext
      ? getApprovedItemizedBatchDesignIDs(itemizedBatchContext.detail).length
      : undefined,
    selectedIds,
    selection: activeSelection,
  });
  const handlePromptChange = useCallback(
    (value: string) => {
      saveDedicatedBatchDraftSnapshot({
        prompt: value,
      });
      setPrompt(value);
    },
    [saveDedicatedBatchDraftSnapshot, setPrompt],
  );
  const analyzeReferenceStyle = useCallback(
    (input: { referenceImageUrls: string[]; basePrompt?: string }) =>
      analyzeSheinStudioReferenceStyle({
        referenceImageUrls: input.referenceImageUrls,
        basePrompt: input.basePrompt,
        productName: activeSelection?.productName,
        categoryPath: activeSelection?.categoryPath,
      }),
    [activeSelection?.categoryPath, activeSelection?.productName],
  );

  useEffect(() => {
    hasLocalWorkflowStateRef.current = false;
    hasCustomizedSdsSelectionRef.current = false;
  }, [selectionVariantId]);

  const focusGenerateStep = useCallback(() => {
    navigateToStep("generate");
    window.setTimeout(() => {
      const generator = document.getElementById("shein-studio-generator");
      generator?.scrollIntoView({ behavior: "smooth", block: "start" });
      promptInputRef.current?.focus();
    }, 0);
  }, [navigateToStep]);

  const openSDSLoginStep = useCallback(() => {
    router.push("/listing-kits/sds-login");
  }, [router]);

  const handleGenerationWarningAction = useCallback(() => {
    if (generationWarningAction?.intent === "focus_generate") {
      focusGenerateStep();
      return;
    }
    if (generationWarningAction?.intent === "open_sds_login") {
      openSDSLoginStep();
    }
  }, [focusGenerateStep, generationWarningAction?.intent, openSDSLoginStep]);

  const { handleWarmBaselineAction, isExecutingWarningAction } =
    useBaselineWarmupAction({
      activeSelection,
      baselineStatuses,
      setBaselineStatuses,
      setGenerationError: (message) =>
        workbenchController.setField("generationError", message),
      setGenerationWarning: (message) =>
        workbenchController.setField("generationWarning", message),
      setGenerationWarningAction: (action) =>
        workbenchController.setField("generationWarningAction", action),
      warmBaseline: warmSDSBaselineForSelection,
    });

  useEffect(() => {
    setGroupedSelections((current) =>
      projectGroupedSelectionBaselineEligibility({
        activeSelection,
        baselineStatuses,
        groupedSelections: current,
      }),
    );
  }, [activeSelection, baselineStatuses, setGroupedSelections]);

  useSheinStudioWorkspaceLoader({
    activeSelection,
    activeSelectionKey,
    activeStepRef,
    hasDedicatedBatchContext: Boolean(initialBatchId),
    hasExplicitSelection: Boolean(selection?.variantId),
    hasCustomizedSdsSelectionRef,
    hasLocalWorkflowStateRef,
    setEffectiveStep,
    workbench: workbenchController,
  });

  useEffect(() => {
    const nextDefaults = projectDefaultSelectedSDSImages({
      availableSdsImages,
      currentSelectedSdsImages: selectedSdsImages,
      hasCustomizedSdsSelection: hasCustomizedSdsSelectionRef.current,
      imageStrategy,
      renderSizeImagesWithSds,
    });
    if (!nextDefaults) {
      return;
    }
    const timer = window.setTimeout(() => {
      setSelectedSdsImages(nextDefaults);
    }, 0);
    return () => {
      window.clearTimeout(timer);
    };
  }, [
    availableSdsImages,
    imageStrategy,
    renderSizeImagesWithSds,
    selectedSdsImages,
    setSelectedSdsImages,
  ]);

  useEffect(() => {
    writeListingKitTraceContext(batchTraceContext);
  }, [batchTraceContext]);

  useEffect(
    () => () => {
      clearListingKitTraceContext();
    },
    [],
  );

  const { handleCreateTasks, handleGenerate, handleRegenerate } =
    useSheinStudioDesignActions({
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
      activeSelectionBaselineStatus: activeSelectionBaseline.status,
      activeSelectionBaselineReason,
      workbench: workbenchController,
      batchGenerationContext,
      sheinStoreId,
      styleCount,
      transparentBackground,
      variationIntensity,
      hasLocalWorkflowStateRef,
      itemizedBatchContext,
      batchTraceContext,
    });

  const {
    handleDeleteBatch,
    handleLoadBatch,
    handleLoadHydratedBatch,
    handleSaveBatch,
  } = useSheinStudioBatchActions({
    activeBatchId,
    activeStep,
    buildDraftInput: buildResultBackedDraftInput,
    hasCustomizedSdsSelectionRef,
    hasLocalWorkflowStateRef,
    setActiveBatchId,
    setEffectiveStep,
    workbench: workbenchController,
  });
  const handleLoadBatchRef = useRef(handleLoadBatch);
  const handleLoadHydratedBatchRef = useRef(handleLoadHydratedBatch);
  useEffect(() => {
    handleLoadBatchRef.current = handleLoadBatch;
  }, [handleLoadBatch]);
  useEffect(() => {
    handleLoadHydratedBatchRef.current = handleLoadHydratedBatch;
  }, [handleLoadHydratedBatch]);
  const setDedicatedBatchGenerationError = useCallback(
    (message: string) => {
      workbenchController.setField("generationError", message);
    },
    [workbenchController],
  );
  useSheinStudioInitialBatchHydration({
    initialBatchId,
    getHydratedBatch: getSheinStudioHydratedBatch,
    loadLocalSnapshot: loadLocalSheinStudioDraftSnapshotDetail,
    loadHydratedBatch: handleLoadHydratedBatch,
    promptOverride: dedicatedBatchPromptOverride,
    setGenerationError: setDedicatedBatchGenerationError,
    setLoaded: setIsDedicatedBatchLoaded,
    setQueueMessage,
  });

  const itemizedBatchGenerationInFlight =
    useSheinStudioItemizedGenerationInFlight(itemizedBatchDetail);
  const pendingItemizedTaskDesignIDs =
    usePendingItemizedTaskDesignIds(itemizedBatchDetail);
  const effectiveIsGenerating = isGenerating || itemizedBatchGenerationInFlight;

  useEffect(() => {
    if (!initialBatchId || !activeBatchId || !itemizedBatchGenerationInFlight) {
      return;
    }

    let cancelled = false;
    const timer = window.setInterval(() => {
      void (async () => {
        const hydratedBatch = await loadItemizedGenerationPollBatch({
          activeBatchId,
          getHydratedBatch: getSheinStudioHydratedBatch,
        });
        if (cancelled || !hydratedBatch) {
          return;
        }
        handleLoadHydratedBatchRef.current(hydratedBatch);
      })();
    }, 5_000);

    return () => {
      cancelled = true;
      window.clearInterval(timer);
    };
  }, [activeBatchId, initialBatchId, itemizedBatchGenerationInFlight]);

  useEffect(() => {
    if (!itemizedBatchDetail) {
      return;
    }
    const progress = projectItemizedTaskCreationProgress({
      creatingMessage,
      detail: itemizedBatchDetail,
      isCreatingTasks,
    });
    const effects = projectItemizedTaskCreationProgressEffects({
      currentCompletionSignature: taskCreationToastSignatureRef.current,
      progress,
    });
    if (effects.kind === "unchanged") {
      return;
    }
    workbenchController.setField(
      "isCreatingTasks",
      effects.fields.isCreatingTasks,
    );
    if (typeof effects.fields.creatingWarning === "string") {
      workbenchController.setField(
        "creatingWarning",
        effects.fields.creatingWarning,
      );
    }
    if (typeof effects.fields.creatingMessage === "string") {
      workbenchController.setField(
        "creatingMessage",
        effects.fields.creatingMessage,
      );
    }
    if (effects.completionSignature) {
      taskCreationToastSignatureRef.current = effects.completionSignature;
    }
    if (effects.toast) {
      toast[effects.toast.type](
        effects.toast.title,
        effects.toast.message,
        effects.toast.duration,
      );
    }
  }, [
    creatingMessage,
    isCreatingTasks,
    itemizedBatchDetail,
    toast,
    workbenchController,
  ]);

  const handleSelectRecentBatchSummary = useCallback(
    (
      summary: (typeof recentBatchSummaries)[number],
      action?: "generate" | "review" | "tasks",
    ) => {
      void runRecentBatchSummarySelection({
        action,
        advanceRequestVersion: () => {
          const requestVersion = recentBatchOpenRequestVersionRef.current + 1;
          recentBatchOpenRequestVersionRef.current = requestVersion;
          return requestVersion;
        },
        getCurrentRequestVersion: () =>
          recentBatchOpenRequestVersionRef.current,
        hasLocalDraft: Boolean(localDraftSnapshot),
        loadHydratedBatch: getSheinStudioHydratedBatch,
        openHydratedBatch: handleLoadHydratedBatch,
        openLocalDraft: (targetStep) => {
          if (!localDraftSnapshot) {
            return;
          }
          const recovery = projectLocalDraftRecovery({
            draft: localDraftSnapshot,
          });
          hasLocalWorkflowStateRef.current = true;
          hasCustomizedSdsSelectionRef.current =
            recovery.hasCustomizedSdsSelection;
          applyLocalDraftRecoveryToWorkbench({
            recovery,
            workbench: workbenchController,
          });
          setEffectiveStep(targetStep);
        },
        openSavedBatch: handleLoadBatch,
        savedBatches,
        setEffectiveStep,
        summary,
      });
    },
    [
      handleLoadHydratedBatch,
      handleLoadBatch,
      localDraftSnapshot,
      savedBatches,
      setEffectiveStep,
      workbenchController,
    ],
  );

  const refreshSavedBatches = useCallback(async () => {
    await refreshRecentSavedBatches({
      listBatches: listSheinStudioBatches,
      setSavedBatches: (batches) => {
        workbenchController.setField("savedBatches", batches);
      },
    });
  }, [workbenchController]);

  const resolveRecentBatchForMutation = useCallback(
    (batchId: string) =>
      resolveRecentBatchForMutationTarget({
        batchId,
        cacheHydratedBatch: (targetBatchId, hydratedBatch) => {
          setSelectedRecentBatchHydrations((current) =>
            mergeRecentBatchHydrations(current, [
              [targetBatchId, hydratedBatch],
            ]),
          );
        },
        loadHydratedBatch: getSheinStudioHydratedBatch,
        savedBatches,
        selectedRecentBatchHydrations,
      }),
    [savedBatches, selectedRecentBatchHydrations],
  );

  const handleRenameRecentBatchSummary = useCallback(
    async (summary: (typeof recentBatchSummaries)[number], name: string) => {
      await renameRecentBatchSummary({
        name,
        refreshSavedBatches,
        resolveBatch: resolveRecentBatchForMutation,
        saveBatch: saveSheinStudioBatch,
        summary,
      });
    },
    [refreshSavedBatches, resolveRecentBatchForMutation],
  );

  const handleDuplicateRecentBatchSummary = useCallback(
    async (summary: (typeof recentBatchSummaries)[number]) => {
      await duplicateRecentBatchSummary({
        refreshSavedBatches,
        resolveBatch: resolveRecentBatchForMutation,
        saveBatch: saveSheinStudioBatch,
        summary,
      });
    },
    [refreshSavedBatches, resolveRecentBatchForMutation],
  );

  const handleDeleteRecentBatchSummary = useCallback(
    async (summary: (typeof recentBatchSummaries)[number]) => {
      await deleteRecentBatchSummary({
        clearLocalDraft: () => {
          clearLocalSheinStudioDraftSnapshot();
          setLocalDraftSnapshotDetail(null);
        },
        deleteBatch: handleDeleteBatch,
        removeSelection: (summary) => {
          setRawSelectedRecentBatchSummaryIds((current) =>
            removeRecentBatchSummarySelection(current, summary),
          );
        },
        summary,
      });
    },
    [handleDeleteBatch],
  );

  const handleBulkDeleteRecentBatchSummaries = useCallback(
    async (summaryIds: string[]) => {
      await runRecentBatchBulkDelete(summaryIds, handleDeleteBatch);
    },
    [handleDeleteBatch],
  );

  const handleBulkUpdateRecentBatchStore = useCallback(
    async (summaryIds: string[], storeId: string) => {
      await runRecentBatchBulkStoreUpdate({
        batchIds: summaryIds,
        refreshSavedBatches,
        resolveBatch: resolveRecentBatchForMutation,
        saveBatch: saveSheinStudioBatch,
        storeId,
      });
    },
    [refreshSavedBatches, resolveRecentBatchForMutation],
  );

  const {
    clearQueuedSelectionContext,
    handleAdvanceBatchQueue,
    handleExitBatchQueue,
    handleOpenBatchQueue,
    handleResumeBatchQueue,
  } = useSheinStudioQueueController({
    batchQueueMode,
    currentQueuedBatchId,
    effectiveStep,
    getBatchRunStartErrorMessage,
    hydrateBatchSelection: hydrateRecentBatchSelection,
    loadBatch: handleLoadBatch,
    loadHydratedBatch: handleLoadHydratedBatch,
    promptInputRef,
    queueResumeState,
    queuedBatchIds,
    queuedBatchIndex,
    recentOpenVersionRef: recentBatchOpenRequestVersionRef,
    requestVersionRef: batchQueueRequestVersionRef,
    savedBatches,
    selectedRecentBatchHydrations,
    setActiveBatchRunId,
    setBatchQueueMode,
    setBatchRunError,
    setEffectiveStep,
    setQueueMessage,
    setQueueResumeState,
    setQueuedBatchIds,
    setQueuedBatchIndex,
    setSelectedRecentBatchSummaryIds,
    startBatchRun: startSheinStudioBatchRun,
  });

  const {
    handleReturnFromDedicatedBatchRun,
    handleStartDedicatedBatchRun,
    isStartingDedicatedBatchRun,
  } = useSheinStudioDedicatedBatchRunController({
    getBatchRunStartErrorMessage,
    getHydratedBatch: getSheinStudioHydratedBatch,
    initialBatchId,
    loadHydratedBatch: handleLoadHydratedBatch,
    refreshSavedBatches,
    setActiveBatchRunId,
    setBatchRunError,
    startBatchRun: startSheinStudioBatchRun,
  });

  const applyItemizedBatchDetail = useCallback(
    (
      nextDetail: NonNullable<typeof itemizedBatchDetail>,
      nextCreatedTasks = createdTasks,
    ) => {
      if (!activeBatchId) {
        return false;
      }
      const { savedBatch } = projectItemizedBatchDetail({
        activeBatchId,
        activeSelection,
        artworkModel,
        createdTasks: nextCreatedTasks,
        currentActiveBatch,
        detail: nextDetail,
        generationJobs,
        groupedImageMode,
        groupedSelections,
        groups,
        imageStrategy,
        persistedUpdatedAt,
        productImageCount,
        productImagePrompt,
        productImagePrompts,
        hotStyleReferenceImageUrls,
        hotStyleReferenceBrief,
        hotStyleReferencePrompt,
        prompt,
        renderSizeImagesWithSds,
        selectedSdsImages,
        sheinStoreId,
        styleCount,
        transparentBackground,
        variationIntensity,
      });
      workbenchController.setField("savedBatches", (current) =>
        upsertRecentSavedBatch(current, savedBatch),
      );
      workbenchController.applyHydratedBatch({
        savedBatch,
        detail: nextDetail,
      });
      return true;
    },
    [
      activeBatchId,
      activeSelection,
      artworkModel,
      createdTasks,
      currentActiveBatch,
      generationJobs,
      groupedImageMode,
      groupedSelections,
      groups,
      hotStyleReferenceBrief,
      hotStyleReferenceImageUrls,
      hotStyleReferencePrompt,
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
      workbenchController,
    ],
  );

  function toggleSelection(designId: string) {
    const toggle = projectItemizedSelectionToggle({
      activeBatchId,
      detail: itemizedBatchDetail,
      designId,
      selectedIds,
    });
    if (toggle.kind === "itemized") {
      const previousDetail = itemizedBatchDetail;
      if (!applyItemizedBatchDetail(toggle.detail) || !previousDetail) {
        return;
      }
      void (async () => {
        try {
          const nextDetail = await runItemizedDesignApproval({
            activeBatchId,
            approveDesigns: approveSheinStudioBatchDesigns,
            currentActiveBatch,
            detail: previousDetail,
            selectedIds: toggle.selectedIds,
          });
          workbenchController.setField("creatingWarning", "");
          if (nextDetail) {
            applyItemizedBatchDetail(nextDetail);
          }
        } catch (error) {
          applyItemizedBatchDetail(previousDetail);
          workbenchController.setField(
            "creatingWarning",
            `批准状态保存失败：${formatSubscriptionApiError(error)}`,
          );
        }
      })();
      return;
    }
    setSelectedIds(
      (current) =>
        projectItemizedSelectionToggle({
          activeBatchId: "",
          detail: null,
          designId,
          selectedIds: current,
        }).selectedIds,
    );
  }

  function handleNoteChange(designId: string, note: string) {
    const update = projectItemizedReviewNoteUpdate({
      activeBatchId,
      designs,
      detail: itemizedBatchDetail,
      designId,
      note,
    });
    if (update.kind === "itemized") {
      applyItemizedBatchDetail(update.detail);
      return;
    }
    setDesigns((current) => {
      const flatUpdate = projectItemizedReviewNoteUpdate({
        activeBatchId: "",
        designs: current,
        detail: null,
        designId,
        note,
      });
      return flatUpdate.kind === "flat" ? flatUpdate.designs : current;
    });
  }

  async function handleRetryFailedItem(itemId: string) {
    const retryRequest = projectItemizedFailedRetryRequest({
      activeBatchId,
      currentActiveBatch,
      detail: itemizedBatchDetail,
      itemId,
    });
    if (!retryRequest) {
      return;
    }

    setRetryingFailedItemId(itemId);
    clearWorkbenchTaskRecoveryAlerts(workbenchController);

    try {
      const nextDetail = await runItemizedFailedRetry({
        activeBatchId,
        currentActiveBatch,
        detail: itemizedBatchDetail,
        itemId,
        retryItems: retrySheinStudioBatchItems,
      });
      if (!nextDetail) {
        return;
      }
      applyItemizedBatchDetail(nextDetail);
      const nextStep = projectItemizedFailedRetryStep(nextDetail);
      if (nextStep) {
        setEffectiveStep(nextStep);
      }
    } catch (error) {
      workbenchController.setField(
        "generationError",
        `重试失败项失败：${formatSubscriptionApiError(error)}`,
      );
    } finally {
      setRetryingFailedItemId("");
    }
  }

  const busyMessage = useSheinStudioBusyMessage({
    isCreatingTasks,
    isGenerating: effectiveIsGenerating,
    regeneratingId,
  });
  const {
    dedicatedGenerateButtonLabel,
    hasRetryableFailedItems,
    retryableFailedItemCount,
    shouldPrioritizeTaskCreationRecovery,
  } = projectItemizedTaskRecoveryState({
    detail: itemizedBatchDetail,
    generationInFlight: itemizedBatchGenerationInFlight,
    pendingTaskDesignIds: pendingItemizedTaskDesignIDs,
  });
  useSheinStudioPendingNavigationGuard({
    enabled: Boolean(effectiveIsGenerating || regeneratingId),
    message:
      "当前正在生成款式图或创建 SHEIN 资料。现在离开会中断当前页面上的进度承接，确认还要离开吗？",
  });
  const dedicatedBatchHeader = initialBatchId ? (
    <div className="rounded-[1.75rem] border border-border bg-card px-5 py-5 shadow-sm">
      <div className="flex flex-col gap-5 xl:flex-row xl:items-start xl:justify-between">
        <div className="max-w-3xl space-y-4">
          <div className="flex flex-wrap gap-2 text-sm">
            <Button
              onClick={() => router.push("/listing-kits/sds")}
              size="sm"
              type="button"
              variant="ghost"
            >
              返回最近批次首页
            </Button>
            <Button
              onClick={() => {
                setActiveSheinStudioBatchId(initialBatchId);
                router.push(
                  `/listing-kits/sds/new?targetBatchId=${initialBatchId}`,
                );
              }}
              size="sm"
              type="button"
              variant="secondary"
            >
              去 SDS 选品并加入当前批次
            </Button>
          </div>

          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
              BATCH WORKBENCH
            </p>
            <div className="space-y-1">
              <h1 className="text-2xl font-semibold tracking-tight text-foreground">
                批次工作台
              </h1>
              <p className="text-base font-semibold text-foreground">
                当前批次 · {currentDedicatedBatch?.name || "未命名批次"}
              </p>
            </div>
            <p className="text-sm leading-6 text-muted-foreground">
              当前正在继续处理批次 {initialBatchId}
              ，可以在这里继续生成、审核和创建任务。
            </p>
          </div>

          <BatchStoreSettings
            currentStoreLabel={currentStoreLabel}
            requiredMessage={storeRequiredMessage}
            setSheinStoreId={setSheinStoreId}
            sheinStoreId={sheinStoreId}
          />
        </div>

        <div className="flex flex-wrap gap-2 xl:justify-end">
          {shouldPrioritizeTaskCreationRecovery ? (
            <Button
              disabled={isCreatingTasks}
              onClick={() => {
                void handleCreateTasks();
              }}
              size="sm"
              type="button"
              variant="default"
            >
              {isCreatingTasks ? "正在补建..." : "补建 SHEIN 资料"}
            </Button>
          ) : (
            <Button
              disabled={isStartingDedicatedBatchRun}
              onClick={handleStartDedicatedBatchRun}
              size="sm"
              type="button"
              variant="default"
            >
              {isStartingDedicatedBatchRun
                ? "正在启动..."
                : dedicatedGenerateButtonLabel}
            </Button>
          )}
          <Button
            onClick={() => navigateToStep("generate")}
            size="sm"
            type="button"
            variant={effectiveStep === "generate" ? "secondary" : "ghost"}
          >
            前往生成区
          </Button>
        </div>
      </div>
    </div>
  ) : null;

  if (activeBatchRunId) {
    return (
      <section className="relative space-y-6">
        <SheinStudioBatchRunProgress
          onBack={
            initialBatchId
              ? handleReturnFromDedicatedBatchRun
              : () => {
                  setActiveBatchRunId("");
                  void refreshSavedBatches();
                }
          }
          runId={activeBatchRunId}
        />
      </section>
    );
  }

  return (
    <section className="relative space-y-6">
      {dedicatedBatchHeader}

      {busyMessage ? <SheinStudioBusyOverlay message={busyMessage} /> : null}

      {!initialBatchId ? (
        <SheinStudioRecentBatchesDashboard
          onBulkDeleteSummaries={handleBulkDeleteRecentBatchSummaries}
          onBulkUpdateStore={handleBulkUpdateRecentBatchStore}
          onCreateBatch={() => {
            window.location.assign("/listing-kits/sds/new");
          }}
          onDeleteSummary={handleDeleteRecentBatchSummary}
          onDuplicateSummary={handleDuplicateRecentBatchSummary}
          onOpenBatchQueue={handleOpenBatchQueue}
          onRenameSummary={handleRenameRecentBatchSummary}
          onSelectedSummaryIdsChange={setSelectedRecentBatchSummaryIds}
          onSelectSummaryAction={handleSelectRecentBatchSummary}
          onSelectSummary={handleSelectRecentBatchSummary}
          selectedSummaryIds={selectedRecentBatchSummaryIds}
          storeOptions={recentBatchStoreOptions}
          summaries={recentBatchSummaries}
        />
      ) : null}

      {batchQueueMode && currentQueuedBatch ? (
        <SheinStudioBatchQueueBanner
          currentBatchName={currentQueuedBatch.name}
          currentIndex={queuedBatchIndex}
          guidance={batchQueueGuidance}
          mode={batchQueueMode}
          onExit={handleExitBatchQueue}
          onNext={handleAdvanceBatchQueue}
          onSkip={handleAdvanceBatchQueue}
          total={queuedBatchIds.length}
        />
      ) : null}

      {!batchQueueMode &&
      queueResumeState &&
      resumableQueueBatchIds.length > 0 ? (
        <div className="rounded-2xl border border-emerald-200 bg-emerald-50/70 px-4 py-4 text-sm text-emerald-900">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="space-y-1">
              <p className="font-medium">
                已停在第 {queueResumeState.startIndex + 1} /{" "}
                {queueResumeState.total} 个批次； 当前还保留{" "}
                {resumableQueueBatchIds.length} 个已勾选批次。
              </p>
              <p className="text-emerald-800/90">
                可继续本轮
                {queueResumeState.mode === "create_tasks"
                  ? "创建任务"
                  : "继续生成"}
                处理，或清除这轮选择后重新开始。
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button
                onClick={handleResumeBatchQueue}
                type="button"
                variant="secondary"
              >
                继续本轮处理
              </Button>
              <Button
                onClick={clearQueuedSelectionContext}
                type="button"
                variant="ghost"
              >
                清除这轮选择
              </Button>
            </div>
          </div>
        </div>
      ) : null}

      {queueMessage ? (
        <div className="rounded-2xl border border-border bg-muted px-4 py-3 text-sm text-muted-foreground">
          {queueMessage}
        </div>
      ) : null}

      {batchRunError ? (
        <div className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-900">
          {batchRunError}
        </div>
      ) : null}

      {isRecentBatchesHomepage ? null : (
        <>
          <SheinStudioWorkbenchAlerts
            creatingError={creatingError}
            creatingMessage={creatingMessage}
            draftWarning={draftWarning}
            creatingWarning={creatingWarning}
            generationWarning={generationWarning}
            generationWarningAction={
              generationWarningAction
                ? {
                    ...generationWarningAction,
                    label:
                      isExecutingWarningAction &&
                      generationWarningAction.intent === "warm_baseline"
                        ? "校验中..."
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
                activeSelectionBaselineAction={
                  activeSelectionBaselineHandoff
                    ? {
                        label:
                          isExecutingWarningAction &&
                          activeSelectionBaselineHandoff.action ===
                            "warm_baseline"
                            ? "校验中..."
                            : (activeSelectionBaselineHandoff.actionLabel ??
                              "处理 baseline"),
                        onClick:
                          activeSelectionBaselineHandoff.action ===
                          "warm_baseline"
                            ? () => {
                                void handleWarmBaselineAction();
                              }
                            : activeSelectionBaselineHandoff.action ===
                                "open_sds_login"
                              ? openSDSLoginStep
                              : focusGenerateStep,
                      }
                    : null
                }
                activeSelectionBaselineReason={activeSelectionBaselineReason}
                activeSelectionBaselineStatus={activeSelectionBaseline.status}
                currentStoreId={effectiveCurrentStoreId}
                currentStoreLabel={currentStoreLabel}
                groupedSelections={groupedSelections}
                onBulkUpdateSelectionStore={(selectionIds, storeId) =>
                  setGroupedSelections((current) =>
                    current.map((item) =>
                      selectionIds.includes(item.selectionId)
                        ? { ...item, sheinStoreId: storeId }
                        : item,
                    ),
                  )
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
                printableAreaLabel={printableAreaLabel}
                selectedColorCount={selectedColorCount}
                selectedSizeCount={selectedSizeCount}
                selectedVariantCount={selectedVariants.length}
                storeOptions={enabledProfiles}
              />
              <SheinStudioGenerationPanel
                actions={{
                  onCreateTasks: handleCreateTasks,
                  onDeleteBatch: handleDeleteBatch,
                  onGenerate: handleGenerate,
                  onLoadBatch: handleLoadBatch,
                  onRetryFailedItem: (itemId) => {
                    void handleRetryFailedItem(itemId);
                  },
                  onRestorePrompt: handlePromptChange,
                  onSaveBatch: handleSaveBatch,
                  analyzeReferenceStyle,
                  setArtworkModel,
                  setGroupedImageMode,
                  setHotStyleReferenceBrief,
                  setHotStyleReferenceImageUrls,
                  setHotStyleReferencePrompt,
                  setImageStrategy,
                  setProductImageCount,
                  setProductImagePrompt,
                  setProductImagePrompts,
                  setPrompt: handlePromptChange,
                  setPromptMode,
                  setRenderSizeImagesWithSds,
                  setSelectedSdsImages: (value) => {
                    hasCustomizedSdsSelectionRef.current = true;
                    setSelectedSdsImages(value);
                  },
                  setStyleCount,
                  setTransparentBackground,
                  setVariationIntensity,
                }}
                form={{
                  artworkModel,
                  availableSdsImages,
                  groupedImageMode,
                  hotStyleReferenceBrief,
                  hotStyleReferenceImageUrls,
                  hotStyleReferencePrompt,
                  imageStrategy,
                  productImageCount,
                  productImagePrompt,
                  productImagePrompts,
                  prompt,
                  promptMode,
                  promptHistory: activeGroupPromptHistory,
                  promptInputRef,
                  renderSizeImagesWithSds,
                  selectedSdsImages,
                  styleCount,
                  transparentBackground,
                  variationIntensity,
                }}
                status={{
                  batchProductCount: countSelectionsWithPrimary(
                    activeSelection,
                    groupedSelections,
                  ),
                  batchStoreLabel: currentStoreLabel || "未设置",
                  createTaskButtonLabel:
                    groupedSelections.length > 0
                      ? `为 ${countSelectionsWithPrimary(
                          activeSelection,
                          groupedSelections,
                        )} 款商品生成 SHEIN 资料`
                      : "生成 SHEIN 资料",
                  createdTasks,
                  creatingError,
                  creatingMessage,
                  failedBatchItems: hasRetryableFailedItems
                    ? (itemizedBatchDetail?.items
                        .filter((entry) => entry.item.status === "failed")
                        .map((entry) => entry.item) ?? [])
                    : [],
                  failedTasks: itemizedBatchDetail?.failedTasks ?? [],
                  generateButtonLabel: hasRetryableFailedItems
                    ? "重试失败批次"
                    : "生成款式图",
                  generationError,
                  generationNotice: hasRetryableFailedItems
                    ? `当前批次有 ${retryableFailedItemCount} 个失败项。点击“重试失败批次”只会重试失败部分，不会重复生成已成功内容。`
                    : "",
                  isCreatingTasks,
                  isGenerating: effectiveIsGenerating,
                  isRetryingFailedItems: hasRetryableFailedItems,
                  rejectedTasks: itemizedBatchDetail?.rejectedTasks ?? [],
                  retryingFailedItemId,
                  reusedTasks: itemizedBatchDetail?.reusedTasks ?? [],
                  savedBatches,
                  saveMessage,
                  selectedStyleCount: selectedIds.length,
                  selectionReady: Boolean(activeSelection?.variantId),
                  showSavedBatches: !initialBatchId,
                  statusGroups: itemizedBatchDetail?.statusGroups,
                  storeRequiredMessage,
                  subscriptionBlockedMessage,
                }}
              />
            </div>
          ) : null}

          {effectiveStep === "review" ? (
            <SheinStudioReviewStep
              createdTaskCount={createdTasks.length}
              createActionDisabledReason={createActionDisabledReason}
              designs={designs}
              groupedImageMode={groupedImageMode}
              groupedSelections={groupedSelections}
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
            <SheinStudioTasksStep
              createdTasks={createdTasks}
              failedTasks={itemizedBatchDetail?.failedTasks ?? []}
              onContinueCreateTasks={
                pendingItemizedTaskDesignIDs.length > 0
                  ? () => navigateToStep("review")
                  : undefined
              }
              pendingTaskDesignCount={pendingItemizedTaskDesignIDs.length}
              rejectedTasks={itemizedBatchDetail?.rejectedTasks ?? []}
              reusedTasks={itemizedBatchDetail?.reusedTasks ?? []}
            />
          ) : null}
        </>
      )}
    </section>
  );
}
