import { Button } from "@/components/ui/button";
import { Checkbox } from "@/components/ui/checkbox";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
import { formatSDSPrice } from "@/lib/sds/format";
import type { SDSProductSummary, SDSProductVariant } from "@/lib/types/sds";
import { formatVariantWeight } from "@/components/listingkit/sds/sds-variant-picker-model";

export function SDSVariantPickerHeader({
  onClose,
  product,
}: {
  onClose: () => void;
  product?: SDSProductSummary;
}) {
  return (
    <div className="flex items-start justify-between gap-4 border-b border-zinc-200/80 px-5 py-5 md:px-6">
      <div className="space-y-2">
        <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
          变体选择
        </p>
        <div className="text-xl font-semibold tracking-[-0.03em] text-zinc-950">
          {product?.name ?? "选择具体子 SKU"}
        </div>
        <div className="flex flex-wrap gap-2 text-sm text-zinc-500">
          {product?.sku ? <span>SKU {product.sku}</span> : null}
          {product?.issuingBayArea?.name ? (
            <span>{product.issuingBayArea.name}</span>
          ) : null}
          {product?.currentPrice || product?.min_price ? (
            <span>{formatSDSPrice(product.currentPrice ?? product.min_price)}</span>
          ) : null}
        </div>
      </div>
      <Button className="shrink-0" onClick={onClose} variant="secondary">
        关闭
      </Button>
    </div>
  );
}

export function SDSVariantPickerStatus({
  hasError,
  isLoading,
  variantCount,
}: {
  hasError: boolean;
  isLoading: boolean;
  variantCount: number;
}) {
  if (isLoading) {
    return (
      <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-8 text-sm text-zinc-600">
        正在加载变体...
      </div>
    );
  }
  if (hasError) {
    return (
      <div className="rounded-[1.25rem] border border-amber-200 bg-amber-50 px-4 py-8 text-sm text-amber-900">
        SDS 商品详情加载失败。
      </div>
    );
  }
  if (variantCount === 0) {
    return (
      <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-8 text-sm text-zinc-600">
        这个商品没有返回子变体。
      </div>
    );
  }
  return null;
}

export function SDSVariantFilters({
  colorFilter,
  colorOptions,
  filteredCount,
  setColorFilter,
  setSizeFilter,
  sizeFilter,
  sizeOptions,
}: {
  colorFilter: string;
  colorOptions: string[];
  filteredCount: number;
  setColorFilter: (value: string) => void;
  setSizeFilter: (value: string) => void;
  sizeFilter: string;
  sizeOptions: string[];
}) {
  return (
    <div className="grid gap-3 rounded-[1.25rem] border border-zinc-200/80 bg-white px-4 py-4 lg:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
      <Select
        className="h-11 rounded-2xl px-4"
        onChange={(event) => setSizeFilter(event.target.value)}
        value={sizeFilter}
      >
        <option value="">全部尺码</option>
        {sizeOptions.map((size) => (
          <option key={size} value={size}>
            {size}
          </option>
        ))}
      </Select>
      <Select
        className="h-11 rounded-2xl px-4"
        onChange={(event) => setColorFilter(event.target.value)}
        value={colorFilter}
      >
        <option value="">全部颜色</option>
        {colorOptions.map((color) => (
          <option key={color} value={color}>
            {color}
          </option>
        ))}
      </Select>
      <div className="flex items-center text-sm text-zinc-500">
        {filteredCount} 个变体
      </div>
    </div>
  );
}

export function SDSVariantSelectionSummary({
  addSelectedVariantsToCurrentBatch,
  clearFilteredVariants,
  createBatchFromSelectedVariants,
  currentBatchLabel,
  isTargetingExistingBatch = false,
  isSubmittingToBatch = false,
  openOtherBatchPicker,
  selectFilteredVariants,
  selectedColorCount,
  selectedSizeCount,
  selectedVariantCount,
  useSelectedVariantsLabel = "使用已选变体",
  useSelectedVariants,
}: {
  addSelectedVariantsToCurrentBatch?: () => void;
  clearFilteredVariants: () => void;
  createBatchFromSelectedVariants?: () => void;
  currentBatchLabel?: string;
  isTargetingExistingBatch?: boolean;
  isSubmittingToBatch?: boolean;
  openOtherBatchPicker?: () => void;
  selectFilteredVariants: () => void;
  selectedColorCount: number;
  selectedSizeCount: number;
  selectedVariantCount: number;
  useSelectedVariantsLabel?: string;
  useSelectedVariants: () => void;
}) {
  return (
    <div className="flex flex-col gap-3 rounded-[1.25rem] border border-emerald-200 bg-emerald-50 px-4 py-4 text-sm text-emerald-900 lg:flex-row lg:flex-wrap lg:items-center lg:justify-between">
      <div>
        {isTargetingExistingBatch && currentBatchLabel
          ? `已选 ${selectedVariantCount} 个 SKU，将加入批次 ${currentBatchLabel}`
          : `已选 ${selectedVariantCount} 个 SKU · ${selectedColorCount} 个颜色 · ${selectedSizeCount} 个尺码`}
      </div>
      <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap">
        <Button className="w-full sm:w-auto" onClick={selectFilteredVariants} variant="secondary" type="button">
          选中当前筛选
        </Button>
        <Button className="w-full sm:w-auto" onClick={clearFilteredVariants} variant="ghost" type="button">
          清除当前筛选
        </Button>
        {addSelectedVariantsToCurrentBatch ? (
          <Button
            className="w-full sm:w-auto"
            disabled={selectedVariantCount === 0 || isSubmittingToBatch}
            onClick={addSelectedVariantsToCurrentBatch}
            type="button"
            variant={isTargetingExistingBatch ? "primary" : "ghost"}
          >
            {isSubmittingToBatch
              ? "加入中..."
              : isTargetingExistingBatch
                ? "加入当前批次，继续选下一个"
              : `加入当前批次${currentBatchLabel ? ` · ${currentBatchLabel}` : ""}`}
          </Button>
        ) : null}
        {openOtherBatchPicker ? (
          <Button
            className="w-full sm:w-auto"
            disabled={selectedVariantCount === 0 || isSubmittingToBatch}
            onClick={openOtherBatchPicker}
            type="button"
            variant="ghost"
          >
            加入其他批次
          </Button>
        ) : null}
        {createBatchFromSelectedVariants && !isTargetingExistingBatch ? (
          <Button
            className="w-full sm:w-auto"
            disabled={selectedVariantCount === 0}
            onClick={createBatchFromSelectedVariants}
            type="button"
            variant="ghost"
          >
            创建新批次并加入
          </Button>
        ) : null}
        {!isTargetingExistingBatch ? (
          <Button
            className="w-full sm:w-auto"
            disabled={selectedVariantCount === 0}
            onClick={useSelectedVariants}
            type="button"
          >
            {useSelectedVariantsLabel}
          </Button>
        ) : null}
      </div>
    </div>
  );
}

export function SDSVariantGrid({
  allowPrimarySelection = true,
  filteredVariants,
  onSelectAsPrimary,
  selectedIds,
  selectedVariantId,
  toggleVariant,
}: {
  allowPrimarySelection?: boolean;
  filteredVariants: SDSProductVariant[];
  onSelectAsPrimary: (variant: SDSProductVariant) => void;
  selectedIds: number[];
  selectedVariantId?: number;
  toggleVariant: (variantId: number) => void;
}) {
  return (
    <>
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {filteredVariants.map((variant) => (
          <SDSVariantCard
            active={selectedIds.includes(variant.id)}
            allowPrimarySelection={allowPrimarySelection}
            key={variant.id}
            onSelectAsPrimary={() => onSelectAsPrimary(variant)}
            primary={selectedVariantId === variant.id}
            toggleVariant={() => toggleVariant(variant.id)}
            variant={variant}
          />
        ))}
      </div>
      {filteredVariants.length === 0 ? (
        <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-8 text-sm text-zinc-600">
          当前尺码或颜色筛选下没有匹配变体。
        </div>
      ) : null}
    </>
  );
}

function SDSVariantCard({
  active,
  allowPrimarySelection = true,
  onSelectAsPrimary,
  primary,
  toggleVariant,
  variant,
}: {
  active: boolean;
  allowPrimarySelection?: boolean;
  onSelectAsPrimary: () => void;
  primary: boolean;
  toggleVariant: () => void;
  variant: SDSProductVariant;
}) {
  return (
    <div
      className={`rounded-[1.5rem] border px-4 py-4 shadow-sm ${
        active
          ? "border-emerald-800 bg-[linear-gradient(135deg,_#052e2b,_#115e59)] text-white"
          : "border-zinc-200 bg-white"
      }`}
    >
      <div className="space-y-3">
        <div className="space-y-1">
          <div className="text-sm font-semibold">
            <Label className="flex items-center gap-2">
              <Checkbox
                checked={active}
                onChange={toggleVariant}
              />
              <span>
                {variant.size || "均码"} · {variant.color_name || "默认"}
              </span>
            </Label>
          </div>
          <div className={active ? "text-emerald-100" : "text-zinc-500"}>
            变体 ID {variant.id} · SKU {variant.sku ?? "-"}
          </div>
        </div>

        <div className="flex flex-wrap gap-2">
          {variant.on_sale_status === 2 ? (
            <span
              className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${
                active
                  ? "bg-white/12 text-white"
                  : "bg-emerald-50 text-emerald-700"
              }`}
            >
              在售
            </span>
          ) : null}
          {variant.hotSellStatus === 1 ? (
            <span
              className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${
                active
                  ? "bg-rose-400/20 text-rose-50"
                  : "bg-rose-50 text-rose-700"
              }`}
            >
              热卖
            </span>
          ) : null}
          {variant.issuingBayArea?.name ? (
            <span
              className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${
                active ? "bg-white/12 text-white" : "bg-zinc-100 text-zinc-700"
              }`}
            >
              {variant.issuingBayArea.name}
            </span>
          ) : null}
        </div>

        <div
          className={`space-y-1 text-sm ${
            active ? "text-emerald-100" : "text-zinc-500"
          }`}
        >
          <div>模板组 {variant.designPrototype?.prototypeGroupId ?? "-"}</div>
          <div>价格 {formatSDSPrice(variant.currentPrice)}</div>
          <div>重量 {formatVariantWeight(variant.weight)}</div>
          <div>
            生产周期{" "}
            {variant.productionCycle ? `${variant.productionCycle}h` : "-"}
          </div>
        </div>
        {allowPrimarySelection ? (
          <Button
            className="w-full"
            onClick={onSelectAsPrimary}
            variant={primary ? "secondary" : "primary"}
            type="button"
          >
            {primary ? "默认变体" : "设为默认变体"}
          </Button>
        ) : null}
      </div>
    </div>
  );
}
