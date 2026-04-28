import type { SDSProductVariantSelection } from "@/lib/types/sds";

export function SheinStudioSelectionOverview({
  printableAreaLabel,
  selectedColorCount,
  selectedSizeCount,
  selectedVariantCount,
  selection,
}: {
  printableAreaLabel: string;
  selectedColorCount: number;
  selectedSizeCount: number;
  selectedVariantCount: number;
  selection?: SDSProductVariantSelection;
}) {
  return (
    <div className="grid gap-4 rounded-[1.75rem] border border-zinc-200/80 bg-white px-5 py-5 shadow-sm lg:grid-cols-[minmax(0,1.1fr)_minmax(0,1.4fr)]">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
          当前上下文
        </p>
        <h2 className="mt-1 font-serif text-2xl leading-tight tracking-[-0.04em] text-zinc-950">
          商品信息
        </h2>
        <p className="mt-2 text-sm leading-6 text-zinc-600">
          这里仅用于确认当前商品。需要更换商品、变体矩阵或印刷区域时，请回到上方 SDS 商品选择区。
        </p>
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        <OverviewMetric label="变体" value={String(selection?.variantId ?? "未选择")} />
        <OverviewMetric label="印刷区域" value={printableAreaLabel} />
        <OverviewMetric
          label="变体矩阵"
          value={
            selectedVariantCount > 0
              ? `${selectedColorCount} 个颜色 · ${selectedSizeCount} 个尺码 · ${selectedVariantCount} 个 SKU`
              : "请先选择商品"
          }
        />
      </div>
    </div>
  );
}

function OverviewMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-3">
      <div className="text-[11px] uppercase tracking-[0.24em] text-zinc-400">
        {label}
      </div>
      <div className="mt-2 text-base font-semibold text-zinc-950">{value}</div>
    </div>
  );
}
