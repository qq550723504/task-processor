"use client";

import { useMemo, useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
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
  manualCostPrice?: number | null;
  fallbackCostPrice?: number | null;
};

export function SheinCostPriceTable({
  groups,
  items,
  onSave,
  saving,
}: {
  groups: SheinSDSCostGroupRecord[];
  items: SheinSyncedProductRecord[];
  onSave: (
    target: SheinCostPriceSaveTarget,
    manualCostPrice: number | null,
  ) => Promise<void>;
  saving: boolean;
}) {
  const [drafts, setDrafts] = useState<Record<string, string>>({});
  const rows = useMemo(() => buildSheinCostGroupRows(items, groups), [items, groups]);

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
}: {
  draft?: string;
  onDraftChange: (value: string) => void;
  onSave: (
    target: SheinCostPriceSaveTarget,
    manualCostPrice: number | null,
  ) => Promise<void>;
  row: SheinCostGroupRow;
  saving: boolean;
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
        manualCostPrice: groupByKey.get(identity.groupKey)?.manual_cost_price,
        fallbackCostPrice: null,
      };
    row.products.push(item);
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
