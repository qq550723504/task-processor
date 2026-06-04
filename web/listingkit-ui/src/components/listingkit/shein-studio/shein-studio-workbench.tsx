"use client";

import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { SheinStudioBatchQueueBanner } from "@/components/listingkit/shein-studio/shein-studio-batch-queue-banner";
import { SheinStudioBusyOverlay } from "@/components/listingkit/shein-studio/shein-studio-busy-overlay";
import { BatchStoreSettings } from "@/components/listingkit/shein-studio/shein-studio-generation-form-sections";
import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import { SheinStudioGroupedSelectionPanel } from "@/components/listingkit/shein-studio/shein-studio-grouped-selection-panel";
import { SheinStudioRecentBatchesDashboard } from "@/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard";
import { SheinStudioTasksStep } from "@/components/listingkit/shein-studio/shein-studio-tasks-step";
import { useSheinStudioDesignActions } from "@/components/listingkit/shein-studio/shein-studio-workbench-actions";
import {
  clearLocalSheinStudioDraftSnapshot,
  useHydratedSDSVariantSelection,
  loadLocalSheinStudioDraftSnapshotDetail,
  loadLocalSheinStudioDraftSnapshot,
  saveLocalSheinStudioDraftSnapshot,
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
  flattenItemizedBatchDesigns,
  getApprovedItemizedBatchDesignIDs,
  getSheinStudioCreateActionDisabledReason,
  hasInFlightItemizedBatchGeneration,
  mergeSheinStudioDraftState,
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
import { buildDuplicatedSheinStudioBatchInput } from "@/lib/shein-studio/duplicate-batch";
import { buildRecentBatchSummaries } from "@/lib/shein-studio/recent-batch-summaries";
import { formatSheinStoreOptionLabel } from "@/lib/shein-studio/store-option-label";
import {
  clearListingKitTraceContext,
  writeListingKitTraceContext,
} from "@/lib/listingkit/request-trace";
import { getSDSBaselineReadiness } from "@/lib/api/sds-baseline";
import { warmSDSBaselineForSelection } from "@/lib/api/sds-baseline";
import { getCurrentSubscription } from "@/lib/api/subscription";
import { formatSubscriptionApiError } from "@/lib/api/subscription";
import { useSheinStoreSelector } from "@/lib/query/use-shein-store-selector";
import { approveSheinStudioBatchDesigns } from "@/lib/api/shein-studio-batches";
import {
  getSheinStudioHydratedBatch,
  listSheinStudioBatches,
  saveSheinStudioBatch,
  setActiveSheinStudioBatchId,
} from "@/lib/utils/shein-studio-batches";
import {
  buildGroupedSDSSelectionID,
  countSelectionsWithPrimary,
  type GroupedSDSSelectionEligibility,
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

const dedicatedBatchPromptOverrides = new Map<string, string>();

function isMissingStudioBatchDeleteError(error: unknown) {
  return error instanceof Error && /studio session not found/i.test(error.message);
}

function isLocalSnapshotNewerThanBatch(
  snapshotUpdatedAt: string | undefined,
  batchUpdatedAt: string | undefined,
) {
  const snapshotTime = Date.parse(snapshotUpdatedAt ?? "");
  const batchTime = Date.parse(batchUpdatedAt ?? "");
  if (!Number.isFinite(snapshotTime) || !Number.isFinite(batchTime)) {
    return false;
  }
  return snapshotTime > batchTime;
}

function pickLocalStringValue(
  localValue: string | undefined,
  remoteValue: string | undefined,
) {
  return localValue?.trim() ? localValue : (remoteValue ?? "");
}

function pickLocalArrayValue<T>(
  localValue: T[] | undefined,
  remoteValue: T[] | undefined,
) {
  return ((localValue?.length ?? 0) > 0 ? localValue : (remoteValue ?? [])) as T[];
}

function mergeDedicatedBatchWithLocalSnapshot(
  batchId: string,
  hydratedBatch: SheinStudioWorkbenchHydratedBatch,
  localSnapshot: NonNullable<ReturnType<typeof loadLocalSheinStudioDraftSnapshotDetail>>,
): SheinStudioWorkbenchHydratedBatch {
  const savedBatch = hydratedBatch.savedBatch;
  const localDraft = localSnapshot.draft;
  return {
    savedBatch: {
      ...savedBatch,
      prompt:
        dedicatedBatchPromptOverrides.get(batchId) ??
        pickLocalStringValue(localDraft.prompt, savedBatch.prompt),
      styleCount: pickLocalStringValue(localDraft.styleCount, savedBatch.styleCount),
      variationIntensity:
        localDraft.variationIntensity ?? savedBatch.variationIntensity,
      productImageCount: pickLocalStringValue(
        localDraft.productImageCount,
        savedBatch.productImageCount,
      ),
      productImagePrompt: pickLocalStringValue(
        localDraft.productImagePrompt,
        savedBatch.productImagePrompt,
      ),
      productImagePrompts: pickLocalArrayValue(
        localDraft.productImagePrompts,
        savedBatch.productImagePrompts,
      ),
      artworkModel: pickLocalStringValue(
        localDraft.artworkModel,
        savedBatch.artworkModel,
      ),
      transparentBackground:
        localDraft.transparentBackground ?? savedBatch.transparentBackground,
      sheinStoreId: pickLocalStringValue(
        localDraft.sheinStoreId,
        savedBatch.sheinStoreId,
      ),
      imageStrategy: localDraft.imageStrategy ?? savedBatch.imageStrategy,
      groupedImageMode:
        localDraft.groupedImageMode ?? savedBatch.groupedImageMode,
      selectedSdsImages: pickLocalArrayValue(
        localDraft.selectedSdsImages,
        savedBatch.selectedSdsImages,
      ),
      renderSizeImagesWithSds:
        localDraft.renderSizeImagesWithSds ?? savedBatch.renderSizeImagesWithSds,
      selection: localDraft.selection ?? savedBatch.selection,
      groupedSelections: pickLocalArrayValue(
        localDraft.groupedSelections,
        savedBatch.groupedSelections,
      ),
      groups: pickLocalArrayValue(localDraft.groups, savedBatch.groups),
    },
    detail: hydratedBatch.detail,
  };
}

function upsertSavedBatch(
  batches: SheinStudioSavedBatch[],
  nextBatch: SheinStudioSavedBatch,
) {
  return [nextBatch, ...batches.filter((batch) => batch.id !== nextBatch.id)].sort(
    (left, right) => right.updatedAt.localeCompare(left.updatedAt),
  );
}

export function resetDedicatedBatchPromptOverrides() {
  dedicatedBatchPromptOverrides.clear();
}

export function SheinStudioWorkbench({
  activeStep = "generate",
  initialBatchId,
  selection,
}: SheinStudioWorkbenchProps) {
  const router = useRouter();
  const [activeBatchId, setActiveBatchId] = useState(() => initialBatchId ?? "");
  const [isDedicatedBatchLoaded, setIsDedicatedBatchLoaded] = useState(
    () => !initialBatchId,
  );
  const [localDraftSnapshotDetail, setLocalDraftSnapshotDetail] = useState(() =>
    loadLocalSheinStudioDraftSnapshotDetail(),
  );
  const [isEditingCurrentBatchName, setIsEditingCurrentBatchName] = useState(false);
  const [currentBatchDraftName, setCurrentBatchDraftName] = useState("");
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
  const [rawSelectedRecentBatchSummaryIds, setRawSelectedRecentBatchSummaryIds] = useState<
    string[]
  >([]);
  const [queueResumeState, setQueueResumeState] = useState<{
    batchIds: string[];
    mode: SheinStudioBatchQueueMode;
    startIndex: number;
    total: number;
  } | null>(null);
  const [selectedRecentBatchHydrations, setSelectedRecentBatchHydrations] = useState<
    Record<string, SheinStudioWorkbenchHydratedBatch>
  >({});
  const selectedRecentBatchHydrationRequestsRef = useRef(
    new Map<string, Promise<SheinStudioWorkbenchHydratedBatch | null>>(),
  );
  const recentBatchOpenRequestVersionRef = useRef(0);
  const batchQueueRequestVersionRef = useRef(0);
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
    () =>
      new Set(
        recentBatchSummaries.map((summary) => `${summary.source}:${summary.id}`),
      ),
    [recentBatchSummaries],
  );
  const selectedRecentBatchSummaryIds = useMemo(
    () =>
      rawSelectedRecentBatchSummaryIds.filter((key) =>
        validRecentBatchSummaryKeys.has(key),
      ),
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
  const selectedPersistedRecentBatchIds = useMemo(
    () =>
      selectedRecentBatchSummaryIds.flatMap((key) => {
        const [source, id] = key.split(":");
        return source === "batch" && id ? [id] : [];
      }),
    [selectedRecentBatchSummaryIds],
  );
  const currentQueuedBatchId = batchQueueMode
    ? queuedBatchIds[queuedBatchIndex] ?? ""
    : "";
  const isRecentBatchesHomepage = effectiveStep === "select";
  const currentQueuedBatch = useMemo(
    () => savedBatches.find((item) => item.id === currentQueuedBatchId) ?? null,
    [currentQueuedBatchId, savedBatches],
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
  const batchQueueGuidance = useMemo(() => {
    if (effectiveStep === "tasks") {
      return "已定位到任务区，可继续查看已创建的任务。";
    }
    if (effectiveStep === "review") {
      return "已定位到审核区，可直接创建任务或调整款式。";
    }
    if (batchQueueMode === "generate") {
      return "已定位到生成区，可直接修改提示词或继续生成。";
    }
    return "当前批次还没有可用设计，已回到生成区继续处理。";
  }, [batchQueueMode, effectiveStep]);
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
  const queueCompletionMessage = useCallback(
    (mode: SheinStudioBatchQueueMode, batchCount: number) => {
      const actionLabel =
        mode === "create_tasks" ? "创建任务处理" : "继续生成处理";
      return batchCount > 0
        ? `已完成这轮${actionLabel}，共处理 ${batchCount} 个已保存批次。首页勾选已保留，可继续调整或再次发起批量处理。`
        : `当前没有可继续的已保存批次。首页勾选已保留，可重新检查后再发起批量处理。`;
    },
    [],
  );
  const resumableQueueBatchIds = useMemo(
    () =>
      queueResumeState
        ? queueResumeState.batchIds.filter((batchId) =>
            savedBatches.some((item) => item.id === batchId),
          )
        : [],
    [queueResumeState, savedBatches],
  );
  const createActionDisabledReason = getSheinStudioCreateActionDisabledReason({
    selection: activeSelection,
    galleryRatioCheck,
    selectedIds,
  });
  const draftPersistenceState = useMemo(
    () => ({
      activeSelection,
      artworkModel,
      createdTasks,
      designs,
      generationError,
      generationJobId: currentActiveBatch?.generationJobId,
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
  const buildResultBackedDraftInput = useCallback(
    () =>
      buildDraftInput({
        designs,
        selectedIds,
        createdTasks,
        generationJobs,
        generationError,
        generationJobId: currentActiveBatch?.generationJobId ?? "",
      }),
    [
      buildDraftInput,
      createdTasks,
      currentActiveBatch?.generationJobId,
      designs,
      generationError,
      generationJobs,
      selectedIds,
    ],
  );
  const saveDedicatedBatchDraftSnapshot = useCallback(
    (overrides?: Partial<ReturnType<typeof buildDraftInput>>) => {
      if (!initialBatchId) {
        return;
      }
      if (typeof overrides?.prompt === "string") {
        dedicatedBatchPromptOverrides.set(initialBatchId, overrides.prompt);
      }
      saveLocalSheinStudioDraftSnapshot(
        {
          ...buildDraftInput(),
          ...overrides,
        },
        {
          batchId: initialBatchId,
        },
      );
    },
    [buildDraftInput, initialBatchId, isDedicatedBatchLoaded],
  );
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
    if (!initialBatchId) {
      setActiveBatchId("");
    }
  }, [selection?.variantId]);

  useEffect(() => {
    setIsDedicatedBatchLoaded(!initialBatchId);
    setActiveBatchId(initialBatchId ?? "");
  }, [initialBatchId]);

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
      setWorkbenchField(
        "generationWarning",
        readiness.status === "ready"
          ? "这款 SDS 商品的 baseline 已通过校验，现在可以继续加入 grouped 批量上品。"
          : readiness.status === "baseline_cached" &&
              !readiness.reason?.trim() &&
              !readiness.reasonCode?.trim()
            ? "这款 SDS 商品已经完成 baseline 缓存，当前没有更多校验结果。可以继续使用，必要时再手动复查。"
          : readiness.reason ||
            getSDSBaselineReasonMessage(readiness.reasonCode) ||
            "baseline 预热与校验已发起，请稍后再试。",
      );
      const handoff = buildGroupedSDSBaselineHandoff({
        status: readiness.status,
        reason: readiness.reason,
        reasonCode: readiness.reasonCode,
      });
      setWorkbenchField(
        "generationWarningAction",
        handoff?.action && handoff.actionLabel
          ? {
              intent: handoff.action,
              label: handoff.actionLabel,
            }
          : null,
      );
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
    persistDraft,
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
      promptInputRef,
      renderSizeImagesWithSds,
      selectedIds,
      selectedSdsImages,
      groupedSelections,
      generationJobs,
      activeSelectionBaselineStatus: activeSelectionBaseline.status,
      activeSelectionBaselineReason,
      workbench: workbenchController,
      batchGenerationContext: activeSelection?.variantId
        ? {
            ensureBatch: async () => {
              const currentBatchId = activeBatchId || initialBatchId || "";
              const latestHydratedBatch =
                currentBatchId && initialBatchId
                  ? await getSheinStudioHydratedBatch(currentBatchId).catch(
                      () => null,
                    )
                  : null;
              const saved = await saveSheinStudioBatch(
                {
                  ...buildDraftInput({
                    designs,
                    selectedIds,
                    createdTasks,
                    generationJobs,
                    generationError,
                    generationJobId: currentActiveBatch?.generationJobId ?? "",
                  }),
                  ...(currentBatchId ? { id: currentBatchId } : {}),
                  updatedAt:
                    latestHydratedBatch?.detail.batch.draftUpdatedAt ||
                    latestHydratedBatch?.savedBatch.draftUpdatedAt ||
                    latestHydratedBatch?.savedBatch.updatedAt ||
                    buildDraftInput().updatedAt,
                },
                currentBatchId ? { makeActive: false } : undefined,
              );
              if (!saved) {
                return null;
              }
              setActiveBatchId(saved.id);
              setActiveSheinStudioBatchId(saved.id);
              workbenchController.setField("savedBatches", (current) =>
                upsertSavedBatch(current, saved),
              );
              return saved;
            },
            onGenerated: ({ savedBatch, detail }) => {
              const nextSavedBatch: SheinStudioSavedBatch = {
                ...(currentActiveBatch ?? {}),
                ...savedBatch,
                id: savedBatch.id,
                name: savedBatch.name || currentActiveBatch?.name || "未命名批次",
                prompt,
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
                selection: activeSelection,
                groupedSelections,
                groups,
                designs: flattenItemizedBatchDesigns(detail),
                selectedIds: getApprovedItemizedBatchDesignIDs(detail),
                createdTasks: [],
                generationJobs: [],
                draftUpdatedAt:
                  savedBatch.draftUpdatedAt || savedBatch.updatedAt,
                updatedAt: detail.batch.updatedAt,
              };
              setActiveBatchId(savedBatch.id);
              setActiveSheinStudioBatchId(savedBatch.id);
              workbenchController.setField("savedBatches", (current) =>
                upsertSavedBatch(current, nextSavedBatch),
              );
              workbenchController.applyHydratedBatch({
                savedBatch: nextSavedBatch,
                detail,
              });
            },
            detail: itemizedBatchDetail,
            recoverInFlightGeneration: async ({ batchId, error }) => {
              const hydratedBatch = await getSheinStudioHydratedBatch(batchId);
              if (
                !hydratedBatch ||
                !hasInFlightItemizedBatchGeneration(hydratedBatch.detail)
              ) {
                return false;
              }
              setActiveBatchId(batchId);
              setActiveSheinStudioBatchId(batchId);
              workbenchController.setField("savedBatches", (current) =>
                upsertSavedBatch(current, hydratedBatch.savedBatch),
              );
              workbenchController.applyHydratedBatch(hydratedBatch);
              workbenchController.setField("generationError", "");
              workbenchController.setField(
                "generationWarning",
                `${
                  error instanceof Error ? error.message : "当前批次生成请求超时。"
                } 后台仍在继续生成，请等待结果刷新，不要重复点击“生成款式图”。`,
              );
              return true;
            },
          }
        : undefined,
      sheinStoreId,
      styleCount,
      transparentBackground,
      variationIntensity,
      hasLocalWorkflowStateRef,
      itemizedBatchContext:
        activeBatchId && itemizedBatchDetail
          ? {
              batchId: activeBatchId,
              detail: itemizedBatchDetail,
              onCreated: (result) => {
                const nextDetail = {
                  batch: result.batch,
                  items: result.items,
                };
                const nextSavedBatch: SheinStudioSavedBatch = {
                  ...(currentActiveBatch ?? {}),
                  id: activeBatchId,
                  name: currentActiveBatch?.name ?? "未命名批次",
                  prompt,
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
                  selection: activeSelection,
                  groupedSelections,
                  groups,
                  designs: flattenItemizedBatchDesigns(nextDetail),
                  selectedIds: getApprovedItemizedBatchDesignIDs(nextDetail),
                  createdTasks: result.createdTasks,
                  generationJobs: [],
                  draftUpdatedAt:
                    currentActiveBatch?.draftUpdatedAt || persistedUpdatedAt,
                  updatedAt:
                    currentActiveBatch?.updatedAt ||
                    nextDetail.batch.updatedAt ||
                    persistedUpdatedAt,
                };
                workbenchController.setField("savedBatches", (current) =>
                  upsertSavedBatch(current, nextSavedBatch),
                );
                workbenchController.applyHydratedBatch({
                  savedBatch: nextSavedBatch,
                  detail: nextDetail,
                });
              },
            }
          : undefined,
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
  const loadedInitialBatchIdRef = useRef<string | null>(null);
  useEffect(() => {
    handleLoadBatchRef.current = handleLoadBatch;
  }, [handleLoadBatch]);
  useEffect(() => {
    handleLoadHydratedBatchRef.current = handleLoadHydratedBatch;
  }, [handleLoadHydratedBatch]);
  useEffect(() => {
    if (!initialBatchId) {
      loadedInitialBatchIdRef.current = null;
      return;
    }
    const batchId = initialBatchId;
    if (loadedInitialBatchIdRef.current === batchId) {
      return;
    }

    let cancelled = false;

    async function loadInitialBatch() {
      try {
        const hydratedBatch = await getSheinStudioHydratedBatch(batchId);
        if (cancelled || !hydratedBatch) {
          return;
        }
        const localSnapshot = loadLocalSheinStudioDraftSnapshotDetail();
        const batchWithLocalSnapshot: SheinStudioWorkbenchHydratedBatch =
          localSnapshot?.batchId === batchId &&
          isLocalSnapshotNewerThanBatch(
            localSnapshot.draft.updatedAt,
            hydratedBatch.savedBatch.draftUpdatedAt ??
              hydratedBatch.savedBatch.updatedAt,
          )
            ? mergeDedicatedBatchWithLocalSnapshot(
                batchId,
                hydratedBatch,
                localSnapshot,
              )
            : {
                savedBatch: {
                  ...hydratedBatch.savedBatch,
                  prompt:
                    dedicatedBatchPromptOverrides.get(batchId) ??
                    hydratedBatch.savedBatch.prompt,
                },
                detail: hydratedBatch.detail,
              };
        loadedInitialBatchIdRef.current = batchId;
        handleLoadHydratedBatchRef.current(batchWithLocalSnapshot);
        setIsDedicatedBatchLoaded(true);
      } catch (error) {
        if (cancelled) {
          return;
        }
        const message = `当前批次加载失败：${
          error instanceof Error ? error.message : "未知错误"
        }。请重新登录后再继续。`;
        workbenchController.setField("generationError", message);
        setQueueMessage(message);
        setIsDedicatedBatchLoaded(true);
      }
    }

    void loadInitialBatch();

    return () => {
      cancelled = true;
    };
  }, [initialBatchId]);

  const itemizedBatchGenerationInFlight = hasInFlightItemizedBatchGeneration(
    itemizedBatchDetail,
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

  const stepForRecentBatchAction = useCallback(
    (action?: "generate" | "review" | "tasks"): SheinStudioStepKey => {
      if (action === "tasks") {
        return "tasks";
      }
      if (action === "review") {
        return "review";
      }
      return "generate";
    },
    [],
  );

  const handleSelectRecentBatchSummary = useCallback(
    (
      summary: (typeof recentBatchSummaries)[number],
      action?: "generate" | "review" | "tasks",
    ) => {
      const requestVersion = recentBatchOpenRequestVersionRef.current + 1;
      recentBatchOpenRequestVersionRef.current = requestVersion;
      const targetStep = stepForRecentBatchAction(action);
      if (summary.source === "local_draft" && localDraftSnapshot) {
        const draftState = mergeSheinStudioDraftState({
          draft: localDraftSnapshot,
        });
        hasLocalWorkflowStateRef.current = true;
        hasCustomizedSdsSelectionRef.current =
          draftState.hasCustomizedSdsSelection;
        workbenchController.applyDraft({
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
          groups: draftState.groups,
          groupedSelections: draftState.groupedSelections,
          renderSizeImagesWithSds: draftState.renderSizeImagesWithSds,
        });
        workbenchController.setField("designs", draftState.designs);
        workbenchController.setField("selectedIds", draftState.selectedIds);
        workbenchController.setField("createdTasks", draftState.createdTasks);
        workbenchController.setField("generationJobs", draftState.generationJobs);
        workbenchController.setField("generationError", draftState.generationError);
        setEffectiveStep(targetStep);
        return;
      }
      const batch = savedBatches.find((item) => item.id === summary.id);
      if (!batch) {
        return;
      }
      void (async () => {
        try {
          const hydratedBatch = await getSheinStudioHydratedBatch(summary.id);
          if (recentBatchOpenRequestVersionRef.current !== requestVersion) {
            return;
          }
          handleLoadHydratedBatch(hydratedBatch);
        } catch {
          if (recentBatchOpenRequestVersionRef.current !== requestVersion) {
            return;
          }
          handleLoadBatch(batch);
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
      stepForRecentBatchAction,
      workbenchController,
    ],
  );

  const buildSaveInputFromBatch = useCallback(
    (
      batch: SheinStudioSavedBatch,
      overrides?: Partial<SheinStudioSavedBatch>,
    ) => ({
      id: overrides?.id ?? batch.id,
      updatedAt:
        overrides?.draftUpdatedAt ??
        overrides?.updatedAt ??
        batch.draftUpdatedAt ??
        batch.updatedAt,
      name: overrides?.name ?? batch.name,
      prompt: overrides?.prompt ?? batch.prompt,
      styleCount: overrides?.styleCount ?? batch.styleCount,
      variationIntensity:
        overrides?.variationIntensity ?? batch.variationIntensity,
      productImageCount: overrides?.productImageCount ?? batch.productImageCount,
      productImagePrompt:
        overrides?.productImagePrompt ?? batch.productImagePrompt,
      productImagePrompts:
        overrides?.productImagePrompts ?? batch.productImagePrompts,
      artworkModel: overrides?.artworkModel ?? batch.artworkModel,
      transparentBackground:
        overrides?.transparentBackground ?? batch.transparentBackground,
      sheinStoreId: overrides?.sheinStoreId ?? batch.sheinStoreId,
      imageStrategy: overrides?.imageStrategy ?? batch.imageStrategy,
      groupedImageMode:
        overrides?.groupedImageMode ?? batch.groupedImageMode,
      selectedSdsImages:
        overrides?.selectedSdsImages ?? batch.selectedSdsImages,
      renderSizeImagesWithSds:
        overrides?.renderSizeImagesWithSds ?? batch.renderSizeImagesWithSds,
      selection: overrides?.selection ?? batch.selection,
      groupedSelections:
        overrides?.groupedSelections ?? batch.groupedSelections,
      groups: overrides?.groups ?? batch.groups,
      designs: overrides?.designs ?? batch.designs,
      selectedIds: overrides?.selectedIds ?? batch.selectedIds,
      createdTasks: overrides?.createdTasks ?? batch.createdTasks,
      generationJobs: overrides?.generationJobs ?? batch.generationJobs,
      generationError: overrides?.generationError ?? batch.generationError,
      generationJobId: overrides?.generationJobId ?? batch.generationJobId,
    }),
    [],
  );
  const buildSaveInputForCurrentDedicatedBatch = useCallback(
    (name?: string) => ({
      ...buildResultBackedDraftInput(),
      id: initialBatchId,
      name: name?.trim() || undefined,
    }),
    [buildResultBackedDraftInput, initialBatchId],
  );

  const refreshSavedBatches = useCallback(async () => {
    workbenchController.setField("savedBatches", await listSheinStudioBatches());
  }, [workbenchController]);

  const resolveRecentBatchForMutation = useCallback(
    async (batchId: string) => {
      const savedBatch = savedBatches.find((item) => item.id === batchId);
      if (!savedBatch) {
        return null;
      }
      const cachedHydratedBatch = selectedRecentBatchHydrations[batchId];
      if (
        cachedHydratedBatch &&
        Date.parse(cachedHydratedBatch.savedBatch.updatedAt) >=
          Date.parse(savedBatch.updatedAt)
      ) {
        return cachedHydratedBatch.savedBatch;
      }
      try {
        const hydratedBatch = await getSheinStudioHydratedBatch(batchId);
        setSelectedRecentBatchHydrations((current) => ({
          ...current,
          [batchId]: hydratedBatch,
        }));
        return hydratedBatch.savedBatch;
      } catch {
        return savedBatch;
      }
    },
    [savedBatches, selectedRecentBatchHydrations],
  );

  useEffect(() => {
    if (!isEditingCurrentBatchName) {
      setCurrentBatchDraftName(currentDedicatedBatch?.name ?? "");
    }
  }, [currentDedicatedBatch?.name, isEditingCurrentBatchName]);

  const handleRenameCurrentDedicatedBatch = useCallback(async () => {
    if (!initialBatchId) {
      return;
    }
    const nextName = currentBatchDraftName.trim();
    if (!nextName) {
      setQueueMessage("批次名称不能为空。");
      return;
    }
    const saved = await saveSheinStudioBatch(
      buildSaveInputForCurrentDedicatedBatch(nextName),
      { makeActive: false },
    );
    if (!saved) {
      setQueueMessage("批次重命名失败。");
      return;
    }
    setIsEditingCurrentBatchName(false);
    setCurrentBatchDraftName(saved.name);
    setQueueMessage(`已重命名为：${saved.name}`);
    await refreshSavedBatches();
  }, [
    buildSaveInputForCurrentDedicatedBatch,
    currentBatchDraftName,
    initialBatchId,
    refreshSavedBatches,
    setQueueMessage,
  ]);

  const handleDeleteCurrentDedicatedBatch = useCallback(async () => {
    if (!initialBatchId) {
      return;
    }
    if (!window.confirm("确认删除当前批次吗？删除后无法恢复。")) {
      return;
    }
    await handleDeleteBatch(initialBatchId);
    router.push("/listing-kits/sds");
  }, [handleDeleteBatch, initialBatchId, router]);

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
        buildSaveInputFromBatch(batch, { name }),
        { makeActive: false },
      );
      await refreshSavedBatches();
    },
    [buildSaveInputFromBatch, refreshSavedBatches, resolveRecentBatchForMutation],
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
          current.filter((key) => key !== `${summary.source}:${summary.id}`),
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
      const failed = results.find(
        (result) =>
          result.status === "rejected" &&
          !isMissingStudioBatchDeleteError(result.reason),
      );
      if (failed?.status === "rejected") {
        throw failed.reason;
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
        targets.map((batch) =>
          saveSheinStudioBatch(
            buildSaveInputFromBatch(batch, {
              sheinStoreId: storeId,
              groupedSelections: (batch.groupedSelections ?? []).map((item) => ({
                ...item,
                sheinStoreId: storeId,
              })),
              groups: (batch.groups ?? []).map((group) => ({
                ...group,
                sheinStoreId: storeId,
                groupedSelections: group.groupedSelections.map((item) => ({
                  ...item,
                  sheinStoreId: storeId,
                })),
              })),
            }),
            { makeActive: false },
          ),
        ),
      );
      await refreshSavedBatches();
    },
    [buildSaveInputFromBatch, refreshSavedBatches, resolveRecentBatchForMutation],
  );

  const clearBatchQueue = useCallback(() => {
    setBatchQueueMode(null);
    setQueuedBatchIds([]);
    setQueuedBatchIndex(0);
  }, [setBatchQueueMode, setQueuedBatchIds, setQueuedBatchIndex]);

  const stepForQueuedBatch = useCallback(
    (batch: SheinStudioSavedBatch, mode: SheinStudioBatchQueueMode) => {
      if (batch.createdTasks.length > 0) {
        return "tasks" as const;
      }
      if (batch.designs.length > 0) {
        return "review" as const;
      }
      if (mode === "generate") {
        return "generate" as const;
      }
      return "generate" as const;
    },
    [],
  );

  const loadQueuedBatch = useCallback(
    async (
      batchIds: string[],
      index: number,
      mode: SheinStudioBatchQueueMode,
      options?: {
        keepResumeState?: boolean;
        hydratedBatches?: Record<string, SheinStudioWorkbenchHydratedBatch>;
        requestVersion?: number;
      },
    ) => {
      for (let nextIndex = index; nextIndex < batchIds.length; nextIndex += 1) {
        if (
          options?.requestVersion != null &&
          batchQueueRequestVersionRef.current !== options.requestVersion
        ) {
          return false;
        }
        const batchId = batchIds[nextIndex];
        const hydratedBatch =
          options?.hydratedBatches?.[batchId] ??
          selectedRecentBatchHydrations[batchId] ??
          (await hydrateRecentBatchSelection([batchId]))[batchId];
        if (
          options?.requestVersion != null &&
          batchQueueRequestVersionRef.current !== options.requestVersion
        ) {
          return false;
        }
        const batch =
          hydratedBatch?.savedBatch ??
          savedBatches.find((item) => item.id === batchId);
        if (!batch) {
          continue;
        }
        if (hydratedBatch) {
          handleLoadHydratedBatch(hydratedBatch);
        } else {
          handleLoadBatch(batch);
        }
        setQueuedBatchIndex(nextIndex);
        setEffectiveStep(stepForQueuedBatch(batch, mode));
        setQueueMessage("");
        return true;
      }
      clearBatchQueue();
      if (!options?.keepResumeState) {
        setQueueResumeState(null);
      }
      setQueueMessage(queueCompletionMessage(mode, batchIds.length));
      return false;
    },
    [
      clearBatchQueue,
      handleLoadBatch,
      handleLoadHydratedBatch,
      hydrateRecentBatchSelection,
      queueCompletionMessage,
      savedBatches,
      selectedRecentBatchHydrations,
      setEffectiveStep,
      setQueueMessage,
      setQueuedBatchIndex,
      setQueueResumeState,
      stepForQueuedBatch,
    ],
  );

  const startBatchQueue = useCallback(
    async (input: {
      batchIds: string[];
      mode: SheinStudioBatchQueueMode;
      startIndex?: number;
    }) => {
      const requestVersion = batchQueueRequestVersionRef.current + 1;
      batchQueueRequestVersionRef.current = requestVersion;
      recentBatchOpenRequestVersionRef.current += 1;
      const validBatchIds = input.batchIds.filter((batchId) =>
        savedBatches.some((item) => item.id === batchId),
      );
      if (validBatchIds.length === 0) {
        clearBatchQueue();
        setQueueResumeState(null);
        setQueueMessage(queueCompletionMessage(input.mode, 0));
        return;
      }
      const startIndex = Math.max(
        0,
        Math.min(input.startIndex ?? 0, validBatchIds.length - 1),
      );
      setBatchQueueMode(input.mode);
      setQueuedBatchIds(validBatchIds);
      setQueuedBatchIndex(startIndex);
      setQueueResumeState(null);
      const hydratedBatches = await hydrateRecentBatchSelection(validBatchIds);
      if (batchQueueRequestVersionRef.current !== requestVersion) {
        return;
      }
      await loadQueuedBatch(validBatchIds, startIndex, input.mode, {
        hydratedBatches,
        requestVersion,
      });
    },
    [
      clearBatchQueue,
      hydrateRecentBatchSelection,
      loadQueuedBatch,
      queueCompletionMessage,
      savedBatches,
      setBatchQueueMode,
      setQueueMessage,
      setQueuedBatchIds,
      setQueuedBatchIndex,
      setQueueResumeState,
    ],
  );
  const handleOpenBatchQueue = useCallback(
    (input: { batchIds: string[]; mode: SheinStudioBatchQueueMode }) => {
      void startBatchQueue(input);
    },
    [startBatchQueue],
  );

  const handleExitBatchQueue = useCallback(() => {
    if (batchQueueMode && queuedBatchIds.length > 0) {
      setQueueResumeState({
        batchIds: queuedBatchIds,
        mode: batchQueueMode,
        startIndex: queuedBatchIndex,
        total: queuedBatchIds.length,
      });
      setQueueMessage("");
    }
    batchQueueRequestVersionRef.current += 1;
    clearBatchQueue();
  }, [
    batchQueueMode,
    clearBatchQueue,
    queuedBatchIds,
    queuedBatchIndex,
    setQueueMessage,
  ]);

  const handleResumeBatchQueue = useCallback(() => {
    if (!queueResumeState) {
      return;
    }
    void startBatchQueue({
      batchIds: queueResumeState.batchIds,
      mode: queueResumeState.mode,
      startIndex: queueResumeState.startIndex,
    });
  }, [queueResumeState, startBatchQueue]);

  const clearQueuedSelectionContext = useCallback(() => {
    setQueueResumeState(null);
    setSelectedRecentBatchSummaryIds([]);
    setQueueMessage("");
  }, [setQueueMessage]);

  const handleAdvanceBatchQueue = useCallback(() => {
    if (!batchQueueMode) {
      return;
    }
    void loadQueuedBatch(queuedBatchIds, queuedBatchIndex + 1, batchQueueMode, {
      requestVersion: batchQueueRequestVersionRef.current,
    });
  }, [batchQueueMode, loadQueuedBatch, queuedBatchIds, queuedBatchIndex]);

  useEffect(() => {
    if (!batchQueueMode || !currentQueuedBatchId) {
      return;
    }
    const timer = window.setTimeout(() => {
      if (batchQueueMode === "generate" || effectiveStep === "generate") {
        document
          .getElementById("shein-studio-generator")
          ?.scrollIntoView({ behavior: "smooth", block: "start" });
        const promptInput =
          promptInputRef.current ??
          (document.getElementById("prompt") as HTMLInputElement | HTMLTextAreaElement | null);
        promptInput?.focus();
        return;
      }
      if (effectiveStep === "review") {
        document
          .getElementById("shein-style-review")
          ?.scrollIntoView({ behavior: "smooth", block: "start" });
        return;
      }
      if (effectiveStep === "tasks") {
        document
          .getElementById("shein-created-tasks")
          ?.scrollIntoView({ behavior: "smooth", block: "start" });
      }
    }, 0);
    return () => {
      window.clearTimeout(timer);
    };
  }, [batchQueueMode, currentQueuedBatchId, effectiveStep]);

  const applyItemizedBatchDetail = useCallback(
    (
      nextDetail: NonNullable<typeof itemizedBatchDetail>,
      nextCreatedTasks = createdTasks,
    ) => {
      if (!activeBatchId) {
        return false;
      }
      const nextSavedBatch: SheinStudioSavedBatch = {
        ...(currentActiveBatch ?? {}),
        id: activeBatchId,
        name: currentActiveBatch?.name ?? "未命名批次",
        prompt,
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
        selection: activeSelection,
        groupedSelections,
        groups,
        designs: flattenItemizedBatchDesigns(nextDetail),
        selectedIds: getApprovedItemizedBatchDesignIDs(nextDetail),
        createdTasks: nextCreatedTasks,
        generationJobs,
        draftUpdatedAt: currentActiveBatch?.draftUpdatedAt || persistedUpdatedAt,
        updatedAt:
          nextDetail.batch.updatedAt ||
          currentActiveBatch?.updatedAt ||
          persistedUpdatedAt,
      };
      workbenchController.setField("savedBatches", (current) =>
        upsertSavedBatch(current, nextSavedBatch),
      );
      workbenchController.applyHydratedBatch({
        savedBatch: nextSavedBatch,
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
          const nextDetail = await approveSheinStudioBatchDesigns(
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

  const busyMessage = sheinStudioBusyMessage({
    isCreatingTasks,
    isGenerating: effectiveIsGenerating,
    regeneratingId,
  });
  useSheinStudioPendingNavigationGuard({
    enabled: Boolean(effectiveIsGenerating || isCreatingTasks || regeneratingId),
    message:
      "当前正在生成款式图或创建 SHEIN 资料。现在离开会中断当前页面上的进度承接，确认还要离开吗？",
  });

  return (
    <section className="relative space-y-6">
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
        <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-700">
          {queueMessage}
        </div>
      ) : null}

      {initialBatchId ? (
        <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-4 shadow-sm">
          <div className="flex flex-wrap items-start justify-between gap-3">
            <div className="space-y-1">
              <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
                当前批次
              </p>
              {isEditingCurrentBatchName ? (
                <label className="block">
                  <span className="sr-only">当前批次名称</span>
                  <input
                    aria-label="当前批次名称"
                    className="w-full min-w-[280px] rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-900"
                    onChange={(event) => setCurrentBatchDraftName(event.target.value)}
                    value={currentBatchDraftName}
                  />
                </label>
              ) : (
                <p className="text-lg font-semibold text-zinc-950">
                  {currentDedicatedBatch?.name || "未命名批次"}
                </p>
              )}
              <p className="text-sm text-zinc-600">
                在这里可以管理当前这一个批次；批量管理仍然保留在最近批次首页。
              </p>
              <div className="pt-3">
                <BatchStoreSettings
                  currentStoreLabel={currentStoreLabel}
                  requiredMessage={storeRequiredMessage}
                  setSheinStoreId={setSheinStoreId}
                  sheinStoreId={sheinStoreId}
                />
              </div>
              <div className="pt-1">
                <Button
                  onClick={() => {
                    if (initialBatchId) {
                      setActiveSheinStudioBatchId(initialBatchId);
                    }
                    router.push(
                      initialBatchId
                        ? `/listing-kits/sds/new?targetBatchId=${initialBatchId}`
                        : "/listing-kits/sds/new",
                    );
                  }}
                  size="sm"
                  type="button"
                  variant="secondary"
                >
                  去 SDS 选品并加入当前批次
                </Button>
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              {isEditingCurrentBatchName ? (
                <>
                  <Button onClick={() => void handleRenameCurrentDedicatedBatch()} size="sm" type="button">
                    保存名称
                  </Button>
                  <Button
                    onClick={() => {
                      setIsEditingCurrentBatchName(false);
                      setCurrentBatchDraftName(currentDedicatedBatch?.name ?? "");
                    }}
                    size="sm"
                    type="button"
                    variant="ghost"
                  >
                    取消
                  </Button>
                </>
              ) : (
                <Button
                  onClick={() => setIsEditingCurrentBatchName(true)}
                  size="sm"
                  type="button"
                  variant="ghost"
                >
                  重命名当前批次
                </Button>
              )}
              <Button
                onClick={() => void handleDeleteCurrentDedicatedBatch()}
                size="sm"
                type="button"
                variant="ghost"
              >
                删除当前批次
              </Button>
            </div>
          </div>
        </div>
      ) : null}

      {isRecentBatchesHomepage ? null : (
        <>
          <SheinStudioWorkbenchAlerts
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
                artworkModel={artworkModel}
                availableSdsImages={availableSdsImages}
                batchProductCount={countSelectionsWithPrimary(
                  activeSelection,
                  groupedSelections,
                )}
                batchStoreLabel={currentStoreLabel || "未设置"}
                createTaskButtonLabel={
                  groupedSelections.length > 0
                    ? `为 ${countSelectionsWithPrimary(
                        activeSelection,
                        groupedSelections,
                      )} 款商品生成 SHEIN 资料`
                    : "生成 SHEIN 资料"
                }
                createdTasks={createdTasks}
                creatingError={creatingError}
                creatingMessage={creatingMessage}
                generationError={generationError}
                groupedImageMode={groupedImageMode}
                imageStrategy={imageStrategy}
                isCreatingTasks={isCreatingTasks}
                isGenerating={effectiveIsGenerating}
                onCreateTasks={handleCreateTasks}
                onDeleteBatch={handleDeleteBatch}
                onGenerate={handleGenerate}
                onLoadBatch={handleLoadBatch}
                onSaveBatch={handleSaveBatch}
                productImageCount={productImageCount}
                productImagePrompt={productImagePrompt}
                productImagePrompts={productImagePrompts}
                prompt={prompt}
                promptHistory={activeGroupPromptHistory}
                promptInputRef={promptInputRef}
                renderSizeImagesWithSds={renderSizeImagesWithSds}
                saveMessage={saveMessage}
                savedBatches={savedBatches}
                selectedSdsImages={selectedSdsImages}
                selectedStyleCount={selectedIds.length}
                selectionReady={Boolean(activeSelection?.variantId)}
                storeRequiredMessage={storeRequiredMessage}
                showSavedBatches={!initialBatchId}
                subscriptionBlockedMessage={subscriptionBlockedMessage}
                setArtworkModel={setArtworkModel}
                setGroupedImageMode={setGroupedImageMode}
                setImageStrategy={setImageStrategy}
                setProductImageCount={setProductImageCount}
                setProductImagePrompt={setProductImagePrompt}
                setProductImagePrompts={setProductImagePrompts}
                onRestorePrompt={handlePromptChange}
                setPrompt={handlePromptChange}
                setRenderSizeImagesWithSds={setRenderSizeImagesWithSds}
                setSelectedSdsImages={(value) => {
                  hasCustomizedSdsSelectionRef.current = true;
                  setSelectedSdsImages(value);
                }}
                setStyleCount={setStyleCount}
                setVariationIntensity={setVariationIntensity}
                setTransparentBackground={setTransparentBackground}
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
            <SheinStudioTasksStep createdTasks={createdTasks} />
          ) : null}
        </>
      )}
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
    return { compatible: false, reason: "这个商品已经在当前批次里，无需重复加入。" };
  }
  return { compatible: true, reason: "" };
}
