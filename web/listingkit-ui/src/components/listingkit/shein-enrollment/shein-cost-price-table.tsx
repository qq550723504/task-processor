"use client";

import Image from "next/image";
import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";

import { ImagePreviewDialog } from "@/components/listingkit/shein/shein-data-image-gallery-dialog";
import type { SheinPreviewImage } from "@/components/listingkit/shein/shein-preview-image";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { getSheinSourceSDSMetadata } from "@/lib/api/shein-enrollment";
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
  legacyGroupKeys: string[];
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
  const [activeImage, setActiveImage] = useState<SheinPreviewImage | null>(null);
  const rows = useMemo(() => buildSheinCostGroupRows(items, groups), [items, groups]);
  const sourceSDSCodes = useMemo(
    () => Array.from(new Set(rows.flatMap((row) => row.sourceSDSCodes))).sort(),
    [rows],
  );
  const sourceTaskMetadataQuery = useQuery({
    queryKey: [
      "listingkit",
      "shein-enrollment",
      "source-sds-task-metadata-v5",
      storeId,
      sourceSDSCodes,
    ],
    queryFn: () =>
      getSheinSourceTaskMetadata(storeId, sourceSDSCodes),
    enabled: storeId > 0 && sourceSDSCodes.length > 0,
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
          onPreviewImage={setActiveImage}
          sourceTaskMetadataByCode={sourceTaskMetadataQuery.data}
        />
      ))}
      <ImagePreviewDialog
        activeImage={activeImage}
        activeImageCanRegenerate={false}
        canRegenerate={false}
        onClose={() => setActiveImage(null)}
        regenerationPrompt=""
        setRegenerationPrompt={() => undefined}
      />
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
  onPreviewImage,
  sourceTaskMetadataByCode,
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
  onPreviewImage: (image: SheinPreviewImage) => void;
  sourceTaskMetadataByCode?: Map<string, SheinCostSourceTaskMetadata>;
}) {
  const value = draft ?? String(row.manualCostPrice ?? "");
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
        {row.sourceSDSCodes.length > 0 ? (
          <div className="mt-2 grid gap-1 text-xs text-zinc-600">
            {row.sourceSDSCodes.slice(0, 3).map((sourceCode) => (
              <SheinCostSourceProduct
                fallbackShipmentArea={shipmentArea}
                key={sourceCode}
                metadata={sourceTaskMetadataByCode?.get(normalizeSourceCode(sourceCode))}
                onPreviewImage={onPreviewImage}
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
  fallbackShipmentArea,
  metadata,
  onPreviewImage,
  sourceCode,
}: {
  fallbackShipmentArea?: string;
  metadata?: SheinCostSourceTaskMetadata;
  onPreviewImage: (image: SheinPreviewImage) => void;
  sourceCode: string;
}) {
  const title = metadata?.title || "";
  const price = formatSDSSourcePrice(metadata?.price);
  const shipmentArea = fallbackShipmentArea?.trim() || "";
  const variantLabel = metadata?.variantLabel || "";
  const imageURL = metadata?.imageURL || "";
  const imageLabel = `${title || sourceCode}首图`;

  return (
    <div className="flex gap-2 rounded-md bg-zinc-50 px-2 py-1">
      {imageURL ? (
        <div className="group relative h-12 w-12 shrink-0">
          <button
            aria-label={`查看${imageLabel}`}
            className="h-12 w-12 cursor-zoom-in overflow-hidden rounded border border-zinc-200 bg-white transition hover:border-zinc-400 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-zinc-950 focus-visible:ring-offset-2"
            onClick={() =>
              onPreviewImage({
                id: `source-sds:${sourceCode}`,
                label: imageLabel,
                url: imageURL,
              })
            }
            type="button"
          >
            <Image
              alt={`${title || sourceCode} 首图`}
              className="h-full w-full object-cover"
              height={48}
              loading="lazy"
              src={imageURL}
              unoptimized
              width={48}
            />
          </button>
          <div className="pointer-events-none absolute left-0 top-14 z-30 hidden h-60 w-60 overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-xl group-hover:block group-focus-within:block sm:left-14 sm:top-1/2 sm:-translate-y-1/2">
            <Image
              alt={`${title || sourceCode} 悬浮预览`}
              className="h-full w-full object-contain"
              height={240}
              loading="lazy"
              src={imageURL}
              unoptimized
              width={240}
            />
          </div>
        </div>
      ) : null}
      <div className="min-w-0 flex-1">
        <div className="truncate font-medium text-zinc-800">
          {title || "来源 POD/SDS 商品"}
        </div>
        <div className="mt-0.5 flex flex-wrap gap-x-3 gap-y-1 text-zinc-500">
          <span>POD/SDS: {sourceCode}</span>
          {variantLabel ? <span>变体 {variantLabel}</span> : null}
          {price ? <span>POD 价 {price}</span> : null}
          {shipmentArea ? <span>发货地 {shipmentArea}</span> : null}
        </div>
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
        legacyGroupKeys: identity.legacyGroupKeys,
        productId: identity.productId,
        products: [],
        sourceSDSCodes: [],
        manualCostPrice: findSheinCostGroupRecord(groupByKey, identity)?.manual_cost_price,
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

function sheinCostGroupIdentity(item: SheinSyncedProductRecord) {
  const supplierCode = item.supplier_code?.trim() ?? "";
  if (supplierCode) {
    const sourceSDSCode = sheinSourceSDSCode(supplierCode);
    const suffix = sheinSDSStyleSuffix(supplierCode);
    if (sourceSDSCode) {
      return {
        groupKey: `source:${sourceSDSCode}`,
        groupLabel: sourceSDSCode,
        legacyGroupKeys: [
          suffix ? `style:${suffix}` : "",
          `supplier:${supplierCode}`,
        ].filter(Boolean),
      };
    }
    if (suffix) {
      return {
        groupKey: `style:${suffix}`,
        groupLabel: suffix,
        legacyGroupKeys: [],
      };
    }
    return {
      groupKey: `supplier:${supplierCode}`,
      groupLabel: supplierCode,
      legacyGroupKeys: [],
    };
  }
  const productId = item.id ?? 0;
  return {
    groupKey: `product:${productId}`,
    groupLabel: item.skc_name || item.skc_code || `商品 ${productId}`,
    legacyGroupKeys: [],
    productId,
  };
}

function findSheinCostGroupRecord(
  groupByKey: Map<string, SheinSDSCostGroupRecord>,
  identity: ReturnType<typeof sheinCostGroupIdentity>,
) {
  return (
    groupByKey.get(identity.groupKey) ??
    identity.legacyGroupKeys.map((key) => groupByKey.get(key)).find(Boolean)
  );
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

type SheinCostSourceTaskMetadata = {
  title: string;
  productSKU: string;
  variantSKU: string;
  price?: number;
  variantLabel: string;
  imageURL: string;
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
      imageURL: item.image_url?.trim() ?? "",
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

function formatSDSSourcePrice(price?: number | null) {
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
