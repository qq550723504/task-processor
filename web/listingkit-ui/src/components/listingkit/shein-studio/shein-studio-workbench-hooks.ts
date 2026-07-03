import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePathname } from "next/navigation";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import { projectActiveSelectionBaselineState } from "@/components/listingkit/shein-studio/shein-studio-generation-controller";
import { projectSheinStudioQueueState } from "@/components/listingkit/shein-studio/shein-studio-queue-controller";
import {
  buildSheinStudioSelectionKey,
  getSheinStudioCreateActionDisabledReason,
  hasInFlightItemizedBatchGeneration,
  projectStudioSubscriptionGate,
  projectWorkbenchStateFallback,
  projectWorkbenchTraceContext,
  projectSheinStudioStoreSelectionState,
  resolveCurrentSheinStudioSavedBatch,
  selectActiveGroupPromptHistory,
  selectActiveGroupPrimarySelection,
  selectCurrentDedicatedBatch,
  sheinStudioBusyMessage,
  summarizeSheinStudioSelection,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import type { SheinStudioWorkbenchState } from "@/components/listingkit/shein-studio/shein-studio-workbench-state";
import { buildSheinStudioDraftInput } from "@/lib/shein-studio/draft-input";
import {
  type DraftSaveOptions,
  getSheinStudioAutosaveDelayMs,
  persistSheinStudioDraft,
  runSheinStudioDraftAutosave,
} from "@/lib/shein-studio/draft-persistence";
import { saveLocalSheinStudioDraftSnapshot } from "@/lib/shein-studio/local-draft-cache";
import { hydrateSDSVariantSelection } from "@/lib/shein-studio/hydrate-sds-selection";
import { buildSheinStudioStepHref } from "@/lib/shein-studio/navigation";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { SDSRatioMatch } from "@/lib/shein-studio/gallery-handoff";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioArtworkModel,
  SheinStudioArtworkGenerationMode,
  SheinStudioBatchQueueMode,
  SheinStudioCreatedTask,
  SheinStudioGenerationJob,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioGroupedWorkspace,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioSavedBatch,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";
import type { SheinStudioBatchDetail } from "@/lib/types/shein-studio-batch";
import type { SheinStudioBatchQueueResumeState } from "@/lib/shein-studio/batch-queue";
import type { formatSheinStoreOptionLabel } from "@/lib/shein-studio/store-option-label";
import type { SDSBaselineStatus } from "@/lib/types/sds-baseline";
import { replaceBrowserHistory } from "@/lib/utils/browser-history";
import { useLiveSearchParams } from "@/lib/utils/live-search-params";
import {
  saveSheinStudioBatch,
  saveSheinStudioDraftWithOptions,
} from "@/lib/utils/shein-studio-batches";
import type { SubscriptionSummary } from "@/lib/api/subscription";

export {
  clearLocalSheinStudioDraftSnapshot,
  loadLocalSheinStudioDraftSnapshot,
  loadLocalSheinStudioDraftSnapshotDetail,
  saveLocalSheinStudioDraftSnapshot,
} from "@/lib/shein-studio/local-draft-cache";

type DraftOverrides = Partial<{
  designs: SheinStudioGeneratedDesign[];
  groups: SheinStudioGroupedWorkspace[];
  selectedIds: string[];
  selection: SDSProductVariantSelection;
  createdTasks: SheinStudioCreatedTask[];
  generationJobs: SheinStudioGenerationJob[];
  generationError: string;
  generationJobId: string;
}>;

type WorkbenchDraftState = {
  activeSelection?: SDSProductVariantSelection;
  artworkModel: SheinStudioArtworkModel;
  createdTasks: SheinStudioCreatedTask[];
  generationError: string;
  generationJobId?: string;
  generationJobs: SheinStudioGenerationJob[];
  designs: SheinStudioGeneratedDesign[];
  groups: SheinStudioGroupedWorkspace[];
  imageStrategy: SheinStudioImageStrategy;
  groupedImageMode: SheinStudioGroupedImageMode;
  isCreatingTasks: boolean;
  isGenerating: boolean;
  isLoadingWorkspace: boolean;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  hotStyleReferenceImageUrls: string[];
  hotStyleReferenceBrief: string;
  hotStyleReferencePrompt: string;
  artworkGenerationMode: SheinStudioArtworkGenerationMode;
  prompt: string;
  promptMode: "managed" | "raw";
  regeneratingId: string;
  renderSizeImagesWithSds: boolean;
  groupedSelections: GroupedSDSSelectionEligibility[];
  persistedUpdatedAt: string;
  selectedIds: string[];
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  setDraftWarning: (value: string | ((current: string) => string)) => void;
  setPersistedUpdatedAt: (value: string) => void;
  sheinStoreId: string;
  styleCount: string;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
};

export function useSheinStudioActiveBatchScope({
  initialBatchId,
  selectionVariantId,
}: {
  initialBatchId?: string;
  selectionVariantId: number | null;
}) {
  const [activeBatchScope, setActiveBatchScope] = useState(() => ({
    batchId: initialBatchId ?? "",
    selectionVariantId,
  }));
  const setActiveBatchId = useCallback(
    (batchId: string) => {
      setActiveBatchScope({
        batchId,
        selectionVariantId,
      });
    },
    [selectionVariantId],
  );
  const activeBatchId =
    initialBatchId ??
    (activeBatchScope.selectionVariantId === selectionVariantId
      ? activeBatchScope.batchId
      : "");

  return {
    activeBatchId,
    setActiveBatchId,
  };
}

export function useSheinStudioWorkbenchTraceContext({
  activeBatchId,
  batchQueueMode,
  currentQueuedBatchId,
  initialBatchId,
  queuedBatchIds,
  queuedBatchIndex,
}: {
  activeBatchId: string;
  batchQueueMode: SheinStudioBatchQueueMode | null;
  currentQueuedBatchId: string;
  initialBatchId?: string;
  queuedBatchIds: string[];
  queuedBatchIndex: number;
}) {
  const traceBatchId =
    currentQueuedBatchId || activeBatchId || initialBatchId || "";
  return useMemo(
    () =>
      projectWorkbenchTraceContext({
        batchQueueMode,
        queuedBatchIds,
        queuedBatchIndex,
        traceBatchId,
      }),
    [batchQueueMode, queuedBatchIds, queuedBatchIndex, traceBatchId],
  );
}

export function useSheinStudioQueueState({
  batchQueueMode,
  effectiveStep,
  queueResumeState,
  queuedBatchIds,
  queuedBatchIndex,
  savedBatches,
}: {
  batchQueueMode: SheinStudioBatchQueueMode | null;
  effectiveStep: SheinStudioStepKey;
  queueResumeState: SheinStudioBatchQueueResumeState | null;
  queuedBatchIds: string[];
  queuedBatchIndex: number;
  savedBatches: SheinStudioSavedBatch[];
}) {
  return useMemo(
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
}

export function useSheinStudioCurrentBatchSelection({
  activeBatchId,
  initialBatchId,
  savedBatches,
  workbenchState,
}: {
  activeBatchId: string;
  initialBatchId?: string;
  savedBatches: SheinStudioSavedBatch[];
  workbenchState: SheinStudioWorkbenchState;
}) {
  const currentActiveBatch = useMemo(
    () =>
      resolveCurrentSheinStudioSavedBatch({
        activeBatchId,
        fallback: projectWorkbenchStateFallback(workbenchState),
        initialBatchId,
        savedBatches,
      }),
    [activeBatchId, initialBatchId, savedBatches, workbenchState],
  );
  const currentDedicatedBatch = useMemo(
    () =>
      selectCurrentDedicatedBatch({
        currentActiveBatch,
        initialBatchId,
      }),
    [currentActiveBatch, initialBatchId],
  );

  return {
    currentActiveBatch,
    currentDedicatedBatch,
  };
}

export function useSheinStudioStoreSelection({
  currentStoreId,
  enabledProfiles,
}: {
  currentStoreId: string;
  enabledProfiles: Array<Parameters<typeof formatSheinStoreOptionLabel>[0]>;
}) {
  return useMemo(
    () =>
      projectSheinStudioStoreSelectionState({
        currentStoreId,
        enabledProfiles,
      }),
    [currentStoreId, enabledProfiles],
  );
}

export function useSheinStudioActiveGroupPromptHistory({
  activeGroupId,
  groups,
}: {
  activeGroupId: string;
  groups: SheinStudioGroupedWorkspace[];
}) {
  return useMemo(
    () =>
      selectActiveGroupPromptHistory({
        activeGroupId,
        groups,
      }),
    [activeGroupId, groups],
  );
}

export function useSheinStudioActiveGroupPrimarySelection({
  activeGroupId,
  groups,
}: {
  activeGroupId: string;
  groups: SheinStudioGroupedWorkspace[];
}) {
  return useMemo(
    () =>
      selectActiveGroupPrimarySelection({
        activeGroupId,
        groups,
      }),
    [activeGroupId, groups],
  );
}

export function useSheinStudioActiveSelectionSummary(
  activeSelection?: SDSProductVariantSelection,
) {
  return useMemo(
    () => ({
      activeSelectionKey: buildSheinStudioSelectionKey(activeSelection),
      ...summarizeSheinStudioSelection(activeSelection),
    }),
    [activeSelection],
  );
}

export function useSheinStudioActiveSelectionBaselineState({
  activeGroupedSelectionID,
  baselineStatuses,
  hasActiveSelection,
}: {
  activeGroupedSelectionID: string;
  baselineStatuses: Record<
    string,
    {
      baselineKey?: string;
      reason: string;
      reasonCode?: string;
      status: SDSBaselineStatus;
    }
  >;
  hasActiveSelection: boolean;
}) {
  return useMemo(
    () =>
      projectActiveSelectionBaselineState({
        activeGroupedSelectionID,
        baselineStatuses,
        hasActiveSelection,
      }),
    [activeGroupedSelectionID, baselineStatuses, hasActiveSelection],
  );
}

export function useSheinStudioSubscriptionGate(
  subscription?: SubscriptionSummary,
) {
  return useMemo(
    () => projectStudioSubscriptionGate(subscription),
    [subscription],
  );
}

export function useSheinStudioCreateActionDisabledReason({
  galleryRatioCheck,
  hasItemizedBatchContext,
  itemizedApprovedCount,
  selectedIds,
  selection,
}: {
  galleryRatioCheck?: SDSRatioMatch | null;
  hasItemizedBatchContext?: boolean;
  itemizedApprovedCount?: number;
  selectedIds: string[];
  selection?: SDSProductVariantSelection;
}) {
  return useMemo(
    () =>
      getSheinStudioCreateActionDisabledReason({
        galleryRatioCheck,
        hasItemizedBatchContext,
        itemizedApprovedCount,
        selectedIds,
        selection,
      }),
    [
      galleryRatioCheck,
      hasItemizedBatchContext,
      itemizedApprovedCount,
      selectedIds,
      selection,
    ],
  );
}

export function useSheinStudioBusyMessage({
  isCreatingTasks,
  isGenerating,
  regeneratingId,
}: {
  isCreatingTasks: boolean;
  isGenerating: boolean;
  regeneratingId?: string;
}) {
  return useMemo(
    () =>
      sheinStudioBusyMessage({
        isCreatingTasks,
        isGenerating,
        regeneratingId,
      }),
    [isCreatingTasks, isGenerating, regeneratingId],
  );
}

export function useSheinStudioItemizedGenerationInFlight(
  detail?: SheinStudioBatchDetail | null,
) {
  return useMemo(() => hasInFlightItemizedBatchGeneration(detail), [detail]);
}

export function useHydratedSDSVariantSelection(
  selection?: SDSProductVariantSelection,
) {
  const selectionKey = buildSheinStudioSelectionKey(selection);
  const [hydratedSelection, setHydratedSelection] = useState<{
    key: string;
    selection?: SDSProductVariantSelection;
  } | null>(null);

  useEffect(() => {
    let cancelled = false;

    void hydrateSDSVariantSelection(selection).then((nextSelection) => {
      if (!cancelled) {
        setHydratedSelection({
          key: selectionKey,
          selection: nextSelection,
        });
      }
    });

    return () => {
      cancelled = true;
    };
  }, [selection, selectionKey]);

  return hydratedSelection?.key === selectionKey
    ? (hydratedSelection.selection ?? selection)
    : selection;
}

export function useSheinStudioStepNavigation(activeStep: SheinStudioStepKey) {
  const pathname = usePathname();
  const searchParams = useLiveSearchParams();
  const [navigationOverride, setNavigationOverride] = useState<{
    baseStep: SheinStudioStepKey;
    step: SheinStudioStepKey;
  } | null>(null);
  const activeStepRef = useRef(activeStep);
  const effectiveStep =
    navigationOverride?.baseStep === activeStep
      ? navigationOverride.step
      : activeStep;

  useEffect(() => {
    activeStepRef.current = activeStep;
  }, [activeStep]);

  const setEffectiveStep = useCallback((step: SheinStudioStepKey) => {
    setNavigationOverride({
      baseStep: activeStepRef.current,
      step,
    });
  }, []);

  const navigateToStep = useCallback(
    (step: SheinStudioStepKey) => {
      setEffectiveStep(step);
      try {
        replaceBrowserHistory(
          buildSheinStudioStepHref(pathname, searchParams, step),
        );
      } catch (error) {
        console.warn(
          "shein studio step navigation failed",
          error instanceof Error ? error.message : error,
        );
      }
    },
    [pathname, searchParams, setEffectiveStep],
  );

  return {
    activeStepRef,
    effectiveStep,
    navigateToStep,
    setEffectiveStep,
  };
}

export function useSheinStudioDraftPersistence(
  state: WorkbenchDraftState,
  options?: {
    activeBatchId?: string;
    persistenceEnabled?: boolean;
  },
) {
  const autosaveFingerprintRef = useRef("");
  const activeBatchId = options?.activeBatchId?.trim() || "";
  const persistenceEnabled = options?.persistenceEnabled ?? true;
  const autosaveDelayMs = getSheinStudioAutosaveDelayMs(activeBatchId);

  const buildDraftInput = useCallback(
    (overrides?: DraftOverrides) =>
      buildSheinStudioDraftInput({
        updatedAt: state.persistedUpdatedAt,
        artworkGenerationMode: state.artworkGenerationMode,
        prompt: state.prompt,
        promptMode: state.promptMode,
        styleCount: state.styleCount,
        variationIntensity: state.variationIntensity,
        productImageCount: state.productImageCount,
        productImagePrompt: state.productImagePrompt,
        productImagePrompts: state.productImagePrompts,
        hotStyleReferenceImageUrls: state.hotStyleReferenceImageUrls,
        hotStyleReferenceBrief: state.hotStyleReferenceBrief,
        hotStyleReferencePrompt: state.hotStyleReferencePrompt,
        artworkModel: state.artworkModel,
        transparentBackground: state.transparentBackground,
        sheinStoreId: state.sheinStoreId,
        imageStrategy: state.imageStrategy,
        groupedImageMode: state.groupedImageMode,
        selectedSdsImages: state.selectedSdsImages,
        renderSizeImagesWithSds: state.renderSizeImagesWithSds,
        selection: overrides?.selection ?? state.activeSelection,
        groups: overrides?.groups ?? state.groups,
        groupedSelections: state.groupedSelections,
        designs: overrides?.designs ?? state.designs,
        selectedIds: overrides?.selectedIds ?? state.selectedIds,
        createdTasks: overrides?.createdTasks ?? state.createdTasks,
        generationJobs: overrides?.generationJobs ?? state.generationJobs,
        generationError: overrides?.generationError ?? state.generationError,
        generationJobId: overrides?.generationJobId ?? state.generationJobId,
      }),
    [state],
  );

  const persistDraft = useCallback(
    async (overrides?: DraftOverrides, options?: DraftSaveOptions) => {
      const nextDraftInput = buildDraftInput(overrides);
      return persistSheinStudioDraft({
        activeBatchId,
        draftInput: nextDraftInput,
        options,
        saveLocalSnapshot: saveLocalSheinStudioDraftSnapshot,
        saveBatch: saveSheinStudioBatch,
        saveDraft: saveSheinStudioDraftWithOptions,
        setPersistedUpdatedAt: state.setPersistedUpdatedAt,
        setDraftWarning: state.setDraftWarning,
      });
    },
    [activeBatchId, buildDraftInput, state],
  );

  const currentDraftInput = useMemo(() => buildDraftInput(), [buildDraftInput]);

  useEffect(() => {
    if (!activeBatchId || !persistenceEnabled) {
      return;
    }
    saveLocalSheinStudioDraftSnapshot(currentDraftInput, {
      batchId: activeBatchId,
    });
  }, [activeBatchId, currentDraftInput, persistenceEnabled]);

  useEffect(() => {
    const timer = window.setTimeout(() => {
      runSheinStudioDraftAutosave({
        activeBatchId,
        draftInput: currentDraftInput,
        fingerprintRef: autosaveFingerprintRef,
        persistenceEnabled,
        isLoadingWorkspace: state.isLoadingWorkspace,
        isGenerating: state.isGenerating,
        isCreatingTasks: state.isCreatingTasks,
        regeneratingId: state.regeneratingId,
        saveLocalSnapshot: saveLocalSheinStudioDraftSnapshot,
        setDraftWarning: state.setDraftWarning,
      });
    }, autosaveDelayMs);

    return () => {
      window.clearTimeout(timer);
    };
  }, [
    activeBatchId,
    autosaveDelayMs,
    currentDraftInput,
    persistenceEnabled,
    state,
  ]);

  return {
    buildDraftInput,
    persistDraft,
  };
}

export function useSheinStudioPendingNavigationGuard({
  enabled,
  message,
}: {
  enabled: boolean;
  message: string;
}) {
  useEffect(() => {
    if (!enabled) {
      return;
    }

    const handleBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = message;
      return message;
    };

    const handleDocumentClick = (event: MouseEvent) => {
      if (event.defaultPrevented || event.button !== 0) {
        return;
      }
      if (event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) {
        return;
      }

      const target = event.target instanceof Element ? event.target : null;
      const anchor = target?.closest("a[href]") as HTMLAnchorElement | null;
      if (!anchor) {
        return;
      }
      if ((anchor.getAttribute("target") ?? "").trim() === "_blank") {
        return;
      }

      const href = anchor.getAttribute("href") ?? "";
      if (!href || href.startsWith("#")) {
        return;
      }

      const currentUrl = new URL(window.location.href);
      const nextUrl = new URL(anchor.href, currentUrl.href);
      const changingPage =
        nextUrl.origin !== currentUrl.origin ||
        nextUrl.pathname !== currentUrl.pathname ||
        nextUrl.search !== currentUrl.search;
      if (!changingPage) {
        return;
      }

      if (window.confirm(message)) {
        return;
      }
      event.preventDefault();
      event.stopPropagation();
      event.stopImmediatePropagation?.();
    };

    window.addEventListener("beforeunload", handleBeforeUnload);
    document.addEventListener("click", handleDocumentClick, true);

    return () => {
      window.removeEventListener("beforeunload", handleBeforeUnload);
      document.removeEventListener("click", handleDocumentClick, true);
    };
  }, [enabled, message]);
}
