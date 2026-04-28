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
    <div className="sticky top-4 z-20 rounded-[1.75rem] border border-emerald-900/80 bg-[linear-gradient(135deg,_#052e2b,_#115e59)] px-5 py-5 text-white shadow-[0_18px_50px_rgba(5,46,43,0.28)]">
      <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div className="space-y-2">
          <div className="text-[11px] font-semibold uppercase tracking-[0.28em] text-emerald-100/90">
            当前 SDS 选择
          </div>
          <div className="text-lg font-semibold tracking-[-0.03em]">{selection.productName}</div>
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
