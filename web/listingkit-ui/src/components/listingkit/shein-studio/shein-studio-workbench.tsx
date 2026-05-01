"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

import { SheinDesignPreviewGrid } from "@/components/listingkit/shein-studio/shein-design-preview-grid";
import { SheinCreatedTasksList } from "@/components/listingkit/shein-studio/shein-created-tasks-list";
import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import { SheinStudioProgressStrip } from "@/components/listingkit/shein-studio/shein-studio-progress-strip";
import { SheinStudioSelectionOverview } from "@/components/listingkit/shein-studio/shein-studio-selection-overview";
import type { SheinStudioStepKey } from "@/components/listingkit/shein-studio/shein-studio-step-tabs";
import { generateSheinStudioDesigns } from "@/lib/api/shein-studio";
import {
  DEFAULT_SHEIN_STORE_ID,
  createSheinReviewTasks,
  parsePositiveInt,
} from "@/lib/shein-studio/create-review-tasks";
import { hydrateSDSVariantSelection } from "@/lib/shein-studio/hydrate-sds-selection";
import { buildSDSProductReferenceImageUrls } from "@/lib/shein-studio/sds-reference-images";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import {
  DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL,
  DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
  DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
} from "@/lib/shein-studio/storage-shared";
import { buildSheinStudioDraftInput } from "@/lib/shein-studio/draft-input";
import { buildSheinStudioStepHref } from "@/lib/shein-studio/navigation";
import { resolveSheinStudioEffectiveStep } from "@/lib/shein-studio/workbench-step";
import type {
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";
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
  const [hydratedSelection, setHydratedSelection] = useState(selection);
  const [savedBatches, setSavedBatches] = useState<SheinStudioSavedBatch[]>([]);
  const [isLoadingWorkspace, setIsLoadingWorkspace] = useState(true);
  const [saveMessage, setSaveMessage] = useState("");
  const [draftWarning, setDraftWarning] = useState("");
  const [localStepOverride, setLocalStepOverride] =
    useState<SheinStudioStepKey | null>(null);
  const [localWorkflowSelectionId, setLocalWorkflowSelectionId] = useState(
    selection?.variantId,
  );
  const promptInputRef = useRef<HTMLTextAreaElement>(null);
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useSearchParams();
  const activeSelection =
    hydratedSelection?.variantId === selection?.variantId
      ? hydratedSelection ?? selection
      : selection;
  const effectiveStep = localWorkflowSelectionId === selection?.variantId
    ? localStepOverride ?? activeStep
    : activeStep;

  const printableAreaLabel =
    activeSelection?.printableWidth && activeSelection?.printableHeight
      ? `${activeSelection.printableWidth} × ${activeSelection.printableHeight}px`
      : "自动";
  const selectedVariants =
    activeSelection?.variants?.length
      ? activeSelection.variants
      : activeSelection?.selectedVariantIds?.length
        ? activeSelection.selectedVariantIds.map((variantId) => ({
            variantId,
            size: undefined,
            color: undefined,
          }))
      : activeSelection?.variantId
        ? [
            {
              variantId: activeSelection.variantId,
              size: activeSelection.variantLabel,
              color: "默认",
            },
          ]
        : [];
  const selectedColorCount = new Set(
    selectedVariants.map((variant) => variant.color || "default"),
  ).size;
  const selectedSizeCount = new Set(
    selectedVariants.map((variant) => variant.size || "One size"),
  ).size;
  const createActionDisabledReason = !activeSelection?.variantId
    ? "请先选择 SDS 商品变体。生成 SHEIN 资料前需要锁定商品模板。"
    : selectedIds.length === 0
      ? "请至少批准 1 个款式后再生成 SHEIN 资料。"
      : undefined;

  useEffect(() => {
    let cancelled = false;

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
          loadSheinStudioDraft(activeSelection),
          listSheinStudioBatches(),
        ]);

        if (cancelled) {
          return;
        }

        if (draft || localWorkflowSelectionId !== selection?.variantId) {
          setPrompt(draft?.prompt ?? "");
          setStyleCount(draft?.styleCount ?? "1");
          setProductImageCount(
            draft?.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
          );
          setProductImagePrompt(draft?.productImagePrompt ?? "");
          setProductImagePrompts(draft?.productImagePrompts ?? []);
          setArtworkModel(draft?.artworkModel ?? DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL);
          setTransparentBackground(draft?.transparentBackground ?? false);
          setSheinStoreId(draft?.sheinStoreId || DEFAULT_SHEIN_STORE_ID);
          setImageStrategy(draft?.imageStrategy ?? DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY);
          setRenderSizeImagesWithSds(draft?.renderSizeImagesWithSds ?? true);
          setDesigns(draft?.designs ?? []);
          setSelectedIds(draft?.selectedIds ?? []);
          setCreatedTasks(draft?.createdTasks ?? []);
        }
        setSavedBatches(batches);
        if (draft) {
          setLocalWorkflowSelectionId(selection?.variantId);
          setLocalStepOverride(
            resolveSheinStudioEffectiveStep({
              activeStep,
              createdTaskCount: draft.createdTasks.length,
              designCount: draft.designs.length,
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
  }, [activeSelection, activeStep, localWorkflowSelectionId, selection?.variantId]);

  const buildDraftInput = useCallback(
    (
      overrides?: Partial<{
        designs: SheinStudioGeneratedDesign[];
        selectedIds: string[];
        createdTasks: SheinStudioCreatedTask[];
      }>,
    ) =>
      buildSheinStudioDraftInput({
        prompt,
        styleCount,
        productImageCount,
        productImagePrompt,
        productImagePrompts,
        artworkModel,
        transparentBackground,
        sheinStoreId,
        imageStrategy,
        renderSizeImagesWithSds,
        selection: activeSelection,
        designs: overrides?.designs ?? designs,
        selectedIds: overrides?.selectedIds ?? selectedIds,
        createdTasks: overrides?.createdTasks ?? createdTasks,
      }),
    [
      activeSelection,
      artworkModel,
      createdTasks,
      designs,
      imageStrategy,
      productImageCount,
      productImagePrompt,
      productImagePrompts,
      prompt,
      renderSizeImagesWithSds,
      selectedIds,
      sheinStoreId,
      styleCount,
      transparentBackground,
    ],
  );

  const persistDraft = useCallback(
    async (
      overrides?: Partial<{
        designs: SheinStudioGeneratedDesign[];
        selectedIds: string[];
        createdTasks: SheinStudioCreatedTask[];
      }>,
      options?: {
        navigationTriggered?: boolean;
        source?: string;
      },
    ) => {
      try {
        const draft = await saveSheinStudioDraftWithOptions(
          buildDraftInput(overrides),
          options,
        );
        setDraftWarning("");
        return draft;
      } catch (error) {
        setDraftWarning(
          "款式图已生成，但草稿保存失败，刷新后可能丢失。可继续审核，或先保存批次。",
        );
        throw error;
      }
    },
    [buildDraftInput],
  );

  useEffect(() => {
    if (isLoadingWorkspace) {
      return;
    }
    if (isGenerating || isCreatingTasks || Boolean(regeneratingId)) {
      return;
    }

    const timer = window.setTimeout(() => {
      void persistDraft().catch((error) => {
        console.warn(
          "shein studio draft autosave failed",
          error instanceof Error ? error.message : error,
        );
      });
    }, 1200);

    return () => {
      window.clearTimeout(timer);
    };
  }, [
    isLoadingWorkspace,
    isGenerating,
    isCreatingTasks,
    regeneratingId,
    persistDraft,
  ]);

  function navigateToStep(step: SheinStudioStepKey) {
    setLocalWorkflowSelectionId(selection?.variantId);
    setLocalStepOverride(step);
    try {
      router.replace(buildSheinStudioStepHref(pathname, searchParams, step), {
        scroll: false,
      });
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

    try {
      const response = await generateSheinStudioDesigns({
        prompt: prompt.trim(),
        count: parsePositiveInt(styleCount) ?? 1,
        printableWidth: activeSelection.printableWidth,
        printableHeight: activeSelection.printableHeight,
        productReferenceImageUrls: buildSDSProductReferenceImageUrls(activeSelection),
        imageModel: transparentBackground ? "gpt-image-2" : artworkModel,
        transparentBackground,
      });
      const nextSelectedIds = response.images.map((item) => item.id);
      console.info("[shein-studio] generation succeeded", {
        designCount: response.images.length,
        draftSaveStatus: "pending",
        selectionVariantId: activeSelection.variantId,
      });
      setLocalWorkflowSelectionId(selection?.variantId);
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
      setGenerationError(
        error instanceof Error ? error.message : "款式图生成失败。",
      );
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
      const response = await generateSheinStudioDesigns({
        prompt: prompt.trim(),
        count: 1,
        printableWidth: activeSelection.printableWidth,
        printableHeight: activeSelection.printableHeight,
        productReferenceImageUrls: buildSDSProductReferenceImageUrls(activeSelection),
        imageModel: transparentBackground ? "gpt-image-2" : artworkModel,
        transparentBackground,
      });
      const replacement = response.images[0];
      if (!replacement) {
        throw new Error("没有返回重新生成的图片。");
      }

      setLocalWorkflowSelectionId(selection?.variantId);
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
      setGenerationError(
        error instanceof Error ? error.message : "重新生成款式失败。",
      );
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
      productImageCount,
      productImagePrompt,
      productImagePrompts,
      artworkModel,
      transparentBackground,
      sheinStoreId,
      imageStrategy,
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
    setLocalWorkflowSelectionId(selection?.variantId);
    setPrompt(batch.prompt);
    setStyleCount(batch.styleCount);
    setProductImageCount(
      batch.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    );
    setProductImagePrompt(batch.productImagePrompt ?? "");
    setProductImagePrompts(batch.productImagePrompts ?? []);
    setArtworkModel(batch.artworkModel ?? DEFAULT_SHEIN_STUDIO_ARTWORK_MODEL);
    setTransparentBackground(batch.transparentBackground ?? false);
    setSheinStoreId(batch.sheinStoreId);
    setImageStrategy(batch.imageStrategy ?? "sds_official");
    setRenderSizeImagesWithSds(batch.renderSizeImagesWithSds ?? true);
    setDesigns(batch.designs);
    setSelectedIds(batch.selectedIds);
    setCreatedTasks(batch.createdTasks);
    setLocalStepOverride(
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

    setCreatingError("");
    setCreatingMessage("正在开始生成 SHEIN 资料...");
    setIsCreatingTasks(true);

    try {
      const created = await createSheinReviewTasks({
        prompt,
        sheinStoreId,
        imageStrategy,
        productImageCount,
        productImagePrompt,
        productImagePrompts,
        renderSizeImagesWithSds,
        selection: activeSelection,
        designs: approved,
        selectedIds: approved.map((design) => design.id),
        onProgress: setCreatingMessage,
      });
      setLocalWorkflowSelectionId(selection?.variantId);
      setCreatedTasks(created);
      setCreatingMessage(
        `已生成 ${created.length} 个 SHEIN 资料任务。请在下方打开并审核。`,
      );
      navigateToStep("tasks");
      void persistDraft(
        { createdTasks: created },
        {
          navigationTriggered: true,
          source: "task_creation_success",
        },
      ).catch(() => undefined);
    } catch (error) {
      setCreatingError(
        error instanceof Error ? error.message : "SHEIN 任务创建失败。",
      );
      setCreatingMessage("");
    } finally {
      setIsCreatingTasks(false);
    }
  }

  const busyMessage = isGenerating
    ? "正在生成款式图"
    : regeneratingId
      ? "正在重新生成图片"
      : isCreatingTasks
        ? "正在生成商品图和 SHEIN 资料"
        : "";

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

      {effectiveStep === "generate" ? (
        <SheinStudioGenerationPanel
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
          artworkModel={artworkModel}
          transparentBackground={transparentBackground}
          renderSizeImagesWithSds={renderSizeImagesWithSds}
          prompt={prompt}
          promptInputRef={promptInputRef}
          savedBatches={savedBatches}
          saveMessage={saveMessage}
          selectedStyleCount={selectedIds.length}
          selectionReady={Boolean(activeSelection?.variantId)}
          setArtworkModel={setArtworkModel}
          setImageStrategy={setImageStrategy}
          setProductImageCount={setProductImageCount}
          setProductImagePrompt={setProductImagePrompt}
          setProductImagePrompts={setProductImagePrompts}
          setPrompt={setPrompt}
          setRenderSizeImagesWithSds={setRenderSizeImagesWithSds}
          setSheinStoreId={setSheinStoreId}
          setStyleCount={setStyleCount}
          setTransparentBackground={setTransparentBackground}
          sheinStoreId={sheinStoreId}
          styleCount={styleCount}
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
          designs={designs}
          onNoteChange={handleNoteChange}
          onCreateReviewTasks={handleCreateTasks}
          onRegenerate={handleRegenerate}
          onToggle={toggleSelection}
          createActionDisabledReason={createActionDisabledReason}
          isCreatingTasks={isCreatingTasks}
          regeneratingId={regeneratingId || undefined}
          selectedIds={selectedIds}
          selection={activeSelection}
        />
      </div>
      ) : null}

      {effectiveStep === "tasks" ? (
        <div
          id="shein-created-tasks"
          className="scroll-mt-6 rounded-[1.75rem] border border-zinc-200/80 bg-white p-5 shadow-sm"
        >
          <div className="mb-4 flex flex-wrap items-start justify-between gap-3">
            <div>
              <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
                第 4 步 · SHEIN 任务
              </p>
              <h2 className="mt-1 font-serif text-2xl tracking-[-0.03em] text-zinc-950">
                审核已生成的工作区
              </h2>
              <p className="mt-1 max-w-2xl text-sm leading-6 text-zinc-600">
                打开每个任务的工作区，完成最终图片、价格、属性和提交确认。
              </p>
            </div>
            <span className="rounded-full bg-zinc-100 px-3 py-1 text-xs font-semibold text-zinc-600">
              {createdTasks.length} 个任务
            </span>
          </div>
          {createdTasks.length ? (
            <SheinCreatedTasksList tasks={createdTasks} />
          ) : (
            <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm leading-6 text-amber-900">
              还没有创建 SHEIN 任务。先回到“审核款式”步骤批准款式，再在“生成图片”
              步骤点击“生成 SHEIN 资料”。
            </div>
          )}
        </div>
      ) : null}
    </section>
  );
}

function SheinStudioBusyOverlay({ message }: { message: string }) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/45 px-4 backdrop-blur-sm">
      <div className="w-full max-w-md rounded-[2rem] border border-white/20 bg-white px-6 py-6 text-center shadow-2xl">
        <div className="mx-auto h-12 w-12 animate-spin rounded-full border-4 border-zinc-200 border-t-zinc-950" />
        <h3 className="mt-5 text-lg font-semibold text-zinc-950">{message}</h3>
        <p className="mt-2 text-sm leading-6 text-zinc-600">
          图片生成耗时较长，通常需要 1-3 分钟。请不要刷新页面或重复点击，完成后界面会自动更新。
        </p>
        <div className="mt-4 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-xs leading-5 text-amber-900">
          当前已锁定操作，避免重复提交导致多次扣费或生成重复任务。
        </div>
      </div>
    </div>
  );
}
