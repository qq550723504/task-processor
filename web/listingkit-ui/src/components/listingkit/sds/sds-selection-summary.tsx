"use client";

import { Button } from "@/components/shared/button";
import type { SDSProductVariantSelection } from "@/lib/types/sds";

export function SDSSelectionSummary({
  selection,
  onChange,
  onClear,
}: {
  selection?: SDSProductVariantSelection;
  onChange: () => void;
  onClear: () => void;
}) {
  if (!selection) {
    return null;
  }

  return (
    <div className="sticky top-4 z-20 rounded-lg border border-emerald-800 bg-emerald-950 px-4 py-4 text-white shadow-lg shadow-emerald-950/15">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div className="space-y-2">
          <div className="text-xs font-semibold uppercase tracking-[0.18em] text-emerald-100/90">
            已选择 SDS 变体
          </div>
          <div className="text-base font-semibold">{selection.productName}</div>
          <div className="flex flex-wrap gap-x-4 gap-y-1 text-sm text-emerald-100">
            <span>变体 ID {selection.variantId}</span>
            <span>{selection.variantLabel}</span>
            <span>模板组 {selection.prototypeGroupId}</span>
            <span>图层 {selection.layerId || "-"}</span>
            {selection.printableWidth && selection.printableHeight ? (
              <span>
                印刷区域 {selection.printableWidth} × {selection.printableHeight}
              </span>
            ) : null}
          </div>
        </div>
        <div className="flex gap-3">
          <Button onClick={onClear} tone="ghost">
            清除
          </Button>
          <Button onClick={onChange} tone="secondary">
            更换变体
          </Button>
        </div>
      </div>
    </div>
  );
}
