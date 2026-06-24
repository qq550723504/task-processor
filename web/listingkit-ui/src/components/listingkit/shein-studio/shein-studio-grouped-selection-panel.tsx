"use client";

import { useMemo, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { SheinStudioSelectionOverview } from "@/components/listingkit/shein-studio/shein-studio-selection-overview";
import { Select } from "@/components/ui/select";
import { formatSheinStoreOptionLabel } from "@/lib/shein-studio/store-option-label";
import type {
  GroupedSDSSelectionEligibility,
  SDSBaselineStatus,
} from "@/lib/types/sds-baseline";
import {
  getSDSBaselineStatusBadgeVariant,
  getSDSBaselineStatusLabel,
  getSDSBaselineReasonMessage,
} from "@/lib/shein-studio/sds-baseline-ui";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { ListingKitStoreProfile } from "@/lib/types/listingkit";

type StoreOption = Pick<ListingKitStoreProfile, "store_id" | "store" | "site">;
type GroupedStoreFilter = "all" | "following_current" | "current_store" | "cross_store";

type GroupedSelectionBaselineStatus = {
  baselineKey?: string;
  reason: string;
  reasonCode?: string;
  status: SDSBaselineStatus;
};

export function evaluateGroupedSelectionCompatibility(
  activeSelection?: SDSProductVariantSelection,
  candidate?: SDSProductVariantSelection,
) {
  if (!activeSelection?.variantId || !candidate?.variantId) {
    return { compatible: false, reason: "缺少 SDS 选择信息，暂时无法加入分组。" };
  }
  if (activeSelection.variantId === candidate.variantId) {
    return { compatible: false, reason: "这个商品已经在当前批次里，无需重复加入。" };
  }
  return { compatible: true, reason: "" };
}

export function projectGroupedSelectionBaselineEligibility({
  activeSelection,
  baselineStatuses,
  groupedSelections,
}: {
  activeSelection?: SDSProductVariantSelection;
  baselineStatuses: Record<string, GroupedSelectionBaselineStatus>;
  groupedSelections: GroupedSDSSelectionEligibility[];
}): GroupedSDSSelectionEligibility[] {
  return groupedSelections.map((item) => {
    const baseline = baselineStatuses[item.selectionId] ?? {
      baselineKey: item.baselineKey,
      reason: item.baselineReason,
      reasonCode: undefined,
      status: item.baselineStatus,
    };
    const baselineReason =
      baseline.reason || getSDSBaselineReasonMessage(baseline.reasonCode);
    const compatibility = evaluateGroupedSelectionCompatibility(
      activeSelection,
      item.selection,
    );
    return {
      ...item,
      baselineKey: baseline.baselineKey,
      baselineStatus: baseline.status,
      baselineReason,
      baselineReasonCode: baseline.reasonCode,
      eligible: baseline.status === "ready" && compatibility.compatible,
      eligibilityReason:
        baseline.status !== "ready"
          ? baselineReason || "只有通过 baseline 校验的 SDS 商品才能加入分组。"
          : compatibility.reason,
    };
  });
}

export function SheinStudioGroupedSelectionPanel({
  activeSelection,
  activeSelectionBaselineAction,
  activeSelectionBaselineStatus,
  activeSelectionBaselineReason,
  currentStoreId,
  groupedSelections,
  currentStoreLabel,
  onBulkUpdateSelectionStore,
  onRemoveSelection,
  onUpdateSelectionStore,
  printableAreaLabel,
  selectedColorCount,
  selectedSizeCount,
  selectedVariantCount,
  storeOptions,
}: {
  activeSelection?: SDSProductVariantSelection;
  activeSelectionBaselineAction?: {
    label: string;
    onClick: () => void;
  } | null;
  activeSelectionBaselineStatus: SDSBaselineStatus;
  activeSelectionBaselineReason: string;
  currentStoreId?: string;
  currentStoreLabel?: string;
  groupedSelections: GroupedSDSSelectionEligibility[];
  onBulkUpdateSelectionStore: (selectionIds: string[], storeId: string) => void;
  onRemoveSelection: (selectionId: string) => void;
  onUpdateSelectionStore: (selectionId: string, storeId: string) => void;
  printableAreaLabel: string;
  selectedColorCount: number;
  selectedSizeCount: number;
  selectedVariantCount: number;
  storeOptions: StoreOption[];
}) {
  const selectedIDs = new Set(groupedSelections.map((item) => item.selectionId));
  const [activeStoreFilter, setActiveStoreFilter] = useState<GroupedStoreFilter>("all");
  const [bulkStoreId, setBulkStoreId] = useState("");
  const [bulkUpdateFeedback, setBulkUpdateFeedback] = useState("");
  const storeLabelById = useMemo(
    () =>
      new Map(
        storeOptions.map((option) => [
          String(option.store_id),
          formatSheinStoreOptionLabel(option),
        ]),
      ),
    [storeOptions],
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

  function handleStoreFilterChange(nextFilter: GroupedStoreFilter) {
    setBulkUpdateFeedback("");
    setActiveStoreFilter((current) => (current === nextFilter ? "all" : nextFilter));
  }

  if (!activeSelection?.variantId) {
    return null;
  }

  return (
    <section className="rounded-[1.5rem] border border-zinc-200 bg-zinc-50/80 px-4 py-4">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
            批次商品
          </p>
          <h3 className="mt-1 text-lg font-semibold text-zinc-950">
            当前批次商品
          </h3>
          <p className="mt-1 max-w-3xl text-sm leading-6 text-zinc-600">
            这些商品会随批次一起保存，并参与后续生成或创建 SHEIN 资料。
          </p>
        </div>
        <Badge className="rounded-full px-3 py-1 text-xs" variant="neutral">
          已加入 {groupedSelections.length} 款
        </Badge>
      </div>

      <div className="mt-4 space-y-3">
        <SheinStudioSelectionOverview
          footer={
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="min-w-0 flex-1">
                <div className="flex flex-wrap items-center gap-2">
                  <BaselineStatusBadge
                    status={activeSelectionBaselineStatus}
                    reason={activeSelectionBaselineReason}
                  />
                  <span className="text-xs leading-6 text-zinc-600">
                    {activeSelectionBaselineStatus === "ready"
                      ? "当前入口商品已就绪，可作为批次起点继续生成。"
                      : activeSelectionBaselineReason || "当前商品的 baseline 还需要先处理。"}
                  </span>
                </div>
              </div>
              {activeSelectionBaselineStatus !== "ready" && activeSelectionBaselineAction ? (
                <Button
                  className="shrink-0"
                  onClick={activeSelectionBaselineAction.onClick}
                  size="sm"
                  type="button"
                  variant="secondary"
                >
                  {activeSelectionBaselineAction.label}
                </Button>
              ) : null}
            </div>
          }
          printableAreaLabel={printableAreaLabel}
          selectedColorCount={selectedColorCount}
          selectedSizeCount={selectedSizeCount}
          selectedVariantCount={selectedVariantCount}
          selection={activeSelection}
        />
      </div>

      {groupedSelections.length > 0 ? (
        <div className="mt-4 space-y-3">
          <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
            已加入当前批次
          </div>
          <div className="flex flex-wrap gap-2">
            {groupedStoreSummary.followingCurrent > 0 ? (
              <button
                className="cursor-pointer"
                onClick={() => handleStoreFilterChange("following_current")}
                type="button"
              >
                <Badge
                  className="rounded-full px-3 py-1 text-xs"
                  variant={activeStoreFilter === "following_current" ? "default" : "neutral"}
                >
                跟随批次店铺 {groupedStoreSummary.followingCurrent} 款
                </Badge>
              </button>
            ) : null}
            {groupedStoreSummary.currentStore > 0 ? (
              <button
                className="cursor-pointer"
                onClick={() => handleStoreFilterChange("current_store")}
                type="button"
              >
                <Badge
                  className="rounded-full px-3 py-1 text-xs"
                  variant={activeStoreFilter === "current_store" ? "default" : "success"}
                >
                批次店铺 {groupedStoreSummary.currentStore} 款
                </Badge>
              </button>
            ) : null}
            {groupedStoreSummary.crossStore > 0 ? (
              <button
                className="cursor-pointer"
                onClick={() => handleStoreFilterChange("cross_store")}
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
                  <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap sm:items-end">
                    <div className="min-w-0 flex-1">
                      <div className="text-sm font-semibold text-zinc-950">
                        批量改店铺
                      </div>
                      <div className="mt-1 text-xs text-zinc-500">
                        当前筛选命中 {filteredSelectionIds.length} 款商品，可统一改成同一家店，或改回跟随批次店铺。
                      </div>
                    </div>
                    <div className="w-full sm:max-w-sm">
                      <label
                        className="mb-1 block text-xs font-medium text-zinc-600"
                        htmlFor="grouped-selection-bulk-store"
                      >
                        应用到当前筛选
                      </label>
                      <Select
                        className="h-11 rounded-2xl px-4 py-2 leading-5"
                        id="grouped-selection-bulk-store"
                        onChange={(event) => {
                          setBulkUpdateFeedback("");
                          setBulkStoreId(event.target.value);
                        }}
                        value={bulkStoreId}
                      >
                        <option value="">
                          {currentStoreLabel
                            ? "跟随批次店铺"
                            : "请先设置批次店铺"}
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
                            ? `跟随批次店铺（${currentStoreLabel}）`
                            : "请先设置批次店铺";
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
          <div className="grid gap-3 xl:grid-cols-2">
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
                  <div className="space-y-4">
                    <div className="min-w-0 flex-1">
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
                    </div>
                    <div className="w-full rounded-2xl border border-zinc-200/80 bg-zinc-50/70 px-3 py-3">
                      <label className="mb-1 block text-xs font-medium text-zinc-600">
                        商品店铺
                      </label>
                      <div className="mb-2 flex flex-wrap items-center gap-2 text-xs text-zinc-500">
                        {item.sheinStoreId.trim()
                          ? `已指定店铺：${
                              storeLabelById.get(item.sheinStoreId.trim()) ?? item.sheinStoreId.trim()
                            }`
                          : currentStoreLabel
                            ? `默认跟随批次店铺：${currentStoreLabel}`
                            : "无法跟随批次店铺，请先设置批次店铺"}
                        {isCrossStore ? (
                          <Badge className="rounded-full px-2 py-0.5 text-[10px]" variant="warning">
                            跨店铺
                          </Badge>
                        ) : null}
                      </div>
                      <Select
                        className="h-11 rounded-2xl px-4 py-2 leading-5"
                        onChange={(event) =>
                          onUpdateSelectionStore(item.selectionId, event.target.value)
                        }
                        value={item.sheinStoreId}
                      >
                        <option value="">
                          {currentStoreLabel ? "跟随批次店铺" : "请先设置批次店铺"}
                        </option>
                        {storeOptions.map((option) => (
                          <option key={option.store_id} value={String(option.store_id)}>
                            {formatSheinStoreOptionLabel(option)}
                          </option>
                        ))}
                      </Select>
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      ) : null}
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
  const label = getSDSBaselineStatusLabel(status);
  const variant = getSDSBaselineStatusBadgeVariant(status);
  return (
    <Badge title={reason || label} variant={variant}>
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
