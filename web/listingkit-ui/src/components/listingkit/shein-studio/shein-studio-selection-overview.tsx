import type { ReactNode } from "react";

import type { SDSProductVariantSelection } from "@/lib/types/sds";

export function SheinStudioSelectionOverview({
  footer,
  printableAreaLabel,
  selectedColorCount,
  selectedSizeCount,
  selectedVariantCount,
  selection,
}: {
  footer?: ReactNode;
  printableAreaLabel: string;
  selectedColorCount: number;
  selectedSizeCount: number;
  selectedVariantCount: number;
  selection?: SDSProductVariantSelection;
}) {
  return (
    <div className="rounded-2xl border border-dashed border-zinc-200 bg-zinc-50/90 px-4 py-4">
      <div className="flex flex-col gap-4 xl:flex-row xl:items-start xl:justify-between">
        <div className="max-w-2xl">
          <p className="text-[10px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
            入口商品
          </p>
          <p className="mt-1 text-sm font-medium text-zinc-900">
            {selection?.productName ?? "这款商品是创建该批次时最先带入的规格。"}
          </p>
          <p className="mt-1 text-xs leading-6 text-zinc-500">
            这款商品是创建该批次时最先带入的规格，用于记录批次起点，不代表当前批次只围绕它生成。
          </p>
        </div>

        <div className="grid gap-2 sm:grid-cols-2 xl:grid-cols-3">
          <OverviewMetric label="入口变体" value={String(selection?.variantId ?? "未选择")} />
          <OverviewMetric label="印刷区域" value={printableAreaLabel} />
          <OverviewMetric
            label="规格覆盖"
            value={
              selectedVariantCount > 0
                ? `${selectedColorCount} 个颜色 · ${selectedSizeCount} 个尺码 · ${selectedVariantCount} 个 SKU`
                : "请先选择商品"
            }
          />
        </div>
      </div>
      {footer ? <div className="mt-4 border-t border-zinc-200/80 pt-3">{footer}</div> : null}
    </div>
  );
}

function OverviewMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-2xl border border-zinc-200 bg-white px-3 py-3">
      <div className="text-[10px] uppercase tracking-[0.2em] text-zinc-400">
        {label}
      </div>
      <div className="mt-1 text-sm font-semibold text-zinc-900">{value}</div>
    </div>
  );
}
