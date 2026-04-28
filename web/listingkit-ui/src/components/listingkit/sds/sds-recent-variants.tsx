"use client";

import { Button } from "@/components/shared/button";
import type { SDSProductVariantSelection } from "@/lib/types/sds";

export function SDSRecentVariants({
  items,
  activeVariantId,
  onSelect,
}: {
  items: SDSProductVariantSelection[];
  activeVariantId?: number;
  onSelect: (selection: SDSProductVariantSelection) => void;
}) {
  if (items.length === 0) {
    return null;
  }

  return (
    <div className="space-y-3">
      <div className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
        最近使用的变体
      </div>
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {items.map((item) => {
          const active = activeVariantId === item.variantId;
          return (
            <div
              className={`rounded-[1.5rem] border px-4 py-4 shadow-sm ${
                active
                  ? "border-emerald-800 bg-[linear-gradient(135deg,_#052e2b,_#115e59)] text-white"
                  : "border-zinc-200 bg-white"
              }`}
              key={item.variantId}
            >
              <div className="space-y-2">
                <div className="line-clamp-2 text-sm font-semibold leading-6">{item.productName}</div>
                <div className={active ? "text-emerald-100" : "text-zinc-500"}>
                  变体 ID {item.variantId}
                </div>
                <div className={active ? "text-emerald-100" : "text-zinc-500"}>
                  {item.variantLabel}
                </div>
                <Button
                  className="w-full"
                  onClick={() => onSelect(item)}
                  tone={active ? "secondary" : "primary"}
                  type="button"
                >
                  {active ? "已选择" : "使用这个变体"}
                </Button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
