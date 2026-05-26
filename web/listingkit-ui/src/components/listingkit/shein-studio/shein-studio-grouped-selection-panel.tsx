"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Select } from "@/components/ui/select";
import type {
  GroupedSDSSelectionEligibility,
  SDSBaselineStatus,
} from "@/lib/types/sds-baseline";
import type { SDSProductVariantSelection } from "@/lib/types/sds";

type GroupedCandidate = {
  selectionId: string;
  selection: SDSProductVariantSelection;
  baselineStatus: SDSBaselineStatus;
  baselineKey?: string;
  baselineReason: string;
  eligible: boolean;
  eligibilityReason?: string;
};

type StoreOption = {
  store_id: number;
  name?: string;
  site?: string;
};

export function SheinStudioGroupedSelectionPanel({
  activeSelection,
  activeSelectionBaselineStatus,
  activeSelectionBaselineReason,
  candidates,
  groupedSelections,
  onAddSelection,
  onRemoveSelection,
  onUpdateSelectionStore,
  storeOptions,
}: {
  activeSelection?: SDSProductVariantSelection;
  activeSelectionBaselineStatus: SDSBaselineStatus;
  activeSelectionBaselineReason: string;
  candidates: GroupedCandidate[];
  groupedSelections: GroupedSDSSelectionEligibility[];
  onAddSelection: (candidate: GroupedCandidate) => void;
  onRemoveSelection: (selectionId: string) => void;
  onUpdateSelectionStore: (selectionId: string, storeId: string) => void;
  storeOptions: StoreOption[];
}) {
  if (!activeSelection?.variantId) {
    return null;
  }

  const selectedIDs = new Set(groupedSelections.map((item) => item.selectionId));

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
            当前版本只允许加入 baseline 已准备好、且印刷区域与当前商品兼容的 SDS 变体。
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
          <div className="grid gap-3">
            {groupedSelections.map((item) => (
              <div
                className="rounded-2xl border border-emerald-200 bg-white px-4 py-4"
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
                  <Select
                    onChange={(event) =>
                      onUpdateSelectionStore(item.selectionId, event.target.value)
                    }
                    value={item.sheinStoreId}
                  >
                    <option value="">跟随当前店铺</option>
                    {storeOptions.map((option) => (
                      <option key={option.store_id} value={String(option.store_id)}>
                        {formatStoreLabel(option)}
                      </option>
                    ))}
                  </Select>
                </div>
              </div>
            ))}
          </div>
        </div>
      ) : null}

      <div className="mt-4 space-y-3">
        <div className="text-[11px] font-semibold uppercase tracking-[0.24em] text-zinc-500">
          最近使用的 SDS 变体
        </div>
        {candidates.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-zinc-200 bg-white px-4 py-4 text-sm text-zinc-500">
            暂时没有可加入分组的最近变体。先去选过其他 SDS 商品，再回来这里批量创建。
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

function formatStoreLabel(option: StoreOption) {
  return [option.name?.trim(), option.site?.trim()].filter(Boolean).join(" · ") || String(option.store_id);
}
