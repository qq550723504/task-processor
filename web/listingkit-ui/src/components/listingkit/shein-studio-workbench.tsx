"use client";

import { useEffect, useState } from "react";

import { SheinCreatedTasksList } from "@/components/listingkit/shein-created-tasks-list";
import { Button } from "@/components/shared/button";
import { SheinDesignPreviewGrid } from "@/components/listingkit/shein-design-preview-grid";
import { SheinSavedBatchesPanel } from "@/components/listingkit/shein-saved-batches-panel";
import { generateSheinStudioDesigns } from "@/lib/api/shein-studio";
import {
  createSheinReviewTasks,
  parsePositiveInt,
} from "@/lib/shein-studio/create-review-tasks";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type {
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
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
  const [sheinStoreId, setSheinStoreId] = useState("");
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

  const printableAreaLabel =
    selection?.printableWidth && selection?.printableHeight
      ? `${selection.printableWidth} × ${selection.printableHeight}px`
      : "Auto";
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
        setSheinStoreId(draft?.sheinStoreId ?? "");
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
        sheinStoreId,
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
    isLoadingWorkspace,
    prompt,
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
      sheinStoreId,
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
    setSheinStoreId(batch.sheinStoreId);
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
      <div className="grid gap-6 lg:grid-cols-[0.78fr_1.22fr]">
        <div className="space-y-4 rounded-[1.75rem] border border-zinc-200/80 bg-white px-5 py-5 shadow-sm">
          <div className="space-y-2">
            <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
              SHEIN Studio
            </p>
            <h2 className="font-serif text-3xl leading-tight tracking-[-0.04em] text-zinc-950">
              Generate multiple design styles from one prompt.
            </h2>
            <p className="text-sm leading-7 text-zinc-600">
              Choose an SDS variant first, then generate a batch of print graphics.
              Approved styles will be converted into normal SHEIN review tasks.
            </p>
          </div>

          <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-1">
            <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
              <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
                Variant
              </div>
              <div className="mt-2 text-lg font-semibold text-zinc-950">
                {selection?.variantId ?? "Not selected"}
              </div>
            </div>
            <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
              <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
                Printable area
              </div>
              <div className="mt-2 text-lg font-semibold text-zinc-950">
                {printableAreaLabel}
              </div>
            </div>
            <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
              <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
                Product
              </div>
              <div className="mt-2 text-sm font-semibold leading-6 text-zinc-950">
                {selection?.productName ?? "Choose a product first"}
              </div>
            </div>
          </div>
        </div>

        <div className="space-y-4 rounded-[1.75rem] border border-zinc-200/80 bg-white px-5 py-5 shadow-sm">
          <div className="grid gap-4 md:grid-cols-2">
            <label className="space-y-2">
              <span className="text-sm font-medium text-zinc-700">Theme prompt</span>
              <textarea
                className="min-h-32 w-full rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 focus:bg-white"
                onChange={(event) => setPrompt(event.target.value)}
                placeholder="Retro varsity cherries, bold collegiate typography, spring palette."
                value={prompt}
              />
              <p className="text-xs leading-6 text-zinc-500">
                Studio will automatically bias generation toward print-safe graphics:
                larger shapes, cleaner contrast, and fewer tiny details or thin lines.
              </p>
            </label>

            <div className="grid gap-4 content-start">
              <label className="space-y-2">
                <span className="text-sm font-medium text-zinc-700">
                  Style count
                </span>
                <input
                  className="w-full rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 focus:bg-white"
                  inputMode="numeric"
                  max={8}
                  min={1}
                  onChange={(event) => setStyleCount(event.target.value)}
                  value={styleCount}
                />
              </label>

              <label className="space-y-2">
                <span className="text-sm font-medium text-zinc-700">
                  SHEIN store ID
                </span>
                <input
                  className="w-full rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 focus:bg-white"
                  inputMode="numeric"
                  onChange={(event) => setSheinStoreId(event.target.value)}
                  placeholder="Optional"
                  value={sheinStoreId}
                />
              </label>

              <div className="rounded-[1.25rem] border border-dashed border-zinc-200 bg-zinc-50 px-4 py-4 text-sm leading-7 text-zinc-500">
                Generated styles are flat print graphics. The right-hand preview uses
                the SDS template surface only for operator review.
              </div>
            </div>
          </div>

          {generationError ? (
            <div className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
              {generationError}
            </div>
          ) : null}
          {creatingError ? (
            <div className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
              {creatingError}
            </div>
          ) : null}
          {creatingMessage ? (
            <div className="rounded-2xl border border-sky-200 bg-sky-50 px-4 py-3 text-sm text-sky-800">
              {creatingMessage}
            </div>
          ) : null}

          <div className="flex flex-wrap gap-3">
            <Button disabled={isGenerating} onClick={handleGenerate}>
              {isGenerating ? "Generating..." : "Generate styles"}
            </Button>
            <Button onClick={handleSaveBatch} tone="ghost">
              Save batch
            </Button>
            <Button
              disabled={
                isCreatingTasks || selectedIds.length === 0 || !selection?.variantId
              }
              onClick={handleCreateTasks}
              tone="secondary"
            >
              {isCreatingTasks
                ? "Generating SHEIN data..."
                : "Generate SHEIN data"}
            </Button>
          </div>
          {selectedIds.length > 0 ? (
            <div className="rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">
              {selectedIds.length} style{selectedIds.length > 1 ? "s" : ""} selected for
              SHEIN review.
            </div>
          ) : null}
          {saveMessage ? (
            <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-600">
              {saveMessage}
            </div>
          ) : null}

          <SheinCreatedTasksList tasks={createdTasks} />
          <SheinSavedBatchesPanel
            batches={savedBatches}
            onDelete={handleDeleteBatch}
            onLoad={handleLoadBatch}
          />
        </div>
      </div>

      <SheinDesignPreviewGrid
        designs={designs}
        onNoteChange={handleNoteChange}
        onRegenerate={handleRegenerate}
        onCreateReviewTasks={handleCreateTasks}
        onToggle={toggleSelection}
        createActionDisabledReason={createActionDisabledReason}
        isCreatingTasks={isCreatingTasks}
        regeneratingId={regeneratingId || undefined}
        selectedIds={selectedIds}
        selection={selection}
      />
    </section>
  );
}
