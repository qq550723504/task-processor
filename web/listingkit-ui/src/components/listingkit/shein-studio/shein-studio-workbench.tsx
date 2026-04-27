"use client";

import { useEffect, useRef, useState } from "react";

import { SheinDesignPreviewGrid } from "@/components/listingkit/shein-studio/shein-design-preview-grid";
import { SheinStudioGenerationPanel } from "@/components/listingkit/shein-studio/shein-studio-generation-panel";
import { SheinStudioProgressStrip } from "@/components/listingkit/shein-studio/shein-studio-progress-strip";
import { SheinStudioSelectionOverview } from "@/components/listingkit/shein-studio/shein-studio-selection-overview";
import { generateSheinStudioDesigns } from "@/lib/api/shein-studio";
import {
  DEFAULT_SHEIN_STORE_ID,
  createSheinReviewTasks,
  parsePositiveInt,
} from "@/lib/shein-studio/create-review-tasks";
import { buildSDSProductReferenceImageUrls } from "@/lib/shein-studio/sds-reference-images";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import {
  DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
  DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
} from "@/lib/shein-studio/storage-shared";
import type {
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
  saveSheinStudioDraft,
} from "@/lib/utils/shein-studio-batches";

export function SheinStudioWorkbench({
  selection,
}: {
  selection?: SDSProductVariantSelection;
}) {
  const [prompt, setPrompt] = useState("");
  const [styleCount, setStyleCount] = useState("4");
  const [productImageCount, setProductImageCount] = useState(
    DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
  );
  const [productImagePrompt, setProductImagePrompt] = useState("");
  const [productImagePrompts, setProductImagePrompts] = useState<
    SheinStudioProductImagePrompt[]
  >([]);
  const [sheinStoreId, setSheinStoreId] = useState(DEFAULT_SHEIN_STORE_ID);
  const [imageStrategy, setImageStrategy] = useState<SheinStudioImageStrategy>(
    DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY,
  );
  const [designs, setDesigns] = useState<SheinStudioGeneratedDesign[]>([]);
  const [selectedIds, setSelectedIds] = useState<string[]>([]);
  const [generationError, setGenerationError] = useState<string>("");
  const [creatingError, setCreatingError] = useState<string>("");
  const [creatingMessage, setCreatingMessage] = useState<string>("");
  const [isGenerating, setIsGenerating] = useState(false);
  const [isCreatingTasks, setIsCreatingTasks] = useState(false);
  const [regeneratingId, setRegeneratingId] = useState<string>("");
  const [createdTasks, setCreatedTasks] = useState<SheinStudioCreatedTask[]>([]);
  const [savedBatches, setSavedBatches] = useState<SheinStudioSavedBatch[]>([]);
  const [isLoadingWorkspace, setIsLoadingWorkspace] = useState(true);
  const [saveMessage, setSaveMessage] = useState("");
  const promptInputRef = useRef<HTMLTextAreaElement>(null);

  const printableAreaLabel =
    selection?.printableWidth && selection?.printableHeight
      ? `${selection.printableWidth} × ${selection.printableHeight}px`
      : "Auto";
  const selectedVariants =
    selection?.variants?.length
      ? selection.variants
      : selection?.selectedVariantIds?.length
        ? selection.selectedVariantIds.map((variantId) => ({
            variantId,
            size: undefined,
            color: undefined,
          }))
      : selection?.variantId
        ? [
            {
              variantId: selection.variantId,
              size: selection.variantLabel,
              color: "Default",
            },
          ]
        : [];
  const selectedColorCount = new Set(
    selectedVariants.map((variant) => variant.color || "default"),
  ).size;
  const selectedSizeCount = new Set(
    selectedVariants.map((variant) => variant.size || "One size"),
  ).size;
  const createActionDisabledReason = !selection?.variantId
    ? "Select an SDS product variant first. Approved artwork needs a product template before SHEIN data can be generated."
    : selectedIds.length === 0
      ? "Approve at least one generated style before creating SHEIN data."
      : undefined;
  useEffect(() => {
    let cancelled = false;

    async function loadWorkspaceState() {
      setIsLoadingWorkspace(true);
      try {
        const [draft, batches] = await Promise.all([
          loadSheinStudioDraft(selection),
          listSheinStudioBatches(),
        ]);

        if (cancelled) {
          return;
        }

        setPrompt(draft?.prompt ?? "");
        setStyleCount(draft?.styleCount ?? "4");
        setProductImageCount(
          draft?.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
        );
        setProductImagePrompt(draft?.productImagePrompt ?? "");
        setProductImagePrompts(draft?.productImagePrompts ?? []);
        setSheinStoreId(draft?.sheinStoreId || DEFAULT_SHEIN_STORE_ID);
        setImageStrategy(draft?.imageStrategy ?? DEFAULT_SHEIN_STUDIO_IMAGE_STRATEGY);
        setDesigns(draft?.designs ?? []);
        setSelectedIds(draft?.selectedIds ?? []);
        setCreatedTasks(draft?.createdTasks ?? []);
        setSavedBatches(batches);
        setGenerationError("");
        setCreatingError("");
        setCreatingMessage("");
        setSaveMessage("");
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
  }, [selection]);

  useEffect(() => {
    if (isLoadingWorkspace) {
      return;
    }

    const timer = window.setTimeout(() => {
      void saveSheinStudioDraft({
        prompt,
        styleCount,
        productImageCount,
        productImagePrompt,
        productImagePrompts,
        sheinStoreId,
        imageStrategy,
        selection,
        designs,
        selectedIds,
        createdTasks,
      });
    }, 400);

    return () => {
      window.clearTimeout(timer);
    };
  }, [
    createdTasks,
    designs,
    imageStrategy,
    isLoadingWorkspace,
    prompt,
    productImageCount,
    productImagePrompt,
    productImagePrompts,
    selectedIds,
    selection,
    sheinStoreId,
    styleCount,
  ]);

  async function handleGenerate() {
    if (!selection?.variantId) {
      setGenerationError("Select an SDS variant first.");
      return;
    }
    if (!prompt.trim()) {
      setGenerationError("Theme prompt is required.");
      promptInputRef.current?.scrollIntoView({ behavior: "smooth", block: "center" });
      promptInputRef.current?.focus();
      return;
    }

    setGenerationError("");
    setCreatingError("");
    setCreatingMessage("");
    setCreatedTasks([]);
    setIsGenerating(true);

    try {
      const response = await generateSheinStudioDesigns({
        prompt: prompt.trim(),
        count: parsePositiveInt(styleCount) ?? 1,
        printableWidth: selection.printableWidth,
        printableHeight: selection.printableHeight,
        productReferenceImageUrls: buildSDSProductReferenceImageUrls(selection),
      });
      setDesigns(response.images);
      setSelectedIds(response.images.map((item) => item.id));
    } catch (error) {
      setDesigns([]);
      setSelectedIds([]);
      setGenerationError(
        error instanceof Error ? error.message : "Failed to generate styles.",
      );
    } finally {
      setIsGenerating(false);
    }
  }

  async function handleRegenerate(designId: string) {
    if (!selection?.variantId) {
      setGenerationError("Select an SDS variant first.");
      return;
    }
    if (!prompt.trim()) {
      setGenerationError("Theme prompt is required.");
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
        printableWidth: selection.printableWidth,
        printableHeight: selection.printableHeight,
        productReferenceImageUrls: buildSDSProductReferenceImageUrls(selection),
      });
      const replacement = response.images[0];
      if (!replacement) {
        throw new Error("No regenerated design was returned.");
      }

      setDesigns((current) =>
        current.map((design) =>
          design.id === designId ? { ...replacement, id: designId } : design,
        ),
      );
      setSelectedIds((current) =>
        current.includes(designId) ? current : [...current, designId],
      );
    } catch (error) {
      setGenerationError(
        error instanceof Error ? error.message : "Failed to regenerate style.",
      );
    } finally {
      setRegeneratingId("");
    }
  }

  async function handleSaveBatch() {
    if (!prompt.trim()) {
      setSaveMessage("Theme prompt is required before saving a batch.");
      return;
    }

    const saved = await saveSheinStudioBatch({
      prompt,
      styleCount,
      productImageCount,
      productImagePrompt,
      productImagePrompts,
      sheinStoreId,
      imageStrategy,
      selection,
      designs,
      selectedIds,
      createdTasks,
    });

    if (!saved) {
      setSaveMessage("Failed to save batch.");
      return;
    }

    setSavedBatches(await listSheinStudioBatches());
    setSaveMessage(`Batch saved: ${saved.name}`);
  }

  function handleLoadBatch(batch: SheinStudioSavedBatch) {
    setPrompt(batch.prompt);
    setStyleCount(batch.styleCount);
    setProductImageCount(
      batch.productImageCount ?? DEFAULT_SHEIN_STUDIO_PRODUCT_IMAGE_COUNT,
    );
    setProductImagePrompt(batch.productImagePrompt ?? "");
    setProductImagePrompts(batch.productImagePrompts ?? []);
    setSheinStoreId(batch.sheinStoreId);
    setImageStrategy(batch.imageStrategy ?? "sds_official");
    setDesigns(batch.designs);
    setSelectedIds(batch.selectedIds);
    setCreatedTasks(batch.createdTasks);
    setSaveMessage(`Batch loaded: ${batch.name}`);
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
    if (!selection?.variantId) {
      setCreatingError("Select an SDS variant first.");
      return;
    }
    const approved = designs.filter((design) => selectedIds.includes(design.id));
    if (approved.length === 0) {
      setCreatingError("Approve at least one style before creating SHEIN tasks.");
      return;
    }

    setCreatingError("");
    setCreatingMessage("Starting SHEIN data generation...");
    setIsCreatingTasks(true);

    try {
      const created = await createSheinReviewTasks({
        prompt,
        sheinStoreId,
        imageStrategy,
        productImageCount,
        productImagePrompt,
        productImagePrompts,
        selection,
        designs: approved,
        selectedIds: approved.map((design) => design.id),
        onProgress: setCreatingMessage,
      });
      setCreatedTasks(created);
      setCreatingMessage(
        `Generated ${created.length} SHEIN data task${created.length === 1 ? "" : "s"}. Open Review SHEIN data below.`,
      );
    } catch (error) {
      setCreatingError(
        error instanceof Error ? error.message : "Failed to create SHEIN tasks.",
      );
      setCreatingMessage("");
    } finally {
      setIsCreatingTasks(false);
    }
  }

  return (
    <section className="space-y-6">
      <SheinStudioSelectionOverview
        printableAreaLabel={printableAreaLabel}
        selectedColorCount={selectedColorCount}
        selectedSizeCount={selectedSizeCount}
        selectedVariantCount={selectedVariants.length}
        selection={selection}
      />

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
        prompt={prompt}
        promptInputRef={promptInputRef}
        savedBatches={savedBatches}
        saveMessage={saveMessage}
        selectedStyleCount={selectedIds.length}
        selectionReady={Boolean(selection?.variantId)}
        setImageStrategy={setImageStrategy}
        setProductImageCount={setProductImageCount}
        setProductImagePrompt={setProductImagePrompt}
        setProductImagePrompts={setProductImagePrompts}
        setPrompt={setPrompt}
        setSheinStoreId={setSheinStoreId}
        setStyleCount={setStyleCount}
        sheinStoreId={sheinStoreId}
        styleCount={styleCount}
      />

      <div id="shein-style-review" className="scroll-mt-6">
        <SheinStudioProgressStrip
          createdTaskCount={createdTasks.length}
          generatedStyleCount={designs.length}
          selectedStyleCount={selectedIds.length}
        />
        <SheinDesignPreviewGrid
          designs={designs}
          onNoteChange={handleNoteChange}
          onRegenerate={handleRegenerate}
          onToggle={toggleSelection}
          createActionDisabledReason={createActionDisabledReason}
          isCreatingTasks={isCreatingTasks}
          regeneratingId={regeneratingId || undefined}
          selectedIds={selectedIds}
          selection={selection}
        />
      </div>
    </section>
  );
}
