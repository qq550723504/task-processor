"use client";

import { useEffect, useMemo, useState } from "react";
import { usePathname, useRouter } from "next/navigation";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Select } from "@/components/ui/select";
import { SDSPagination } from "@/components/listingkit/sds/sds-pagination";
import { SDSProductCard } from "@/components/listingkit/sds/sds-product-card";
import { SDSProductBrowserFilters } from "@/components/listingkit/sds/sds-product-browser-filters";
import { SDSRecentVariants } from "@/components/listingkit/sds/sds-recent-variants";
import { SDSSelectionSummary } from "@/components/listingkit/sds/sds-selection-summary";
import { SDSVariantPicker } from "@/components/listingkit/sds/sds-variant-picker";
import {
  getSDSBaselineReadiness,
  warmSDSBaselineForSelection,
} from "@/lib/api/sds-baseline";
import { useSDSCategories } from "@/lib/query/use-sds-categories";
import { useSDSProductDetail } from "@/lib/query/use-sds-product-detail";
import { useSDSProducts } from "@/lib/query/use-sds-products";
import { useSDSRecentVariants } from "@/lib/query/use-sds-recent-variants";
import { useSDSShipmentAreas } from "@/lib/query/use-sds-shipment-areas";
import { buildSDSVariantSelection } from "@/lib/sds/variant-selection";
import type { SDSProductVariant, SDSProductVariantSelection } from "@/lib/types/sds";
import {
  buildGroupedSDSSelectionID,
} from "@/lib/types/sds-baseline";
import {
  buildGroupedSDSBaselineHandoff,
  getSDSBaselineReasonMessage,
} from "@/lib/shein-studio/sds-baseline-ui";
import { replaceBrowserHistory } from "@/lib/utils/browser-history";
import { useLiveSearchParams } from "@/lib/utils/live-search-params";
import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";
import {
  getSheinStudioBatch,
  getActiveSheinStudioBatchId,
  listSheinStudioBatches,
  saveSheinStudioBatch,
  setActiveSheinStudioBatchId,
} from "@/lib/utils/shein-studio-batches";
import { saveSDSGroupedCandidateHandoff } from "@/lib/utils/sds-grouped-candidate-handoff";
import { saveRecentSDSVariant } from "@/lib/utils/sds-recent-variants";

export function SDSProductBrowser({
  initialKeyword = "",
  initialPage = 1,
  initialShipmentArea = "US",
}: {
  initialKeyword?: string;
  initialPage?: number;
  initialShipmentArea?: string;
}) {
  const pathname = usePathname();
  const router = useRouter();
  const searchParams = useLiveSearchParams();
  const recentVariants = useSDSRecentVariants();
  const [pickerProductId, setPickerProductId] = useState<number | undefined>();
  const [addedToTargetBatchCount, setAddedToTargetBatchCount] = useState(0);
  const [isAddingToTargetBatch, setIsAddingToTargetBatch] = useState(false);
  const [isWarmingBlockedBaseline, setIsWarmingBlockedBaseline] = useState(false);
  const [targetBatchFeedback, setTargetBatchFeedback] = useState("");
  const [targetBatchError, setTargetBatchError] = useState("");
  const [blockedBaselineSelection, setBlockedBaselineSelection] =
    useState<SDSProductVariantSelection | null>(null);
  const [recentBatches, setRecentBatches] = useState<
    Array<{ id: string; name: string }>
  >([]);
  const [activeBatchId, setActiveBatchId] = useState("");

  const queryKeyword = searchParams.get("keyword") ?? initialKeyword;
  const currentPage = Number(searchParams.get("page") ?? initialPage) || 1;
  const shipmentArea = searchParams.get("shipmentArea") ?? initialShipmentArea;
  const categoryId = Number(searchParams.get("categoryId") ?? 0) || undefined;
  const onSaleOnly = searchParams.get("onSaleStatus") === "2";
  const hotSellOnly = searchParams.get("hotSellStatus") === "1";
  const sortValue = searchParams.get("sort") ?? "";
  const weightBand = searchParams.get("weightBand") ?? "";
  const cycleBand = searchParams.get("cycleBand") ?? "";
  const [sortField, sortType] = sortValue ? sortValue.split(":") : ["", ""];
  const selectedProductId = Number(searchParams.get("productId") ?? 0);
  const selectedVariantId = Number(searchParams.get("variantId") ?? 0);
  const selectedPrintableWidth = Number(searchParams.get("printWidth") ?? 0) || undefined;
  const selectedPrintableHeight = Number(searchParams.get("printHeight") ?? 0) || undefined;
  const selectedTemplateImageUrl = searchParams.get("templateImageUrl") ?? undefined;
  const selectedMaskImageUrl = searchParams.get("maskImageUrl") ?? undefined;
  const selectedBlankDesignUrl = searchParams.get("blankDesignUrl") ?? undefined;
  const selectedMockupImageUrl = searchParams.get("mockupImageUrl") ?? undefined;
  const targetBatchId = searchParams.get("targetBatchId") ?? "";
  const shipmentAreas = useSDSShipmentAreas();
  const categories = useSDSCategories(shipmentArea);
  const products = useSDSProducts({
    keyword: queryKeyword,
    page: currentPage,
    size: 12,
    shipmentArea,
    categoryId,
    onSaleStatus: onSaleOnly ? 2 : undefined,
    hotSellStatus: hotSellOnly ? 1 : undefined,
    sortField: sortField || undefined,
    sortType: sortType || undefined,
    weightBand: weightBand || undefined,
    cycleBand: cycleBand || undefined,
  });
  const detail = useSDSProductDetail(pickerProductId);

  const variants = useMemo(
    () => detail.data?.subproducts?.items ?? [],
    [detail.data?.subproducts?.items],
  );
  const pageCount = useMemo(() => {
    const totalCount = products.data?.totalCount ?? 0;
    const pageSize = products.data?.size ?? 12;
    return Math.max(1, Math.ceil(totalCount / pageSize));
  }, [products.data?.size, products.data?.totalCount]);
  const availableShipmentAreas = shipmentAreas.data ?? [];
  const availableCategories = categories.data ?? [];
  const activeShipmentAreaLabel =
    availableShipmentAreas.find((item) => item.value === shipmentArea)?.label ?? shipmentArea;
  const activeCategoryLabel =
    availableCategories.find((item) => item.id === categoryId)?.name ?? "全部分类";
  const currentSelection = useMemo(
    () => recentVariants.find((item) => item.variantId === selectedVariantId),
    [recentVariants, selectedVariantId],
  );
  const activeBatchLabel = useMemo(
    () => recentBatches.find((batch) => batch.id === activeBatchId)?.name ?? "",
    [activeBatchId, recentBatches],
  );
  const effectiveTargetBatchId = targetBatchId || activeBatchId;
  const isTargetingExistingBatchFlow =
    pathname === "/listing-kits/sds/new" && Boolean(targetBatchId);

  function setCurrentTargetBatch(batchId: string) {
    setActiveSheinStudioBatchId(batchId);
    setActiveBatchId(batchId);
  }

  async function refreshRecentBatches(options?: { preserveCurrentOnError?: boolean }) {
    try {
      const items = await listSheinStudioBatches();
      const nextBatches = items.map((item) => ({
        id: item.id,
        name: item.name,
      }));
      setRecentBatches(nextBatches);
      if (
        activeBatchId &&
        !nextBatches.some((batch) => batch.id === activeBatchId)
      ) {
        setCurrentTargetBatch("");
      }
    } catch {
      if (!options?.preserveCurrentOnError) {
        setRecentBatches([]);
      }
    }
  }

  useEffect(() => {
    let cancelled = false;
    void listSheinStudioBatches()
      .then((items) => {
        if (cancelled) {
          return;
        }
        setRecentBatches(
          items.map((item) => ({
            id: item.id,
            name: item.name,
          })),
        );
      })
      .catch(() => {
        if (cancelled) {
          return;
        }
        setRecentBatches([]);
      })
      .finally(() => {
        if (!cancelled) {
          setActiveBatchId(targetBatchId || getActiveSheinStudioBatchId());
        }
      });
    return () => {
      cancelled = true;
    };
  }, [targetBatchId]);

  useEffect(() => {
    setAddedToTargetBatchCount(0);
    setTargetBatchFeedback("");
    setTargetBatchError("");
    setBlockedBaselineSelection(null);
  }, [targetBatchId]);

  function updateQuery(next: Record<string, string | undefined>) {
    const params = sanitizedNavigationSearchParams(searchParams);
    Object.entries(next).forEach(([key, value]) => {
      if (!value) {
        params.delete(key);
        return;
      }
      params.set(key, value);
    });
    const suffix = params.toString();
    replaceBrowserHistory(suffix ? `${pathname}?${suffix}` : pathname);
  }

  function applySelection(selection: SDSProductVariantSelection) {
    saveRecentSDSVariant(selection);
    if (isTargetingExistingBatchFlow && effectiveTargetBatchId) {
      void handleAddCandidateToBatch(selection, effectiveTargetBatchId, {
        stayOnPageAfterAdd: true,
      });
      return;
    }
    if (pathname === "/listing-kits/sds/new") {
      void handleCreateBatchFromSelection(selection);
      return;
    }
    updateQuery({
      productId: String(selection.productId),
      variantId: String(selection.variantId),
      parentProductId: String(selection.parentProductId),
      prototypeGroupId: String(selection.prototypeGroupId),
      layerId: selection.layerId,
      printWidth: selection.printableWidth ? String(selection.printableWidth) : undefined,
      printHeight: selection.printableHeight ? String(selection.printableHeight) : undefined,
      templateImageUrl: undefined,
      maskImageUrl: undefined,
      blankDesignUrl: undefined,
      mockupImageUrl: undefined,
      mockupImageUrls: undefined,
      variantIds:
        selection.selectedVariantIds && selection.selectedVariantIds.length > 0
          ? selection.selectedVariantIds.join(",")
          : selection.variants?.map((variant) => variant.variantId).join(","),
      productName: undefined,
      variantLabel: undefined,
      step: "generate",
    });
    setPickerProductId(undefined);
    window.setTimeout(() => {
      document
        .getElementById("shein-studio-generator")
        ?.scrollIntoView({ behavior: "smooth", block: "start" });
    }, 100);
  }

  function applyVariants(primary: SDSProductVariant, selectedVariants: SDSProductVariant[]) {
    applySelection(buildSDSVariantSelection(detail.data, primary, selectedVariants));
  }

  function openVariantPicker(productId: number) {
    setTargetBatchError("");
    setBlockedBaselineSelection(null);
    setPickerProductId(productId);
  }

  function clearSelection() {
    updateQuery({
      productId: undefined,
      variantId: undefined,
      parentProductId: undefined,
      prototypeGroupId: undefined,
      layerId: undefined,
      printWidth: undefined,
      printHeight: undefined,
      templateImageUrl: undefined,
      maskImageUrl: undefined,
      blankDesignUrl: undefined,
      mockupImageUrl: undefined,
      mockupImageUrls: undefined,
      variantIds: undefined,
      productName: undefined,
      variantLabel: undefined,
      step: "select",
    });
  }

  function handoffBlockedGroupedSelection(
    selection: SDSProductVariantSelection,
    message: string,
  ) {
    saveRecentSDSVariant(selection);
    if (isTargetingExistingBatchFlow && effectiveTargetBatchId) {
      setPickerProductId(undefined);
      setTargetBatchFeedback("");
      setBlockedBaselineSelection(selection);
      setTargetBatchError(`这款商品还没有加入当前批次。${message}`);
      return;
    }
    applySelection(selection);
  }

  async function handleWarmBlockedBaseline() {
    if (!blockedBaselineSelection || !effectiveTargetBatchId) {
      return;
    }
    setIsWarmingBlockedBaseline(true);
    setTargetBatchFeedback("");
    setTargetBatchError("");
    try {
      const readiness = await warmSDSBaselineForSelection(blockedBaselineSelection);
      if (readiness.status === "ready") {
        setBlockedBaselineSelection(null);
        setTargetBatchFeedback("baseline 已通过校验，正在加入当前批次...");
        await handleAddCandidateToBatch(blockedBaselineSelection, effectiveTargetBatchId, {
          stayOnPageAfterAdd: true,
          skipBaselineGate: true,
        });
        return;
      }
      setTargetBatchError(
        `这款商品还没有加入当前批次。${
          readiness.reason ||
          getSDSBaselineReasonMessage(readiness.reasonCode) ||
          "baseline 预热与校验已发起，请稍后再试。"
        }`,
      );
    } catch (error) {
      setTargetBatchError(
        error instanceof Error ? error.message : "baseline 预热失败，请重试。",
      );
    } finally {
      setIsWarmingBlockedBaseline(false);
    }
  }

  async function withGroupedBaselineGate(
    selection: SDSProductVariantSelection,
    onReady: () => Promise<void>,
  ) {
    try {
      const readiness = await getSDSBaselineReadiness({
        parentProductId: selection.parentProductId,
        prototypeGroupId: selection.prototypeGroupId,
        variantId: selection.variantId,
        selectedVariantIds: selection.selectedVariantIds,
      });
      if (readiness.status === "ready") {
        await onReady();
        return;
      }
      const handoff = buildGroupedSDSBaselineHandoff({
        status: readiness.status,
        reason: readiness.reason,
        reasonCode: readiness.reasonCode,
      });
      if (handoff) {
        saveSDSGroupedCandidateHandoff(handoff);
      }
      handoffBlockedGroupedSelection(
        selection,
        handoff?.message ||
          readiness.reason ||
          getSDSBaselineReasonMessage(readiness.reasonCode) ||
          "这款 SDS 商品还不能加入当前批次，请先处理 baseline。",
      );
    } catch (error) {
      const message =
        error instanceof Error ? error.message : "读取 SDS baseline 状态失败。";
      saveSDSGroupedCandidateHandoff(
        message,
      );
      handoffBlockedGroupedSelection(selection, message);
    }
  }

  async function handleAddCandidateToBatch(
    selection: SDSProductVariantSelection,
    batchId: string,
    options?: { stayOnPageAfterAdd?: boolean; skipBaselineGate?: boolean },
  ) {
    setTargetBatchError("");
    setTargetBatchFeedback("");
    setIsAddingToTargetBatch(true);
    try {
      const target = await getSheinStudioBatch(batchId);
      if (!target) {
        setTargetBatchError("没有找到目标批次，请返回批次页重试。");
        return;
      }
      const persistSelection = async () => {
        const selectionId = buildGroupedSDSSelectionID(selection);
        const groupedSelections = [
          ...(target.groupedSelections ?? []).filter(
            (item) => item.selectionId !== selectionId,
          ),
          {
            selectionId,
            selection,
            baselineStatus: "ready" as const,
            baselineReason: "",
            sheinStoreId: target.sheinStoreId,
            eligible: true,
          },
        ];
        const saved = await saveSheinStudioBatch({
          id: target.id,
          prompt: target.prompt,
          styleCount: target.styleCount,
          variationIntensity: target.variationIntensity,
          productImageCount: target.productImageCount,
          productImagePrompt: target.productImagePrompt,
          productImagePrompts: target.productImagePrompts,
          artworkModel: target.artworkModel,
          transparentBackground: target.transparentBackground,
          sheinStoreId: target.sheinStoreId,
          imageStrategy: target.imageStrategy,
          groupedImageMode: target.groupedImageMode,
          selectedSdsImages: target.selectedSdsImages,
          renderSizeImagesWithSds: target.renderSizeImagesWithSds,
          selection: target.selection ?? selection,
          groupedSelections,
          groups: target.groups,
          designs: target.designs,
          selectedIds: target.selectedIds,
          createdTasks: target.createdTasks,
        }, {
          makeActive: batchId === activeBatchId,
        });
        await refreshRecentBatches({ preserveCurrentOnError: true }).catch(() => {
          // Preserve the successful add flow even if the recent-batches sidebar
          // refresh flakes; the batch save above is the source of truth.
        });
        setBlockedBaselineSelection(null);
        if (saved?.id && options?.stayOnPageAfterAdd) {
          setCurrentTargetBatch(saved.id);
          setPickerProductId(undefined);
          setAddedToTargetBatchCount((current) => current + 1);
          setTargetBatchFeedback(`已加入 1 款商品到批次 ${target.name}，可以继续选下一款。`);
          return;
        }
        if (saved?.id && batchId === activeBatchId) {
          setCurrentTargetBatch(saved.id);
        }
      };
      if (options?.skipBaselineGate) {
        await persistSelection();
      } else {
        await withGroupedBaselineGate(selection, persistSelection);
      }
    } catch (error) {
      setTargetBatchError(
        error instanceof Error ? error.message : "加入当前批次失败，请重试。",
      );
    } finally {
      setIsAddingToTargetBatch(false);
    }
  }

  function finishAddingToTargetBatch() {
    if (!effectiveTargetBatchId) {
      return;
    }
    router.push(`/listing-kits/sds/batches/${effectiveTargetBatchId}`);
  }

  async function handleCreateBatchFromSelectionAndVariants(
    primary: SDSProductVariant,
    selectedVariants: SDSProductVariant[],
  ) {
    const selection = buildSDSVariantSelection(detail.data, primary, selectedVariants);
    await withGroupedBaselineGate(selection, async () => {
      const saved = await createBatchFromSelection(selection);
      await refreshRecentBatches();
      setPickerProductId(undefined);
      if (saved?.id) {
        setCurrentTargetBatch(saved.id);
        router.push(`/listing-kits/sds/batches/${saved.id}`);
      }
    });
  }

  async function handleCreateBatchFromSelection(
    selection: SDSProductVariantSelection,
  ) {
    const saved = await createBatchFromSelection(selection);
    await refreshRecentBatches();
    setPickerProductId(undefined);
    if (saved?.id) {
      setCurrentTargetBatch(saved.id);
      router.push(`/listing-kits/sds/batches/${saved.id}`);
    }
  }

  async function createBatchFromSelection(selection: SDSProductVariantSelection) {
    const selectionId = buildGroupedSDSSelectionID(selection);
    return saveSheinStudioBatch({
      prompt: "",
      styleCount: "1",
      variationIntensity: "medium",
      productImageCount: "5",
      productImagePrompt: "",
      productImagePrompts: [],
      artworkModel: "",
      transparentBackground: false,
      sheinStoreId: "",
      imageStrategy: "sds_official",
      groupedImageMode: "shared_by_size",
      selectedSdsImages: [],
      renderSizeImagesWithSds: true,
      selection,
      groupedSelections: [
        {
          selectionId,
          selection,
          baselineStatus: "ready",
          baselineReason: "",
          sheinStoreId: "",
          eligible: true,
        },
      ],
      groups: [],
      designs: [],
      selectedIds: [],
      createdTasks: [],
    });
  }

  const pickerOpen = Boolean(pickerProductId);

  return (
    <Card className="min-w-0 w-full overflow-hidden rounded-lg border-zinc-200 bg-white p-0 shadow-sm">
      <div className="space-y-4 p-4 lg:p-5">
        <div className="flex flex-col gap-3 border-b border-zinc-200 pb-4 lg:flex-row lg:items-end lg:justify-between">
          <div className="space-y-1">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-emerald-700">
              1. SDS 商品库
            </p>
            <h2 className="text-xl font-semibold tracking-tight text-zinc-950">
              选择底版商品和子 SKU
            </h2>
            <p className="max-w-2xl text-sm leading-6 text-zinc-600">
              按发货地、分类、重量、生产周期和 SKU 搜索。选择变体后会进入下一步生成流程。
            </p>
          </div>
          <div className="flex flex-wrap gap-2 text-sm">
            <Badge className="rounded-md px-3 py-2 text-sm" variant="neutral">
              {activeShipmentAreaLabel}
            </Badge>
            <Badge className="rounded-md px-3 py-2 text-sm" variant="neutral">
              {products.data?.totalCount ?? 0} 个商品
            </Badge>
            <Badge className="rounded-md px-3 py-2 text-sm" variant="neutral">
              变体 {selectedVariantId > 0 ? selectedVariantId : "未选择"}
            </Badge>
          </div>
        </div>

        <SDSProductBrowserFilters
          availableCategories={availableCategories}
          availableShipmentAreas={availableShipmentAreas}
          categoriesLoading={categories.isLoading}
          categoryId={categoryId}
          cycleBand={cycleBand}
          hotSellOnly={hotSellOnly}
          onSaleOnly={onSaleOnly}
          queryKeyword={queryKeyword}
          shipmentArea={shipmentArea}
          shipmentAreasLoading={shipmentAreas.isLoading}
          sortValue={sortValue}
          updateQuery={updateQuery}
          weightBand={weightBand}
        />

        {recentBatches.length > 0 ? (
          <div className="rounded-[1.25rem] border border-emerald-200 bg-emerald-50 px-4 py-4">
            <div className="flex flex-col gap-3 lg:flex-row lg:items-center lg:justify-between">
              <div className="space-y-1">
                <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-emerald-700">
                  当前接收批次
                </p>
                <p className="text-sm font-medium text-emerald-950">
                  {activeBatchLabel
                    ? `现在选到的商品会优先加入“${activeBatchLabel}”`
                    : "先选择一个已有批次，后面选中的商品就能直接加入它。"}
                </p>
                <p className="text-xs leading-6 text-emerald-800/80">
                  {isTargetingExistingBatchFlow
                    ? addedToTargetBatchCount > 0
                      ? `本轮已加入 ${addedToTargetBatchCount} 款商品。可以继续挑下一款，完成后再返回批次。`
                      : "你现在处于“给已有批次追加商品”模式。每次加入后会留在选品页，方便继续追加更多商品。"
                    : "打开商品变体后，会直接看到“加入当前批次”；如果想换目标，也可以改成加入其他批次或新建批次。"}
                </p>
                {targetBatchFeedback ? (
                  <p className="text-xs leading-6 text-emerald-700">{targetBatchFeedback}</p>
                ) : null}
                {targetBatchError ? (
                  <p className="text-xs leading-6 text-rose-700">{targetBatchError}</p>
                ) : null}
                {blockedBaselineSelection ? (
                  <div className="pt-1">
                    <Button
                      className="rounded-2xl"
                      disabled={isWarmingBlockedBaseline || isAddingToTargetBatch}
                      onClick={() => void handleWarmBlockedBaseline()}
                      type="button"
                      variant="secondary"
                    >
                      {isWarmingBlockedBaseline
                        ? "预热中..."
                        : "一键预热并校验 baseline"}
                    </Button>
                  </div>
                ) : null}
              </div>
              <div className="flex flex-col gap-2 sm:min-w-[18rem]">
                <Select
                  aria-label="当前接收批次"
                  className="h-11 rounded-2xl border-emerald-300 bg-white px-4"
                  onChange={(event) => setCurrentTargetBatch(event.target.value)}
                  value={activeBatchId}
                >
                  <option value="">先选择一个已有批次</option>
                  {recentBatches.map((batch) => (
                    <option key={batch.id} value={batch.id}>
                      {batch.name}
                    </option>
                  ))}
                </Select>
                {isTargetingExistingBatchFlow ? (
                  <Button
                    className="rounded-2xl"
                    onClick={finishAddingToTargetBatch}
                    type="button"
                    variant="primary"
                  >
                    {addedToTargetBatchCount > 0
                      ? `完成并返回批次（已加 ${addedToTargetBatchCount} 款）`
                      : "完成并返回批次"}
                  </Button>
                ) : null}
              </div>
            </div>
          </div>
        ) : null}

        <SDSRecentVariants
          activeVariantId={selectedVariantId > 0 ? selectedVariantId : undefined}
          items={recentVariants}
          onSelect={applySelection}
        />
        <SDSSelectionSummary
          onChange={() => {
            if (selectedProductId > 0) {
              openVariantPicker(selectedProductId);
            }
          }}
          onClear={clearSelection}
          selection={
            currentSelection
              ? {
                  ...currentSelection,
                  printableWidth:
                    currentSelection.printableWidth ?? selectedPrintableWidth,
                  printableHeight:
                    currentSelection.printableHeight ?? selectedPrintableHeight,
                  templateImageUrl:
                    currentSelection.templateImageUrl ?? selectedTemplateImageUrl,
                  maskImageUrl: currentSelection.maskImageUrl ?? selectedMaskImageUrl,
                  blankDesignUrl: currentSelection.blankDesignUrl ?? selectedBlankDesignUrl,
                  mockupImageUrl:
                    currentSelection.mockupImageUrl ?? selectedMockupImageUrl,
                  mockupImageUrls: currentSelection.mockupImageUrls,
              }
            : undefined
          }
        />

        <div className="space-y-3">
          <div className="flex items-center justify-between gap-3 px-1">
            <div className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
              商品列表
            </div>
            <div className="text-sm text-zinc-500">
              {products.data?.totalCount ?? 0} 个商品 · {activeShipmentAreaLabel} · {activeCategoryLabel}
            </div>
          </div>
          {products.isLoading ? (
            <div className="rounded-[1.5rem] border border-zinc-200 bg-zinc-50 px-4 py-8 text-sm text-zinc-600">
              正在加载 SDS 商品...
            </div>
          ) : products.error ? (
            <div className="rounded-[1.5rem] border border-amber-200 bg-amber-50 px-4 py-8 text-sm text-amber-900">
              SDS 商品加载失败。
            </div>
          ) : (
            <>
              <div className="grid gap-3 md:grid-cols-2 2xl:grid-cols-3">
                {(products.data?.items ?? []).map((product) => {
                  const isSelected =
                    selectedProductId === product.id || pickerProductId === product.id;
                  return (
                    <SDSProductCard
                      isSelected={isSelected}
                      isVariantSelected={selectedProductId === product.id && selectedVariantId > 0}
                      key={product.id}
                      onOpenVariants={() => openVariantPicker(product.id)}
                      product={product}
                    />
                  );
                })}
              </div>
              <SDSPagination
                onPageChange={(page) => updateQuery({ page: String(page) })}
                page={currentPage}
                pageCount={pageCount}
              />
            </>
          )}
        </div>
      </div>
      {pickerOpen ? (
        <SDSVariantPicker
          activeBatchId={effectiveTargetBatchId}
          activeBatchLabel={activeBatchLabel}
          batchOptions={recentBatches
            .filter((batch) => batch.id !== effectiveTargetBatchId)
            .map((batch) => ({ id: batch.id, title: batch.name }))}
          hasError={Boolean(detail.error)}
          isSubmittingToBatch={isAddingToTargetBatch}
          isTargetingExistingBatch={isTargetingExistingBatchFlow}
          isLoading={detail.isLoading}
          onAddSelectedVariantsToBatch={(primary, selectedVariants, batchId) => {
            const selection = buildSDSVariantSelection(detail.data, primary, selectedVariants);
            void handleAddCandidateToBatch(selection, batchId, {
              stayOnPageAfterAdd: isTargetingExistingBatchFlow,
            });
          }}
          onClose={() => setPickerProductId(undefined)}
          onCreateBatchFromSelectedVariants={(primary, selectedVariants) => {
            void handleCreateBatchFromSelectionAndVariants(primary, selectedVariants);
          }}
          onSelectVariants={applyVariants}
          open={pickerOpen}
          product={
            (products.data?.items ?? []).find((product) => product.id === pickerProductId) ??
            detail.data
          }
          selectedVariantId={selectedVariantId > 0 ? selectedVariantId : undefined}
          variants={variants}
        />
      ) : null}
    </Card>
  );
}
