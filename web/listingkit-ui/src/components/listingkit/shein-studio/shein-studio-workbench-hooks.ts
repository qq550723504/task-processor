import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { usePathname } from "next/navigation";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import { buildSheinStudioSelectionKey } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import { normalizeDraft } from "@/lib/shein-studio/storage-shared";
import { buildSheinStudioDraftInput } from "@/lib/shein-studio/draft-input";
import { hydrateSDSVariantSelection } from "@/lib/shein-studio/hydrate-sds-selection";
import { buildSheinStudioStepHref } from "@/lib/shein-studio/navigation";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioDraft,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioGroupedWorkspace,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";
import { replaceBrowserHistory } from "@/lib/utils/browser-history";
import { useLiveSearchParams } from "@/lib/utils/live-search-params";
import {
  saveSheinStudioBatch,
  saveSheinStudioDraftWithOptions,
} from "@/lib/utils/shein-studio-batches";

type DraftOverrides = Partial<{
  designs: SheinStudioGeneratedDesign[];
  groups: SheinStudioGroupedWorkspace[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
}>;

type DraftSaveOptions = {
  navigationTriggered?: boolean;
  source?: string;
  signal?: AbortSignal;
};

type WorkbenchDraftState = {
  activeSelection?: SDSProductVariantSelection;
  artworkModel: SheinStudioArtworkModel;
  createdTasks: SheinStudioCreatedTask[];
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
  prompt: string;
  regeneratingId: string;
  renderSizeImagesWithSds: boolean;
  groupedSelections: GroupedSDSSelectionEligibility[];
  selectedIds: string[];
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  setDraftWarning: (value: string | ((current: string) => string)) => void;
  sheinStoreId: string;
  styleCount: string;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
};

const DRAFT_SAVE_WARNING =
  "款式图已生成，但草稿保存失败，刷新后可能丢失。可继续审核，或先保存批次。";
const LOCAL_DRAFT_SNAPSHOT_KEY = "listingkit:shein-studio:recent-draft";

function appendDraftSaveWarning(current: string) {
  if (current.includes(DRAFT_SAVE_WARNING)) {
    return current;
  }
  return current ? `${current} ${DRAFT_SAVE_WARNING}` : DRAFT_SAVE_WARNING;
}

function clearDraftSaveWarning(current: string) {
  return current.replace(DRAFT_SAVE_WARNING, "").trim();
}

function canUseLocalDraftSnapshot() {
  return typeof window !== "undefined" && typeof window.localStorage !== "undefined";
}

type LocalDraftSnapshotInput =
  | ReturnType<typeof buildSheinStudioDraftInput>
  | SheinStudioDraft
  | null
  | undefined;

type LocalDraftSnapshotPayload = {
  batchId?: string;
  draft: SheinStudioDraft;
};

function parseLocalSheinStudioDraftSnapshot() {
  if (!canUseLocalDraftSnapshot()) {
    return null;
  }
  const raw = window.localStorage.getItem(LOCAL_DRAFT_SNAPSHOT_KEY);
  if (!raw) {
    return null;
  }
  try {
    const parsed = JSON.parse(raw) as {
      batchId?: unknown;
      draft?: unknown;
    };
    const normalizedDraft = normalizeDraft(
      parsed && typeof parsed === "object" && "draft" in parsed
        ? (parsed.draft as Partial<SheinStudioDraft> | null | undefined)
        : (parsed as Partial<SheinStudioDraft> | null | undefined),
    );
    if (!normalizedDraft) {
      return null;
    }
    return {
      batchId: typeof parsed?.batchId === "string" ? parsed.batchId : undefined,
      draft: normalizedDraft,
    } satisfies LocalDraftSnapshotPayload;
  } catch (error) {
    console.warn(
      "shein studio local draft snapshot parse failed",
      error instanceof Error ? error.message : error,
    );
    return null;
  }
}

export function loadLocalSheinStudioDraftSnapshot() {
  return parseLocalSheinStudioDraftSnapshot()?.draft ?? null;
}

export function loadLocalSheinStudioDraftSnapshotDetail() {
  return parseLocalSheinStudioDraftSnapshot();
}

export function saveLocalSheinStudioDraftSnapshot(
  input: LocalDraftSnapshotInput,
  options?: {
    batchId?: string;
  },
) {
  if (!canUseLocalDraftSnapshot() || !input) {
    return;
  }
  const draft = {
    ...input,
    updatedAt:
      "updatedAt" in input && typeof input.updatedAt === "string"
        ? input.updatedAt
        : new Date().toISOString(),
  } satisfies SheinStudioDraft;
  const payload = {
    batchId: options?.batchId?.trim() || undefined,
    draft,
  };
  try {
    window.localStorage.setItem(LOCAL_DRAFT_SNAPSHOT_KEY, JSON.stringify(payload));
  } catch (error) {
    console.warn(
      "shein studio local draft snapshot save failed",
      error instanceof Error ? error.message : error,
    );
  }
}

export function clearLocalSheinStudioDraftSnapshot() {
  if (!canUseLocalDraftSnapshot()) {
    return;
  }
  window.localStorage.removeItem(LOCAL_DRAFT_SNAPSHOT_KEY);
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
    ? hydratedSelection.selection ?? selection
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
        replaceBrowserHistory(buildSheinStudioStepHref(pathname, searchParams, step));
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
  const autosaveAbortRef = useRef<AbortController | null>(null);
  const autosaveFingerprintRef = useRef("");
  const activeBatchId = options?.activeBatchId?.trim() || "";
  const persistenceEnabled = options?.persistenceEnabled ?? true;
  const autosaveDelayMs = activeBatchId ? 250 : 1200;

  const buildDraftInput = useCallback(
    (overrides?: DraftOverrides) =>
      buildSheinStudioDraftInput({
        prompt: state.prompt,
        styleCount: state.styleCount,
        variationIntensity: state.variationIntensity,
        productImageCount: state.productImageCount,
        productImagePrompt: state.productImagePrompt,
        productImagePrompts: state.productImagePrompts,
        artworkModel: state.artworkModel,
        transparentBackground: state.transparentBackground,
        sheinStoreId: state.sheinStoreId,
        imageStrategy: state.imageStrategy,
        groupedImageMode: state.groupedImageMode,
        selectedSdsImages: state.selectedSdsImages,
        renderSizeImagesWithSds: state.renderSizeImagesWithSds,
        selection: state.activeSelection,
        groups: overrides?.groups ?? state.groups,
        groupedSelections: state.groupedSelections,
        designs: overrides?.designs ?? state.designs,
        selectedIds: overrides?.selectedIds ?? state.selectedIds,
        createdTasks: overrides?.createdTasks ?? state.createdTasks,
      }),
    [state],
  );

  const persistDraft = useCallback(
    async (overrides?: DraftOverrides, options?: DraftSaveOptions) => {
      const nextDraftInput = buildDraftInput(overrides);
      saveLocalSheinStudioDraftSnapshot(nextDraftInput, {
        batchId: activeBatchId,
      });
      try {
        const draft = activeBatchId
          ? await saveSheinStudioBatch(
              {
                ...nextDraftInput,
                id: activeBatchId,
              },
              { makeActive: false },
            )
          : await saveSheinStudioDraftWithOptions(nextDraftInput, options);
        saveLocalSheinStudioDraftSnapshot(draft, {
          batchId: activeBatchId,
        });
        state.setDraftWarning((current) => clearDraftSaveWarning(current));
        return draft;
      } catch (error) {
        if (options?.signal?.aborted) {
          return null;
        }
        state.setDraftWarning((current) => appendDraftSaveWarning(current));
        throw error;
      }
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
    if (!persistenceEnabled) {
      return;
    }
    if (!activeBatchId && state.isLoadingWorkspace) {
      return;
    }
    if (
      state.isGenerating ||
      state.isCreatingTasks ||
      Boolean(state.regeneratingId)
    ) {
      return;
    }

    const timer = window.setTimeout(() => {
      const draftInput = currentDraftInput;
      const fingerprint = JSON.stringify(draftInput);
      if (autosaveFingerprintRef.current === fingerprint) {
        return;
      }

      autosaveAbortRef.current?.abort("superseded");
      const controller = new AbortController();
      autosaveAbortRef.current = controller;

      const timeout = window.setTimeout(() => {
        controller.abort("timeout");
      }, 15000);

      saveLocalSheinStudioDraftSnapshot(draftInput, {
        batchId: activeBatchId,
      });
      const autosavePromise = activeBatchId
        ? saveSheinStudioBatch(
            {
              ...draftInput,
              id: activeBatchId,
            },
            { makeActive: false },
          )
        : saveSheinStudioDraftWithOptions(draftInput, {
            signal: controller.signal,
            source: "autosave",
          });

      void autosavePromise
        .then((draft) => {
          saveLocalSheinStudioDraftSnapshot(draft, {
            batchId: activeBatchId,
          });
          state.setDraftWarning((current) => clearDraftSaveWarning(current));
          autosaveFingerprintRef.current = fingerprint;
        })
        .catch((error) => {
          if (controller.signal.aborted) {
            return;
          }
          state.setDraftWarning((current) => appendDraftSaveWarning(current));
          console.warn(
            "shein studio draft autosave failed",
            error instanceof Error ? error.message : error,
          );
        })
        .finally(() => {
          window.clearTimeout(timeout);
          if (autosaveAbortRef.current === controller) {
            autosaveAbortRef.current = null;
          }
        });
    }, autosaveDelayMs);

    return () => {
      window.clearTimeout(timer);
    };
  }, [activeBatchId, autosaveDelayMs, currentDraftInput, state]);

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

      const target =
        event.target instanceof Element ? event.target : null;
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
