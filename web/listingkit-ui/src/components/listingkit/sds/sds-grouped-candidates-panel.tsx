"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import { buildGroupedSDSSelectionID } from "@/lib/types/sds-baseline";

export function SDSGroupedCandidatesPanel({
  items,
  activeSelection,
  onRemove,
  onSelect,
}: {
  items: SDSProductVariantSelection[];
  activeSelection?: SDSProductVariantSelection;
  onRemove: (selection: SDSProductVariantSelection) => void;
  onSelect: (selection: SDSProductVariantSelection) => void;
}) {
  if (items.length === 0) {
    return null;
  }

  const activeSelectionId = buildGroupedSDSSelectionID(activeSelection);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
            批量候选池
          </div>
          <p className="mt-1 text-sm text-zinc-600">
            这里存放准备进入 grouped SDS 批量上品的候选商品，可以随时回选或移除。
          </p>
        </div>
        <Badge className="rounded-md px-3 py-2 text-sm" variant="neutral">
          {items.length} 款候选
        </Badge>
      </div>
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {items.map((item) => {
          const active = buildGroupedSDSSelectionID(item) === activeSelectionId;
          return (
            <div
              className={`rounded-[1.5rem] border px-4 py-4 shadow-sm ${
                active
                  ? "border-emerald-800 bg-[linear-gradient(135deg,_#052e2b,_#115e59)] text-white"
                  : "border-zinc-200 bg-white"
              }`}
              key={buildGroupedSDSSelectionID(item)}
            >
              <div className="space-y-2">
                <div className="line-clamp-2 text-sm font-semibold leading-6">
                  {item.productName}
                </div>
                <div className={active ? "text-emerald-100" : "text-zinc-500"}>
                  变体 ID {item.variantId}
                </div>
                <div className={active ? "text-emerald-100" : "text-zinc-500"}>
                  {item.variantLabel}
                </div>
                {item.printableWidth && item.printableHeight ? (
                  <div className={active ? "text-emerald-100" : "text-zinc-500"}>
                    印刷区域 {item.printableWidth} × {item.printableHeight}
                  </div>
                ) : null}
                <div className="flex gap-2 pt-1">
                  <Button
                    className="flex-1"
                    onClick={() => onSelect(item)}
                    type="button"
                    variant={active ? "secondary" : "primary"}
                  >
                    {active ? "当前已选" : "回选这个变体"}
                  </Button>
                  <Button
                    onClick={() => onRemove(item)}
                    type="button"
                    variant="ghost"
                  >
                    移除
                  </Button>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
