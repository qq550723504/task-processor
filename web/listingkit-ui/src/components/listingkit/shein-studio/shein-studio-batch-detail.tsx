"use client";

import Link from "next/link";
import { useRouter } from "next/navigation";
import { useEffect, useMemo, useState } from "react";

import { SheinBatchPublishGate } from "@/components/listingkit/shein-studio/shein-batch-publish-gate";
import { SheinBatchTaskTracker } from "@/components/listingkit/shein-studio/shein-batch-task-tracker";
import { SheinDesignPreviewGrid } from "@/components/listingkit/shein-studio/shein-design-preview-grid";
import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import {
  approveSheinStudioBatchDesigns,
  createSheinStudioBatchTasks,
} from "@/lib/api/shein-studio-batches";
import { ApiError } from "@/lib/api/client";
import { formatSubscriptionApiError } from "@/lib/api/subscription";
import { buildGroupedGenerationTargets } from "@/lib/shein-studio/grouped-image-mode";
import type {
  SheinStudioBatchDetail as SheinStudioItemizedBatchDetail,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";
import {
  deleteSheinStudioBatch,
  flattenSheinStudioBatchDetailDesigns,
  getApprovedSheinStudioBatchDesignIDs,
  getSheinStudioHydratedBatch,
  saveSheinStudioBatch,
  setActiveSheinStudioBatchId,
  updateSheinStudioBatchDetailReviewNote,
} from "@/lib/utils/shein-studio-batches";

export function SheinStudioBatchDetail({ batchId }: { batchId: string }) {
  const router = useRouter();
  const [batch, setBatch] = useState<SheinStudioSavedBatch | null>(null);
  const [detail, setDetail] = useState<SheinStudioItemizedBatchDetail | null>(null);
  const [isLoadingBatch, setIsLoadingBatch] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [isCreatingTasks, setIsCreatingTasks] = useState(false);
  const [actionError, setActionError] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const [reloadToken, setReloadToken] = useState(0);
  const currentDesigns = useMemo(
    () => (detail ? flattenSheinStudioBatchDetailDesigns(detail) : []),
    [detail],
  );
  const currentSelectedIDs = useMemo(
    () => (detail ? getApprovedSheinStudioBatchDesignIDs(detail) : []),
    [detail],
  );

  const approvedCount = useMemo(
    () => currentSelectedIDs.length,
    [currentSelectedIDs.length],
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
      setLoadError("");
      try {
        const nextBatch = await getSheinStudioHydratedBatch(batchId);
        if (!cancelled) {
          setBatch(nextBatch.savedBatch);
          setDetail(nextBatch.detail);
        }
      } catch (error) {
        if (!cancelled) {
          if (error instanceof ApiError && error.status === 404) {
            setBatch(null);
            setDetail(null);
            return;
          }
          setLoadError(formatSubscriptionApiError(error));
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
  }, [batchId, reloadToken]);

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

  if (!batch || !detail) {
    return (
      <section className="rounded-[1.75rem] border border-zinc-200/80 bg-white px-6 py-8 shadow-sm">
        <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
          SHEIN 批次
        </p>
        <h1 className="mt-2 font-serif text-3xl tracking-[-0.04em] text-zinc-950">
          {loadError ? "批次加载失败" : "未找到批次"}
        </h1>
        <p className="mt-3 max-w-2xl text-sm leading-7 text-zinc-600">
          {loadError
            ? "服务器没有返回这个批次的可用数据。请先重试；如果持续失败，再回到工作室重新打开。"
            : "这个批次已不在服务器上。请回到工作室重新保存。"}
        </p>
        {loadError ? (
          <Alert className="mt-5" variant="destructive">
            <AlertDescription>{loadError}</AlertDescription>
          </Alert>
        ) : null}
        <div className="mt-5">
          <div className="flex flex-wrap gap-3">
            {loadError ? (
              <Button onClick={() => setReloadToken((value) => value + 1)}>
                重试加载
              </Button>
            ) : null}
            <Link href="/listing-kits/sds">
              <Button variant={loadError ? "secondary" : "default"}>
                返回 POD 工作室
              </Button>
            </Link>
          </div>
        </div>
      </section>
    );
  }

  const currentBatch = batch;
  const currentDetail = detail;

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
    return query ? `/listing-kits/sds?${query}` : "/listing-kits/sds";
  })();

  async function updateBatchContext(
    next: Partial<SheinStudioSavedBatch>,
    nextDetail: SheinStudioItemizedBatchDetail = currentDetail,
  ) {
    const saved = await saveSheinStudioBatch({
      id: currentBatch.id,
      updatedAt: currentBatch.updatedAt,
      name: next.name ?? currentBatch.name,
      prompt: next.prompt ?? nextDetail.batch.prompt,
      styleCount: next.styleCount ?? nextDetail.batch.styleCount,
      variationIntensity:
        next.variationIntensity ?? currentBatch.variationIntensity,
      productImageCount: next.productImageCount ?? currentBatch.productImageCount,
      productImagePrompt: next.productImagePrompt ?? currentBatch.productImagePrompt,
      productImagePrompts:
        next.productImagePrompts ?? currentBatch.productImagePrompts,
      artworkModel: next.artworkModel ?? currentBatch.artworkModel,
      transparentBackground:
        next.transparentBackground ?? currentBatch.transparentBackground,
      sheinStoreId:
        next.sheinStoreId ??
        currentBatch.sheinStoreId ??
        (nextDetail.batch.sheinStoreId > 0
          ? String(nextDetail.batch.sheinStoreId)
          : ""),
      imageStrategy: next.imageStrategy ?? currentBatch.imageStrategy,
      groupedImageMode: next.groupedImageMode ?? currentBatch.groupedImageMode,
      selectedSdsImages: next.selectedSdsImages ?? currentBatch.selectedSdsImages,
      renderSizeImagesWithSds:
        next.renderSizeImagesWithSds ?? currentBatch.renderSizeImagesWithSds,
      selection: next.selection ?? currentBatch.selection,
      groupedSelections: next.groupedSelections ?? currentBatch.groupedSelections,
      groups: next.groups ?? currentBatch.groups,
      designs: next.designs ?? flattenSheinStudioBatchDetailDesigns(nextDetail),
      selectedIds:
        next.selectedIds ?? getApprovedSheinStudioBatchDesignIDs(nextDetail),
      createdTasks: next.createdTasks ?? currentBatch.createdTasks,
      generationJobs: next.generationJobs ?? currentBatch.generationJobs,
    });
    if (saved) {
      setBatch(saved);
      return;
    }
    setBatch((previous) =>
      previous
        ? {
            ...previous,
            ...next,
            prompt: next.prompt ?? nextDetail.batch.prompt,
            styleCount: next.styleCount ?? nextDetail.batch.styleCount,
            designs: next.designs ?? flattenSheinStudioBatchDetailDesigns(nextDetail),
            selectedIds:
              next.selectedIds ?? getApprovedSheinStudioBatchDesignIDs(nextDetail),
            updatedAt: nextDetail.batch.updatedAt,
          }
        : previous,
    );
  }

  async function handleDelete() {
    await deleteSheinStudioBatch(currentBatch.id);
    setBatch(null);
  }

  function handleContinueSelecting() {
    setActiveSheinStudioBatchId(currentBatch.id);
    router.push(studioHref);
  }

  function handleToggle(designId: string) {
    const nextSelected = currentSelectedIDs.includes(designId)
      ? currentSelectedIDs.filter((item) => item !== designId)
      : [...currentSelectedIDs, designId];

    setActionMessage("");
    setActionError("");
    void (async () => {
      try {
        const nextDetail = await approveSheinStudioBatchDesigns(
          currentBatch.id,
          nextSelected,
        );
        setDetail(nextDetail);
        await updateBatchContext({ selectedIds: nextSelected }, nextDetail);
      } catch (error) {
        setActionError(formatSubscriptionApiError(error));
      }
    })();
  }

  function handleNoteChange(designId: string, note: string) {
    const nextDetail = updateSheinStudioBatchDetailReviewNote(
      currentDetail,
      designId,
      note,
    );
    setDetail(nextDetail);
    void updateBatchContext(
      { designs: flattenSheinStudioBatchDetailDesigns(nextDetail) },
      nextDetail,
    );
  }

  const selectionByTargetGroupKey = new Map(
    buildGroupedGenerationTargets({
      activeSelection: currentBatch.selection,
      groupedSelections: (currentBatch.groupedSelections ?? []).map(
        (item) => item.selection,
      ),
      groupedImageMode: currentBatch.groupedImageMode ?? "shared_by_size",
    }).map((target) => [target.key, target.selection] as const),
  );

  async function handleCreateTasks() {
    setActionError("");
    setActionMessage("");
    setIsCreatingTasks(true);

    try {
      const result = await createSheinStudioBatchTasks(
        currentBatch.id,
        currentSelectedIDs,
      );
      setDetail({
        batch: result.batch,
        items: result.items,
      });
      await updateBatchContext(
        { createdTasks: result.createdTasks },
        {
          batch: result.batch,
          items: result.items,
        },
      );
      setActionMessage(
        `已生成 ${result.createdTasks.length} 个 SHEIN 资料任务。`,
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
            <Link href="/listing-kits/sds">
              <Button>返回工作室</Button>
            </Link>
            <Button onClick={handleContinueSelecting} variant="secondary">
              继续选品并加入当前批次
            </Button>
            <Button
              disabled={isCreatingTasks || Boolean(createActionDisabledReason)}
              onClick={handleCreateTasks}
              variant="secondary"
            >
              {isCreatingTasks ? "正在生成 SHEIN 资料..." : "生成 SHEIN 资料"}
            </Button>
            <Button onClick={handleDelete} variant="ghost">
              删除批次
            </Button>
          </div>
          {actionError ? (
            <Alert variant="destructive">
              <AlertDescription>{actionError}</AlertDescription>
            </Alert>
          ) : null}
          {actionMessage ? (
            <Alert variant="success">
              <AlertDescription>{actionMessage}</AlertDescription>
            </Alert>
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
              共 {currentDesigns.length} 个 / 已批准 {approvedCount} 个
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
        designs={currentDesigns}
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
        selectedIds={currentSelectedIDs}
        selection={currentBatch.selection}
        selectionByTargetGroupKey={selectionByTargetGroupKey}
      />
    </div>
  );
}
