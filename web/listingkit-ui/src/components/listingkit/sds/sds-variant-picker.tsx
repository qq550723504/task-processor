"use client";

import { useMemo, useState } from "react";

import { Button } from "@/components/shared/button";
import { formatSDSPrice } from "@/lib/sds/format";
import type { SDSProductSummary, SDSProductVariant } from "@/lib/types/sds";

function formatVariantWeight(value?: number) {
  if (!value) {
    return "-";
  }
  return `${value}g`;
}

export function SDSVariantPicker({
  open,
  product,
  variants,
  selectedVariantId,
  isLoading,
  hasError,
  onClose,
  onSelectVariants,
}: {
  open: boolean;
  product?: SDSProductSummary;
  variants: SDSProductVariant[];
  selectedVariantId?: number;
  isLoading: boolean;
  hasError: boolean;
  onClose: () => void;
  onSelectVariants: (primary: SDSProductVariant, variants: SDSProductVariant[]) => void;
}) {
  const [sizeFilter, setSizeFilter] = useState("");
  const [colorFilter, setColorFilter] = useState("");
  const [selectionState, setSelectionState] = useState<{
    key: string;
    ids: number[];
  }>({ key: "", ids: [] });

  const sizeOptions = useMemo(
    () =>
      Array.from(new Set(variants.map((variant) => variant.size).filter(Boolean))).sort((a, b) =>
        String(a).localeCompare(String(b)),
      ),
    [variants],
  );
  const colorOptions = useMemo(
    () =>
      Array.from(new Set(variants.map((variant) => variant.color_name).filter(Boolean))).sort((a, b) =>
        String(a).localeCompare(String(b)),
      ),
    [variants],
  );
  const filteredVariants = useMemo(
    () =>
      variants.filter((variant) => {
        if (sizeFilter && variant.size !== sizeFilter) {
          return false;
        }
        if (colorFilter && variant.color_name !== colorFilter) {
          return false;
        }
        return true;
      }),
    [colorFilter, sizeFilter, variants],
  );
  const variantKey = useMemo(
    () => variants.map((variant) => variant.id).join(":"),
    [variants],
  );
  const selectedIds =
    selectionState.key === variantKey
      ? selectionState.ids
      : variants.map((variant) => variant.id);
  const selectedVariants = useMemo(
    () => variants.filter((variant) => selectedIds.includes(variant.id)),
    [selectedIds, variants],
  );
  const selectedColors = useMemo(
    () =>
      Array.from(
        new Set(selectedVariants.map((variant) => variant.color_name || "default")),
      ),
    [selectedVariants],
  );
  const selectedSizes = useMemo(
    () =>
      Array.from(
        new Set(selectedVariants.map((variant) => variant.size || "One size")),
      ),
    [selectedVariants],
  );

  function toggleVariant(variantId: number) {
    setSelectionState({
      key: variantKey,
      ids: selectedIds.includes(variantId)
        ? selectedIds.filter((id) => id !== variantId)
        : [...selectedIds, variantId],
    });
  }

  function selectFilteredVariants() {
    const filteredIds = filteredVariants.map((variant) => variant.id);
    setSelectionState({
      key: variantKey,
      ids: Array.from(new Set([...selectedIds, ...filteredIds])),
    });
  }

  function clearFilteredVariants() {
    const filteredIds = new Set(filteredVariants.map((variant) => variant.id));
    setSelectionState({
      key: variantKey,
      ids: selectedIds.filter((id) => !filteredIds.has(id)),
    });
  }

  function useSelectedVariants() {
    const selected = selectedVariants.length > 0 ? selectedVariants : variants.slice(0, 1);
    const primary =
      selected.find((variant) => variant.id === selectedVariantId) ?? selected[0];
    if (!primary) {
      return;
    }
    onSelectVariants(primary, selected);
  }

  if (!open) {
    return null;
  }

  return (
    <div className="fixed inset-0 z-50 flex items-end justify-center bg-zinc-950/40 p-4 backdrop-blur-sm md:items-center">
      <div className="flex max-h-[90vh] w-full max-w-4xl flex-col overflow-hidden rounded-[2rem] border border-white/60 bg-[linear-gradient(180deg,_rgba(255,255,255,0.97),_rgba(244,244,245,0.95))] shadow-[0_24px_96px_rgba(24,24,27,0.24)]">
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
              {product?.issuingBayArea?.name ? <span>{product.issuingBayArea.name}</span> : null}
              {product?.currentPrice || product?.min_price ? (
                <span>{formatSDSPrice(product.currentPrice ?? product.min_price)}</span>
              ) : null}
            </div>
          </div>
          <Button className="shrink-0" onClick={onClose} tone="secondary">
            关闭
          </Button>
        </div>

        <div className="overflow-auto px-5 py-5 md:px-6">
          {isLoading ? (
            <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-8 text-sm text-zinc-600">
              正在加载变体...
            </div>
          ) : hasError ? (
            <div className="rounded-[1.25rem] border border-amber-200 bg-amber-50 px-4 py-8 text-sm text-amber-900">
              SDS 商品详情加载失败。
            </div>
          ) : variants.length === 0 ? (
            <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-8 text-sm text-zinc-600">
              这个商品没有返回子变体。
            </div>
          ) : (
            <div className="space-y-4">
              <div className="grid gap-3 rounded-[1.25rem] border border-zinc-200/80 bg-white px-4 py-4 md:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_auto]">
                <select
                  className="h-11 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
                  onChange={(event) => setSizeFilter(event.target.value)}
                  value={sizeFilter}
                >
                  <option value="">全部尺码</option>
                  {sizeOptions.map((size) => (
                    <option key={size} value={size}>
                      {size}
                    </option>
                  ))}
                </select>
                <select
                  className="h-11 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-950 outline-none transition focus:border-zinc-950"
                  onChange={(event) => setColorFilter(event.target.value)}
                  value={colorFilter}
                >
                  <option value="">全部颜色</option>
                  {colorOptions.map((color) => (
                    <option key={color} value={color}>
                      {color}
                    </option>
                  ))}
                </select>
                <div className="flex items-center text-sm text-zinc-500">
                  {filteredVariants.length} 个变体
                </div>
              </div>
              <div className="flex flex-wrap items-center justify-between gap-3 rounded-[1.25rem] border border-emerald-200 bg-emerald-50 px-4 py-4 text-sm text-emerald-900">
                <div>
                  已选 {selectedVariants.length} 个 SKU · {selectedColors.length} 个颜色 · {selectedSizes.length} 个尺码
                </div>
                <div className="flex flex-wrap gap-2">
                  <Button onClick={selectFilteredVariants} tone="secondary" type="button">
                    选中当前筛选
                  </Button>
                  <Button onClick={clearFilteredVariants} tone="ghost" type="button">
                    清除当前筛选
                  </Button>
                  <Button
                    disabled={selectedVariants.length === 0}
                    onClick={useSelectedVariants}
                    type="button"
                  >
                    使用已选变体
                  </Button>
                </div>
              </div>

              <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
              {filteredVariants.map((variant) => {
                const active = selectedIds.includes(variant.id);
                const primary = selectedVariantId === variant.id;
                return (
                  <div
                    className={`rounded-[1.5rem] border px-4 py-4 shadow-sm ${
                      active
                        ? "border-emerald-800 bg-[linear-gradient(135deg,_#052e2b,_#115e59)] text-white"
                        : "border-zinc-200 bg-white"
                    }`}
                    key={variant.id}
                  >
                    <div className="space-y-3">
                      <div className="space-y-1">
                        <div className="text-sm font-semibold">
                          <label className="flex items-center gap-2">
                            <input
                              checked={active}
                              className="h-4 w-4 rounded border-zinc-300"
                              onChange={() => toggleVariant(variant.id)}
                              type="checkbox"
                            />
                            <span>
                              {variant.size || "均码"} · {variant.color_name || "默认"}
                            </span>
                          </label>
                        </div>
                        <div className={active ? "text-emerald-100" : "text-zinc-500"}>
                          变体 ID {variant.id} · SKU {variant.sku ?? "-"}
                        </div>
                      </div>

                      <div className="flex flex-wrap gap-2">
                        {variant.on_sale_status === 2 ? (
                          <span
                            className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${
                              active ? "bg-white/12 text-white" : "bg-emerald-50 text-emerald-700"
                            }`}
                          >
                            在售
                          </span>
                        ) : null}
                        {variant.hotSellStatus === 1 ? (
                          <span
                            className={`rounded-full px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${
                              active ? "bg-rose-400/20 text-rose-50" : "bg-rose-50 text-rose-700"
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

                      <div className={`space-y-1 text-sm ${active ? "text-emerald-100" : "text-zinc-500"}`}>
                        <div>模板组 {variant.designPrototype?.prototypeGroupId ?? "-"}</div>
                        <div>价格 {formatSDSPrice(variant.currentPrice)}</div>
                        <div>重量 {formatVariantWeight(variant.weight)}</div>
                        <div>生产周期 {variant.productionCycle ? `${variant.productionCycle}h` : "-"}</div>
                      </div>

                      <Button
                        className="w-full"
                        onClick={() => {
                          const selected =
                            selectedVariants.length > 0 ? selectedVariants : [variant];
                          onSelectVariants(
                            variant,
                            selected.some((item) => item.id === variant.id)
                              ? selected
                              : [variant, ...selected],
                          );
                        }}
                        tone={primary ? "secondary" : "primary"}
                        type="button"
                      >
                        {primary ? "主变体" : "设为主变体"}
                      </Button>
                    </div>
                  </div>
                );
              })}
              </div>
              {filteredVariants.length === 0 ? (
                <div className="rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-8 text-sm text-zinc-600">
                  当前尺码或颜色筛选下没有匹配变体。
                </div>
              ) : null}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
