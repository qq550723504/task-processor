"use client";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import {
  buildGroupedSDSSelectionID,
  type SDSBaselineStatus,
} from "@/lib/types/sds-baseline";

type GroupedCandidateBaselineState = {
  reason: string;
  status: SDSBaselineStatus | "loading";
};

export function SDSGroupedCandidatesPanel({
  items,
  activeSelection,
  baselineStatuses,
  onRemove,
  onSelect,
}: {
  items: SDSProductVariantSelection[];
  activeSelection?: SDSProductVariantSelection;
  baselineStatuses: Record<string, GroupedCandidateBaselineState>;
  onRemove: (selection: SDSProductVariantSelection) => void;
  onSelect: (
    selection: SDSProductVariantSelection,
    baseline: GroupedCandidateBaselineState,
  ) => void;
}) {
  if (items.length === 0) {
    return null;
  }

  const activeSelectionId = buildGroupedSDSSelectionID(activeSelection);

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <div className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
            批量候选池
          </div>
          <p className="mt-1 text-sm text-zinc-600">
            这里存放准备进入 grouped SDS 批量上品的候选商品，可以随时回选或移除。
          </p>
        </div>
        <Badge className="rounded-md px-3 py-2 text-sm" variant="neutral">
          {items.length} 款候选
        </Badge>
      </div>
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-3">
        {items.map((item) => {
          const selectionId = buildGroupedSDSSelectionID(item);
          const active = selectionId === activeSelectionId;
          const baseline = baselineStatuses[selectionId] ?? {
            status: "loading" as const,
            reason: "正在检查 baseline 状态...",
          };
          return (
            <div
              className={`rounded-[1.5rem] border px-4 py-4 shadow-sm ${
                active
                  ? "border-emerald-800 bg-[linear-gradient(135deg,_#052e2b,_#115e59)] text-white"
                  : "border-zinc-200 bg-white"
              }`}
              key={buildGroupedSDSSelectionID(item)}
            >
              <div className="space-y-2">
                <div className="flex items-start justify-between gap-3">
                  <div className="line-clamp-2 text-sm font-semibold leading-6">
                    {item.productName}
                  </div>
                  <BaselineStatusBadge
                    reason={baseline.reason}
                    status={baseline.status}
                  />
                </div>
                <div className={active ? "text-emerald-100" : "text-zinc-500"}>
                  变体 ID {item.variantId}
                </div>
                <div className={active ? "text-emerald-100" : "text-zinc-500"}>
                  {item.variantLabel}
                </div>
                {item.printableWidth && item.printableHeight ? (
                  <div className={active ? "text-emerald-100" : "text-zinc-500"}>
                    印刷区域 {item.printableWidth} × {item.printableHeight}
                  </div>
                ) : null}
                {baseline.status !== "ready" ? (
                  <div
                    className={`rounded-xl px-3 py-2 text-xs leading-5 ${
                      active
                        ? "bg-white/10 text-emerald-50"
                        : baseline.status === "failed"
                          ? "bg-rose-50 text-rose-700"
                          : baseline.status === "missing"
                            ? "bg-amber-50 text-amber-700"
                            : "bg-zinc-100 text-zinc-600"
                    }`}
                  >
                    {buildBaselineHelperText(baseline)}
                  </div>
                ) : null}
                <div className="flex gap-2 pt-1">
                  <Button
                    className="flex-1"
                    onClick={() => onSelect(item, baseline)}
                    type="button"
                    variant={active ? "secondary" : "primary"}
                  >
                    {active
                      ? "当前已选"
                      : buildBaselineActionLabel(baseline)}
                  </Button>
                  <Button
                    onClick={() => onRemove(item)}
                    type="button"
                    variant="ghost"
                  >
                    移除
                  </Button>
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

function BaselineStatusBadge({
  status,
  reason,
}: {
  status: SDSBaselineStatus | "loading";
  reason?: string;
}) {
  const label =
    status === "ready"
      ? "Baseline 已就绪"
      : status === "failed"
        ? "Baseline 异常"
        : status === "missing"
          ? "Baseline 缺失"
          : "Baseline 检查中";
  const variant =
    status === "ready"
      ? "success"
      : status === "failed"
        ? "danger"
        : status === "missing"
          ? "warning"
          : "neutral";
  return (
    <Badge
      className="shrink-0"
      title={reason || label}
      variant={variant as "success" | "danger" | "warning" | "neutral"}
    >
      {label}
    </Badge>
  );
}

function buildBaselineHelperText(baseline: GroupedCandidateBaselineState) {
  if (baseline.status === "loading") {
    return baseline.reason || "正在读取 baseline 状态，稍后就能判断是否可加入分组。";
  }
  if (baseline.status === "failed") {
    return baseline.reason || "baseline 检查失败，建议先排查这款 SDS 商品的缓存或转换链路。";
  }
  if (baseline.status === "missing") {
    return baseline.reason || "这款商品还没有 baseline 缓存，暂时不能加入 grouped 批量上品。";
  }
  return "";
}

function buildBaselineActionLabel(baseline: GroupedCandidateBaselineState) {
  if (baseline.status === "missing") {
    return "回选并预热";
  }
  if (baseline.status === "failed") {
    return "回选并重试";
  }
  if (baseline.status === "loading") {
    return "回选并等待";
  }
  return "回选这个变体";
}
