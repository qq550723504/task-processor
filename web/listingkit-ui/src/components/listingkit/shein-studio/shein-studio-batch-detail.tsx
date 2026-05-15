"use client";

import Link from "next/link";
import { useEffect, useMemo, useState } from "react";

import { SheinBatchPublishGate } from "@/components/listingkit/shein-studio/shein-batch-publish-gate";
import { SheinBatchTaskTracker } from "@/components/listingkit/shein-studio/shein-batch-task-tracker";
import { SheinDesignPreviewGrid } from "@/components/listingkit/shein-studio/shein-design-preview-grid";
import { Button } from "@/components/shared/button";
import { formatSubscriptionApiError } from "@/lib/api/subscription";
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
    ? "请先选择 SDS 商品变体。生成 SHEIN 资料前需要商品模板。"
    : approvedCount === 0
      ? "请至少批准 1 个款式后再生成 SHEIN 资料。"
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
          SHEIN 批次
        </p>
        <h1 className="mt-2 font-serif text-3xl tracking-[-0.04em] text-zinc-950">
          正在加载批次
        </h1>
        <p className="mt-3 max-w-2xl text-sm leading-7 text-zinc-600">
          正在从服务器读取已保存批次。
        </p>
      </section>
    );
  }

  if (!batch) {
    return (
      <section className="rounded-[1.75rem] border border-zinc-200/80 bg-white px-6 py-8 shadow-sm">
        <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
          SHEIN 批次
        </p>
        <h1 className="mt-2 font-serif text-3xl tracking-[-0.04em] text-zinc-950">
          未找到批次
        </h1>
        <p className="mt-3 max-w-2xl text-sm leading-7 text-zinc-600">
          这个批次已不在服务器上。请回到工作室重新保存。
        </p>
        <div className="mt-5">
          <Link href="/listing-kits/shein">
            <Button>返回 SHEIN 工作室</Button>
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
    if (currentBatch.selection?.printableWidth) {
      params.set("printWidth", String(currentBatch.selection.printableWidth));
    }
    if (currentBatch.selection?.printableHeight) {
      params.set("printHeight", String(currentBatch.selection.printableHeight));
    }
    if (currentBatch.selection?.selectedVariantIds?.length) {
      params.set("variantIds", currentBatch.selection.selectedVariantIds.join(","));
    }
    const query = params.toString();
    return query ? `/listing-kits/shein?${query}` : "/listing-kits/shein";
  })();

  async function updateBatch(next: Partial<SheinStudioSavedBatch>) {
    const saved = await saveSheinStudioBatch({
      id: currentBatch.id,
      prompt: next.prompt ?? currentBatch.prompt,
      styleCount: next.styleCount ?? currentBatch.styleCount,
      productImageCount: next.productImageCount ?? currentBatch.productImageCount,
      productImagePrompt: next.productImagePrompt ?? currentBatch.productImagePrompt,
      productImagePrompts:
        next.productImagePrompts ?? currentBatch.productImagePrompts,
      artworkModel: next.artworkModel ?? currentBatch.artworkModel,
      transparentBackground:
        next.transparentBackground ?? currentBatch.transparentBackground,
      sheinStoreId: next.sheinStoreId ?? currentBatch.sheinStoreId,
      imageStrategy: next.imageStrategy ?? currentBatch.imageStrategy,
      renderSizeImagesWithSds:
        next.renderSizeImagesWithSds ?? currentBatch.renderSizeImagesWithSds,
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
        `已生成 ${createdTasks.length} 个 SHEIN 资料任务。`,
      );
    } catch (error) {
      setActionError(formatSubscriptionApiError(error));
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
              已保存批次
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
              <Button>返回工作室</Button>
            </Link>
            <Link href={studioHref}>
              <Button tone="secondary">用当前选择打开</Button>
            </Link>
            <Button
              disabled={isCreatingTasks || Boolean(createActionDisabledReason)}
              onClick={handleCreateTasks}
              tone="secondary"
            >
              {isCreatingTasks ? "正在生成 SHEIN 资料..." : "生成 SHEIN 资料"}
            </Button>
            <Button onClick={handleDelete} tone="ghost">
              删除批次
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
              商品
            </div>
            <div className="mt-2 text-sm font-semibold leading-6 text-zinc-950">
              {currentBatch.selection?.productName ?? "未知商品"}
            </div>
          </div>
          <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
            <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
              款式
            </div>
            <div className="mt-2 text-lg font-semibold text-zinc-950">
              共 {currentBatch.designs.length} 个 / 已批准 {approvedCount} 个
            </div>
          </div>
          <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
            <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
              印刷区域
            </div>
            <div className="mt-2 text-lg font-semibold text-zinc-950">
              {currentBatch.selection?.printableWidth &&
              currentBatch.selection?.printableHeight
                ? `${currentBatch.selection.printableWidth} × ${currentBatch.selection.printableHeight}px`
                : "自动"}
            </div>
          </div>
          <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-4">
            <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
              更新时间
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
        imageStrategy={currentBatch.imageStrategy ?? "sds_official"}
        onNoteChange={handleNoteChange}
        onRegenerate={() => {}}
        onCreateReviewTasks={handleCreateTasks}
        onToggle={handleToggle}
        productImageCount={currentBatch.productImageCount ?? "1"}
        createActionDisabledReason={createActionDisabledReason}
        isCreatingTasks={isCreatingTasks}
        createActionLabel="为这个批次生成 SHEIN 资料"
        renderSizeImagesWithSds={currentBatch.renderSizeImagesWithSds ?? true}
        selectedIds={currentBatch.selectedIds}
        selection={currentBatch.selection}
      />
    </div>
  );
}
