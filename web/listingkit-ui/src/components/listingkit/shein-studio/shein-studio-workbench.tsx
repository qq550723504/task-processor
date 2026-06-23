"use client";

import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { SheinStudioBatchQueueBanner } from "@/components/listingkit/shein-studio/shein-studio-batch-queue-banner";
import { SheinStudioBatchRunProgress } from "@/components/listingkit/shein-studio/shein-studio-batch-run-progress";
import { SheinStudioBusyOverlay } from "@/components/listingkit/shein-studio/shein-studio-busy-overlay";
import { BatchStoreSettings } from "@/components/listingkit/shein-studio/shein-studio-generation-form-sections";
import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import {
  evaluateGroupedSelectionCompatibility,
  SheinStudioGroupedSelectionPanel,
} from "@/components/listingkit/shein-studio/shein-studio-grouped-selection-panel";
import { SheinStudioRecentBatchesDashboard } from "@/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard";
import { SheinStudioTasksStep } from "@/components/listingkit/shein-studio/shein-studio-tasks-step";
import { useSheinStudioDedicatedBatchRunController } from "@/components/listingkit/shein-studio/shein-studio-dedicated-batch-run-controller";
import {
  projectBaselineWarmupFeedback,
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
  projectSheinStudioQueueState,
  useSheinStudioQueueController,
} from "@/components/listingkit/shein-studio/shein-studio-queue-controller";
import {
  buildRecentBatchSummaryKeys,
  buildRecentBatchBulkStoreUpdateInputs,
  buildRecentBatchSaveInput,
  projectRecentBatchSelectionState,
  projectRecentBatchTargetStep,
  removeRecentBatchSummarySelection,
  resolveRecentBatchSelectionTarget,
  selectRecentBatchBulkDeleteFailure,
  resolveRecentBatchForMutation as resolveRecentBatchForMutationTarget,
  upsertRecentSavedBatch,
} from "@/components/listingkit/shein-studio/shein-studio-recent-batch-controller";
import {
  projectItemizedBatchDetail,
  projectItemizedFailedRetryRequest,
  projectItemizedTaskCreationProgress,
  useSheinStudioItemizedBatchContext,
} from "@/components/listingkit/shein-studio/shein-studio-task-creation-controller";
import { useSheinStudioDesignActions } from "@/components/listingkit/shein-studio/shein-studio-workbench-actions";
import {
  clearLocalSheinStudioDraftSnapshot,
  useHydratedSDSVariantSelection,
  loadLocalSheinStudioDraftSnapshotDetail,
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
  getItemizedBatchPendingTaskDesignIDs,
  getSheinStudioCreateActionDisabledReason,
  hasInFlightItemizedBatchGeneration,
  projectWorkbenchStateToSavedBatch,
  sheinStudioBusyMessage,
  summarizeSheinStudioSelection,
  type SheinStudioWorkbenchHydratedBatch,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import {
  applySheinStudioWorkbenchBatch,
  applySheinStudioWorkbenchDraft,
  applySheinStudioWorkbenchHydratedBatch,
  buildInitialSheinStudioWorkbenchState,
  selectSheinStudioWorkbenchGroup,
  setSheinStudioWorkbenchField,
  sheinStudioWorkbenchReducer,
  type SheinStudioWorkbenchState,
  type SheinStudioWorkbenchStateUpdater,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import {
  buildDefaultSelectedSDSImages,
  buildSelectableSDSImages,
} from "@/lib/shein-studio/sds-selectable-images";
import {
  buildGroupedSDSBaselineHandoff,
  getSDSBaselineReasonMessage,
} from "@/lib/shein-studio/sds-baseline-ui";
import {
  type SheinStudioBatchQueueResumeState,
} from "@/lib/shein-studio/batch-queue";
import { buildDuplicatedSheinStudioBatchInput } from "@/lib/shein-studio/duplicate-batch";
import { buildRecentBatchSummaries } from "@/lib/shein-studio/recent-batch-summaries";
import { formatSheinStoreOptionLabel } from "@/lib/shein-studio/store-option-label";
import {
  clearListingKitTraceContext,
  writeListingKitTraceContext,
} from "@/lib/listingkit/request-trace";
import { getSDSBaselineReadiness } from "@/lib/api/sds-baseline";
import { warmSDSBaselineForSelection } from "@/lib/api/sds-baseline";
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
  type SDSBaselineStatus,
} from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioBatchQueueMode,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";

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
  const [activeBatchScope, setActiveBatchScope] = useState(() => ({
    batchId: initialBatchId ?? "",
    selectionVariantId,
  }));
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
  const setActiveBatchId = useCallback(
    (batchId: string) => {
      setActiveBatchScope({
        batchId,
        selectionVariantId,
      });
    },
    [selectionVariantId],
  );
  const activeBatchId = initialBatchId ??
    (activeBatchScope.selectionVariantId === selectionVariantId
      ? activeBatchScope.batchId
      : "");
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
      applyHydratedBatch: (
        batch: Parameters<typeof applySheinStudioWorkbenchHydratedBatch>[0],
      ) => dispatchWorkbenchState(applySheinStudioWorkbenchHydratedBatch(batch)),
      applyDraft: (draft: Parameters<typeof applySheinStudioWorkbenchDraft>[0]) =>
        dispatchWorkbenchState(applySheinStudioWorkbenchDraft(draft)),
      selectGroup: (groupId: string) =>
        dispatchWorkbenchState(selectSheinStudioWorkbenchGroup(groupId)),
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
      setCreatingWarning: (
        value: SheinStudioWorkbenchStateUpdater<"creatingWarning">,
      ) => setWorkbenchField("creatingWarning", value),
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
      setGroupedImageMode: (
        value: SheinStudioWorkbenchStateUpdater<"groupedImageMode">,
      ) => setWorkbenchField("groupedImageMode", value),
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
      setPersistedUpdatedAt: (
        value: SheinStudioWorkbenchStateUpdater<"persistedUpdatedAt">,
      ) => setWorkbenchField("persistedUpdatedAt", value),
      setBatchQueueMode: (
        value: SheinStudioWorkbenchStateUpdater<"batchQueueMode">,
      ) => setWorkbenchField("batchQueueMode", value),
      setGroupedSelections: (
        value: SheinStudioWorkbenchStateUpdater<"groupedSelections">,
      ) => setWorkbenchField("groupedSelections", value),
      setPrompt: (value: SheinStudioWorkbenchStateUpdater<"prompt">) =>
        setWorkbenchField("prompt", value),
      setPromptMode: (value: SheinStudioWorkbenchStateUpdater<"promptMode">) =>
        setWorkbenchField("promptMode", value),
      setQueueMessage: (
        value: SheinStudioWorkbenchStateUpdater<"queueMessage">,
      ) => setWorkbenchField("queueMessage", value),
      setQueuedBatchIds: (
        value: SheinStudioWorkbenchStateUpdater<"queuedBatchIds">,
      ) => setWorkbenchField("queuedBatchIds", value),
      setQueuedBatchIndex: (
        value: SheinStudioWorkbenchStateUpdater<"queuedBatchIndex">,
      ) => setWorkbenchField("queuedBatchIndex", value),
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
  const activeGroupSelection = useHydratedSDSVariantSelection(
    groups.find((group) => group.id === activeGroupId)?.primarySelection,
  );
  const activeSelection =
    directSelection ?? activeGroupSelection ?? loadedBatchSelection;
  const [isExecutingWarningAction, setIsExecutingWarningAction] = useState(false);
  const [retryingFailedItemId, setRetryingFailedItemId] = useState("");
  const [rawSelectedRecentBatchSummaryIds, setRawSelectedRecentBatchSummaryIds] = useState<
    string[]
  >([]);
  const [queueResumeState, setQueueResumeState] =
    useState<SheinStudioBatchQueueResumeState | null>(null);
  const [activeBatchRunId, setActiveBatchRunId] = useState("");
  const [batchRunError, setBatchRunError] = useState("");
  const [selectedRecentBatchHydrations, setSelectedRecentBatchHydrations] = useState<
    Record<string, SheinStudioWorkbenchHydratedBatch>
  >({});
  const selectedRecentBatchHydrationRequestsRef = useRef(
    new Map<string, Promise<SheinStudioWorkbenchHydratedBatch | null>>(),
  );
  const recentBatchOpenRequestVersionRef = useRef(0);
  const batchQueueRequestVersionRef = useRef(0);
  const taskCreationToastSignatureRef = useRef("");
  const [baselineStatuses, setBaselineStatuses] = useReducer(
    (
      _current: Record<
        string,
        {
          status: SDSBaselineStatus;
          reason: string;
          reasonCode?: string;
          baselineKey?: string;
        }
      >,
      next: Record<
        string,
        {
          status: SDSBaselineStatus;
          reason: string;
          reasonCode?: string;
          baselineKey?: string;
        }
      >,
    ) => next,
    {},
  );
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
  const resolvedActiveSelectionBaseline = activeGroupedSelectionID
    ? baselineStatuses[activeGroupedSelectionID]
    : undefined;
  const activeSelectionBaseline = baselineStatuses[activeGroupedSelectionID] ?? {
    status: "missing" as SDSBaselineStatus,
    reasonCode: undefined,
    reason: activeSelection?.variantId ? "正在检查 baseline 状态..." : "",
  };
  const activeSelectionBaselineReason =
    activeSelectionBaseline.reason ||
    getSDSBaselineReasonMessage(activeSelectionBaseline.reasonCode);
  const activeSelectionBaselineHandoff = useMemo(() => {
    if (!resolvedActiveSelectionBaseline) {
      return null;
    }
    return buildGroupedSDSBaselineHandoff({
      status: resolvedActiveSelectionBaseline.status,
      reason: resolvedActiveSelectionBaseline.reason,
      reasonCode: resolvedActiveSelectionBaseline.reasonCode,
    });
  }, [resolvedActiveSelectionBaseline]);
  const studioAccessAllowed =
    subscriptionQuery.data?.entitlements?.find(
      (view) => view.module.code === "studio",
    )?.allowed ?? true;
  const subscriptionBlockedMessage =
    subscriptionQuery.data && !studioAccessAllowed
      ? "当前租户未开通 Studio 模块。请在“当前租户订阅”里开通 Studio，或切换到已开通的租户后再生成款式图。"
      : "";
  const effectiveCurrentStoreId = (sheinStoreId ?? "").trim();
  const storeRequiredMessage = effectiveCurrentStoreId
    ? ""
    : "请先选择批次店铺，再生成款式图或创建 SHEIN 资料。";
  const currentStoreLabel = useMemo(() => {
    const matched = enabledProfiles.find(
      (item) => String(item.store_id) === effectiveCurrentStoreId,
    );
    return matched ? formatSheinStoreOptionLabel(matched) : "";
  }, [effectiveCurrentStoreId, enabledProfiles]);
  const recentBatchStoreOptions = useMemo(
    () =>
      enabledProfiles.map((profile) => ({
        id: String(profile.store_id),
        label: formatSheinStoreOptionLabel(profile),
      })),
    [enabledProfiles],
  );
  const activeGroupPromptHistory = useMemo(
    () => groups.find((group) => group.id === activeGroupId)?.promptHistory ?? [],
    [activeGroupId, groups],
  );
  const localDraftSnapshot = localDraftSnapshotDetail?.draft ?? null;
  const recentBatchSummaries = useMemo(() => {
    const baseSummaries = buildRecentBatchSummaries(savedBatches, {
      draft: localDraftSnapshot,
      draftBatchId: localDraftSnapshotDetail?.batchId,
    });
    return baseSummaries.map((summary) => {
      if (summary.source !== "batch") {
        return summary;
      }
      const hydratedBatch = selectedRecentBatchHydrations[summary.id];
      if (!hydratedBatch) {
        return summary;
      }
      return buildRecentBatchSummaries([hydratedBatch.savedBatch])[0] ?? summary;
    });
  }, [
    localDraftSnapshot,
    localDraftSnapshotDetail?.batchId,
    savedBatches,
    selectedRecentBatchHydrations,
  ]);
  const validRecentBatchSummaryKeys = useMemo(
    () => buildRecentBatchSummaryKeys(recentBatchSummaries),
    [recentBatchSummaries],
  );
  const { selectedPersistedRecentBatchIds, selectedRecentBatchSummaryIds } =
    useMemo(
      () =>
        projectRecentBatchSelectionState({
          rawSelectedRecentBatchSummaryIds,
          validRecentBatchSummaryKeys,
        }),
      [rawSelectedRecentBatchSummaryIds, validRecentBatchSummaryKeys],
    );
  const setSelectedRecentBatchSummaryIds = useCallback(
    (value: string[] | ((current: string[]) => string[])) => {
      setRawSelectedRecentBatchSummaryIds((current) => {
        const next = typeof value === "function" ? value(current) : value;
        return next.filter((key) => validRecentBatchSummaryKeys.has(key));
      });
    },
    [validRecentBatchSummaryKeys],
  );
  const isRecentBatchesHomepage = effectiveStep === "select";
  const {
    batchQueueGuidance,
    currentQueuedBatch,
    currentQueuedBatchId,
    resumableQueueBatchIds,
  } = useMemo(
    () =>
      projectSheinStudioQueueState({
        batchQueueMode,
        effectiveStep,
        queueResumeState,
        queuedBatchIds,
        queuedBatchIndex,
        savedBatches,
      }),
    [
      batchQueueMode,
      effectiveStep,
      queueResumeState,
      queuedBatchIds,
      queuedBatchIndex,
      savedBatches,
    ],
  );
  const traceBatchId = currentQueuedBatchId || activeBatchId || initialBatchId || "";
  const currentActiveBatch = useMemo(
    () => {
      const resolvedBatchId = activeBatchId || initialBatchId || "";
      if (!resolvedBatchId) {
        return null;
      }
      const matched = savedBatches.find((item) => item.id === resolvedBatchId);
      if (matched) {
        return matched;
      }
      return projectWorkbenchStateToSavedBatch({
        id: resolvedBatchId,
        prompt,
        promptMode,
        styleCount,
        variationIntensity,
        productImageCount,
        productImagePrompt,
        productImagePrompts,
        artworkModel,
        transparentBackground,
        sheinStoreId,
        imageStrategy,
        groupedImageMode,
        selectedSdsImages,
        renderSizeImagesWithSds,
        selection: loadedSelection,
        groupedSelections,
        groups,
        designs,
        selectedIds,
        createdTasks,
        generationJobs,
        updatedAt: persistedUpdatedAt,
      });
    },
    [
      activeBatchId,
      artworkModel,
      createdTasks,
      designs,
      generationJobs,
      groupedImageMode,
      groupedSelections,
      groups,
      imageStrategy,
      initialBatchId,
      loadedSelection,
      persistedUpdatedAt,
      productImageCount,
      productImagePrompt,
      productImagePrompts,
      prompt,
      promptMode,
      renderSizeImagesWithSds,
      savedBatches,
      selectedIds,
      selectedSdsImages,
      sheinStoreId,
      styleCount,
      transparentBackground,
      variationIntensity,
    ],
  );
  const currentDedicatedBatch = useMemo(
    () =>
      initialBatchId && currentActiveBatch?.id === initialBatchId
        ? currentActiveBatch
        : null,
    [currentActiveBatch, initialBatchId],
  );
  const hydrateRecentBatchSelection = useCallback(
    async (batchIds: string[]) => {
      const nextEntries = await Promise.all(
        batchIds.map(async (batchId) => {
          const batch = savedBatches.find((item) => item.id === batchId);
          if (!batch) {
            return null;
          }
          const cached = selectedRecentBatchHydrations[batchId];
          if (cached && cached.savedBatch.updatedAt === batch.updatedAt) {
            return [batchId, cached] as const;
          }
          let pending = selectedRecentBatchHydrationRequestsRef.current.get(batchId);
          if (!pending) {
            pending = getSheinStudioHydratedBatch(batchId)
              .catch(() => null)
              .finally(() => {
                selectedRecentBatchHydrationRequestsRef.current.delete(batchId);
              });
            selectedRecentBatchHydrationRequestsRef.current.set(batchId, pending);
          }
          const hydratedBatch = await pending;
          return hydratedBatch ? ([batchId, hydratedBatch] as const) : null;
        }),
      );
      const hydratedEntries = nextEntries.filter(
        (entry): entry is readonly [string, SheinStudioWorkbenchHydratedBatch] =>
          entry != null,
      );
      if (hydratedEntries.length > 0) {
        setSelectedRecentBatchHydrations((current) => ({
          ...current,
          ...Object.fromEntries(hydratedEntries),
        }));
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

    void hydrateRecentBatchSelection(selectedPersistedRecentBatchIds).then(() => {
      if (cancelled) {
        return;
      }
    });

    return () => {
      cancelled = true;
    };
  }, [hydrateRecentBatchSelection, selectedPersistedRecentBatchIds]);
  const createActionDisabledReason = getSheinStudioCreateActionDisabledReason({
    selection: activeSelection,
    galleryRatioCheck,
    selectedIds,
  });
  const draftPersistenceState = useMemo(
    () => buildSheinStudioDraftPersistenceState({
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
  const handlePromptChange = useCallback(
    (value: string) => {
      saveDedicatedBatchDraftSnapshot({
        prompt: value,
      });
      setPrompt(value);
    },
    [saveDedicatedBatchDraftSnapshot, setPrompt],
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

  const handleWarmBaselineAction = useCallback(async () => {
    if (!activeSelection?.variantId) {
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
          reasonCode: readiness.reasonCode,
          baselineKey: readiness.baselineKey,
        },
      });
      const feedback = projectBaselineWarmupFeedback(readiness);
      setWorkbenchField("generationWarning", feedback.message);
      setWorkbenchField("generationWarningAction", feedback.action);
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
    setWorkbenchField,
  ]);

  useEffect(() => {
    const selections = activeSelection?.variantId ? [activeSelection] : [];
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
              reasonCode: readiness.reasonCode,
              baselineKey: readiness.baselineKey,
            },
          ] as const;
        } catch (error) {
          return [
            selectionId,
            {
              status: "failed" as SDSBaselineStatus,
              reasonCode: undefined,
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
  }, [activeSelection]);

  useEffect(() => {
    setGroupedSelections((current) =>
      current.map((item) => {
        const baseline = baselineStatuses[item.selectionId] ?? {
          status: item.baselineStatus,
          reasonCode: undefined,
          reason: item.baselineReason,
          baselineKey: item.baselineKey,
        };
        const baselineReason =
          baseline.reason || getSDSBaselineReasonMessage(baseline.reasonCode);
        const compatibility = evaluateGroupedSelectionCompatibility(
          activeSelection,
          item.selection,
        );
        return {
          ...item,
          baselineKey: baseline.baselineKey,
          baselineStatus: baseline.status,
          baselineReason: baselineReason,
          baselineReasonCode: baseline.reasonCode,
          eligible: baseline.status === "ready" && compatibility.compatible,
          eligibilityReason:
            baseline.status !== "ready"
              ? baselineReason || "只有通过 baseline 校验的 SDS 商品才能加入分组。"
              : compatibility.reason,
        };
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

  useEffect(() => {
    writeListingKitTraceContext({
      batchId: traceBatchId || undefined,
      queueMode: batchQueueMode ?? undefined,
      queueIndex: batchQueueMode ? queuedBatchIndex + 1 : undefined,
      queueTotal: batchQueueMode ? queuedBatchIds.length : undefined,
    });
  }, [
    batchQueueMode,
    queuedBatchIds.length,
    queuedBatchIndex,
    traceBatchId,
  ]);

  useEffect(() => () => {
    clearListingKitTraceContext();
  }, []);

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
      batchTraceContext: {
        batchId: traceBatchId || undefined,
        queueMode: batchQueueMode,
        queueIndex: batchQueueMode ? queuedBatchIndex + 1 : undefined,
        queueTotal: batchQueueMode ? queuedBatchIds.length : undefined,
      },
    });

  const {
    handleDeleteBatch,
    handleLoadBatch,
    handleLoadHydratedBatch,
    handleSaveBatch,
  } =
    useSheinStudioBatchActions({
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

  const itemizedBatchGenerationInFlight = hasInFlightItemizedBatchGeneration(
    itemizedBatchDetail,
  );
  const pendingItemizedTaskDesignIDs = useMemo(
    () => getItemizedBatchPendingTaskDesignIDs(itemizedBatchDetail),
    [itemizedBatchDetail],
  );
  const effectiveIsGenerating = isGenerating || itemizedBatchGenerationInFlight;

  useEffect(() => {
    if (!initialBatchId || !activeBatchId || !itemizedBatchGenerationInFlight) {
      return;
    }

    let cancelled = false;
    const timer = window.setInterval(() => {
      void (async () => {
        try {
          const hydratedBatch = await getSheinStudioHydratedBatch(activeBatchId);
          if (cancelled || !hydratedBatch) {
            return;
          }
          handleLoadHydratedBatchRef.current(hydratedBatch);
        } catch {
          // Keep the current in-flight state and try again on the next interval.
        }
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
    if (progress.kind === "unchanged") {
      return;
    }
    if (progress.kind === "creating") {
      workbenchController.setField("isCreatingTasks", true);
      if (progress.creatingMessage) {
        workbenchController.setField(
          "creatingMessage",
          progress.creatingMessage,
        );
      }
      if (taskCreationToastSignatureRef.current !== progress.completionSignature) {
        taskCreationToastSignatureRef.current = progress.completionSignature;
      }
      return;
    }
    workbenchController.setField("isCreatingTasks", false);
    workbenchController.setField("creatingWarning", progress.creatingWarning);
    workbenchController.setField("creatingMessage", progress.creatingMessage);
    if (taskCreationToastSignatureRef.current !== progress.completionSignature) {
      taskCreationToastSignatureRef.current = progress.completionSignature;
      toast[progress.toast.type](
        progress.toast.title,
        progress.toast.message,
        progress.toast.duration,
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
      const requestVersion = recentBatchOpenRequestVersionRef.current + 1;
      recentBatchOpenRequestVersionRef.current = requestVersion;
      const targetStep = projectRecentBatchTargetStep(action);
      if (summary.source === "local_draft" && localDraftSnapshot) {
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
        return;
      }
      void (async () => {
        const target = await resolveRecentBatchSelectionTarget({
          loadHydratedBatch: getSheinStudioHydratedBatch,
          savedBatches,
          summary,
        });
        if (
          !target ||
          recentBatchOpenRequestVersionRef.current !== requestVersion
        ) {
          return;
        }
        if (target.kind === "hydrated") {
          handleLoadHydratedBatch(target.hydratedBatch);
        } else {
          handleLoadBatch(target.batch);
        }
        if (recentBatchOpenRequestVersionRef.current !== requestVersion) {
          return;
        }
        setEffectiveStep(targetStep);
      })();
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
    workbenchController.setField("savedBatches", await listSheinStudioBatches());
  }, [workbenchController]);

  const resolveRecentBatchForMutation = useCallback(
    (batchId: string) =>
      resolveRecentBatchForMutationTarget({
        batchId,
        cacheHydratedBatch: (targetBatchId, hydratedBatch) => {
          setSelectedRecentBatchHydrations((current) => ({
            ...current,
            [targetBatchId]: hydratedBatch,
          }));
        },
        loadHydratedBatch: getSheinStudioHydratedBatch,
        savedBatches,
        selectedRecentBatchHydrations,
      }),
    [savedBatches, selectedRecentBatchHydrations],
  );

  const handleRenameRecentBatchSummary = useCallback(
    async (summary: (typeof recentBatchSummaries)[number], name: string) => {
      if (summary.source !== "batch") {
        return;
      }
      const batch = await resolveRecentBatchForMutation(summary.id);
      if (!batch) {
        return;
      }
      await saveSheinStudioBatch(
        buildRecentBatchSaveInput(batch, { name }),
        { makeActive: false },
      );
      await refreshSavedBatches();
    },
    [refreshSavedBatches, resolveRecentBatchForMutation],
  );

  const handleDuplicateRecentBatchSummary = useCallback(
    async (summary: (typeof recentBatchSummaries)[number]) => {
      if (summary.source !== "batch") {
        return;
      }
      const batch = await resolveRecentBatchForMutation(summary.id);
      if (!batch) {
        return;
      }
      await saveSheinStudioBatch(
        buildDuplicatedSheinStudioBatchInput(batch),
        { makeActive: false },
      );
      await refreshSavedBatches();
    },
    [refreshSavedBatches, resolveRecentBatchForMutation],
  );

  const handleDeleteRecentBatchSummary = useCallback(
    async (summary: (typeof recentBatchSummaries)[number]) => {
      if (summary.source === "local_draft") {
        clearLocalSheinStudioDraftSnapshot();
        setLocalDraftSnapshotDetail(null);
        setRawSelectedRecentBatchSummaryIds((current) =>
          removeRecentBatchSummarySelection(current, summary),
        );
        return;
      }
      if (summary.source !== "batch") {
        return;
      }
      await handleDeleteBatch(summary.id);
    },
    [handleDeleteBatch],
  );

  const handleBulkDeleteRecentBatchSummaries = useCallback(
    async (summaryIds: string[]) => {
      if (summaryIds.length === 0) {
        return;
      }
      const results = await Promise.allSettled(
        summaryIds.map((summaryId) => handleDeleteBatch(summaryId)),
      );
      const failure = selectRecentBatchBulkDeleteFailure(results);
      if (failure) {
        throw failure;
      }
    },
    [handleDeleteBatch],
  );

  const handleBulkUpdateRecentBatchStore = useCallback(
    async (summaryIds: string[], storeId: string) => {
      const targets = (
        await Promise.all(summaryIds.map((summaryId) => resolveRecentBatchForMutation(summaryId)))
      ).filter((batch): batch is SheinStudioSavedBatch => batch != null);
      if (targets.length === 0) {
        return;
      }
      await Promise.all(
        buildRecentBatchBulkStoreUpdateInputs(targets, storeId).map((input) =>
          saveSheinStudioBatch(input, { makeActive: false }),
        ),
      );
      await refreshSavedBatches();
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

  const applyOptimisticItemizedBatchDetail = useCallback(
    (
      updater: (
        detail: NonNullable<typeof itemizedBatchDetail>,
      ) => NonNullable<typeof itemizedBatchDetail>,
    ) => {
      if (!activeBatchId || !itemizedBatchDetail) {
        return false;
      }
      const nextDetail = updater(itemizedBatchDetail);
      return applyItemizedBatchDetail(nextDetail);
    },
    [
      activeBatchId,
      applyItemizedBatchDetail,
      itemizedBatchDetail,
    ],
  );

  function toggleSelection(designId: string) {
    if (activeBatchId && itemizedBatchDetail) {
      const nextSelectedIds = selectedIds.includes(designId)
        ? selectedIds.filter((item) => item !== designId)
        : [...selectedIds, designId];
      const previousDetail = itemizedBatchDetail;
      if (
        !applyOptimisticItemizedBatchDetail((detail) => ({
          ...detail,
          items: detail.items.map((entry) => ({
            ...entry,
            designs: entry.designs.map((design) =>
              design.id !== designId
                ? design
                : {
                    ...design,
                    reviewStatus:
                      design.reviewStatus === "approved"
                        ? "unreviewed"
                        : "approved",
                  },
            ),
          })),
        }))
      ) {
        return;
      }
      void (async () => {
        try {
          const batchTenantId =
            itemizedBatchDetail.batch.tenantId?.trim() ??
            currentActiveBatch?.tenantId?.trim();
          const nextDetail = batchTenantId
            ? await approveSheinStudioBatchDesigns(
                activeBatchId,
                nextSelectedIds,
                { tenantId: batchTenantId },
              )
            : await approveSheinStudioBatchDesigns(
                activeBatchId,
                nextSelectedIds,
              );
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
    if (
      applyOptimisticItemizedBatchDetail((detail) => ({
        ...detail,
        items: detail.items.map((entry) => ({
          ...entry,
          designs: entry.designs.map((design) =>
            design.id !== designId
              ? design
              : {
                  ...design,
                  reviewStatus:
                    design.reviewStatus === "approved"
                      ? "unreviewed"
                      : "approved",
                },
          ),
        })),
      }))
    ) {
      return;
    }
    setSelectedIds((current) =>
      current.includes(designId)
        ? current.filter((item) => item !== designId)
        : [...current, designId],
    );
  }

  function handleNoteChange(designId: string, note: string) {
    if (
      applyOptimisticItemizedBatchDetail((detail) => ({
        ...detail,
        items: detail.items.map((entry) => ({
          ...entry,
          designs: entry.designs.map((design) =>
            design.id === designId ? { ...design, reviewNote: note } : design,
          ),
        })),
      }))
    ) {
      return;
    }
    setDesigns((current) =>
      current.map((design) =>
        design.id === designId ? { ...design, reviewNote: note } : design,
      ),
    );
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
    workbenchController.setField("generationError", "");
    workbenchController.setField("generationWarning", "");
    workbenchController.setField("creatingError", "");
    workbenchController.setField("creatingWarning", "");

    try {
      const nextDetail = retryRequest.tenantId
        ? await retrySheinStudioBatchItems(
            retryRequest.batchId,
            retryRequest.itemIds,
            {
              tenantId: retryRequest.tenantId,
            },
          )
        : await retrySheinStudioBatchItems(
            retryRequest.batchId,
            retryRequest.itemIds,
          );
      applyItemizedBatchDetail(nextDetail);
      if (
        nextDetail.batch.status === "generating" ||
        hasInFlightItemizedBatchGeneration(nextDetail)
      ) {
        setEffectiveStep("generate");
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

  const busyMessage = sheinStudioBusyMessage({
    isCreatingTasks,
    isGenerating: effectiveIsGenerating,
    regeneratingId,
  });
  const retryableFailedItemCount =
    itemizedBatchDetail?.items.filter((entry) => entry.item.status === "failed").length ?? 0;
  const hasRetryableFailedItems =
    retryableFailedItemCount > 0 &&
    (itemizedBatchDetail?.batch.status === "partially_failed" ||
      itemizedBatchDetail?.batch.status === "failed");
  const shouldPrioritizeTaskCreationRecovery =
    pendingItemizedTaskDesignIDs.length > 0 &&
    !hasRetryableFailedItems &&
    !itemizedBatchGenerationInFlight;
  const dedicatedGenerateButtonLabel =
    hasRetryableFailedItems && retryableFailedItemCount > 0
      ? `重试失败款式 ${retryableFailedItemCount} 个`
      : "继续生成剩余款式";
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
                router.push(`/listing-kits/sds/new?targetBatchId=${initialBatchId}`);
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
              当前正在继续处理批次 {initialBatchId}，可以在这里继续生成、审核和创建任务。
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
              {isStartingDedicatedBatchRun ? "正在启动..." : dedicatedGenerateButtonLabel}
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
          onBack={initialBatchId ? handleReturnFromDedicatedBatchRun : () => {
            setActiveBatchRunId("");
            void refreshSavedBatches();
          }}
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

      {!batchQueueMode && queueResumeState && resumableQueueBatchIds.length > 0 ? (
        <div className="rounded-2xl border border-emerald-200 bg-emerald-50/70 px-4 py-4 text-sm text-emerald-900">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="space-y-1">
              <p className="font-medium">
                已停在第 {queueResumeState.startIndex + 1} / {queueResumeState.total} 个批次；
                当前还保留 {resumableQueueBatchIds.length} 个已勾选批次。
              </p>
              <p className="text-emerald-800/90">
                可继续本轮{queueResumeState.mode === "create_tasks" ? "创建任务" : "继续生成"}处理，或清除这轮选择后重新开始。
              </p>
            </div>
            <div className="flex flex-wrap gap-2">
              <Button onClick={handleResumeBatchQueue} type="button" variant="secondary">
                继续本轮处理
              </Button>
              <Button onClick={clearQueuedSelectionContext} type="button" variant="ghost">
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
                          activeSelectionBaselineHandoff.action === "warm_baseline"
                            ? "校验中..."
                            : activeSelectionBaselineHandoff.actionLabel ??
                              "处理 baseline",
                        onClick:
                          activeSelectionBaselineHandoff.action === "warm_baseline"
                            ? () => {
                                void handleWarmBaselineAction();
                              }
                            : activeSelectionBaselineHandoff.action === "open_sds_login"
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
                  setArtworkModel,
                  setGroupedImageMode,
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
                    ? itemizedBatchDetail?.items
                        .filter((entry) => entry.item.status === "failed")
                        .map((entry) => entry.item) ?? []
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
