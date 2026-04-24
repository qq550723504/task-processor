"use client";

import { FormEvent, useMemo, useState } from "react";
import { usePathname, useRouter, useSearchParams } from "next/navigation";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import { SDSPagination } from "@/components/listingkit/sds-pagination";
import { SDSRecentVariants } from "@/components/listingkit/sds-recent-variants";
import { SDSSelectionSummary } from "@/components/listingkit/sds-selection-summary";
import { SDSVariantPicker } from "@/components/listingkit/sds-variant-picker";
import { useSDSCategories } from "@/lib/query/use-sds-categories";
import { useSDSProductDetail } from "@/lib/query/use-sds-product-detail";
import { useSDSProducts } from "@/lib/query/use-sds-products";
import { useSDSRecentVariants } from "@/lib/query/use-sds-recent-variants";
import { useSDSShipmentAreas } from "@/lib/query/use-sds-shipment-areas";
import {
  formatProductionCycle,
  formatWeight,
  sdsCycleBands,
  sdsWeightBands,
} from "@/lib/sds/product-filters";
import type { SDSProductVariant, SDSProductVariantSelection } from "@/lib/types/sds";
import { saveRecentSDSVariant } from "@/lib/utils/sds-recent-variants";

function formatPrice(value?: number) {
  if (!value) {
    return "-";
  }
  return `$${value.toFixed(2)}`;
}

function resolveLayerId(variant: SDSProductVariant) {
  const layers = variant.designPrototype?.prototypeLayerList ?? [];
  return layers.find((layer) => layer.isMasterMap === 1)?.id ?? layers[0]?.id ?? "";
}

function resolvePrimaryLayer(variant: SDSProductVariant) {
  const layers = variant.designPrototype?.prototypeLayerList ?? [];
  return layers.find((layer) => layer.isMasterMap === 1) ?? layers[0];
}

function resolvePrimaryMockup(variant: SDSProductVariant) {
  const groups = variant.designPrototype?.prototypeResultGroups ?? [];
  return groups.find((group) => group.faceSheetState)?.resultImage ?? groups[0]?.resultImage;
}

function resolveMockupImages(variant: SDSProductVariant) {
  const groups = [...(variant.designPrototype?.prototypeResultGroups ?? [])]
    .sort((left, right) => (left.sort ?? 0) - (right.sort ?? 0))
    .map((group) => group.resultImage)
    .filter((image): image is string => Boolean(image));

  return groups.length > 0 ? groups : [];
}

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
    availableCategories.find((item) => item.id === categoryId)?.name ?? "All categories";
  const currentSelection = useMemo(
    () => recentVariants.find((item) => item.variantId === selectedVariantId),
    [recentVariants, selectedVariantId],
  );

  function updateQuery(next: Record<string, string | undefined>) {
    const params = new URLSearchParams(searchParams.toString());
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

  function buildSelection(variant: SDSProductVariant): SDSProductVariantSelection {
    const productId = detail.data?.id ?? variant.parent_id ?? 0;
    return {
      productId,
      parentProductId: productId,
      variantId: variant.id,
      prototypeGroupId: variant.designPrototype?.prototypeGroupId ?? 0,
      layerId: resolveLayerId(variant),
      productName: detail.data?.name ?? "SDS product",
      variantLabel: `${variant.size || "One size"} · ${variant.color_name || "default"}`,
      printableWidth: resolvePrimaryLayer(variant)?.printWidth,
      printableHeight: resolvePrimaryLayer(variant)?.printHeight,
      templateImageUrl:
        resolvePrimaryLayer(variant)?.thumbnailUrl ?? resolvePrimaryLayer(variant)?.imageUrl,
      maskImageUrl:
        resolvePrimaryLayer(variant)?.maskUrl ??
        resolvePrimaryLayer(variant)?.maskShowUrl ??
        resolvePrimaryLayer(variant)?.maskThumbnailUrl,
      blankDesignUrl: detail.data?.blankDesignUrl,
      mockupImageUrl: resolvePrimaryMockup(variant),
      mockupImageUrls: resolveMockupImages(variant),
    };
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
      templateImageUrl: selection.templateImageUrl,
      maskImageUrl: selection.maskImageUrl,
      blankDesignUrl: selection.blankDesignUrl,
      mockupImageUrl: selection.mockupImageUrl,
      mockupImageUrls:
        selection.mockupImageUrls && selection.mockupImageUrls.length > 0
          ? JSON.stringify(selection.mockupImageUrls)
          : undefined,
      productName: selection.productName,
      variantLabel: selection.variantLabel,
    });
    saveRecentSDSVariant(selection);
    setPickerProductId(undefined);
  }

  function applyVariant(variant: SDSProductVariant) {
    applySelection(buildSelection(variant));
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
      productName: undefined,
      variantLabel: undefined,
    });
  }

  const pickerOpen = Boolean(pickerProductId);

  function renderProductThumb(imageUrl?: string) {
    if (!imageUrl) {
      return (
        <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-zinc-100 text-xs font-semibold uppercase tracking-[0.16em] text-zinc-400">
          SDS
        </div>
      );
    }

    return (
      <div
        className="h-16 w-16 rounded-2xl bg-zinc-100 bg-cover bg-center"
        style={{ backgroundImage: `url(${imageUrl})` }}
      />
    );
  }

  return (
    <Card className="w-full max-w-7xl overflow-hidden rounded-[2rem] border-white/70 bg-white/75 p-0 shadow-[0_20px_80px_rgba(24,24,27,0.08)] backdrop-blur">
      <div className="space-y-6 p-6 lg:p-8">
        <div className="grid gap-5 rounded-[1.75rem] border border-zinc-200/80 bg-[linear-gradient(135deg,_rgba(250,250,249,0.98),_rgba(244,244,245,0.92))] px-5 py-5 lg:grid-cols-[1.2fr_0.8fr] lg:px-6">
          <div className="space-y-2">
            <p className="text-[11px] font-semibold uppercase tracking-[0.3em] text-emerald-700">
            SDS Catalog
          </p>
            <h2 className="font-serif text-3xl leading-tight tracking-[-0.04em] text-zinc-950">
              Pick the product family first, then lock the exact child SKU.
            </h2>
            <p className="max-w-2xl text-sm leading-7 text-zinc-600">
              Use shipment area filters and SKU search to narrow the catalog. Once a
              variant is selected, the SDS sync form below is prefilled automatically.
            </p>
          </div>
          <div className="grid gap-3 sm:grid-cols-3 lg:grid-cols-1">
            <div className="rounded-[1.25rem] border border-zinc-200/80 bg-white px-4 py-4">
              <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
                Active market
              </div>
              <div className="mt-2 text-lg font-semibold text-zinc-950">
                {activeShipmentAreaLabel}
              </div>
            </div>
            <div className="rounded-[1.25rem] border border-zinc-200/80 bg-white px-4 py-4">
              <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
                Catalog size
              </div>
              <div className="mt-2 text-lg font-semibold text-zinc-950">
                {products.data?.totalCount ?? 0}
              </div>
            </div>
            <div className="rounded-[1.25rem] border border-zinc-200/80 bg-white px-4 py-4">
              <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
                Selected variant
              </div>
              <div className="mt-2 text-lg font-semibold text-zinc-950">
                {selectedVariantId > 0 ? selectedVariantId : "Pending"}
              </div>
            </div>
          </div>
        </div>

        <form
          className="grid gap-3 rounded-[1.5rem] border border-zinc-200/80 bg-white px-4 py-4 shadow-sm lg:grid-cols-[200px_220px_180px_180px_220px_minmax(0,1fr)_auto]"
          onSubmit={(event: FormEvent<HTMLFormElement>) => {
            event.preventDefault();
            const formData = new FormData(event.currentTarget);
            applySearch(String(formData.get("keyword") ?? ""));
          }}
        >
          <select
            className="h-12 min-w-[180px] rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
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
            className="h-12 min-w-[180px] rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
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
            <option value="">All categories</option>
            {availableCategories.map((category) => (
              <option key={category.id} value={category.id}>
                {category.name} ({category.count})
              </option>
            ))}
          </select>
          <select
            className="h-12 min-w-[180px] rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
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
            <option value="">Default sort</option>
            <option value="min_price:asc">Price low to high</option>
            <option value="min_price:desc">Price high to low</option>
          </select>
          <select
            className="h-12 min-w-[160px] rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
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
            className="h-12 min-w-[160px] rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
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
            className="min-w-[240px] rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-950 outline-none transition focus:border-zinc-950 focus:bg-white"
            defaultValue={queryKeyword}
            key={queryKeyword}
            name="keyword"
            placeholder="Search by name or SKU"
          />
          <Button type="submit">Search</Button>
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
            On sale only
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
            Hot sale only
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
              Products
            </div>
            <div className="text-sm text-zinc-500">
              {products.data?.totalCount ?? 0} items · {activeShipmentAreaLabel} · {activeCategoryLabel}
            </div>
          </div>
          {products.isLoading ? (
            <div className="rounded-[1.5rem] border border-zinc-200 bg-zinc-50 px-4 py-8 text-sm text-zinc-600">
              Loading SDS products...
            </div>
          ) : products.error ? (
            <div className="rounded-[1.5rem] border border-amber-200 bg-amber-50 px-4 py-8 text-sm text-amber-900">
              Failed to load SDS products.
            </div>
          ) : (
            <>
              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
                {(products.data?.items ?? []).map((product) => {
                  const isSelected =
                    selectedProductId === product.id || pickerProductId === product.id;
                  return (
                    <div
                      className={`rounded-[1.5rem] border px-4 py-4 shadow-sm transition ${
                        isSelected
                          ? "border-emerald-800 bg-[linear-gradient(135deg,_#052e2b,_#115e59)] text-white"
                          : "border-zinc-200 bg-white text-zinc-900 hover:-translate-y-0.5 hover:border-zinc-400 hover:shadow-md"
                      }`}
                      key={product.id}
                    >
                      <div className="flex items-start gap-4">
                        {renderProductThumb(product.img_url)}
                        <div className="min-w-0 flex-1 space-y-2">
                          <div className="flex flex-wrap gap-2">
                            {product.on_sale_status === 2 ? (
                              <span
                                className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${
                                  isSelected
                                    ? "bg-white/12 text-white"
                                    : "bg-emerald-50 text-emerald-700"
                                }`}
                              >
                                On sale
                              </span>
                            ) : null}
                            {product.hotSellStatus === 1 ? (
                              <span
                                className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${
                                  isSelected
                                    ? "bg-rose-400/20 text-rose-50"
                                    : "bg-rose-50 text-rose-700"
                                }`}
                              >
                                Hot sale
                              </span>
                            ) : null}
                            {product.issuingBayArea?.name ? (
                              <span
                                className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${
                                  isSelected
                                    ? "bg-white/12 text-white"
                                    : "bg-zinc-100 text-zinc-700"
                                }`}
                              >
                                {product.issuingBayArea.name}
                              </span>
                            ) : null}
                          </div>
                          <div className="line-clamp-2 text-sm font-semibold leading-6">
                            {product.name}
                          </div>
                          <div className={isSelected ? "text-emerald-100" : "text-zinc-500"}>
                            SKU {product.sku ?? "-"} · {formatPrice(product.currentPrice ?? product.min_price)}
                          </div>
                          <div className={isSelected ? "text-emerald-100" : "text-zinc-500"}>
                            Weight {formatWeight(product)} · Cycle {formatProductionCycle(product)}
                          </div>
                          {product.categories?.length ? (
                            <div
                              className={`line-clamp-2 text-sm ${
                                isSelected ? "text-emerald-100" : "text-zinc-500"
                              }`}
                            >
                              {product.categories.map((category) => category.name).join(" / ")}
                            </div>
                          ) : null}
                          <div className="flex gap-3 pt-1">
                            {selectedProductId === product.id && selectedVariantId > 0 ? (
                              <span
                                className={`inline-flex items-center rounded-full px-3 text-xs font-semibold uppercase tracking-[0.16em] ${
                                  isSelected
                                    ? "bg-white/12 text-white"
                                    : "bg-emerald-50 text-emerald-700"
                                }`}
                              >
                                Selected
                              </span>
                            ) : null}
                            <Button
                              className="flex-1"
                              onClick={() => openVariantPicker(product.id)}
                              tone={isSelected ? "secondary" : "primary"}
                              type="button"
                            >
                              {selectedProductId === product.id && selectedVariantId > 0
                                ? "Change variant"
                                : "View specs"}
                            </Button>
                          </div>
                        </div>
                      </div>
                    </div>
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
          onSelectVariant={applyVariant}
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
