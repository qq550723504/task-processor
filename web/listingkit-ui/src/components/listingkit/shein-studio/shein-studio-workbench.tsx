"use client";

import { useCallback, useEffect, useMemo, useReducer, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { SheinStudioBatchQueueBanner } from "@/components/listingkit/shein-studio/shein-studio-batch-queue-banner";
import { SheinStudioBusyOverlay } from "@/components/listingkit/shein-studio/shein-studio-busy-overlay";
import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import { SheinStudioGroupedSelectionPanel } from "@/components/listingkit/shein-studio/shein-studio-grouped-selection-panel";
import { SheinStudioRecentBatchesDashboard } from "@/components/listingkit/shein-studio/shein-studio-recent-batches-dashboard";
import { SheinStudioSelectionOverview } from "@/components/listingkit/shein-studio/shein-studio-selection-overview";
import { SheinStudioTasksStep } from "@/components/listingkit/shein-studio/shein-studio-tasks-step";
import { useSheinStudioDesignActions } from "@/components/listingkit/shein-studio/shein-studio-workbench-actions";
import {
  useHydratedSDSVariantSelection,
  loadLocalSheinStudioDraftSnapshot,
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
  mergeSheinStudioDraftState,
  sheinStudioBusyMessage,
  summarizeSheinStudioSelection,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import {
  applySheinStudioWorkbenchBatch,
  applySheinStudioWorkbenchDraft,
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
import { buildRecentBatchSummaries } from "@/lib/shein-studio/recent-batch-summaries";
import { formatSheinStoreOptionLabel } from "@/lib/shein-studio/store-option-label";
import { getSDSBaselineReadiness } from "@/lib/api/sds-baseline";
import { warmSDSBaselineForSelection } from "@/lib/api/sds-baseline";
import { getCurrentSubscription } from "@/lib/api/subscription";
import { useSDSGroupedCandidates } from "@/lib/query/use-sds-grouped-candidates";
import { useSheinStoreSelector } from "@/lib/query/use-shein-store-selector";
import {
  listSheinStudioBatches,
  saveSheinStudioBatch,
} from "@/lib/utils/shein-studio-batches";
import {
  buildGroupedSDSSelectionID,
  type GroupedSDSSelectionEligibility,
  type SDSBaselineStatus,
} from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioBatchQueueMode,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";

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
    productImageCount,
    productImagePrompt,
    productImagePrompts,
    prompt,
    queueMessage,
    regeneratingId,
    renderSizeImagesWithSds,
    groupedSelections,
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
  const activeGroupSelection = useHydratedSDSVariantSelection(
    groups.find((group) => group.id === activeGroupId)?.primarySelection,
  );
  const activeSelection = directSelection ?? activeGroupSelection;
  const groupedCandidateSelections = useSDSGroupedCandidates();
  const [isExecutingWarningAction, setIsExecutingWarningAction] = useState(false);
  const [selectedRecentBatchSummaryIds, setSelectedRecentBatchSummaryIds] = useState<
    string[]
  >([]);
  const [queueResumeState, setQueueResumeState] = useState<{
    batchIds: string[];
    mode: SheinStudioBatchQueueMode;
    startIndex: number;
    total: number;
  } | null>(null);
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
  const effectiveCurrentStoreId = (sheinStoreId ?? "").trim() || recommendedStoreId;
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
  const localDraftSnapshot = useMemo(() => loadLocalSheinStudioDraftSnapshot(), []);
  const recentBatchSummaries = useMemo(
    () =>
      buildRecentBatchSummaries(savedBatches, {
        draft: localDraftSnapshot,
      }),
    [localDraftSnapshot, savedBatches],
  );
  useEffect(() => {
    const validSummaryKeys = new Set(
      recentBatchSummaries.map((summary) => `${summary.source}:${summary.id}`),
    );
    setSelectedRecentBatchSummaryIds((current) =>
      current.filter((key) => validSummaryKeys.has(key)),
    );
  }, [recentBatchSummaries]);
  const currentQueuedBatchId = batchQueueMode
    ? queuedBatchIds[queuedBatchIndex] ?? ""
    : "";
  const currentQueuedBatch = useMemo(
    () => savedBatches.find((item) => item.id === currentQueuedBatchId) ?? null,
    [currentQueuedBatchId, savedBatches],
  );
  const batchQueueGuidance = useMemo(() => {
    if (batchQueueMode === "generate") {
      return "已定位到生成区，可直接修改提示词或继续生成。";
    }
    if (effectiveStep === "tasks") {
      return "已定位到任务区，可继续查看已创建的任务。";
    }
    if (effectiveStep === "review") {
      return "已定位到审核区，可直接创建任务或调整款式。";
    }
    return "当前批次还没有可用设计，已回到生成区继续处理。";
  }, [batchQueueMode, effectiveStep]);
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
  const { buildDraftInput, persistDraft } = useSheinStudioDraftPersistence({
    activeSelection,
    artworkModel,
    createdTasks,
    designs,
    groups,
    groupedImageMode,
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
          designs: draftState.designs,
          selectedIds: draftState.selectedIds,
          createdTasks: draftState.createdTasks,
        });
        setEffectiveStep(targetStep);
        return;
      }
      const batch = savedBatches.find((item) => item.id === summary.id);
      if (batch) {
        handleLoadBatch(batch);
        setEffectiveStep(targetStep);
      }
    },
    [
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
    }),
    [],
  );

  const refreshSavedBatches = useCallback(async () => {
    workbenchController.setField("savedBatches", await listSheinStudioBatches());
  }, [workbenchController]);

  const handleRenameRecentBatchSummary = useCallback(
    async (summary: (typeof recentBatchSummaries)[number], name: string) => {
      if (summary.source !== "batch") {
        return;
      }
      const batch = savedBatches.find((item) => item.id === summary.id);
      if (!batch) {
        return;
      }
      await saveSheinStudioBatch(
        buildSaveInputFromBatch(batch, { name }),
        { makeActive: false },
      );
      await refreshSavedBatches();
    },
    [buildSaveInputFromBatch, refreshSavedBatches, recentBatchSummaries, savedBatches],
  );

  const handleDuplicateRecentBatchSummary = useCallback(
    async (summary: (typeof recentBatchSummaries)[number]) => {
      if (summary.source !== "batch") {
        return;
      }
      const batch = savedBatches.find((item) => item.id === summary.id);
      if (!batch) {
        return;
      }
      await saveSheinStudioBatch(
        buildSaveInputFromBatch(batch, {
          id: undefined,
          name: `${batch.name} 副本`,
        }),
        { makeActive: false },
      );
      await refreshSavedBatches();
    },
    [buildSaveInputFromBatch, refreshSavedBatches, recentBatchSummaries, savedBatches],
  );

  const handleDeleteRecentBatchSummary = useCallback(
    async (summary: (typeof recentBatchSummaries)[number]) => {
      if (summary.source !== "batch") {
        return;
      }
      await handleDeleteBatch(summary.id);
    },
    [handleDeleteBatch, recentBatchSummaries],
  );

  const handleBulkUpdateRecentBatchStore = useCallback(
    async (summaryIds: string[], storeId: string) => {
      const targets = savedBatches.filter((batch) => summaryIds.includes(batch.id));
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
    [buildSaveInputFromBatch, refreshSavedBatches, savedBatches],
  );

  const clearBatchQueue = useCallback(() => {
    setBatchQueueMode(null);
    setQueuedBatchIds([]);
    setQueuedBatchIndex(0);
  }, [setBatchQueueMode, setQueuedBatchIds, setQueuedBatchIndex]);

  const stepForQueuedBatch = useCallback(
    (batch: SheinStudioSavedBatch, mode: SheinStudioBatchQueueMode) => {
      if (mode === "generate") {
        return "generate" as const;
      }
      if (batch.createdTasks.length > 0) {
        return "tasks" as const;
      }
      if (batch.designs.length > 0) {
        return "review" as const;
      }
      return "generate" as const;
    },
    [],
  );

  const loadQueuedBatch = useCallback(
    (
      batchIds: string[],
      index: number,
      mode: SheinStudioBatchQueueMode,
      options?: { keepResumeState?: boolean },
    ) => {
      for (let nextIndex = index; nextIndex < batchIds.length; nextIndex += 1) {
        const batch = savedBatches.find((item) => item.id === batchIds[nextIndex]);
        if (!batch) {
          continue;
        }
        handleLoadBatch(batch);
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
      queueCompletionMessage,
      savedBatches,
      setEffectiveStep,
      setQueueMessage,
      setQueuedBatchIndex,
      setQueueResumeState,
      stepForQueuedBatch,
    ],
  );

  const startBatchQueue = useCallback(
    (input: {
      batchIds: string[];
      mode: SheinStudioBatchQueueMode;
      startIndex?: number;
    }) => {
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
      loadQueuedBatch(validBatchIds, startIndex, input.mode);
    },
    [
      clearBatchQueue,
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
      startBatchQueue(input);
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
    startBatchQueue({
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
    loadQueuedBatch(queuedBatchIds, queuedBatchIndex + 1, batchQueueMode);
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

      <SheinStudioRecentBatchesDashboard
        onBulkUpdateStore={handleBulkUpdateRecentBatchStore}
        onCreateBatch={() => {
          setEffectiveStep("generate");
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
            currentStoreId={effectiveCurrentStoreId}
            currentStoreLabel={currentStoreLabel}
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
            groupedImageMode={groupedImageMode}
            imageStrategy={imageStrategy}
            isCreatingTasks={isCreatingTasks}
            isGenerating={isGenerating}
            onCreateTasks={handleCreateTasks}
            onDeleteBatch={handleDeleteBatch}
            onGenerate={handleGenerate}
            onLoadBatch={handleLoadBatch}
            onRestorePrompt={setPrompt}
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
            subscriptionBlockedMessage={subscriptionBlockedMessage}
            setArtworkModel={setArtworkModel}
            setGroupedImageMode={setGroupedImageMode}
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
  return { compatible: true, reason: "" };
}
