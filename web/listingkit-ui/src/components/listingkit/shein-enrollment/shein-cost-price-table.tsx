"use client";

import { useState } from "react";

import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { SheinSyncedProductRecord } from "@/lib/types/listingkit/shein-enrollment";

export function SheinCostPriceTable({
  items,
  onSave,
  saving,
}: {
  items: SheinSyncedProductRecord[];
  onSave: (productId: number, manualCostPrice: number | null) => Promise<void>;
  saving: boolean;
}) {
  const [drafts, setDrafts] = useState<Record<number, string>>({});

  return (
    <div className="grid gap-3">
      {items.length === 0 ? (
        <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-6 text-sm text-zinc-500">
          当前没有可维护成本价的同步商品。
        </div>
      ) : null}
      {items.map((item) => (
        <div
          key={item.id}
          className="flex flex-col gap-3 rounded-2xl border border-zinc-200 bg-white p-4 lg:flex-row lg:items-center"
        >
          <div className="min-w-0 flex-1">
            <p className="font-medium text-zinc-950">
              {item.product_name_multi || item.spu_name || "-"}
            </p>
            <p className="mt-1 text-xs text-zinc-500">
              {item.skc_name || item.skc_code || "-"} · 自动成本 {item.auto_cost_price ?? "-"}
            </p>
          </div>
          <Input
            aria-label={`成本价 ${item.product_name_multi || item.spu_name || item.id}`}
            className="w-full lg:w-40"
            onChange={(event) =>
              setDrafts((current) => ({
                ...current,
                [item.id ?? 0]: event.target.value,
              }))
            }
            value={
              drafts[item.id ?? 0] ??
              String(item.manual_cost_price ?? item.auto_cost_price ?? "")
            }
          />
          <Button
            disabled={saving || !item.id}
            onClick={() =>
              void onSave(
                item.id ?? 0,
                drafts[item.id ?? 0]?.trim()
                  ? Number(drafts[item.id ?? 0])
                  : null,
              )
            }
            type="button"
          >
            {saving ? "保存中..." : "保存成本价"}
          </Button>
        </div>
      ))}
    </div>
  );
}
