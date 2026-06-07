import { Button } from "@/components/ui/button";
import { formatSDSPrice } from "@/lib/sds/format";
import { formatProductionCycle, formatWeight } from "@/lib/sds/product-filters";
import type { SDSProductSummary } from "@/lib/types/sds";

function ProductThumb({ imageUrl }: { imageUrl?: string }) {
  if (!imageUrl) {
    return (
      <div className="flex h-16 w-16 items-center justify-center rounded-md bg-muted text-xs font-semibold uppercase tracking-[0.12em] text-muted-foreground">
        SDS
      </div>
    );
  }

  return (
    <div
      className="h-16 w-16 rounded-md bg-muted bg-cover bg-center"
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
      className={`rounded-lg border px-4 py-4 shadow-sm transition ${
        isSelected
          ? "border-emerald-700 bg-emerald-950 text-white"
          : "border-border bg-card text-foreground hover:-translate-y-0.5 hover:border-foreground/30 hover:shadow-md"
      }`}
    >
      <div className="flex items-start gap-4">
        <ProductThumb imageUrl={product.img_url} />
        <div className="min-w-0 flex-1 space-y-2">
          <div className="flex flex-wrap gap-2">
            {product.on_sale_status === 2 ? (
              <span
                className={`rounded-md px-2 py-1 text-[11px] font-semibold uppercase tracking-[0.12em] ${
                  isSelected ? "bg-white/12 text-white" : "bg-emerald-50 text-emerald-700"
                }`}
              >
                在售
              </span>
            ) : null}
            {product.hotSellStatus === 1 ? (
              <span
                className={`rounded-md px-2 py-1 text-[11px] font-semibold uppercase tracking-[0.12em] ${
                  isSelected ? "bg-rose-400/20 text-rose-50" : "bg-rose-50 text-rose-700"
                }`}
              >
                热卖
              </span>
            ) : null}
            {product.issuingBayArea?.name ? (
              <span
                className={`rounded-md px-2 py-1 text-[11px] font-semibold uppercase tracking-[0.12em] ${
                  isSelected ? "bg-white/12 text-white" : "bg-muted text-muted-foreground"
                }`}
              >
                {product.issuingBayArea.name}
              </span>
            ) : null}
          </div>
          <div className="line-clamp-2 text-sm font-semibold leading-6">{product.name}</div>
          <div className={isSelected ? "text-emerald-100" : "text-muted-foreground"}>
            SKU {product.sku ?? "-"} · {formatSDSPrice(product.currentPrice ?? product.min_price)}
          </div>
          <div className={isSelected ? "text-emerald-100" : "text-muted-foreground"}>
            重量 {formatWeight(product)} · 生产周期 {formatProductionCycle(product)}
          </div>
          {product.categories?.length ? (
            <div
              className={`line-clamp-2 text-sm ${isSelected ? "text-emerald-100" : "text-muted-foreground"}`}
            >
              {product.categories.map((category) => category.name).join(" / ")}
            </div>
          ) : null}
          <div className="flex gap-3 pt-1">
            {isVariantSelected ? (
              <span
                className={`inline-flex items-center rounded-md px-3 text-xs font-semibold uppercase tracking-[0.12em] ${
                  isSelected ? "bg-white/12 text-white" : "bg-emerald-50 text-emerald-700"
                }`}
              >
                已选择
              </span>
            ) : null}
            <Button
              className="flex-1"
              onClick={onOpenVariants}
              variant={isSelected ? "secondary" : "primary"}
              type="button"
            >
              {isVariantSelected ? "更换变体" : "选择变体"}
            </Button>
          </div>
          {!isVariantSelected ? (
            <div className={isSelected ? "text-xs text-emerald-100" : "text-xs text-muted-foreground"}>
              打开尺码/颜色选择器，并锁定具体 SDS 子 SKU。
            </div>
          ) : null}
        </div>
      </div>
    </div>
  );
}
