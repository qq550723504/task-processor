"use client";

import { useEffect, useMemo, useState } from "react";
import { usePathname } from "next/navigation";

import { Badge } from "@/components/ui/badge";
import { Card } from "@/components/ui/card";
import { SDSGroupedCandidatesPanel } from "@/components/listingkit/sds/sds-grouped-candidates-panel";
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
import { useSDSGroupedCandidates } from "@/lib/query/use-sds-grouped-candidates";
import { useSDSProductDetail } from "@/lib/query/use-sds-product-detail";
import { useSDSProducts } from "@/lib/query/use-sds-products";
import { useSDSRecentVariants } from "@/lib/query/use-sds-recent-variants";
import { useSDSShipmentAreas } from "@/lib/query/use-sds-shipment-areas";
import { buildSDSVariantSelection } from "@/lib/sds/variant-selection";
import type { SDSProductVariant, SDSProductVariantSelection } from "@/lib/types/sds";
import {
  buildGroupedSDSSelectionID,
  type SDSBaselineStatus,
} from "@/lib/types/sds-baseline";
import { replaceBrowserHistory } from "@/lib/utils/browser-history";
import { useLiveSearchParams } from "@/lib/utils/live-search-params";
import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";
import {
  hasSDSGroupedCandidate,
  removeSDSGroupedCandidate,
  saveSDSGroupedCandidate,
} from "@/lib/utils/sds-grouped-candidates";
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
  const searchParams = useLiveSearchParams();
  const recentVariants = useSDSRecentVariants();
  const groupedCandidates = useSDSGroupedCandidates();
  const [pickerProductId, setPickerProductId] = useState<number | undefined>();
  const [isWarmingGroupedCandidates, setIsWarmingGroupedCandidates] = useState(false);
  const [groupedCandidateBaselineStatuses, setGroupedCandidateBaselineStatuses] =
    useState<Record<string, { reason: string; status: SDSBaselineStatus | "loading" }>>({});

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
  const groupedCandidateCountsByProduct = useMemo(() => {
    const counts = new Map<number, number>();
    groupedCandidates.forEach((item) => {
      counts.set(item.productId, (counts.get(item.productId) ?? 0) + 1);
    });
    return counts;
  }, [groupedCandidates]);

  useEffect(() => {
    if (groupedCandidates.length === 0) {
      setGroupedCandidateBaselineStatuses({});
      return;
    }

    setGroupedCandidateBaselineStatuses((current) => {
      const next: Record<string, { reason: string; status: SDSBaselineStatus | "loading" }> = {};
      groupedCandidates.forEach((item) => {
        const selectionId = buildGroupedSDSSelectionID(item);
        next[selectionId] = current[selectionId] ?? {
          status: "loading",
          reason: "正在检查 baseline 状态...",
        };
      });
      return next;
    });

    let cancelled = false;
    void Promise.all(
      groupedCandidates.map(async (item) => {
        const selectionId = buildGroupedSDSSelectionID(item);
        try {
          const readiness = await getSDSBaselineReadiness({
            parentProductId: item.parentProductId,
            prototypeGroupId: item.prototypeGroupId,
            variantId: item.variantId,
            selectedVariantIds: item.selectedVariantIds,
          });
          return [
            selectionId,
            {
              status: readiness.status,
              reason: readiness.reason ?? "",
            },
          ] as const;
        } catch (error) {
          return [
            selectionId,
            {
              status: "failed" as const,
              reason:
                error instanceof Error
                  ? error.message
                  : "读取 SDS baseline 状态失败。",
            },
          ] as const;
        }
      }),
    ).then((entries) => {
      if (cancelled) {
        return;
      }
      setGroupedCandidateBaselineStatuses(Object.fromEntries(entries));
    });

    return () => {
      cancelled = true;
    };
  }, [groupedCandidates]);

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
    saveRecentSDSVariant(selection);
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

  function addVariantsToGroupedCandidates(
    primary: SDSProductVariant,
    selectedVariants: SDSProductVariant[],
  ) {
    const selection = buildSDSVariantSelection(detail.data, primary, selectedVariants);
    saveSDSGroupedCandidate(selection);
    saveRecentSDSVariant(selection);
  }

  function openVariantPicker(productId: number) {
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

  function toggleGroupedCandidate(selection: SDSProductVariantSelection) {
    if (hasSDSGroupedCandidate(selection)) {
      removeSDSGroupedCandidate(selection);
      return;
    }
    saveSDSGroupedCandidate(selection);
  }

  async function handleWarmGroupedCandidates(items: SDSProductVariantSelection[]) {
    if (items.length === 0) {
      return;
    }
    setIsWarmingGroupedCandidates(true);
    setGroupedCandidateBaselineStatuses((current) => {
      const next = { ...current };
      items.forEach((item) => {
        next[buildGroupedSDSSelectionID(item)] = {
          status: "loading",
          reason: "正在批量预热 baseline...",
        };
      });
      return next;
    });
    try {
      const entries = await Promise.all(
        items.map(async (item) => {
          const selectionId = buildGroupedSDSSelectionID(item);
          try {
            const readiness = await warmSDSBaselineForSelection(item);
            return [
              selectionId,
              {
                status: readiness.status,
                reason:
                  readiness.reason ??
                  (readiness.status === "ready"
                    ? "baseline 预热完成。"
                    : ""),
              },
            ] as const;
          } catch (error) {
            return [
              selectionId,
              {
                status: "failed" as const,
                reason:
                  error instanceof Error ? error.message : "baseline 批量预热失败。",
              },
            ] as const;
          }
        }),
      );
      setGroupedCandidateBaselineStatuses((current) => ({
        ...current,
        ...Object.fromEntries(entries),
      }));
    } finally {
      setIsWarmingGroupedCandidates(false);
    }
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

        <SDSRecentVariants
          activeVariantId={selectedVariantId > 0 ? selectedVariantId : undefined}
          items={recentVariants}
          onSelect={applySelection}
        />
        <SDSGroupedCandidatesPanel
          activeSelection={currentSelection}
          baselineStatuses={groupedCandidateBaselineStatuses}
          isWarmingAll={isWarmingGroupedCandidates}
          items={groupedCandidates}
          onRemove={removeSDSGroupedCandidate}
          onSelect={(selection, baseline) => {
            const handoff = buildGroupedCandidateHandoff(baseline);
            if (handoff) {
              saveSDSGroupedCandidateHandoff(handoff);
            }
            applySelection(selection);
          }}
          onWarmAll={(items) => {
            void handleWarmGroupedCandidates(items);
          }}
        />
        <SDSSelectionSummary
          isGroupedCandidate={currentSelection ? hasSDSGroupedCandidate(currentSelection) : false}
          onChange={() => {
            if (selectedProductId > 0) {
              openVariantPicker(selectedProductId);
            }
          }}
          onClear={clearSelection}
          onToggleGroupedCandidate={() => {
            if (currentSelection) {
              toggleGroupedCandidate(currentSelection);
            }
          }}
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
                      groupedCandidateCount={groupedCandidateCountsByProduct.get(product.id) ?? 0}
                      hasGroupedCandidate={groupedCandidateCountsByProduct.has(product.id)}
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
          hasError={Boolean(detail.error)}
          isLoading={detail.isLoading}
          onAddSelectedVariantsToGroupedCandidates={addVariantsToGroupedCandidates}
          onClose={() => setPickerProductId(undefined)}
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

function buildGroupedCandidateHandoff(baseline: {
  reason: string;
  status: SDSBaselineStatus | "loading";
}) {
  if (baseline.status === "missing") {
    return {
      action: "warm_baseline" as const,
      actionLabel: "一键预热 baseline",
      message:
        baseline.reason ||
        "这款候选商品还没有 baseline 缓存。先在当前工作台完成一次生成或预热，再回来加入 grouped 批量上品。",
    };
  }
  if (baseline.status === "failed") {
    return {
      action: "warm_baseline" as const,
      actionLabel: "重试 baseline 预热",
      message:
        baseline.reason ||
        "这款候选商品的 baseline 检查失败。请先重新生成或排查 SDS 转标准商品链路，再尝试 grouped 批量上品。",
    };
  }
  if (baseline.status === "loading") {
    return {
      action: "focus_generate" as const,
      actionLabel: "去生成区查看",
      message:
        baseline.reason ||
        "这款候选商品的 baseline 状态还在检查中。稍等片刻，确认就绪后再加入 grouped 批量上品。",
    };
  }
  return null;
}
