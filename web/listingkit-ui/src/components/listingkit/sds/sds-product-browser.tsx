"use client";

import { FormEvent, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import { SDSPagination } from "@/components/listingkit/sds/sds-pagination";
import { SDSProductCard } from "@/components/listingkit/sds/sds-product-card";
import { SDSRecentVariants } from "@/components/listingkit/sds/sds-recent-variants";
import { SDSSelectionSummary } from "@/components/listingkit/sds/sds-selection-summary";
import { SDSVariantPicker } from "@/components/listingkit/sds/sds-variant-picker";
import { useSDSCategories } from "@/lib/query/use-sds-categories";
import { useSDSProductDetail } from "@/lib/query/use-sds-product-detail";
import { useSDSProducts } from "@/lib/query/use-sds-products";
import { useSDSRecentVariants } from "@/lib/query/use-sds-recent-variants";
import { useSDSShipmentAreas } from "@/lib/query/use-sds-shipment-areas";
import {
  sdsCycleBands,
  sdsWeightBands,
} from "@/lib/sds/product-filters";
import { buildSDSVariantSelection } from "@/lib/sds/variant-selection";
import type { SDSProductVariant, SDSProductVariantSelection } from "@/lib/types/sds";
import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";
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
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();
  const recentVariants = useSDSRecentVariants();
  const [pickerProductId, setPickerProductId] = useState<number | undefined>();

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
    router.replace(suffix ? `${pathname}?${suffix}` : pathname);
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

  function applySearch(keywordValue: string) {
    updateQuery({
      keyword: keywordValue.trim() || undefined,
      page: "1",
    });
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

  const pickerOpen = Boolean(pickerProductId);

  return (
    <Card className="w-full max-w-7xl overflow-hidden rounded-[2rem] border-white/70 bg-white/75 p-0 shadow-[0_20px_80px_rgba(24,24,27,0.08)] backdrop-blur">
      <div className="space-y-6 p-6 lg:p-8">
        <div className="grid gap-5 rounded-[1.75rem] border border-zinc-200/80 bg-[linear-gradient(135deg,_rgba(250,250,249,0.98),_rgba(244,244,245,0.92))] px-5 py-5 lg:grid-cols-[1.2fr_0.8fr] lg:px-6">
          <div className="space-y-2">
            <p className="text-[11px] font-semibold uppercase tracking-[0.3em] text-emerald-700">
            SDS 商品库
          </p>
            <h2 className="font-serif text-3xl leading-tight tracking-[-0.04em] text-zinc-950">
              先选择 SDS 底版商品，再锁定具体子 SKU。
            </h2>
            <p className="max-w-2xl text-sm leading-7 text-zinc-600">
              可以按发货地、分类、重量、生产周期和 SKU 搜索筛选商品。选择变体后，下方生成设置会自动带入模板信息。
            </p>
          </div>
          <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-1">
            <div className="rounded-[1.25rem] border border-zinc-200/80 bg-white px-4 py-4">
              <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
                当前发货地
              </div>
              <div className="mt-2 text-lg font-semibold text-zinc-950">
                {activeShipmentAreaLabel}
              </div>
            </div>
            <div className="rounded-[1.25rem] border border-zinc-200/80 bg-white px-4 py-4">
              <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
                商品数量
              </div>
              <div className="mt-2 text-lg font-semibold text-zinc-950">
                {products.data?.totalCount ?? 0}
              </div>
            </div>
            <div className="rounded-[1.25rem] border border-zinc-200/80 bg-white px-4 py-4">
              <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
                已选变体
              </div>
              <div className="mt-2 text-lg font-semibold text-zinc-950">
                {selectedVariantId > 0 ? selectedVariantId : "未选择"}
              </div>
            </div>
          </div>
        </div>

        <form
          className="flex flex-wrap items-center gap-3 rounded-[1.5rem] border border-zinc-200/80 bg-white px-4 py-4 shadow-sm"
          onSubmit={(event: FormEvent<HTMLFormElement>) => {
            event.preventDefault();
            const formData = new FormData(event.currentTarget);
            applySearch(String(formData.get("keyword") ?? ""));
          }}
        >
          <select
            className="h-12 min-w-[180px] flex-1 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 md:flex-none md:basis-[200px]"
            disabled={shipmentAreas.isLoading || availableShipmentAreas.length === 0}
            defaultValue={shipmentArea}
            key={shipmentArea}
            name="shipmentArea"
            onChange={(event) =>
              updateQuery({
                shipmentArea: event.target.value,
                page: "1",
                categoryId: undefined,
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
                productName: undefined,
                variantLabel: undefined,
              })
            }
          >
            {availableShipmentAreas.map((area) => (
              <option key={area.value} value={area.value}>
                {area.label} ({area.totalCount})
              </option>
            ))}
          </select>
          <select
            className="h-12 min-w-[180px] flex-1 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 md:flex-none md:basis-[220px]"
            disabled={categories.isLoading}
            key={`${shipmentArea}:${categoryId ?? 0}`}
            name="categoryId"
            onChange={(event) =>
              updateQuery({
                categoryId: event.target.value || undefined,
                page: "1",
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
                productName: undefined,
                variantLabel: undefined,
              })
            }
            defaultValue={categoryId ? String(categoryId) : ""}
          >
            <option value="">全部分类</option>
            {availableCategories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name} ({category.count})
              </option>
            ))}
          </select>
          <select
            className="h-12 min-w-[160px] flex-1 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 md:flex-none md:basis-[180px]"
            defaultValue={sortValue}
            key={`sort:${sortValue || "default"}`}
            name="sort"
            onChange={(event) =>
              updateQuery({
                sort: event.target.value || undefined,
                page: "1",
              })
            }
          >
            <option value="">默认排序</option>
            <option value="min_price:asc">价格从低到高</option>
            <option value="min_price:desc">价格从高到低</option>
          </select>
          <select
            className="h-12 min-w-[150px] flex-1 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 md:flex-none md:basis-[170px]"
            defaultValue={weightBand}
            key={`weight:${weightBand || "all"}`}
            name="weightBand"
            onChange={(event) =>
              updateQuery({
                weightBand: event.target.value || undefined,
                page: "1",
              })
            }
          >
            {sdsWeightBands.map((band) => (
              <option key={band.value || "all"} value={band.value}>
                {band.label}
              </option>
            ))}
          </select>
          <select
            className="h-12 min-w-[150px] flex-1 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 md:flex-none md:basis-[190px]"
            defaultValue={cycleBand}
            key={`cycle:${cycleBand || "all"}`}
            name="cycleBand"
            onChange={(event) =>
              updateQuery({
                cycleBand: event.target.value || undefined,
                page: "1",
              })
            }
          >
            {sdsCycleBands.map((band) => (
              <option key={band.value || "all"} value={band.value}>
                {band.label}
              </option>
            ))}
          </select>
          <input
            className="h-12 min-w-[220px] flex-[2_1_280px] rounded-2xl border border-zinc-200 bg-zinc-50 px-4 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 focus:bg-white"
            defaultValue={queryKeyword}
            key={queryKeyword}
            name="keyword"
            placeholder="按商品名或 SKU 搜索"
          />
          <div className="shrink-0">
            <Button type="submit">搜索</Button>
          </div>
        </form>

        <div className="flex flex-wrap gap-3">
          <button
            className={`rounded-full border px-4 py-2 text-sm font-medium transition ${
              onSaleOnly
                ? "border-emerald-800 bg-emerald-900 text-white"
                : "border-zinc-200 bg-white text-zinc-700 hover:border-zinc-400"
            }`}
            onClick={() =>
              updateQuery({
                onSaleStatus: onSaleOnly ? undefined : "2",
                page: "1",
              })
            }
            type="button"
          >
            只看在售
          </button>
          <button
            className={`rounded-full border px-4 py-2 text-sm font-medium transition ${
              hotSellOnly
                ? "border-rose-800 bg-rose-900 text-white"
                : "border-zinc-200 bg-white text-zinc-700 hover:border-zinc-400"
            }`}
            onClick={() =>
              updateQuery({
                hotSellStatus: hotSellOnly ? undefined : "1",
                page: "1",
              })
            }
            type="button"
          >
            只看热卖
          </button>
        </div>

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
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
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
          hasError={Boolean(detail.error)}
          isLoading={detail.isLoading}
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
