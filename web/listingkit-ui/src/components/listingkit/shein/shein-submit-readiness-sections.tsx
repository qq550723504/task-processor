import { Button } from "@/components/shared/button";
import {
  cacheSourceLabel,
  cacheUpdatedLabel,
  fieldPathsLabel,
} from "@/components/listingkit/shein/shein-submit-readiness-helpers";
import type {
  SheinChecklistGroupItem,
  SheinReadinessItem,
  SheinResolutionCacheInfo,
  SheinResolutionCacheSummary,
} from "@/lib/types/listingkit";

export type ResolutionCacheKind = "category" | "attribute" | "sale_attribute";

export function ResolutionCacheRow({
  title,
  item,
  kind,
  onClear,
  isClearing,
}: {
  title: string;
  item?: SheinResolutionCacheInfo | null;
  kind: ResolutionCacheKind;
  onClear?: ((kind: ResolutionCacheKind) => void) | null;
  isClearing?: boolean;
}) {
  return (
    <div className="rounded-2xl border border-zinc-200 bg-white/80 p-3">
      <div className="flex items-start justify-between gap-3">
        <div className="min-w-0 space-y-1">
          <p className="text-sm font-semibold text-zinc-950">{title}</p>
          <p className="text-xs leading-5 text-zinc-600">
            {item
              ? `${cacheSourceLabel(item.source)} · ${item.short_key ?? "无 key"}`
              : "暂无缓存信息"}
          </p>
          {item ? (
            <p className="text-[11px] leading-5 text-zinc-500">
              {item.status ?? "未知"} · 命中 {item.hit_count ?? 0} ·{" "}
              {cacheUpdatedLabel(item.updated_at)}
              {item.manual ? " · 人工确认" : ""}
            </p>
          ) : null}
        </div>
        {item?.clearable && onClear ? (
          <Button
            className="h-8 shrink-0 px-3 text-xs"
            disabled={isClearing}
            tone="secondary"
            onClick={() => onClear(kind)}
          >
            清除
          </Button>
        ) : null}
      </div>
    </div>
  );
}

export function ReadinessItems({
  title,
  items,
  actionLabel = "去处理",
  canSelectItem,
  onSelectItem,
}: {
  title: string;
  items?: SheinReadinessItem[] | null;
  actionLabel?: string;
  canSelectItem?: ((item: SheinReadinessItem) => boolean) | null;
  onSelectItem?: ((item: SheinReadinessItem) => void) | null;
}) {
  if (!items?.length) {
    return null;
  }

  return (
    <div className="space-y-3">
      <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
        {title}
      </p>
      <div className="space-y-3">
        {items.map((item) => {
          const canAct = canSelectItem ? canSelectItem(item) : false;
          return (
            <div
              className="space-y-2 rounded-2xl border border-zinc-200 bg-white/80 p-4"
              key={`${title}-${item.key}-${item.label}`}
            >
              <div className="space-y-1">
                <p className="text-sm font-semibold text-zinc-950">
                  {item.label ?? item.key ?? "未命名问题"}
                </p>
                {item.message ? (
                  <p className="text-sm leading-6 text-zinc-700">{item.message}</p>
                ) : null}
              </div>
              {item.reason?.summary ? (
                <p className="text-xs leading-5 text-zinc-600">
                  {item.reason.summary}
                </p>
              ) : null}
              {fieldPathsLabel(item.field_paths) ? (
                <p className="text-[11px] uppercase tracking-[0.16em] text-zinc-500">
                  {fieldPathsLabel(item.field_paths)}
                </p>
              ) : null}
              <div className="flex flex-wrap items-center gap-2">
                {item.suggested_action ? (
                  <span className="rounded-full border border-zinc-200 bg-zinc-100 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-700">
                    {item.suggested_action}
                  </span>
                ) : null}
                {canAct && onSelectItem ? (
                  <Button
                    className="h-8 px-3 text-xs"
                    tone="secondary"
                    onClick={() => onSelectItem(item)}
                  >
                    {actionLabel}
                  </Button>
                ) : null}
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}

export function SubmitFailureGuidance({
  detail,
  impact,
  nextStep,
}: {
  detail: string;
  impact: string;
  nextStep: string;
}) {
  return (
    <div className="space-y-3">
      <div className="space-y-1">
        <p className="text-xs font-semibold uppercase tracking-[0.18em] text-rose-700">
          发生了什么
        </p>
        <p className="break-words text-sm leading-6 text-rose-700">{detail}</p>
      </div>
      <div className="space-y-1">
        <p className="text-xs font-semibold uppercase tracking-[0.18em] text-rose-700">
          可能影响
        </p>
        <p className="text-sm leading-6 text-zinc-700">{impact}</p>
      </div>
      <div className="space-y-1">
        <p className="text-xs font-semibold uppercase tracking-[0.18em] text-rose-700">
          下一步怎么做
        </p>
        <p className="text-sm leading-6 text-zinc-700">{nextStep}</p>
      </div>
    </div>
  );
}

export function ChecklistSection({
  title,
  items,
}: {
  title: string;
  items?: SheinChecklistGroupItem[] | null;
}) {
  if (!items?.length) {
    return null;
  }

  return (
    <div className="space-y-2">
      <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
        {title}
      </p>
      <div className="space-y-2">
        {items.map((item) => (
          <div
            className="flex items-start justify-between gap-3 rounded-xl border border-zinc-200/80 bg-white/70 px-3 py-2"
            key={`${title}-${item.key}-${item.label}`}
          >
            <div className="space-y-1">
              <p className="text-sm font-medium text-zinc-900">
                {item.label ?? item.key ?? "未命名检查项"}
              </p>
              {item.message ? (
                <p className="text-xs leading-5 text-zinc-600">{item.message}</p>
              ) : null}
            </div>
            <span className="rounded-full border border-zinc-200 bg-zinc-100 px-2 py-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-zinc-700">
              {item.status ?? "未知"}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

export function ResolutionCacheSummaryCard({
  clearingResolutionCacheKind,
  onClearResolutionCache,
  resolutionCache,
}: {
  clearingResolutionCacheKind?: string | null;
  onClearResolutionCache?: ((kind: ResolutionCacheKind) => void) | null;
  resolutionCache?: SheinResolutionCacheSummary | null;
}) {
  return (
    <div className="rounded-2xl border border-zinc-200 bg-white/70 p-4">
      <div className="space-y-3">
        <div className="space-y-1">
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-500">
            解析缓存
          </p>
          <p className="text-sm leading-6 text-zinc-700">
            分类、普通属性、销售属性的解析来源和缓存状态。
          </p>
        </div>
        <ResolutionCacheRow
          title="类目"
          item={resolutionCache?.category}
          kind="category"
          isClearing={clearingResolutionCacheKind === "category"}
          onClear={onClearResolutionCache}
        />
        <ResolutionCacheRow
          title="普通属性"
          item={resolutionCache?.attributes}
          kind="attribute"
          isClearing={clearingResolutionCacheKind === "attribute"}
          onClear={onClearResolutionCache}
        />
        <ResolutionCacheRow
          title="销售属性"
          item={resolutionCache?.sale_attributes}
          kind="sale_attribute"
          isClearing={clearingResolutionCacheKind === "sale_attribute"}
          onClear={onClearResolutionCache}
        />
      </div>
    </div>
  );
}
