import { useCallback, useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";

import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import { buildSheinStudioSelectionKey } from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import { buildSheinStudioDraftInput } from "@/lib/shein-studio/draft-input";
import { hydrateSDSVariantSelection } from "@/lib/shein-studio/hydrate-sds-selection";
import { buildSheinStudioStepHref } from "@/lib/shein-studio/navigation";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";
import { replaceBrowserHistory } from "@/lib/utils/browser-history";
import { useLiveSearchParams } from "@/lib/utils/live-search-params";
import { saveSheinStudioDraftWithOptions } from "@/lib/utils/shein-studio-batches";

type DraftOverrides = Partial<{
  designs: SheinStudioGeneratedDesign[];
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
  imageStrategy: SheinStudioImageStrategy;
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

function appendDraftSaveWarning(current: string) {
  if (current.includes(DRAFT_SAVE_WARNING)) {
    return current;
  }
  return current ? `${current} ${DRAFT_SAVE_WARNING}` : DRAFT_SAVE_WARNING;
}

function clearDraftSaveWarning(current: string) {
  return current.replace(DRAFT_SAVE_WARNING, "").trim();
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

export function useSheinStudioDraftPersistence(state: WorkbenchDraftState) {
  const autosaveAbortRef = useRef<AbortController | null>(null);
  const autosaveFingerprintRef = useRef("");

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
        selectedSdsImages: state.selectedSdsImages,
        renderSizeImagesWithSds: state.renderSizeImagesWithSds,
        selection: state.activeSelection,
        groupedSelections: state.groupedSelections,
        designs: overrides?.designs ?? state.designs,
        selectedIds: overrides?.selectedIds ?? state.selectedIds,
        createdTasks: overrides?.createdTasks ?? state.createdTasks,
      }),
    [state],
  );

  const persistDraft = useCallback(
    async (overrides?: DraftOverrides, options?: DraftSaveOptions) => {
      try {
        const draft = await saveSheinStudioDraftWithOptions(
          buildDraftInput(overrides),
          options,
        );
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
    [buildDraftInput, state],
  );

  useEffect(() => {
    if (state.isLoadingWorkspace) {
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
      const fingerprint = JSON.stringify(buildDraftInput());
      if (autosaveFingerprintRef.current === fingerprint) {
        return;
      }

      autosaveAbortRef.current?.abort("superseded");
      const controller = new AbortController();
      autosaveAbortRef.current = controller;

      const timeout = window.setTimeout(() => {
        controller.abort("timeout");
      }, 15000);

      void saveSheinStudioDraftWithOptions(buildDraftInput(), {
        signal: controller.signal,
        source: "autosave",
      })
        .then(() => {
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
    }, 1200);

    return () => {
      window.clearTimeout(timer);
    };
  }, [buildDraftInput, state]);

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
