import { Button } from "@/components/shared/button";
import { formatSDSPrice } from "@/lib/sds/format";
import { formatProductionCycle, formatWeight } from "@/lib/sds/product-filters";
import type { SDSProductSummary } from "@/lib/types/sds";

function ProductThumb({ imageUrl }: { imageUrl?: string }) {
  if (!imageUrl) {
    return (
      <div className="flex h-16 w-16 items-center justify-center rounded-2xl bg-zinc-100 text-xs font-semibold uppercase tracking-[0.16em] text-zinc-400">
        SDS
      </div>
    );
  }

  return (
    <div
      className="h-16 w-16 rounded-2xl bg-zinc-100 bg-cover bg-center"
      style={{ backgroundImage: `url(${imageUrl})` }}
    />
  );
}

type SDSProductCardProps = {
  isSelected: boolean;
  isVariantSelected: boolean;
  onOpenVariants: () => void;
  product: SDSProductSummary;
};

export function SDSProductCard({
  isSelected,
  isVariantSelected,
  onOpenVariants,
  product,
}: SDSProductCardProps) {
  return (
    <div
      className={`rounded-[1.5rem] border px-4 py-4 shadow-sm transition ${
        isSelected
          ? "border-emerald-800 bg-[linear-gradient(135deg,_#052e2b,_#115e59)] text-white"
          : "border-zinc-200 bg-white text-zinc-900 hover:-translate-y-0.5 hover:border-zinc-400 hover:shadow-md"
      }`}
    >
      <div className="flex items-start gap-4">
        <ProductThumb imageUrl={product.img_url} />
        <div className="min-w-0 flex-1 space-y-2">
          <div className="flex flex-wrap gap-2">
            {product.on_sale_status === 2 ? (
              <span
                className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${
                  isSelected ? "bg-white/12 text-white" : "bg-emerald-50 text-emerald-700"
                }`}
              >
                On sale
              </span>
            ) : null}
            {product.hotSellStatus === 1 ? (
              <span
                className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${
                  isSelected ? "bg-rose-400/20 text-rose-50" : "bg-rose-50 text-rose-700"
                }`}
              >
                Hot sale
              </span>
            ) : null}
            {product.issuingBayArea?.name ? (
              <span
                className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${
                  isSelected ? "bg-white/12 text-white" : "bg-zinc-100 text-zinc-700"
                }`}
              >
                {product.issuingBayArea.name}
              </span>
            ) : null}
          </div>
          <div className="line-clamp-2 text-sm font-semibold leading-6">{product.name}</div>
          <div className={isSelected ? "text-emerald-100" : "text-zinc-500"}>
            SKU {product.sku ?? "-"} · {formatSDSPrice(product.currentPrice ?? product.min_price)}
          </div>
          <div className={isSelected ? "text-emerald-100" : "text-zinc-500"}>
            Weight {formatWeight(product)} · Cycle {formatProductionCycle(product)}
          </div>
          {product.categories?.length ? (
            <div className={`line-clamp-2 text-sm ${isSelected ? "text-emerald-100" : "text-zinc-500"}`}>
              {product.categories.map((category) => category.name).join(" / ")}
            </div>
          ) : null}
          <div className="flex gap-3 pt-1">
            {isVariantSelected ? (
              <span
                className={`inline-flex items-center rounded-full px-3 text-xs font-semibold uppercase tracking-[0.16em] ${
                  isSelected ? "bg-white/12 text-white" : "bg-emerald-50 text-emerald-700"
                }`}
              >
                Selected
              </span>
            ) : null}
            <Button
              className="flex-1"
              onClick={onOpenVariants}
              tone={isSelected ? "secondary" : "primary"}
              type="button"
            >
              {isVariantSelected ? "Change variant" : "Choose variant"}
            </Button>
          </div>
          {!isVariantSelected ? (
            <div className={isSelected ? "text-xs text-emerald-100" : "text-xs text-zinc-400"}>
              Opens size/color picker and locks the exact SDS child SKU.
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
