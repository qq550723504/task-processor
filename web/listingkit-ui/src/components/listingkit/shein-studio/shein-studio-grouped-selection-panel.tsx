"use client";

import { useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import { formatSheinStoreOptionLabel } from "@/lib/shein-studio/store-option-label";
import type {
  GroupedSDSSelectionEligibility,
  SDSBaselineStatus,
} from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { ListingKitStoreProfile } from "@/lib/types/listingkit";

type GroupedCandidate = {
  selectionId: string;
  selection: SDSProductVariantSelection;
  baselineStatus: SDSBaselineStatus;
  baselineKey?: string;
  baselineReason: string;
  eligible: boolean;
  eligibilityReason?: string;
};

type StoreOption = Pick<ListingKitStoreProfile, "store_id" | "store" | "site">;
type GroupedStoreFilter = "all" | "following_current" | "current_store" | "cross_store";

export function SheinStudioGroupedSelectionPanel({
  activeSelection,
  activeSelectionBaselineStatus,
  activeSelectionBaselineReason,
  candidates,
  currentStoreId,
  groupedSelections,
  onAddSelection,
  currentStoreLabel,
  onBulkUpdateSelectionStore,
  onRemoveSelection,
  onUpdateSelectionStore,
  storeOptions,
}: {
  activeSelection?: SDSProductVariantSelection;
  activeSelectionBaselineStatus: SDSBaselineStatus;
  activeSelectionBaselineReason: string;
  candidates: GroupedCandidate[];
  currentStoreId?: string;
  currentStoreLabel?: string;
  groupedSelections: GroupedSDSSelectionEligibility[];
  onAddSelection: (candidate: GroupedCandidate) => void;
  onBulkUpdateSelectionStore: (selectionIds: string[], storeId: string) => void;
  onRemoveSelection: (selectionId: string) => void;
  onUpdateSelectionStore: (selectionId: string, storeId: string) => void;
  storeOptions: StoreOption[];
}) {
  if (!activeSelection?.variantId) {
    return null;
  }

  const selectedIDs = new Set(groupedSelections.map((item) => item.selectionId));
  const [activeStoreFilter, setActiveStoreFilter] = useState<GroupedStoreFilter>("all");
  const [bulkStoreId, setBulkStoreId] = useState("");
  const [bulkUpdateFeedback, setBulkUpdateFeedback] = useState("");
  const storeLabelById = new Map(
    storeOptions.map((option) => [String(option.store_id), formatSheinStoreOptionLabel(option)]),
  );
  const normalizedCurrentStoreId = currentStoreId?.trim() ?? "";
  const groupedStoreSummary = groupedSelections.reduce(
    (summary, item) => {
      const selectedStoreId = item.sheinStoreId.trim();
      if (!selectedStoreId) {
        summary.followingCurrent += 1;
        return summary;
      }
      if (normalizedCurrentStoreId && selectedStoreId !== normalizedCurrentStoreId) {
        summary.crossStore += 1;
        return summary;
      }
      summary.currentStore += 1;
      return summary;
    },
    { followingCurrent: 0, currentStore: 0, crossStore: 0 },
  );
  const filteredSelectionIds = useMemo(
    () =>
      groupedSelections
        .filter((item) =>
          matchesGroupedStoreFilter(
            activeStoreFilter,
            item.sheinStoreId.trim(),
            normalizedCurrentStoreId,
          ),
        )
        .map((item) => item.selectionId),
    [activeStoreFilter, groupedSelections, normalizedCurrentStoreId],
  );

  useEffect(() => {
    setBulkUpdateFeedback("");
  }, [activeStoreFilter]);

  return (
    <section className="rounded-[1.5rem] border border-zinc-200 bg-zinc-50/80 px-4 py-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
            分组上品
          </p>
          <h3 className="mt-1 text-lg font-semibold text-zinc-950">
            把其他已缓存的 SDS 商品加入当前批次
          </h3>
          <p className="mt-1 max-w-3xl text-sm leading-6 text-zinc-600">
            先在 SDS 选品区把商品加入批量候选池。这里只展示 baseline 已准备好的候选商品；不同尺寸会按出图策略自动分组或单独生成。
          </p>
        </div>
        <Badge className="rounded-full px-3 py-1 text-xs" variant="neutral">
          已加入 {groupedSelections.length} 款
        </Badge>
      </div>

      <div className="mt-4 rounded-2xl border border-zinc-200 bg-white px-4 py-3">
        <div className="flex flex-wrap items-center gap-3">
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-semibold text-zinc-950">
              当前主商品: {activeSelection.productName}
            </div>
            <div className="mt-1 text-xs text-zinc-500">
              变体 {activeSelection.variantId} · {activeSelection.variantLabel}
            </div>
          </div>
          <BaselineStatusBadge
            status={activeSelectionBaselineStatus}
            reason={activeSelectionBaselineReason}
          />
        </div>
      </div>

      {groupedSelections.length > 0 ? (
        <div className="mt-4 space-y-3">
          <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
            已加入分组
          </div>
          <div className="flex flex-wrap gap-2">
            {groupedStoreSummary.followingCurrent > 0 ? (
              <button
                className="cursor-pointer"
                onClick={() =>
                  setActiveStoreFilter((current) =>
                    current === "following_current" ? "all" : "following_current",
                  )
                }
                type="button"
              >
                <Badge
                  className="rounded-full px-3 py-1 text-xs"
                  variant={activeStoreFilter === "following_current" ? "default" : "neutral"}
                >
                跟随当前店铺 {groupedStoreSummary.followingCurrent} 款
                </Badge>
              </button>
            ) : null}
            {groupedStoreSummary.currentStore > 0 ? (
              <button
                className="cursor-pointer"
                onClick={() =>
                  setActiveStoreFilter((current) =>
                    current === "current_store" ? "all" : "current_store",
                  )
                }
                type="button"
              >
                <Badge
                  className="rounded-full px-3 py-1 text-xs"
                  variant={activeStoreFilter === "current_store" ? "default" : "success"}
                >
                当前店铺 {groupedStoreSummary.currentStore} 款
                </Badge>
              </button>
            ) : null}
            {groupedStoreSummary.crossStore > 0 ? (
              <button
                className="cursor-pointer"
                onClick={() =>
                  setActiveStoreFilter((current) =>
                    current === "cross_store" ? "all" : "cross_store",
                  )
                }
                type="button"
              >
                <Badge
                  className="rounded-full px-3 py-1 text-xs"
                  variant={activeStoreFilter === "cross_store" ? "default" : "warning"}
                >
                跨店铺 {groupedStoreSummary.crossStore} 款
                </Badge>
              </button>
            ) : null}
          </div>
          {activeStoreFilter !== "all" ? (
            <div className="space-y-3">
              <div className="text-xs text-zinc-500">
                已按店铺分发状态筛选显示，再点一次当前标签可恢复查看全部。
              </div>
              {filteredSelectionIds.length > 0 ? (
                <div className="rounded-2xl border border-zinc-200 bg-white px-4 py-3">
                  <div className="flex flex-wrap items-end gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="text-sm font-semibold text-zinc-950">
                        批量改店铺
                      </div>
                      <div className="mt-1 text-xs text-zinc-500">
                        当前筛选命中 {filteredSelectionIds.length} 款商品，可以统一改成同一家店，或者改回跟随当前店铺。
                      </div>
                    </div>
                    <div className="w-full min-w-[15rem] max-w-sm">
                      <label
                        className="mb-1 block text-xs font-medium text-zinc-600"
                        htmlFor="grouped-selection-bulk-store"
                      >
                        应用到当前筛选
                      </label>
                      <Select
                        id="grouped-selection-bulk-store"
                        onChange={(event) => setBulkStoreId(event.target.value)}
                        value={bulkStoreId}
                      >
                        <option value="">
                          {currentStoreLabel
                            ? `跟随当前店铺（${currentStoreLabel}）`
                            : "跟随当前店铺"}
                        </option>
                        {storeOptions.map((option) => (
                          <option key={option.store_id} value={String(option.store_id)}>
                            {formatSheinStoreOptionLabel(option)}
                          </option>
                        ))}
                      </Select>
                    </div>
                    <Button
                      onClick={() => {
                        onBulkUpdateSelectionStore(filteredSelectionIds, bulkStoreId);
                        const targetStoreLabel = bulkStoreId
                          ? (storeLabelById.get(bulkStoreId) ?? bulkStoreId)
                          : currentStoreLabel
                            ? `跟随当前店铺（${currentStoreLabel}）`
                            : "跟随当前店铺";
                        setBulkUpdateFeedback(
                          `已把 ${filteredSelectionIds.length} 款商品改到 ${targetStoreLabel}。`,
                        );
                      }}
                      type="button"
                    >
                      批量应用到当前筛选
                    </Button>
                  </div>
                  {bulkUpdateFeedback ? (
                    <div className="mt-3 text-xs text-emerald-700">{bulkUpdateFeedback}</div>
                  ) : null}
                </div>
              ) : null}
            </div>
          ) : null}
          <div className="grid gap-3">
            {groupedSelections.map((item) => {
              const selectedStoreId = item.sheinStoreId.trim();
              const isCrossStore = isGroupedSelectionCrossStore(
                selectedStoreId,
                normalizedCurrentStoreId,
              );
              const matchesActiveFilter = matchesGroupedStoreFilter(
                activeStoreFilter,
                selectedStoreId,
                normalizedCurrentStoreId,
              );
              return (
                <div
                  className={`rounded-2xl border px-4 py-4 transition ${
                    matchesActiveFilter
                      ? "border-emerald-200 bg-white"
                      : "border-zinc-200 bg-zinc-50/60 opacity-55"
                  }`}
                  key={item.selectionId}
                >
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-semibold text-zinc-950">
                        {item.selection.productName}
                      </div>
                    <div className="mt-1 text-xs text-zinc-500">
                      变体 {item.selection.variantId} · {item.selection.variantLabel}
                    </div>
                    </div>
                    <div className="flex flex-wrap items-center gap-2">
                      <BaselineStatusBadge
                        status={item.baselineStatus}
                        reason={item.baselineReason}
                      />
                      <Button
                        onClick={() => onRemoveSelection(item.selectionId)}
                        type="button"
                        variant="ghost"
                      >
                        移除
                      </Button>
                    </div>
                  </div>
                  <div className="mt-3 max-w-xs">
                    <label className="mb-1 block text-xs font-medium text-zinc-600">
                      目标店铺
                    </label>
                    <div className="mb-2 flex flex-wrap items-center gap-2 text-xs text-zinc-500">
                      {item.sheinStoreId.trim()
                        ? `已指定店铺：${
                            storeLabelById.get(item.sheinStoreId.trim()) ?? item.sheinStoreId.trim()
                          }`
                        : currentStoreLabel
                          ? `当前跟随：${currentStoreLabel}`
                          : "当前跟随主商品店铺设置"}
                      {isCrossStore ? (
                        <Badge className="rounded-full px-2 py-0.5 text-[10px]" variant="warning">
                          跨店铺
                        </Badge>
                      ) : null}
                    </div>
                    <Select
                      onChange={(event) =>
                        onUpdateSelectionStore(item.selectionId, event.target.value)
                      }
                      value={item.sheinStoreId}
                    >
                      <option value="">
                        {currentStoreLabel
                          ? `跟随当前店铺（${currentStoreLabel}）`
                          : "跟随当前店铺"}
                      </option>
                      {storeOptions.map((option) => (
                        <option key={option.store_id} value={String(option.store_id)}>
                          {formatSheinStoreOptionLabel(option)}
                        </option>
                      ))}
                    </Select>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      ) : null}

      <div className="mt-4 space-y-3">
        <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
          批量候选池
        </div>
        {candidates.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-zinc-200 bg-white px-4 py-4 text-sm text-zinc-500">
            暂时没有可加入分组的候选商品。先回到 SDS 选品区，把想批量处理的商品加入批量候选池。
          </div>
        ) : (
          <div className="grid gap-3 md:grid-cols-2">
            {candidates.map((candidate) => {
              const selected = selectedIDs.has(candidate.selectionId);
              return (
                <div
                  className="rounded-2xl border border-zinc-200 bg-white px-4 py-4"
                  key={candidate.selectionId}
                >
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-semibold text-zinc-950">
                        {candidate.selection.productName}
                      </div>
                      <div className="mt-1 text-xs text-zinc-500">
                        变体 {candidate.selection.variantId} · {candidate.selection.variantLabel}
                      </div>
                    </div>
                    <BaselineStatusBadge
                      status={candidate.baselineStatus}
                      reason={candidate.baselineReason}
                    />
                  </div>
                  {candidate.eligibilityReason ? (
                    <div className="mt-2 text-xs text-amber-700">
                      {candidate.eligibilityReason}
                    </div>
                  ) : null}
                  <div className="mt-3">
                    <Button
                      disabled={!candidate.eligible || selected}
                      onClick={() => onAddSelection(candidate)}
                      type="button"
                      variant={selected ? "secondary" : "primary"}
                    >
                      {selected ? "已加入分组" : "加入分组"}
                    </Button>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </section>
  );
}

function BaselineStatusBadge({
  status,
  reason,
}: {
  status: SDSBaselineStatus;
  reason?: string;
}) {
  const label =
    status === "ready" ? "Baseline 已就绪" : status === "failed" ? "Baseline 异常" : "Baseline 缺失";
  const variant =
    status === "ready" ? "success" : status === "failed" ? "danger" : "warning";
  return (
    <Badge title={reason || label} variant={variant as "success" | "danger" | "warning"}>
      {label}
    </Badge>
  );
}

function matchesGroupedStoreFilter(
  filter: GroupedStoreFilter,
  selectedStoreId: string,
  currentStoreId: string,
) {
  if (filter === "all") {
    return true;
  }
  if (!selectedStoreId) {
    return filter === "following_current";
  }
  const isCrossStore = isGroupedSelectionCrossStore(selectedStoreId, currentStoreId);
  if (filter === "cross_store") {
    return isCrossStore;
  }
  return filter === "current_store" ? !isCrossStore : false;
}

function isGroupedSelectionCrossStore(selectedStoreId: string, currentStoreId: string) {
  return Boolean(selectedStoreId) && Boolean(currentStoreId) && selectedStoreId !== currentStoreId;
}
