"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import { SheinBatchPublishGate } from "@/components/listingkit/shein-batch-publish-gate";
import { SheinBatchTaskTracker } from "@/components/listingkit/shein-batch-task-tracker";
import { SheinDesignPreviewGrid } from "@/components/listingkit/shein-design-preview-grid";
import { Button } from "@/components/shared/button";
import { createSheinReviewTasks } from "@/lib/shein-studio/create-review-tasks";
import type { SheinStudioSavedBatch } from "@/lib/types/shein-studio";
import {
  deleteSheinStudioBatch,
  getSheinStudioBatch,
  saveSheinStudioBatch,
} from "@/lib/utils/shein-studio-batches";

export function SheinStudioBatchDetail({ batchId }: { batchId: string }) {
  const [batch, setBatch] = useState<SheinStudioSavedBatch | null>(null);
  const [isLoadingBatch, setIsLoadingBatch] = useState(true);
  const [isCreatingTasks, setIsCreatingTasks] = useState(false);
  const [actionError, setActionError] = useState("");
  const [actionMessage, setActionMessage] = useState("");

  const approvedCount = useMemo(
    () => batch?.selectedIds.length ?? 0,
    [batch?.selectedIds.length],
  );
  const createActionDisabledReason = !batch?.selection?.variantId
    ? "Select an SDS product variant first. Approved artwork needs a product template before SHEIN data can be generated."
    : approvedCount === 0
      ? "Approve at least one generated style before creating SHEIN data."
      : undefined;

  useEffect(() => {
    let cancelled = false;

    async function loadBatch() {
      setIsLoadingBatch(true);
      try {
        const nextBatch = await getSheinStudioBatch(batchId);
        if (!cancelled) {
          setBatch(nextBatch);
        }
      } finally {
        if (!cancelled) {
          setIsLoadingBatch(false);
        }
      }
    }

    void loadBatch();

    return () => {
      cancelled = true;
    };
  }, [batchId]);

  if (isLoadingBatch) {
    return (
      <section className="rounded-[1.75rem] border border-zinc-200/80 bg-white px-6 py-8 shadow-sm">
        <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
          SHEIN batch
        </p>
        <h1 className="mt-2 font-serif text-3xl tracking-[-0.04em] text-zinc-950">
          Loading batch
        </h1>
        <p className="mt-3 max-w-2xl text-sm leading-7 text-zinc-600">
          Fetching the saved batch from server storage.
        </p>
      </section>
    );
  }

  if (!batch) {
    return (
      <section className="rounded-[1.75rem] border border-zinc-200/80 bg-white px-6 py-8 shadow-sm">
        <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
          SHEIN batch
        </p>
        <h1 className="mt-2 font-serif text-3xl tracking-[-0.04em] text-zinc-950">
          Batch not found
        </h1>
        <p className="mt-3 max-w-2xl text-sm leading-7 text-zinc-600">
          This saved batch is no longer on the server. Go back to the studio and save
          a new one.
        </p>
        <div className="mt-5">
          <Link href="/listing-kits/shein">
            <Button>Back to SHEIN Studio</Button>
          </Link>
        </div>
      </section>
    );
  }

  const currentBatch = batch;

  const studioHref = (() => {
    const params = new URLSearchParams();
    if (currentBatch.selection?.productId) {
      params.set("productId", String(currentBatch.selection.productId));
    }
    if (currentBatch.selection?.parentProductId) {
      params.set("parentProductId", String(currentBatch.selection.parentProductId));
    }
    if (currentBatch.selection?.variantId) {
      params.set("variantId", String(currentBatch.selection.variantId));
    }
    if (currentBatch.selection?.prototypeGroupId) {
      params.set("prototypeGroupId", String(currentBatch.selection.prototypeGroupId));
    }
    if (currentBatch.selection?.layerId) {
      params.set("layerId", currentBatch.selection.layerId);
    }
    if (currentBatch.selection?.productName) {
      params.set("productName", currentBatch.selection.productName);
    }
    if (currentBatch.selection?.variantLabel) {
      params.set("variantLabel", currentBatch.selection.variantLabel);
    }
    if (currentBatch.selection?.printableWidth) {
      params.set("printWidth", String(currentBatch.selection.printableWidth));
    }
    if (currentBatch.selection?.printableHeight) {
      params.set("printHeight", String(currentBatch.selection.printableHeight));
    }
    if (currentBatch.selection?.templateImageUrl) {
      params.set("templateImageUrl", currentBatch.selection.templateImageUrl);
    }
    if (currentBatch.selection?.maskImageUrl) {
      params.set("maskImageUrl", currentBatch.selection.maskImageUrl);
    }
    if (currentBatch.selection?.blankDesignUrl) {
      params.set("blankDesignUrl", currentBatch.selection.blankDesignUrl);
    }
    if (currentBatch.selection?.mockupImageUrl) {
      params.set("mockupImageUrl", currentBatch.selection.mockupImageUrl);
    }
    if (currentBatch.selection?.mockupImageUrls?.length) {
      params.set("mockupImageUrls", JSON.stringify(currentBatch.selection.mockupImageUrls));
    }
    const query = params.toString();
    return query ? `/listing-kits/shein?${query}` : "/listing-kits/shein";
  })();

  async function updateBatch(next: Partial<SheinStudioSavedBatch>) {
    const saved = await saveSheinStudioBatch({
      id: currentBatch.id,
      prompt: next.prompt ?? currentBatch.prompt,
      styleCount: next.styleCount ?? currentBatch.styleCount,
      sheinStoreId: next.sheinStoreId ?? currentBatch.sheinStoreId,
      selection: next.selection ?? currentBatch.selection,
      designs: next.designs ?? currentBatch.designs,
      selectedIds: next.selectedIds ?? currentBatch.selectedIds,
      createdTasks: next.createdTasks ?? currentBatch.createdTasks,
    });
    if (saved) {
      setBatch(saved);
    }
  }

  async function handleDelete() {
    await deleteSheinStudioBatch(currentBatch.id);
    setBatch(null);
  }

  function handleToggle(designId: string) {
    const nextSelected = currentBatch.selectedIds.includes(designId)
      ? currentBatch.selectedIds.filter((item) => item !== designId)
      : [...currentBatch.selectedIds, designId];

    void updateBatch({ selectedIds: nextSelected });
    setActionMessage("");
    setActionError("");
  }

  function handleNoteChange(designId: string, note: string) {
    void updateBatch({
      designs: currentBatch.designs.map((design) =>
        design.id === designId ? { ...design, reviewNote: note } : design,
      ),
    });
  }

  async function handleCreateTasks() {
    setActionError("");
    setActionMessage("");
    setIsCreatingTasks(true);

    try {
      const createdTasks = await createSheinReviewTasks({
        prompt: currentBatch.prompt,
        sheinStoreId: currentBatch.sheinStoreId,
        selection: currentBatch.selection,
        designs: currentBatch.designs,
        selectedIds: currentBatch.selectedIds,
        onProgress: setActionMessage,
      });
      await updateBatch({ createdTasks });
      setActionMessage(
        `Generated ${createdTasks.length} SHEIN data task${createdTasks.length === 1 ? "" : "s"}.`,
      );
    } catch (error) {
      setActionError(
        error instanceof Error ? error.message : "Failed to create SHEIN tasks.",
      );
    } finally {
      setIsCreatingTasks(false);
    }
  }

  return (
    <div className="space-y-6">
      <section className="grid gap-6 rounded-[1.75rem] border border-zinc-200/80 bg-white px-6 py-6 shadow-sm lg:grid-cols-[1.15fr_0.85fr]">
        <div className="space-y-4">
          <div>
            <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
              Saved batch
            </p>
            <h1 className="mt-2 font-serif text-4xl tracking-[-0.04em] text-zinc-950">
              {currentBatch.name}
            </h1>
            <p className="mt-3 text-sm leading-7 text-zinc-600">
              {currentBatch.prompt}
            </p>
          </div>

          <div className="flex flex-wrap gap-3">
            <Link href="/listing-kits/shein">
              <Button>Back to studio</Button>
            </Link>
            <Link href={studioHref}>
              <Button tone="secondary">Open with current selection</Button>
            </Link>
            <Button
              disabled={isCreatingTasks || Boolean(createActionDisabledReason)}
              onClick={handleCreateTasks}
              tone="secondary"
            >
              {isCreatingTasks ? "Generating SHEIN data..." : "Generate SHEIN data"}
            </Button>
            <Button onClick={handleDelete} tone="ghost">
              Delete batch
            </Button>
          </div>
          {actionError ? (
            <div className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700">
              {actionError}
            </div>
          ) : null}
          {actionMessage ? (
            <div className="rounded-2xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700">
              {actionMessage}
            </div>
          ) : null}
        </div>

        <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-1">
          <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
            <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
              Product
            </div>
            <div className="mt-2 text-sm font-semibold leading-6 text-zinc-950">
              {currentBatch.selection?.productName ?? "Unknown product"}
            </div>
          </div>
          <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
            <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
              Styles
            </div>
            <div className="mt-2 text-lg font-semibold text-zinc-950">
              {currentBatch.designs.length} total / {approvedCount} approved
            </div>
          </div>
          <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
            <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
              Printable area
            </div>
            <div className="mt-2 text-lg font-semibold text-zinc-950">
              {currentBatch.selection?.printableWidth &&
              currentBatch.selection?.printableHeight
                ? `${currentBatch.selection.printableWidth} × ${currentBatch.selection.printableHeight}px`
                : "Auto"}
            </div>
          </div>
          <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
            <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
              Updated
            </div>
            <div className="mt-2 text-sm font-semibold leading-6 text-zinc-950">
              {new Date(currentBatch.updatedAt).toLocaleString()}
            </div>
          </div>
        </div>
      </section>

      <SheinBatchPublishGate tasks={currentBatch.createdTasks} />
      <SheinBatchTaskTracker tasks={currentBatch.createdTasks} />

      <SheinDesignPreviewGrid
        canRegenerate={false}
        designs={currentBatch.designs}
        onNoteChange={handleNoteChange}
        onRegenerate={() => {}}
        onCreateReviewTasks={handleCreateTasks}
        onToggle={handleToggle}
        createActionDisabledReason={createActionDisabledReason}
        isCreatingTasks={isCreatingTasks}
        createActionLabel="Generate SHEIN data for this batch"
        selectedIds={currentBatch.selectedIds}
        selection={currentBatch.selection}
      />
    </div>
  );
}
