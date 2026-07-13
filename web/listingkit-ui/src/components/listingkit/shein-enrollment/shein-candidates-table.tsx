/* eslint-disable @next/next/no-img-element -- SHEIN image hosts are tenant data outside fixed Next image remote patterns. */
"use client";

import { useMemo, useRef, useState } from "react";

import {
  formatSheinCurrencyAmount,
  getSheinSKUPriceSnapshots,
} from "@/components/listingkit/shein-enrollment/shein-price-snapshot";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import type { SheinActivityCandidateRecord } from "@/lib/types/listingkit/shein-enrollment";

export function SheinCandidatesTable({
  enrollmentDisabled,
  enrollmentDisabledReason,
  enrolling,
  items,
  onEnroll,
  onReject,
  onReset,
  resetting,
}: {
  enrollmentDisabled?: boolean;
  enrollmentDisabledReason?: string;
  enrolling: boolean;
  items: SheinActivityCandidateRecord[];
  onEnroll: (candidateIds: number[], activityKey: string) => Promise<void>;
  onReject: (candidateId: number) => Promise<void>;
  onReset: (candidateIds: number[]) => Promise<void>;
  resetting?: boolean;
}) {
  const [activityKey, setActivityKey] = useState("");
  const [selectedIds, setSelectedIds] = useState<number[]>([]);
  const [localEnrolling, setLocalEnrolling] = useState(false);
  const enrollmentInFlightRef = useRef(false);
  const selected = useMemo(() => new Set(selectedIds), [selectedIds]);
  const selectableIds = useMemo(
    () => items.filter((item) => item.id).map((item) => item.id ?? 0),
    [items],
  );
  const executableIds = useMemo(
    () =>
      items
        .filter((item) => item.id && isExecutableEnrollmentCandidate(item))
        .map((item) => item.id ?? 0),
    [items],
  );
  const selectedExecutableIds = useMemo(
    () => selectedIds.filter((id) => executableIds.includes(id)),
    [executableIds, selectedIds],
  );
  const allSelectableSelected =
    selectableIds.length > 0 && selectableIds.every((id) => selected.has(id));
  const enrollmentInFlight = enrolling || localEnrolling;

  async function handleEnroll() {
    if (
      enrollmentInFlightRef.current ||
      enrolling ||
      selectedExecutableIds.length === 0 ||
      enrollmentDisabled
    ) {
      return;
    }
    enrollmentInFlightRef.current = true;
    setLocalEnrolling(true);
    try {
      await onEnroll(selectedExecutableIds, activityKey.trim());
    } finally {
      enrollmentInFlightRef.current = false;
      setLocalEnrolling(false);
    }
  }

  async function handleResetSelected() {
    const resetIds = selectedIds.filter((id) => id > 0);
    if (resetIds.length === 0 || resetting || enrollmentInFlight) {
      return;
    }
    await onReset(resetIds);
  }

  return (
    <section className="space-y-4">
      <div className="flex flex-col gap-3 rounded-2xl border border-zinc-200 bg-white p-4 lg:flex-row lg:items-center">
        <Input
          aria-label="活动 key"
          className="lg:w-64"
          onChange={(event) => setActivityKey(event.target.value)}
          placeholder="可选：活动 key，不填则由后端生成"
          value={activityKey}
        />
        <Button
          disabled={enrollmentInFlight || resetting || selectableIds.length === 0}
          onClick={() => {
            setSelectedIds(allSelectableSelected ? [] : selectableIds);
          }}
          type="button"
          variant="outline"
        >
          {allSelectableSelected ? "取消全选" : "全选"}
        </Button>
        <Button
          disabled={
            enrollmentInFlight || selectedExecutableIds.length === 0 || enrollmentDisabled
          }
          onClick={() => void handleEnroll()}
          type="button"
        >
          {enrollmentInFlight ? "报名中..." : "报名活动"}
        </Button>
        <Button
          disabled={resetting || enrollmentInFlight || selectedIds.length === 0}
          onClick={() => void handleResetSelected()}
          type="button"
          variant="outline"
        >
          {resetting ? "重置中..." : "批量重置已选"}
        </Button>
        <span className="text-xs text-zinc-500">
          已选 {selectedIds.length} 个 · 可报名 {selectedExecutableIds.length} /{" "}
          {executableIds.length}
        </span>
        {enrollmentDisabled && enrollmentDisabledReason ? (
          <span className="text-xs text-amber-700">{enrollmentDisabledReason}</span>
        ) : null}
      </div>

      {items.length === 0 ? (
        <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-6 text-sm text-zinc-500">
          当前活动类型下暂无候选商品。
        </div>
      ) : null}

      <div className="grid gap-3">
        {items.map((item) => {
          const executable = isExecutableEnrollmentCandidate(item);
          return (
            <div key={item.id} className="rounded-2xl border border-zinc-200 bg-white p-4">
              <div className="flex flex-col gap-3 lg:flex-row lg:items-start">
                <label className="flex min-w-0 items-start gap-3">
                  <input
                    aria-label={`选择 ${item.skc_name || item.id}`}
                    checked={selected.has(item.id ?? 0)}
                    disabled={!item.id || resetting || enrollmentInFlight}
                    onChange={(event) => {
                      const currentId = item.id ?? 0;
                      if (!currentId) {
                        return;
                      }
                      setSelectedIds((current) =>
                        event.target.checked
                          ? Array.from(new Set([...current, currentId]))
                          : current.filter((candidateId) => candidateId !== currentId),
                      );
                    }}
                    type="checkbox"
                  />
                  <SheinCandidateImage item={item} />
                  <div className="min-w-0">
                    <p className="font-medium text-zinc-950">{item.skc_name || "-"}</p>
                    <p className="mt-1 text-sm text-zinc-500">
                      {item.eligibility_reason || "未提供候选原因"}
                    </p>
                    <p className="mt-1 text-xs text-zinc-500">
                      状态 {item.review_status || "-"} · 利润率{" "}
                      {item.calculated_profit_rate ?? "-"}
                    </p>
                    <SheinCandidateSKUPriceTable item={item} />
                    {!executable ? (
                      <p className="mt-1 text-xs text-amber-700">当前状态不可报名</p>
                    ) : null}
                    {item.review_status === "failed" && item.last_enrollment_error ? (
                      <p className="mt-1 text-xs text-red-600">
                        报名失败：{item.last_enrollment_error}
                      </p>
                    ) : null}
                  </div>
                </label>
                <div className="flex flex-wrap gap-2 lg:ml-auto">
                  <Button
                    aria-label={`重置 ${item.skc_name || item.id} 状态`}
                    disabled={!item.id || resetting || enrollmentInFlight}
                    onClick={() => void onReset([item.id ?? 0])}
                    size="sm"
                    type="button"
                    variant="outline"
                  >
                    重置
                  </Button>
                  <Button
                    disabled={!item.id}
                    onClick={() => void onReject(item.id ?? 0)}
                    size="sm"
                    type="button"
                    variant="outline"
                  >
                    驳回
                  </Button>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </section>
  );
}

function SheinCandidateSKUPriceTable({ item }: { item: SheinActivityCandidateRecord }) {
  const rows = getSheinCandidateSKUPriceRows(item);
  if (rows.length === 0) {
    return (
      <p className="mt-2 text-xs text-zinc-500">暂未同步 SKU 原价或 SDS 成本明细。</p>
    );
  }

  return (
    <div className="mt-3 overflow-x-auto rounded-lg border border-zinc-100">
      <div aria-label="报名价格明细" role="table" className="min-w-[420px] text-xs">
        <div
          role="row"
          className="grid grid-cols-[minmax(150px,1fr)_130px_130px] border-b border-zinc-100 bg-zinc-50 text-zinc-500"
        >
          <span role="columnheader" className="px-3 py-2 font-medium">SKU</span>
          <span role="columnheader" className="px-3 py-2 text-right font-medium">原价（供货价）</span>
          <span role="columnheader" className="px-3 py-2 text-right font-medium">成本（SDS）</span>
        </div>
        {rows.map((row) => (
          <div
            key={row.skuCode}
            role="row"
            className="grid grid-cols-[minmax(150px,1fr)_130px_130px] border-b border-zinc-100 last:border-b-0"
          >
            <span role="cell" className="px-3 py-2 font-mono text-zinc-700">{row.skuCode}</span>
            <span role="cell" className="px-3 py-2 text-right tabular-nums text-zinc-900">{row.originalPrice ?? "-"}</span>
            <span role="cell" className="px-3 py-2 text-right tabular-nums text-zinc-900">{row.costPrice ?? "-"}</span>
          </div>
        ))}
      </div>
    </div>
  );
}

type SheinCandidateSKUPriceRow = {
  skuCode: string;
  originalPrice: string | null;
  costPrice: string | null;
  currency: string;
};

function getSheinCandidateSKUPriceRows(
  item: SheinActivityCandidateRecord,
): SheinCandidateSKUPriceRow[] {
  const rowsBySKU = new Map<string, SheinCandidateSKUPriceRow>();
  for (const price of getSheinSKUPriceSnapshots(item.price_snapshot)) {
    rowsBySKU.set(price.skuCode, {
      skuCode: price.skuCode,
      originalPrice: price.price,
      costPrice: null,
      currency: price.currency,
    });
  }
  for (const cost of item.sku_cost_price_info_list ?? []) {
    const skuCode = cost.sku_code?.trim() ?? "";
    const costPrice = Number(cost.cost_price);
    if (!skuCode || !Number.isFinite(costPrice)) {
      continue;
    }
    const current = rowsBySKU.get(skuCode) ?? {
      skuCode,
      originalPrice: null,
      costPrice: null,
      currency: "",
    };
    const currency = cost.currency?.trim() || current.currency;
    current.costPrice = currency
      ? formatSheinCurrencyAmount(currency, costPrice)
      : costPrice.toFixed(2);
    rowsBySKU.set(skuCode, current);
  }
  return Array.from(rowsBySKU.values()).sort((left, right) =>
    left.skuCode.localeCompare(right.skuCode),
  );
}

function SheinCandidateImage({ item }: { item: SheinActivityCandidateRecord }) {
  const imageURL = item.main_image_url?.trim();
  const label = `${item.skc_name || "SHEIN 商品"} 图片`;
  if (!imageURL) {
    return (
      <div
        aria-label={label}
        className="flex h-16 w-16 shrink-0 items-center justify-center rounded-lg border border-zinc-200 bg-zinc-50 text-[11px] text-zinc-400"
      >
        无图
      </div>
    );
  }
  return (
    <div className="group relative h-16 w-16 shrink-0 cursor-zoom-in">
      <img
        alt={label}
        className="h-16 w-16 rounded-lg border border-zinc-200 bg-zinc-50 object-cover transition group-hover:border-zinc-400"
        loading="lazy"
        src={imageURL}
      />
      <div className="pointer-events-none absolute left-0 top-20 z-30 hidden h-60 w-60 overflow-hidden rounded-lg border border-zinc-200 bg-white shadow-xl group-hover:block sm:left-20 sm:top-1/2 sm:-translate-y-1/2">
        <img
          alt={`${item.skc_name || "SHEIN 商品"} 悬浮预览`}
          className="h-full w-full object-contain"
          loading="lazy"
          src={imageURL}
        />
      </div>
    </div>
  );
}

function isExecutableEnrollmentCandidate(item: SheinActivityCandidateRecord) {
  if (item.eligibility_status && item.eligibility_status !== "eligible") {
    return false;
  }
  return (
    item.review_status === "approved" ||
    item.review_status === "auto_queued" ||
    item.review_status === "pending_review"
  );
}
