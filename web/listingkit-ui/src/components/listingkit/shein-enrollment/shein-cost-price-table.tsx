"use client";

import { useMemo, useState } from "react";
import { useQueries, useQuery } from "@tanstack/react-query";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { getSDSProductDetail, getSDSProducts } from "@/lib/api/sds-products";
import { getSheinSourceSDSMetadata } from "@/lib/api/shein-enrollment";
import { listingKitKeys } from "@/lib/query/keys";
import type { SDSProductSummary, SDSProductVariant } from "@/lib/types/sds";
import type {
  SheinSDSCostGroupRecord,
  SheinSyncedProductRecord,
} from "@/lib/types/listingkit/shein-enrollment";

export type SheinCostPriceSaveTarget = {
  groupKey: string;
  groupLabel: string;
  productId?: number;
};

type SheinCostGroupRow = {
  groupKey: string;
  groupLabel: string;
  productId?: number;
  products: SheinSyncedProductRecord[];
  sourceSDSCodes: string[];
  manualCostPrice?: number | null;
  fallbackCostPrice?: number | null;
};

export function SheinCostPriceTable({
  groups,
  items,
  onSave,
  saving,
  shipmentArea = "US",
  storeId,
}: {
  groups: SheinSDSCostGroupRecord[];
  items: SheinSyncedProductRecord[];
  onSave: (
    target: SheinCostPriceSaveTarget,
    manualCostPrice: number | null,
  ) => Promise<void>;
  saving: boolean;
  shipmentArea?: string;
  storeId: number;
}) {
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const sourceShipmentArea = shipmentArea.trim() || "US";
  const rows = useMemo(() => buildSheinCostGroupRows(items, groups), [items, groups]);
  const sourceSDSCodes = useMemo(
    () => Array.from(new Set(rows.flatMap((row) => row.sourceSDSCodes))).sort(),
    [rows],
  );
  const sourceProductSearches = useMemo(
    () =>
      sourceSDSCodes.flatMap((sourceCode) =>
        sdsSourceSearchKeywords(sourceCode).map((keyword) => ({
          keyword,
          sourceCode,
        })),
      ),
    [sourceSDSCodes],
  );
  const sourceProductQueries = useQueries({
    queries: sourceProductSearches.map(({ keyword, sourceCode }) => ({
      queryKey: listingKitKeys.sdsProducts({
        keyword,
        page: 1,
        size: 1,
        shipmentArea: sourceShipmentArea,
        preciseSearch: true,
      }),
      queryFn: () =>
        getSDSProducts({
          keyword,
          page: 1,
          size: 1,
          shipmentArea: sourceShipmentArea,
          preciseSearch: true,
        }),
      enabled: sourceCode.length > 0,
      staleTime: 10 * 60 * 1000,
    })),
  });
  const sourceListProductsByCode = useMemo(() => {
    const result = new Map<string, SheinCostSourceProductInfo>();
    sourceProductQueries.forEach((query, index) => {
      const sourceCode = sourceProductSearches[index]?.sourceCode;
      if (!sourceCode) {
        return;
      }
      const product = resolveSDSSourceProduct(sourceCode, query.data?.items);
      const current = result.get(sourceCode);
      if (product && shouldUseSDSSourceProduct(product, current)) {
        result.set(sourceCode, product);
      }
    });
    return result;
  }, [sourceProductQueries, sourceProductSearches]);
  const sourceProductIDsByCode = useMemo(() => {
    const result = new Map<string, number>();
    sourceListProductsByCode.forEach((product, sourceCode) => {
      const productId = sdsSourceDetailProductID(product);
      if (productId > 0) {
        result.set(sourceCode, productId);
      }
    });
    return result;
  }, [sourceListProductsByCode]);
  const sourceProductDetailQueries = useQueries({
    queries: sourceSDSCodes.map((sourceCode) => {
      const productId = sourceProductIDsByCode.get(sourceCode) ?? 0;
      return {
        queryKey: listingKitKeys.sdsProductDetail(productId),
        queryFn: () => getSDSProductDetail(productId),
        enabled: productId > 0,
        staleTime: 10 * 60 * 1000,
      };
    }),
  });
  const sourceProductsByCode = useMemo(() => {
    const result = new Map<string, SheinCostSourceProductInfo>();
    sourceSDSCodes.forEach((sourceCode, index) => {
      if (!sourceCode) {
        return;
      }
      const detailProduct = sourceProductDetailQueries[index]?.data;
      const listProduct = sourceListProductsByCode.get(sourceCode);
      const detailSourceProduct = resolveSDSSourceProduct(
        sourceCode,
        detailProduct ? [detailProduct] : undefined,
      );
      const product = mergeSDSSourceProductInfo(detailSourceProduct, listProduct);
      if (product) {
        result.set(sourceCode, product);
      }
    });
    return result;
  }, [sourceListProductsByCode, sourceProductDetailQueries, sourceSDSCodes]);
  const sourceLookupsSettled =
    sourceProductQueries.every((query) => !query.isFetching) &&
    sourceProductDetailQueries.every((query) => !query.isFetching);
  const missingSourceTaskMetadataCodes = useMemo(
    () =>
      sourceSDSCodes.filter(
        (sourceCode) => !formatSDSSourceTitle(sourceProductsByCode.get(sourceCode)),
      ),
    [sourceProductsByCode, sourceSDSCodes],
  );
  const sourceTaskMetadataQuery = useQuery({
    queryKey: [
      "listingkit",
      "shein-enrollment",
      "source-sds-task-metadata-v4",
      storeId,
      missingSourceTaskMetadataCodes,
    ],
    queryFn: () =>
      getSheinSourceTaskMetadata(storeId, missingSourceTaskMetadataCodes),
    enabled:
      storeId > 0 &&
      sourceLookupsSettled &&
      missingSourceTaskMetadataCodes.length > 0,
    staleTime: 10 * 60 * 1000,
  });

  return (
    <div className="grid gap-3">
      {rows.length === 0 ? (
        <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-6 text-sm text-zinc-500">
          当前没有可维护成本价的同步商品。
        </div>
      ) : null}
      {rows.map((row) => (
        <SheinCostPriceRow
          draft={drafts[row.groupKey]}
          key={row.groupKey}
          onDraftChange={(value) =>
            setDrafts((current) => ({
              ...current,
              [row.groupKey]: value,
            }))
          }
          onSave={onSave}
          row={row}
          saving={saving}
          shipmentArea={shipmentArea}
          sourceTaskMetadataByCode={sourceTaskMetadataQuery.data}
          sourceProductsByCode={sourceProductsByCode}
        />
      ))}
    </div>
  );
}

function SheinCostPriceRow({
  draft,
  onDraftChange,
  onSave,
  row,
  saving,
  shipmentArea,
  sourceTaskMetadataByCode,
  sourceProductsByCode,
}: {
  draft?: string;
  onDraftChange: (value: string) => void;
  onSave: (
    target: SheinCostPriceSaveTarget,
    manualCostPrice: number | null,
  ) => Promise<void>;
  row: SheinCostGroupRow;
  saving: boolean;
  shipmentArea: string;
  sourceTaskMetadataByCode?: Map<string, SheinCostSourceTaskMetadata>;
  sourceProductsByCode: Map<string, SheinCostSourceProductInfo>;
}) {
  const value = draft ?? String(row.manualCostPrice ?? row.fallbackCostPrice ?? "");
  const parsedCost = parseSheinCostDraft(value);

  return (
    <div className="flex flex-col gap-3 rounded-2xl border border-zinc-200 bg-white p-4 lg:flex-row lg:items-center">
      <div className="min-w-0 flex-1">
        <p className="font-medium text-zinc-950">
          {row.groupLabel} · {row.products.length} 个商品
        </p>
        <p className="mt-1 text-xs text-zinc-500">
          {row.products.map((item) => item.skc_name || item.skc_code || "-").join(" / ")}
        </p>
        <p className="mt-1 text-xs text-zinc-500">
          自动/当前成本 {row.fallbackCostPrice ?? "-"}
        </p>
        {row.sourceSDSCodes.length > 0 ? (
          <div className="mt-2 grid gap-1 text-xs text-zinc-600">
            {row.sourceSDSCodes.slice(0, 3).map((sourceCode) => (
              <SheinCostSourceProduct
                fallbackPrice={row.fallbackCostPrice}
                fallbackShipmentArea={shipmentArea}
                key={sourceCode}
                metadata={sourceTaskMetadataByCode?.get(normalizeSourceCode(sourceCode))}
                product={sourceProductsByCode.get(sourceCode)}
                sourceCode={sourceCode}
              />
            ))}
            {row.sourceSDSCodes.length > 3 ? (
              <div className="text-zinc-400">
                还有 {row.sourceSDSCodes.length - 3} 个来源 SDS SKU
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
      <Input
        aria-label={`成本价 ${row.groupLabel}`}
        className="w-full lg:w-40"
        onChange={(event) => onDraftChange(event.target.value)}
        value={value}
      />
      <Button
        disabled={saving || parsedCost.invalid}
        onClick={() =>
          void onSave(
            {
              groupKey: row.groupKey,
              groupLabel: row.groupLabel,
              productId: row.productId,
            },
            parsedCost.value,
          )
        }
        type="button"
      >
        {saving ? "保存中..." : "保存成本价"}
      </Button>
    </div>
  );
}

function SheinCostSourceProduct({
  fallbackPrice,
  fallbackShipmentArea,
  metadata,
  product,
  sourceCode,
}: {
  fallbackPrice?: number | null;
  fallbackShipmentArea?: string;
  metadata?: SheinCostSourceTaskMetadata;
  product?: SheinCostSourceProductInfo;
  sourceCode: string;
}) {
  const title = formatSDSSourceTitle(product) || metadata?.title || "";
  const price = formatSDSSourcePrice(product, metadata?.price ?? fallbackPrice);
  const shipmentArea = formatSDSSourceShipmentArea(product, fallbackShipmentArea);
  const variantLabel = formatSDSSourceVariant(product?.variant) || metadata?.variantLabel || "";

  return (
    <div className="rounded-md bg-zinc-50 px-2 py-1">
      <div className="font-medium text-zinc-800">
        {title || "来源 POD/SDS 商品"}
      </div>
      <div className="mt-0.5 flex flex-wrap gap-x-3 gap-y-1 text-zinc-500">
        <span>POD/SDS: {sourceCode}</span>
        {variantLabel ? <span>变体 {variantLabel}</span> : null}
        {price ? <span>POD 价 {price}</span> : null}
        {shipmentArea ? <span>发货地 {shipmentArea}</span> : null}
      </div>
    </div>
  );
}

export function buildSheinCostGroupRows(
  items: SheinSyncedProductRecord[],
  groups: SheinSDSCostGroupRecord[],
): SheinCostGroupRow[] {
  const groupByKey = new Map<string, SheinSDSCostGroupRecord>();
  for (const group of groups) {
    if (group.group_key) {
      groupByKey.set(group.group_key, group);
    }
  }

  const rowsByKey = new Map<string, SheinCostGroupRow>();
  for (const item of items) {
    const identity = sheinCostGroupIdentity(item);
    const row =
      rowsByKey.get(identity.groupKey) ??
      {
        groupKey: identity.groupKey,
        groupLabel: identity.groupLabel,
        productId: identity.productId,
        products: [],
        sourceSDSCodes: [],
        manualCostPrice: groupByKey.get(identity.groupKey)?.manual_cost_price,
        fallbackCostPrice: null,
      };
    row.products.push(item);
    const sourceSDSCode = sheinSourceSDSCode(item.supplier_code);
    if (sourceSDSCode && !row.sourceSDSCodes.includes(sourceSDSCode)) {
      row.sourceSDSCodes.push(sourceSDSCode);
    }
    row.fallbackCostPrice = maxNullableNumber([
      row.fallbackCostPrice,
      item.manual_cost_price,
      item.effective_cost_price,
      item.auto_cost_price,
    ]);
    rowsByKey.set(identity.groupKey, row);
  }

  return Array.from(rowsByKey.values()).sort((a, b) =>
    a.groupLabel.localeCompare(b.groupLabel),
  );
}

function sheinSourceSDSCode(supplierCode?: string | null) {
  const normalized = supplierCode?.trim() ?? "";
  if (!normalized) {
    return "";
  }
  const parts = normalized.split("-").map((part) => part.trim()).filter(Boolean);
  if (parts.length >= 2 && /^[A-Z0-9]{8}$/i.test(parts.at(-1) ?? "")) {
    return parts.slice(0, -1).join("-");
  }
  return normalized;
}

function sdsSourceSearchKeywords(sourceCode: string) {
  const normalized = sourceCode.trim();
  const parentSKU = sdsParentSKUCandidate(normalized);
  return Array.from(new Set([normalized, parentSKU].filter(Boolean)));
}

function sdsParentSKUCandidate(sourceCode: string) {
  if (sourceCode.length <= 3 || !/\d{3}$/.test(sourceCode)) {
    return "";
  }
  return sourceCode.slice(0, -3);
}

function sheinCostGroupIdentity(item: SheinSyncedProductRecord) {
  const supplierCode = item.supplier_code?.trim() ?? "";
  if (supplierCode) {
    const suffix = sheinSDSStyleSuffix(supplierCode);
    if (suffix) {
      return {
        groupKey: `style:${suffix}`,
        groupLabel: suffix,
      };
    }
    return {
      groupKey: `supplier:${supplierCode}`,
      groupLabel: supplierCode,
    };
  }
  const productId = item.id ?? 0;
  return {
    groupKey: `product:${productId}`,
    groupLabel: item.skc_name || item.skc_code || `商品 ${productId}`,
    productId,
  };
}

function sheinSDSStyleSuffix(supplierCode: string) {
  const suffix = supplierCode.split("-").at(-1)?.trim().toUpperCase() ?? "";
  return /^[A-Z0-9]{8}$/.test(suffix) ? suffix : "";
}

function maxNullableNumber(values: Array<number | null | undefined>) {
  let out: number | null = null;
  for (const value of values) {
    if (typeof value !== "number" || Number.isNaN(value)) {
      continue;
    }
    if (out === null || value > out) {
      out = value;
    }
  }
  return out;
}

type SheinCostSourceProductInfo = {
  product: SDSProductSummary;
  variant?: SDSProductVariant;
};

type SheinCostSourceTaskMetadata = {
  title: string;
  productSKU: string;
  variantSKU: string;
  price?: number;
  variantLabel: string;
};

async function getSheinSourceTaskMetadata(storeId: number, sourceCodes: string[]) {
  const targets = new Set(sourceCodes.map(normalizeSourceCode).filter(Boolean));
  const result = new Map<string, SheinCostSourceTaskMetadata>();
  if (storeId <= 0 || targets.size === 0) {
    return result;
  }

  const response = await getSheinSourceSDSMetadata(storeId, Array.from(targets));
  for (const item of response.items ?? []) {
    const metadata: SheinCostSourceTaskMetadata = {
      title: item.title?.trim() ?? "",
      productSKU: item.product_sku?.trim() ?? "",
      variantSKU: item.variant_sku?.trim() ?? "",
      price: item.price,
      variantLabel: item.variant_label?.trim() ?? "",
    };
    const keys = [
      normalizeSourceCode(item.source_code),
      normalizeSourceCode(metadata.variantSKU),
      normalizeSourceCode(metadata.productSKU),
    ].filter(Boolean);
    for (const key of keys) {
      if (targets.has(key) && metadata.title && !result.has(key)) {
        result.set(key, metadata);
      }
    }
  }
  return result;
}

function normalizeSourceCode(value?: string | null) {
  return value?.trim().toUpperCase() ?? "";
}

function resolveSDSSourceProduct(
  sourceCode: string,
  items?: SDSProductSummary[],
): SheinCostSourceProductInfo | undefined {
  if (!items || items.length === 0) {
    return undefined;
  }
  const normalizedSourceCode = sourceCode.trim().toUpperCase();
  for (const item of items) {
    const variant = item.subproducts?.items?.find(
      (candidate) => candidate.sku?.trim().toUpperCase() === normalizedSourceCode,
    );
    if (variant) {
      return { product: item, variant };
    }
  }
  const product =
    items.find((item) => item.sku?.trim().toUpperCase() === normalizedSourceCode) ??
    items[0];
  return product ? { product } : undefined;
}

function sdsSourceDetailProductID(product?: SheinCostSourceProductInfo) {
  return product?.product.parent_id || product?.variant?.parent_id || product?.product.id || 0;
}

function shouldUseSDSSourceProduct(
  candidate: SheinCostSourceProductInfo,
  current?: SheinCostSourceProductInfo,
) {
  if (!current) {
    return true;
  }
  const candidateTitle = formatSDSSourceTitle(candidate);
  const currentTitle = formatSDSSourceTitle(current);
  if (candidateTitle && !currentTitle) {
    return true;
  }
  return false;
}

function mergeSDSSourceProductInfo(
  detail?: SheinCostSourceProductInfo,
  list?: SheinCostSourceProductInfo,
) {
  if (!detail) {
    return list;
  }
  if (!list || formatSDSSourceTitle(detail) || !formatSDSSourceTitle(list)) {
    return detail;
  }
  return {
    ...detail,
    product: {
      ...detail.product,
      name: detail.product.name || list.product.name,
      product_name: detail.product.product_name || list.product.product_name,
      productName: detail.product.productName || list.product.productName,
      product_name_multi: detail.product.product_name_multi || list.product.product_name_multi,
      declaration_name: detail.product.declaration_name || list.product.declaration_name,
      english_name: detail.product.english_name || list.product.english_name,
      declaration_english_name:
        detail.product.declaration_english_name || list.product.declaration_english_name,
    },
  };
}

function formatSDSSourceTitle(product?: SheinCostSourceProductInfo) {
  const item = product?.product;
  return (
    item?.name?.trim() ||
    item?.product_name?.trim() ||
    item?.productName?.trim() ||
    item?.product_name_multi?.trim() ||
    item?.declaration_name?.trim() ||
    item?.english_name?.trim() ||
    item?.declaration_english_name?.trim() ||
    ""
  );
}

function formatSDSSourcePrice(
  product?: SheinCostSourceProductInfo,
  fallbackPrice?: number | null,
) {
  const price =
    product?.variant?.currentPrice ??
    product?.product.currentPrice ??
    product?.product.min_price ??
    fallbackPrice;
  if (typeof price !== "number" || !Number.isFinite(price) || price <= 0) {
    return "";
  }
  return new Intl.NumberFormat("zh-CN", {
    style: "currency",
    currency: "CNY",
    currencyDisplay: "narrowSymbol",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(price);
}

function formatSDSSourceShipmentArea(
  product?: SheinCostSourceProductInfo,
  fallbackShipmentArea?: string,
) {
  const area = product?.variant?.issuingBayArea ?? product?.product.issuingBayArea;
  const name = area?.name?.trim() ?? "";
  const countryCode = area?.countryCode?.trim() ?? "";
  return [name, countryCode].filter(Boolean).join(" ") || fallbackShipmentArea?.trim() || "";
}

function formatSDSSourceVariant(variant?: SDSProductVariant) {
  if (!variant) {
    return "";
  }
  return [variant.color_name?.trim(), variant.size?.trim()].filter(Boolean).join(" / ");
}

function parseSheinCostDraft(value: string): { invalid: boolean; value: number | null } {
  const trimmed = value.trim();
  if (!trimmed) {
    return { invalid: false, value: null };
  }
  const parsed = Number(trimmed);
  if (!Number.isFinite(parsed)) {
    return { invalid: true, value: null };
  }
  return { invalid: false, value: parsed };
}
