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
  SheinSourceSDSCostGroupRecord,
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
  productCount: number;
  products: SheinSyncedProductRecord[];
  sourceSDSCodes: string[];
  skuCodes: string[];
  skuGroups: SheinCostSKUGroup[];
  manualCostPrice?: number | null;
  fallbackCostPrice?: number | null;
};

type SheinCostSKUGroup = {
  groupKey: string;
  groupLabel: string;
  skuCode: string;
  variantLabel: string;
  skuCodes: string[];
  productCount: number;
  products: SheinSyncedProductRecord[];
  manualCostPrice?: number | null;
};

export function SheinCostPriceTable({
  groups,
  items,
  onSave,
  onSyncSourceSDSProduct,
  saving,
  shipmentArea = "US",
  sourceGroups,
  storeId,
  syncingSourceCode,
}: {
  groups: SheinSDSCostGroupRecord[];
  items: SheinSyncedProductRecord[];
  onSave: (
    target: SheinCostPriceSaveTarget,
    manualCostPrice: number | null,
  ) => Promise<void>;
  onSyncSourceSDSProduct?: (sourceCode: string) => Promise<void>;
  saving: boolean;
  shipmentArea?: string;
  sourceGroups?: SheinSourceSDSCostGroupRecord[];
  storeId: number;
  syncingSourceCode?: string;
}) {
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const [activeImage, setActiveImage] = useState<SheinPreviewImage | null>(null);
  const rows = useMemo(
    () =>
      sourceGroups
        ? buildSheinCostGroupRowsFromSourceGroups(sourceGroups)
        : buildSheinCostGroupRows(items, groups),
    [groups, items, sourceGroups],
  );
  const sourceSDSCodes = useMemo(
    () =>
      Array.from(
        new Set(
          rows.flatMap((row) => [
            ...row.sourceSDSCodes,
            ...row.skuGroups
              .map((skuGroup) => skuGroup.skuCode)
              .filter(isLikelySourceSDSCode),
          ]),
        ),
      ).sort(),
    [rows],
  );
  const sourceTaskMetadataQuery = useQuery({
    queryKey: [
      "listingkit",
      "shein-enrollment",
      "source-sds-task-metadata-v6",
      storeId,
      sourceSDSCodes,
    ],
    queryFn: () =>
      getSheinSourceTaskMetadata(storeId, sourceSDSCodes),
    enabled: storeId > 0 && sourceSDSCodes.length > 0,
    staleTime: 10 * 60 * 1000,
  });
  const displayRows = useMemo(
    () =>
      rows.map((row) =>
        simplifySourceSDSVariantGroups(
          applySourceSDSMetadataSKUGroups(row, sourceTaskMetadataQuery.data),
        ),
      ),
    [rows, sourceTaskMetadataQuery.data],
  );

  return (
    <div className="grid gap-3">
      {displayRows.length === 0 ? (
        <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-6 text-sm text-zinc-500">
          当前没有可维护成本价的同步商品。
        </div>
      ) : null}
      {displayRows.map((row) => (
        <SheinCostPriceRow
          drafts={drafts}
          key={row.groupKey}
          onDraftChange={(key, value) =>
            setDrafts((current) => ({
              ...current,
              [key]: value,
            }))
          }
          onSave={onSave}
          onSyncSourceSDSProduct={onSyncSourceSDSProduct}
          row={row}
          saving={saving}
          shipmentArea={shipmentArea}
          onPreviewImage={setActiveImage}
          sourceTaskMetadataByCode={sourceTaskMetadataQuery.data}
          syncingSourceCode={syncingSourceCode}
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
  drafts,
  onDraftChange,
  onSave,
  onSyncSourceSDSProduct,
  row,
  saving,
  shipmentArea,
  onPreviewImage,
  sourceTaskMetadataByCode,
  syncingSourceCode,
}: {
  drafts: Record<string, string>;
  onDraftChange: (key: string, value: string) => void;
  onSave: (
    target: SheinCostPriceSaveTarget,
    manualCostPrice: number | null,
  ) => Promise<void>;
  onSyncSourceSDSProduct?: (sourceCode: string) => Promise<void>;
  row: SheinCostGroupRow;
  saving: boolean;
  shipmentArea: string;
  onPreviewImage: (image: SheinPreviewImage) => void;
  sourceTaskMetadataByCode?: Map<string, SheinCostSourceTaskMetadata>;
  syncingSourceCode?: string;
}) {
  const usesSKUCosts = row.skuGroups.length > 0;
  const value = drafts[row.groupKey] ?? String(row.manualCostPrice ?? "");
  const parsedCost = parseSheinCostDraft(value);
  const syncSourceCode = row.sourceSDSCodes[0] ?? "";
  const syncing = Boolean(
    syncSourceCode &&
      normalizeSourceCode(syncingSourceCode) === normalizeSourceCode(syncSourceCode),
  );

  return (
    <div className="rounded-2xl border border-zinc-200 bg-white p-4">
      <div className="flex flex-col gap-3 lg:flex-row lg:items-start">
        <div className="min-w-0 flex-1">
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center">
            <p className="font-medium text-zinc-950">
              {row.groupLabel} · {row.productCount} 个商品
            </p>
            {syncSourceCode && onSyncSourceSDSProduct ? (
              <Button
                aria-label={`同步该产品 ${syncSourceCode}`}
                className="h-8 w-fit px-3 text-xs"
                disabled={syncing}
                onClick={() => void onSyncSourceSDSProduct(syncSourceCode)}
                type="button"
                variant="outline"
              >
                {syncing ? "同步中..." : "同步该产品"}
              </Button>
            ) : null}
          </div>
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
        {!usesSKUCosts ? (
          <div className="flex w-full flex-col gap-2 lg:w-40">
            <Input
              aria-label={`成本价 ${row.groupLabel}`}
              className="w-full"
              onChange={(event) => onDraftChange(row.groupKey, event.target.value)}
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
        ) : null}
      </div>
      <SheinCostVariantDetailTable
        drafts={drafts}
        onDraftChange={onDraftChange}
        onSave={onSave}
        row={row}
        saving={saving}
        sourceTaskMetadataByCode={sourceTaskMetadataByCode}
      />
    </div>
  );
}

function SheinCostVariantDetailTable({
  drafts,
  onDraftChange,
  onSave,
  row,
  saving,
  sourceTaskMetadataByCode,
}: {
  drafts: Record<string, string>;
  onDraftChange: (key: string, value: string) => void;
  onSave: (
    target: SheinCostPriceSaveTarget,
    manualCostPrice: number | null,
  ) => Promise<void>;
  row: SheinCostGroupRow;
  saving: boolean;
  sourceTaskMetadataByCode?: Map<string, SheinCostSourceTaskMetadata>;
}) {
  const details = row.products.slice(0, 5).map((product) => {
    const skuCodes = sheinSyncedProductSKUCodes(product);
    return {
      key: String(product.id ?? product.skc_name ?? product.skc_code ?? product.supplier_code ?? ""),
      skc: product.skc_name || product.skc_code || "-",
      supplierCode: product.supplier_code || "-",
      skuCodes: skuCodes.length > 0 ? skuCodes : row.skuCodes,
    };
  });
  if (details.length === 0 && row.skuCodes.length === 0) {
    return null;
  }
  const usesSKUCosts = row.skuGroups.length > 0;
  const expandedProductCount = usesSKUCosts
    ? row.skuGroups.reduce((total, skuGroup) => total + skuGroup.productCount, 0)
    : details.length;

  return (
    <div className="mt-3 overflow-hidden rounded-lg border border-zinc-200">
      <table aria-label={`${row.groupLabel} 明细`} className="w-full text-left text-xs">
        <thead className="bg-zinc-50 text-zinc-500">
          <tr>
            <th className="px-3 py-2 font-medium">{usesSKUCosts ? "底款变体" : "SHEIN SKC"}</th>
            <th className="px-3 py-2 font-medium">{usesSKUCosts ? "商品数" : "供应商编码"}</th>
            <th className="px-3 py-2 font-medium">关联 SHEIN SKU</th>
            {usesSKUCosts ? <th className="px-3 py-2 font-medium">成本价</th> : null}
          </tr>
        </thead>
        <tbody className="divide-y divide-zinc-100 text-zinc-700">
          {usesSKUCosts ? row.skuGroups.map((skuGroup) => (
            <SheinCostSKUDetailRow
              draft={drafts[skuGroup.groupKey]}
              key={skuGroup.groupKey}
              onDraftChange={(value) => onDraftChange(skuGroup.groupKey, value)}
              onSave={onSave}
              row={row}
              saving={saving}
              skuGroup={skuGroup}
              sourceTaskMetadata={sourceTaskMetadataByCode?.get(normalizeSourceCode(skuGroup.skuCode))}
            />
          )) : details.map((detail) => (
            <tr key={detail.key}>
              <td className="px-3 py-2">{detail.skc}</td>
              <td className="px-3 py-2">{detail.supplierCode}</td>
              <td className="px-3 py-2">{formatSheinSKUCodes(detail.skuCodes)}</td>
            </tr>
          ))}
          {!usesSKUCosts && details.length === 0 ? (
            <tr>
              <td className="px-3 py-2 text-zinc-500" colSpan={2}>
                SKU 明细
              </td>
              <td className="px-3 py-2">{formatSheinSKUCodes(row.skuCodes)}</td>
            </tr>
          ) : null}
        </tbody>
      </table>
      {row.productCount > expandedProductCount ? (
        <div className="border-t border-zinc-100 bg-zinc-50 px-3 py-2 text-xs text-zinc-500">
          还有 {row.productCount - expandedProductCount} 个 SHEIN 商品未展开
        </div>
      ) : null}
    </div>
  );
}

function SheinCostSKUDetailRow({
  draft,
  onDraftChange,
  onSave,
  row,
  saving,
  skuGroup,
  sourceTaskMetadata,
}: {
  draft?: string;
  onDraftChange: (value: string) => void;
  onSave: (
    target: SheinCostPriceSaveTarget,
    manualCostPrice: number | null,
  ) => Promise<void>;
  row: SheinCostGroupRow;
  saving: boolean;
  skuGroup: SheinCostSKUGroup;
  sourceTaskMetadata?: SheinCostSourceTaskMetadata;
}) {
  const value = draft ?? String(skuGroup.manualCostPrice ?? "");
  const parsedCost = parseSheinCostDraft(value);
  const variantLabel = formatSDSVariantDisplayLabel(
    sourceTaskMetadata?.variantLabel || skuGroup.variantLabel || skuGroup.skuCode || skuGroup.groupLabel,
  );
  const sourcePrice = formatSDSSourcePrice(sourceTaskMetadata?.price);
  return (
    <tr>
      <td className="px-3 py-2">
        <div className="font-medium text-zinc-900">{variantLabel}</div>
        {sourcePrice ? (
          <div className="mt-0.5 text-zinc-500">
            POD/SDS: {skuGroup.skuCode} · POD 价 {sourcePrice}
          </div>
        ) : null}
      </td>
      <td className="px-3 py-2">{skuGroup.productCount > 0 ? `${skuGroup.productCount} 个商品` : "-"}</td>
      <td className="px-3 py-2">{formatSheinSKUCodes(skuGroup.skuCodes)}</td>
      <td className="px-3 py-2">
        <div className="flex min-w-52 gap-2">
          <Input
            aria-label={`成本价 ${skuGroup.groupLabel}`}
            className="h-8 w-28"
            onChange={(event) => onDraftChange(event.target.value)}
            value={value}
          />
          <Button
            className="h-8 px-3"
            disabled={saving || parsedCost.invalid}
            onClick={() =>
              void onSave(
                {
                  groupKey: skuGroup.groupKey,
                  groupLabel: skuGroup.groupLabel,
                },
                parsedCost.value,
              )
            }
            type="button"
          >
            保存
          </Button>
        </div>
      </td>
    </tr>
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
  const variantLabel = formatSDSVariantDisplayLabel(metadata?.variantLabel || "");
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
        productCount: 0,
        products: [],
        sourceSDSCodes: [],
        skuCodes: [],
        skuGroups: [],
        manualCostPrice: findSheinCostGroupRecord(groupByKey, identity)?.manual_cost_price,
        fallbackCostPrice: null,
      };
    row.products.push(item);
    row.productCount = row.products.length;
    const sourceSDSCode = sheinSourceSDSCode(item.supplier_code);
    if (sourceSDSCode && !row.sourceSDSCodes.includes(sourceSDSCode)) {
      row.sourceSDSCodes.push(sourceSDSCode);
    }
    row.skuCodes = mergeSheinSKUCodes(row.skuCodes, sheinSyncedProductSKUCodes(item));
    row.skuGroups = buildSheinCostSKUGroups(row.groupKey, row.groupLabel, row.skuCodes, row.manualCostPrice);
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

export function buildSheinCostGroupRowsFromSourceGroups(
  sourceGroups: SheinSourceSDSCostGroupRecord[],
): SheinCostGroupRow[] {
  return sourceGroups
    .map((group) => {
      const sourceCode = group.source_code?.trim() || group.group_label?.trim() || "";
      const groupKey = group.group_key?.trim() || (sourceCode ? `source:${sourceCode}` : "");
      const groupLabel = group.group_label?.trim() || sourceCode || groupKey;
      return {
        groupKey,
        groupLabel,
        legacyGroupKeys: group.legacy_group_keys ?? [],
        productCount: group.product_count ?? group.products?.length ?? 0,
        products: group.products ?? [],
        sourceSDSCodes: sourceCode ? [sourceCode] : [],
        skuCodes: mergeSheinSKUCodes(
          normalizeSheinSKUCodeList(group.sku_codes),
          (group.products ?? []).flatMap(sheinSyncedProductSKUCodes),
        ),
        skuGroups: buildSheinSourceSKUGroups(group),
        manualCostPrice: group.manual_cost_price,
        fallbackCostPrice: null,
      };
    })
    .filter((row) => row.groupKey && row.groupLabel)
    .sort((a, b) => a.groupLabel.localeCompare(b.groupLabel));
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

function sheinSyncedProductSKUCodes(item: SheinSyncedProductRecord) {
  const raw = item.site_snapshot?.trim();
  if (!raw) {
    return [];
  }
  try {
    const payload = JSON.parse(raw) as {
      sku_codes?: unknown;
      sku_info?: Array<{ sku_code?: unknown }>;
    };
    const codes = Array.isArray(payload.sku_codes) ? payload.sku_codes : [];
    const skuInfoCodes = Array.isArray(payload.sku_info)
      ? payload.sku_info.map((sku) => sku.sku_code)
      : [];
    return normalizeSheinSKUCodeList([...codes, ...skuInfoCodes]);
  } catch {
    return [];
  }
}

function normalizeSheinSKUCodeList(values?: unknown[]) {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const value of values ?? []) {
    if (typeof value !== "string") {
      continue;
    }
    const code = value.trim().toUpperCase();
    if (!code || seen.has(code)) {
      continue;
    }
    seen.add(code);
    out.push(code);
  }
  return out.sort((a, b) => a.localeCompare(b));
}

function mergeSheinSKUCodes(current: string[], next: string[]) {
  return normalizeSheinSKUCodeList([...current, ...next]);
}

function buildSheinCostSKUGroups(
  groupKey: string,
  groupLabel: string,
  skuCodes: string[],
  manualCostPrice?: number | null,
) {
  return skuCodes.map((skuCode) => ({
    groupKey: `${groupKey}:sku:${skuCode}`,
    groupLabel: `${groupLabel} / ${skuCode}`,
    skuCode,
    variantLabel: skuCode,
    skuCodes: [skuCode],
    productCount: 0,
    products: [],
    manualCostPrice,
  }));
}

function buildSheinSourceSKUGroups(group: SheinSourceSDSCostGroupRecord) {
  const sourceCode = group.source_code?.trim() || group.group_label?.trim() || "";
  const groupKey = group.group_key?.trim() || (sourceCode ? `source:${sourceCode}` : "");
  const groupLabel = group.group_label?.trim() || sourceCode || groupKey;
  const fromAPI = (group.sku_groups ?? [])
    .map((skuGroup) => {
      const variantLabel = skuGroup.variant_label?.trim() || skuGroup.sku_code?.trim() || "";
      const skuCode = skuGroup.sku_code?.trim() || variantLabel;
      const skuGroupKey = skuGroup.group_key?.trim() || (skuCode ? `${groupKey}:variant:${skuCode}` : "");
      const skuGroupLabel = skuGroup.group_label?.trim() || (variantLabel ? `${groupLabel} / ${variantLabel}` : skuGroupKey);
      return {
        groupKey: skuGroupKey,
        groupLabel: skuGroupLabel,
        skuCode,
        variantLabel,
        skuCodes: normalizeSheinSKUCodeList(skuGroup.sku_codes),
        productCount: skuGroup.product_count ?? skuGroup.products?.length ?? 0,
        products: skuGroup.products ?? [],
        manualCostPrice: skuGroup.manual_cost_price,
      };
    })
    .filter((skuGroup) => skuGroup.groupKey && skuGroup.groupLabel && skuGroup.variantLabel);
  if (fromAPI.length > 0) {
    return fromAPI;
  }
  return buildSheinCostSKUGroups(
    groupKey,
    groupLabel,
    normalizeSheinSKUCodeList(group.sku_codes),
    group.manual_cost_price,
  );
}

function formatSheinSKUCodes(skuCodes: string[]) {
  return skuCodes.length > 0 ? skuCodes.join(" / ") : "-";
}

type SheinCostSourceTaskMetadata = {
  sourceCode: string;
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
      sourceCode: item.source_code?.trim() ?? "",
      title: item.title?.trim() ?? "",
      productSKU: item.product_sku?.trim() ?? "",
      variantSKU: item.variant_sku?.trim() ?? "",
      price: item.price,
      variantLabel: item.variant_label?.trim() ?? "",
      imageURL: item.image_url?.trim() ?? "",
    };
    for (const key of [metadata.sourceCode, metadata.variantSKU].map(normalizeSourceCode).filter(Boolean)) {
      if (metadata.title && !result.has(key)) {
        result.set(key, metadata);
      }
    }
    const productKey = normalizeSourceCode(metadata.productSKU);
    if (productKey && targets.has(productKey) && metadata.title && !result.has(productKey)) {
      result.set(productKey, metadata);
    }
  }
  return result;
}

function applySourceSDSMetadataSKUGroups(
  row: SheinCostGroupRow,
  sourceTaskMetadataByCode?: Map<string, SheinCostSourceTaskMetadata>,
) {
  if (!sourceTaskMetadataByCode || row.sourceSDSCodes.length === 0) {
    return row;
  }
  const sourceCodes = new Set(row.sourceSDSCodes.map(normalizeSourceCode).filter(Boolean));
  const variants = uniqueSheinSourceMetadata(Array.from(sourceTaskMetadataByCode.values()))
    .filter((metadata) => {
      const sourceCode = normalizeSourceCode(metadata.sourceCode || metadata.variantSKU);
      const productSKU = normalizeSourceCode(metadata.productSKU);
      return sourceCodes.has(sourceCode) || sourceCodes.has(productSKU);
    })
    .sort((a, b) =>
      normalizeSourceCode(a.sourceCode || a.variantSKU).localeCompare(
        normalizeSourceCode(b.sourceCode || b.variantSKU),
      ),
    );
  if (variants.length <= 1) {
    return row;
  }
  return {
    ...row,
    skuGroups: variants.map((metadata) => {
      const sourceCode = normalizeSourceCode(metadata.sourceCode || metadata.variantSKU || metadata.productSKU);
      const existing = row.skuGroups.find((skuGroup) => normalizeSourceCode(skuGroup.skuCode) === sourceCode);
      return {
        groupKey: existing?.groupKey || `${row.groupKey}:variant:${sourceCode}`,
        groupLabel: existing?.groupLabel || `${row.groupLabel} / ${sourceCode}`,
        skuCode: sourceCode,
        variantLabel: metadata.variantLabel || sourceCode,
        skuCodes: existing?.skuCodes ?? [],
        productCount: existing?.productCount && existing.skuCode === sourceCode ? existing.productCount : row.productCount,
        products: existing?.products ?? [],
        manualCostPrice: existing?.manualCostPrice ?? (row.skuGroups.length === 0 ? row.manualCostPrice : undefined),
      };
    }),
  };
}

function uniqueSheinSourceMetadata(items: SheinCostSourceTaskMetadata[]) {
  const seen = new Set<string>();
  const out: SheinCostSourceTaskMetadata[] = [];
  for (const item of items) {
    const key = normalizeSourceCode(item.sourceCode || item.variantSKU || item.productSKU);
    if (!key || seen.has(key)) {
      continue;
    }
    seen.add(key);
    out.push(item);
  }
  return out;
}

function simplifySourceSDSVariantGroups(row: SheinCostGroupRow) {
  if (row.skuGroups.length <= 1) {
    return row;
  }
  const sizeGroups = row.skuGroups.filter((skuGroup) =>
    isLikelySDSSizeVariant(
      skuGroup.variantLabel || skuGroup.skuCode || skuGroup.groupLabel,
    ),
  );
  if (sizeGroups.length === 0 || sizeGroups.length === row.skuGroups.length) {
    return row;
  }
  return {
    ...row,
    skuGroups: sizeGroups,
  };
}

function isLikelySDSSizeVariant(value?: string | null) {
  const normalized = value?.trim().toLowerCase() ?? "";
  if (!normalized) {
    return false;
  }
  return (
    /\d+\s*[*x×]\s*\d+/.test(normalized) ||
    /\d+(?:\.\d+)?\s*(?:cm|mm|inch|in|英寸|厘米|毫米)\b/.test(normalized)
  );
}

function isLikelySourceSDSCode(value?: string | null) {
  return /^[A-Z]{1,4}\d{6,}/i.test(value?.trim() ?? "");
}

function normalizeSourceCode(value?: string | null) {
  return value?.trim().toUpperCase() ?? "";
}

function formatSDSVariantDisplayLabel(value?: string | null) {
  const label = value?.trim() ?? "";
  if (!label.includes("/")) {
    return label;
  }
  return label.split("/").map((part) => part.trim()).filter(Boolean).at(-1) ?? label;
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
