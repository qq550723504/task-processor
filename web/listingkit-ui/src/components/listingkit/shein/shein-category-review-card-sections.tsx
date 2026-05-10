import { Button } from "@/components/shared/button";
import {
  formatCategoryLabel,
  joinPath,
} from "@/components/listingkit/shein/shein-category-review-card-model";
import type {
  SheinCategorySuggestion,
  SheinManualCategoryCandidate,
} from "@/lib/types/listingkit";

export function SuggestionRow({
  label,
  value,
}: {
  label: string;
  value?: string | number | null;
}) {
  if (value === undefined || value === null || value === "") {
    return null;
  }
  return (
    <div className="grid gap-1 rounded-xl border border-zinc-200/80 bg-white/80 px-3 py-2">
      <dt className="text-[11px] font-medium uppercase tracking-[0.18em] text-zinc-500">
        {label}
      </dt>
      <dd className="text-sm text-zinc-800">{value}</dd>
    </div>
  );
}

export function SuggestedCategoryBlock({
  suggestion,
  isApplied = false,
}: {
  suggestion?: SheinCategorySuggestion | null;
  isApplied?: boolean;
}) {
  if (!suggestion?.category_id) {
    return null;
  }

  return (
    <div className="space-y-3 rounded-2xl border border-emerald-200 bg-emerald-50/70 p-4">
      <div className="space-y-1">
        <p className="text-xs font-semibold uppercase tracking-[0.18em] text-emerald-700">
          {isApplied ? "已应用类目" : "建议类目"}
        </p>
        <p className="text-sm leading-6 text-zinc-700">
          {isApplied
            ? "建议类目已经应用到当前 SHEIN 草稿。"
            : "这是通过重选保护规则筛出的更稳妥候选类目。"}
        </p>
      </div>
      <dl className="grid gap-3">
        <SuggestionRow
          label="类目路径"
          value={formatCategoryLabel(
            suggestion.matched_path,
            suggestion.category_id,
          )}
        />
        <SuggestionRow label="来源" value={suggestion.source} />
        <SuggestionRow label="原因" value={suggestion.reason} />
      </dl>
    </div>
  );
}

export function ManualCategorySearchResults({
  items,
  applyingCategoryId,
  onApply,
}: {
  items: SheinManualCategoryCandidate[];
  applyingCategoryId?: number | null;
  onApply: (candidate: SheinManualCategoryCandidate) => void;
}) {
  if (!items.length) {
    return (
      <div className="rounded-xl border border-dashed border-zinc-300 bg-zinc-50/70 px-4 py-3 text-sm text-zinc-600">
        没有找到匹配类目，试试更短的关键词、中文类目词或英文品类词。
      </div>
    );
  }

  return (
    <div className="space-y-3">
      {items.map((candidate) => {
        const categoryId = candidate.category_id ?? 0;
        return (
          <div
            key={`${categoryId}-${joinPath(candidate.category_path)}`}
            className="rounded-2xl border border-zinc-200 bg-white px-4 py-3"
          >
            <div className="space-y-2">
              <p className="text-sm font-medium text-zinc-900">
                {formatCategoryLabel(
                  candidate.category_path,
                  candidate.category_id,
                )}
              </p>
              <div className="flex flex-wrap gap-2 text-xs text-zinc-500">
                {candidate.product_type_id ? (
                  <span>商品类型: {candidate.product_type_id}</span>
                ) : null}
                {candidate.source ? <span>来源: {candidate.source}</span> : null}
              </div>
            </div>
            <div className="mt-3 flex justify-end">
              <Button
                tone="secondary"
                disabled={!candidate.category_id || applyingCategoryId === categoryId}
                onClick={() => onApply(candidate)}
              >
                {applyingCategoryId === categoryId ? "应用中..." : "使用这个类目"}
              </Button>
            </div>
          </div>
        );
      })}
    </div>
  );
}
