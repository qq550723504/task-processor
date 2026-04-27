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
          Step context
        </p>
        <h2 className="mt-1 font-serif text-2xl leading-tight tracking-[-0.04em] text-zinc-950">
          Product context
        </h2>
        <p className="mt-2 text-sm leading-6 text-zinc-600">
          This block is read-only. Change product, variant matrix, or print area in
          the SDS product selector above.
        </p>
      </div>

      <div className="grid gap-3 sm:grid-cols-3">
        <OverviewMetric label="Variant" value={String(selection?.variantId ?? "Not selected")} />
        <OverviewMetric label="Printable area" value={printableAreaLabel} />
        <OverviewMetric
          label="Variant matrix"
          value={
            selectedVariantCount > 0
              ? `${selectedColorCount} colors · ${selectedSizeCount} sizes · ${selectedVariantCount} SKUs`
              : "Choose a product first"
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
