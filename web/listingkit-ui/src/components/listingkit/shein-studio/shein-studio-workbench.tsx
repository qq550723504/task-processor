"use client";

import { useEffect, useRef, useState } from "react";
import { usePathname } from "next/navigation";

import { SheinDesignPreviewGrid } from "@/components/listingkit/shein-studio/shein-design-preview-grid";
import { SheinStudioBusyOverlay } from "@/components/listingkit/shein-studio/shein-studio-busy-overlay";
import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import { SheinStudioProgressStrip } from "@/components/listingkit/shein-studio/shein-studio-progress-strip";
import { SheinStudioSelectionOverview } from "@/components/listingkit/shein-studio/shein-studio-selection-overview";
import { SheinStudioTasksStep } from "@/components/listingkit/shein-studio/shein-studio-tasks-step";
import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import {
  buildSheinStudioSelectionKey,
  buildSheinStudioGenerateRequest,
  evaluateImportedGalleryDesigns,
  getSheinStudioCreateActionDisabledReason,
  mergeSheinStudioDraftState,
  sheinStudioBusyMessage,
  STUDIO_SESSION_SYNC_TIMEOUT_MS,
  summarizeSheinStudioSelection,
} from "@/components/listingkit/shein-studio/shein-studio-workbench-model";
import { generateSheinStudioDesigns } from "@/lib/api/shein-studio";
import {
  ensureSheinStudioSession,
  updateSheinStudioSession,
} from "@/lib/api/shein-studio-sessions";
import {
  DEFAULT_SHEIN_STORE_ID,
  createSheinReviewTasks,
  parsePositiveInt,
} from "@/lib/shein-studio/create-review-tasks";
import { buildSheinStudioDraftInput } from "@/lib/shein-studio/draft-input";
import {
  consumeSheinStudioGalleryHandoff,
  galleryHandoffToDesign,
  type SDSRatioMatch,
} from "@/lib/shein-studio/gallery-handoff";
import { hydrateSDSVariantSelection } from "@/lib/shein-studio/hydrate-sds-selection";
import { buildSheinStudioStepHref } from "@/lib/shein-studio/navigation";
import { buildSDSProductReferenceImageUrls } from "@/lib/shein-studio/sds-reference-images";
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
import { replaceBrowserHistory } from "@/lib/utils/browser-history";
import { useLiveSearchParams } from "@/lib/utils/live-search-params";
import {
  deleteSheinStudioBatch,
  listSheinStudioBatches,
  loadSheinStudioDraft,
  saveSheinStudioBatch,
  saveSheinStudioDraftWithOptions,
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
  const [hydratedSelection, setHydratedSelection] = useState(selection);
  const [savedBatches, setSavedBatches] = useState<SheinStudioSavedBatch[]>([]);
  const [isLoadingWorkspace, setIsLoadingWorkspace] = useState(true);
  const [saveMessage, setSaveMessage] = useState("");
  const [draftWarning, setDraftWarning] = useState("");
  const [effectiveStep, setEffectiveStep] = useState<SheinStudioStepKey>(activeStep);
  const hasLocalWorkflowStateRef = useRef(false);
  const hasCustomizedSdsSelectionRef = useRef(false);
  const autosaveAbortRef = useRef<AbortController | null>(null);
  const autosaveFingerprintRef = useRef("");
  const promptInputRef = useRef<HTMLTextAreaElement>(null);
  const pathname = usePathname();
  const searchParams = useLiveSearchParams();
  const activeSelection = hydratedSelection ?? selection;
  const activeSelectionRef = useRef(activeSelection);
  const activeStepRef = useRef(activeStep);
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

  useEffect(() => {
    setEffectiveStep(activeStep);
    activeStepRef.current = activeStep;
  }, [activeStep]);

  useEffect(() => {
    activeSelectionRef.current = activeSelection;
  }, [activeSelection]);

  useEffect(() => {
    hasLocalWorkflowStateRef.current = false;
    hasCustomizedSdsSelectionRef.current = false;
  }, [selection?.variantId]);

  useEffect(() => {
    let cancelled = false;
    setHydratedSelection(selection);

    void hydrateSDSVariantSelection(selection).then((nextSelection) => {
      if (!cancelled) {
        setHydratedSelection(nextSelection);
      }
    });

    return () => {
      cancelled = true;
    };
  }, [selection]);

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
  }, [activeSelectionKey]);

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
      setSelectedSdsImages(nextDefaults);
    }
  }, [
    availableSdsImages,
    imageStrategy,
    renderSizeImagesWithSds,
    selectedSdsImages,
  ]);

  useEffect(() => {
    if (isLoadingWorkspace) {
      return;
    }
    if (isGenerating || isCreatingTasks || Boolean(regeneratingId)) {
      return;
    }

    const timer = window.setTimeout(() => {
      const fingerprint = JSON.stringify(
        buildSheinStudioDraftInput({
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
          selectedSdsImages,
          renderSizeImagesWithSds,
          selection: activeSelection,
          designs,
          selectedIds,
          createdTasks,
        }),
      );
      if (autosaveFingerprintRef.current === fingerprint) {
        return;
      }

      autosaveAbortRef.current?.abort("superseded");
      const controller = new AbortController();
      autosaveAbortRef.current = controller;

      const timeout = window.setTimeout(() => {
        controller.abort("timeout");
      }, 15000);

      void saveSheinStudioDraftWithOptions(
        buildSheinStudioDraftInput({
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
          selectedSdsImages,
          renderSizeImagesWithSds,
          selection: activeSelection,
          designs,
          selectedIds,
          createdTasks,
        }),
        {
          signal: controller.signal,
          source: "autosave",
        },
      )
        .then(() => {
          setDraftWarning("");
          autosaveFingerprintRef.current = fingerprint;
        })
        .catch((error) => {
          if (controller.signal.aborted) {
            return;
          }
          setDraftWarning(
            "款式图已生成，但草稿保存失败，刷新后可能丢失。可继续审核，或先保存批次。",
          );
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
  }, [
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
    sheinStoreId,
    styleCount,
    variationIntensity,
    transparentBackground,
  ]);

  function buildDraftInput(overrides?: Partial<{
    designs: SheinStudioGeneratedDesign[];
    selectedIds: string[];
    createdTasks: SheinStudioCreatedTask[];
  }>) {
    return buildSheinStudioDraftInput({
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
      selectedSdsImages,
      renderSizeImagesWithSds,
      selection: activeSelection,
      designs: overrides?.designs ?? designs,
      selectedIds: overrides?.selectedIds ?? selectedIds,
      createdTasks: overrides?.createdTasks ?? createdTasks,
    });
  }

  async function persistDraft(
    overrides?: Partial<{
      designs: SheinStudioGeneratedDesign[];
      selectedIds: string[];
      createdTasks: SheinStudioCreatedTask[];
    }>,
    options?: {
      navigationTriggered?: boolean;
      source?: string;
      signal?: AbortSignal;
    },
  ) {
    try {
      const draft = await saveSheinStudioDraftWithOptions(buildDraftInput(overrides), options);
      setDraftWarning("");
      return draft;
    } catch (error) {
      if (options?.signal?.aborted) {
        return null;
      }
      setDraftWarning(
        "款式图已生成，但草稿保存失败，刷新后可能丢失。可继续审核，或先保存批次。",
      );
      throw error;
    }
  }

  function navigateToStep(step: SheinStudioStepKey) {
    setEffectiveStep(step);
    try {
      replaceBrowserHistory(buildSheinStudioStepHref(pathname, searchParams, step));
    } catch (error) {
      console.warn(
        "shein studio step navigation failed",
        error instanceof Error ? error.message : error,
      );
    }
  }

  async function handleGenerate() {
    if (!activeSelection?.variantId) {
      setGenerationError("请先选择 SDS 变体。");
      return;
    }
    if (!prompt.trim()) {
      setGenerationError("请先填写主题提示词。");
      promptInputRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
      promptInputRef.current?.focus();
      return;
    }

    setGenerationError("");
    setCreatingError("");
    setCreatingMessage("");
    setCreatedTasks([]);
    setDraftWarning("");
    setIsGenerating(true);

    const sessionSyncPromise = (async () => {
      try {
        const sessionDetail = await ensureSheinStudioSession(activeSelection, {
          timeoutMs: STUDIO_SESSION_SYNC_TIMEOUT_MS,
        });
        const sessionId = sessionDetail?.session?.id ?? "";
        if (!sessionId) {
          return "";
        }
        await updateSheinStudioSession(
          sessionId,
          {
            status: "generating",
            prompt: prompt.trim(),
            styleCount,
            variationIntensity,
            productImageCount,
            productImagePrompt,
            productImagePrompts,
            artworkModel,
            imageStrategy,
            selectedSdsImages,
            transparentBackground,
            renderSizeImagesWithSds,
            sheinStoreId,
            generationError: "",
            approvedDesignIds: [],
            createdTasks: [],
          },
          {
            timeoutMs: STUDIO_SESSION_SYNC_TIMEOUT_MS,
          },
        );
        return sessionId;
      } catch (error) {
        console.warn(
          "shein studio generation session sync failed",
          error instanceof Error ? error.message : error,
        );
        return "";
      }
    })();

    try {
      const response = await generateSheinStudioDesigns(
        buildSheinStudioGenerateRequest({
          prompt: prompt.trim(),
          variationIntensity,
          printableWidth: activeSelection.printableWidth,
          printableHeight: activeSelection.printableHeight,
          productReferenceImageUrls: buildSDSProductReferenceImageUrls(activeSelection),
          styleCount: parsePositiveInt(styleCount) ?? 1,
          artworkModel,
          transparentBackground,
        }),
        {
          onJobStarted: (jobId) => {
            void sessionSyncPromise
              .then((sessionId) => {
                if (!sessionId) {
                  return;
                }
                return updateSheinStudioSession(
                  sessionId,
                  {
                    status: "generating",
                    generationJobId: jobId,
                    generationError: "",
                  },
                  {
                    timeoutMs: STUDIO_SESSION_SYNC_TIMEOUT_MS,
                  },
                );
              })
              .catch(() => undefined);
          },
        },
      );
      if (!response.images.length) {
        throw new Error(
          "款式图生成完成，但没有返回任何图片。请重试一次；如果持续出现，说明上游生成链路返回了空结果。",
        );
      }
      const nextSelectedIds = response.images.map((item) => item.id);
      console.info("[shein-studio] generation succeeded", {
        designCount: response.images.length,
        draftSaveStatus: "pending",
        selectionVariantId: activeSelection.variantId,
      });
      hasLocalWorkflowStateRef.current = true;
      setDesigns(response.images);
      setSelectedIds(nextSelectedIds);
      navigateToStep("review");
      void persistDraft(
        {
          designs: response.images,
          selectedIds: nextSelectedIds,
          createdTasks: [],
        },
        {
          navigationTriggered: true,
          source: "generate_success",
        },
      ).catch(() => undefined);
    } catch (error) {
      setDesigns([]);
      setSelectedIds([]);
      void sessionSyncPromise
        .then((sessionId) => {
          if (!sessionId) {
            return;
          }
          return updateSheinStudioSession(
            sessionId,
            {
              status: "failed",
              generationError:
                error instanceof Error ? error.message : "款式图生成失败。",
            },
            {
              timeoutMs: STUDIO_SESSION_SYNC_TIMEOUT_MS,
            },
          );
        })
        .catch(() => undefined);
      setGenerationError(error instanceof Error ? error.message : "款式图生成失败。");
    } finally {
      setIsGenerating(false);
    }
  }

  async function handleRegenerate(designId: string) {
    if (!activeSelection?.variantId) {
      setGenerationError("请先选择 SDS 变体。");
      return;
    }
    if (!prompt.trim()) {
      setGenerationError("请先填写主题提示词。");
      promptInputRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
      promptInputRef.current?.focus();
      return;
    }

    setGenerationError("");
    setRegeneratingId(designId);

    try {
      const response = await generateSheinStudioDesigns(buildSheinStudioGenerateRequest({
        prompt: prompt.trim(),
        variationIntensity,
        printableWidth: activeSelection.printableWidth,
        printableHeight: activeSelection.printableHeight,
        productReferenceImageUrls: buildSDSProductReferenceImageUrls(activeSelection),
        styleCount: 1,
        artworkModel,
        transparentBackground,
      }));
      const replacement = response.images[0];
      if (!replacement) {
        throw new Error("重新生成已完成，但没有返回任何图片。");
      }

      hasLocalWorkflowStateRef.current = true;
      setDesigns((current) =>
        current.map((design) =>
          design.id === designId ? { ...replacement, id: designId } : design,
        ),
      );
      const nextSelectedIds = selectedIds.includes(designId)
        ? selectedIds
        : [...selectedIds, designId];
      setSelectedIds(nextSelectedIds);
      void persistDraft(
        {
          designs: designs.map((design) =>
            design.id === designId ? { ...replacement, id: designId } : design,
          ),
          selectedIds: nextSelectedIds,
        },
        {
          source: "regenerate_success",
        },
      ).catch(() => undefined);
    } catch (error) {
      setGenerationError(error instanceof Error ? error.message : "重新生成款式失败。");
    } finally {
      setRegeneratingId("");
    }
  }

  async function handleSaveBatch() {
    if (!prompt.trim()) {
      setSaveMessage("保存批次前请先填写主题提示词。");
      return;
    }

    const saved = await saveSheinStudioBatch({
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
      selectedSdsImages,
      renderSizeImagesWithSds,
      selection: activeSelection,
      designs,
      selectedIds,
      createdTasks,
    });

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

  async function handleCreateTasks() {
    if (!activeSelection?.variantId) {
      setCreatingError("请先选择 SDS 变体。");
      return;
    }
    const approved = designs.filter((design) => selectedIds.includes(design.id));
    if (approved.length === 0) {
      setCreatingError("请至少批准 1 个款式后再创建 SHEIN 任务。");
      return;
    }
    const latestRatioCheck = evaluateImportedGalleryDesigns(approved, activeSelection);
    setGalleryRatioCheck(latestRatioCheck);
    if (latestRatioCheck?.status === "blocking") {
      setCreatingError(latestRatioCheck.message);
      return;
    }

    setCreatingError("");
    setCreatingMessage("正在开始生成 SHEIN 资料...");
    setIsCreatingTasks(true);

    try {
      const created = await createSheinReviewTasks({
        prompt,
        sheinStoreId,
        imageStrategy,
        selectedSdsImages,
        productImageCount,
        productImagePrompt,
        productImagePrompts,
        renderSizeImagesWithSds,
        selection: activeSelection,
        designs: approved,
        selectedIds: approved.map((design) => design.id),
        onProgress: setCreatingMessage,
      });
      hasLocalWorkflowStateRef.current = true;
      setCreatedTasks(created);
      setCreatingMessage(`已生成 ${created.length} 个 SHEIN 资料任务。请在下方打开并审核。`);
      navigateToStep("tasks");
      void persistDraft(
        { createdTasks: created },
        {
          navigationTriggered: true,
          source: "task_creation_success",
        },
      ).catch(() => undefined);
    } catch (error) {
      setCreatingError(error instanceof Error ? error.message : "SHEIN 任务创建失败。");
      setCreatingMessage("");
    } finally {
      setIsCreatingTasks(false);
    }
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

      {draftWarning ? (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900">
          {draftWarning}
        </div>
      ) : null}

      {galleryRatioCheck && galleryRatioCheck.status !== "pass" ? (
        <div
          className={
            galleryRatioCheck.status === "blocking"
              ? "rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm leading-6 text-red-900"
              : "rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-900"
          }
        >
          {galleryRatioCheck.message}
        </div>
      ) : null}

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
        <div id="shein-style-review" className="scroll-mt-6">
          <SheinStudioProgressStrip
            createdTaskCount={createdTasks.length}
            generatedStyleCount={designs.length}
            selectedStyleCount={selectedIds.length}
          />
          <SheinDesignPreviewGrid
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
        </div>
      ) : null}

      {effectiveStep === "tasks" ? (
        <SheinStudioTasksStep createdTasks={createdTasks} />
      ) : null}
    </section>
  );
}
