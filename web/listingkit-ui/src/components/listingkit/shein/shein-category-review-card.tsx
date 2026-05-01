"use client";

import { useState } from "react";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import { searchSheinCategories } from "@/lib/api/shein-category-search";
import type {
  SheinCategorySuggestion,
  SheinEditorContext,
  SheinManualCategoryCandidate,
} from "@/lib/types/listingkit";

function joinPath(path?: string[] | null): string {
  return path?.filter(Boolean).join(" > ") ?? "";
}

function buildSheinCategoryReviewModel(editorContext?: SheinEditorContext | null) {
  if (!editorContext) {
    return null;
  }

  const currentCategory = editorContext.category?.current;
  const currentSale = editorContext.sale_attributes?.current;
  const revisionSale = editorContext.revision_skeleton?.shein?.sale_attribute_resolution;

  const recommendCategoryReview =
    currentSale?.recommend_category_review ??
    revisionSale?.recommend_category_review ??
    false;
  const categoryReviewReason =
    currentSale?.category_review_reason ??
    revisionSale?.category_review_reason ??
    "";
  const suggestedCategory = currentCategory?.suggested_category;
  const isSuggestionApplied =
    Boolean(suggestedCategory?.category_id) &&
    suggestedCategory?.category_id === currentCategory?.category_id;

  if (
    !recommendCategoryReview &&
    !suggestedCategory?.category_id &&
    !currentCategory?.category_id &&
    !currentCategory?.category_path?.length
  ) {
    return null;
  }

  return {
    recommendCategoryReview,
    categoryReviewReason,
    currentPath: joinPath(currentCategory?.category_path),
    currentCategoryId: currentCategory?.category_id,
    suggestedCategory,
    isSuggestionApplied,
  };
}

function SuggestionRow({
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

function SuggestedCategoryBlock({
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
          {isApplied ? "Applied category" : "Suggested category"}
        </p>
        <p className="text-sm leading-6 text-zinc-700">
          {isApplied
            ? "The suggested category has already been applied to the current SHEIN draft."
            : "Safe alternate candidate accepted by the reselection guardrail."}
        </p>
      </div>
      <dl className="grid gap-3">
        <SuggestionRow
          label="Category path"
          value={joinPath(suggestion.matched_path)}
        />
        <SuggestionRow label="Category ID" value={suggestion.category_id} />
        <SuggestionRow label="Source" value={suggestion.source} />
        <SuggestionRow label="Reason" value={suggestion.reason} />
      </dl>
    </div>
  );
}

function ManualCategorySearchResults({
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
                {joinPath(candidate.category_path)}
              </p>
              <div className="flex flex-wrap gap-2 text-xs text-zinc-500">
                <span>Category ID: {candidate.category_id}</span>
                {candidate.product_type_id ? (
                  <span>Product type: {candidate.product_type_id}</span>
                ) : null}
                {candidate.source ? <span>Source: {candidate.source}</span> : null}
              </div>
            </div>
            <div className="mt-3 flex justify-end">
              <Button
                tone="secondary"
                disabled={!candidate.category_id || applyingCategoryId === categoryId}
                onClick={() => onApply(candidate)}
              >
                {applyingCategoryId === categoryId ? "Applying..." : "使用这个类目"}
              </Button>
            </div>
          </div>
        );
      })}
    </div>
  );
}

export function SheinCategoryReviewCard({
  taskId,
  editorContext,
  isApplying = false,
  onApplySuggestedCategory,
  onApplyManualCategory,
}: {
  taskId: string;
  editorContext?: SheinEditorContext | null;
  isApplying?: boolean;
  onApplySuggestedCategory?: (() => void) | null;
  onApplyManualCategory?: ((candidate: SheinManualCategoryCandidate) => Promise<void> | void) | null;
}) {
  const model = buildSheinCategoryReviewModel(editorContext);
  const [manualQuery, setManualQuery] = useState("");
  const [manualResults, setManualResults] = useState<SheinManualCategoryCandidate[]>([]);
  const [manualSearchError, setManualSearchError] = useState<string | null>(null);
  const [manualSearchLoading, setManualSearchLoading] = useState(false);
  const [manualApplyingCategoryId, setManualApplyingCategoryId] = useState<number | null>(null);

  if (!model) {
    return null;
  }

  const handleSearch = async () => {
    const trimmedQuery = manualQuery.trim();
    if (!trimmedQuery) {
      setManualSearchError("先输入类目关键词再搜索。");
      setManualResults([]);
      return;
    }

    setManualSearchLoading(true);
    setManualSearchError(null);
    try {
      const result = await searchSheinCategories(taskId, trimmedQuery);
      setManualResults(result.items ?? []);
      if (!(result.items?.length ?? 0)) {
        setManualSearchError("没有找到匹配类目。");
      }
    } catch (error) {
      setManualResults([]);
      setManualSearchError(
        error instanceof Error ? error.message : "类目搜索失败，请稍后重试。",
      );
    } finally {
      setManualSearchLoading(false);
    }
  };

  const handleApplyManualCategory = async (
    candidate: SheinManualCategoryCandidate,
  ) => {
    if (!candidate.category_id || !onApplyManualCategory) {
      return;
    }
    setManualApplyingCategoryId(candidate.category_id);
    setManualSearchError(null);
    try {
      await onApplyManualCategory(candidate);
    } finally {
      setManualApplyingCategoryId(null);
    }
  };

  return (
    <Card className="border-sky-200 bg-sky-50/60 p-5">
      <div className="space-y-4">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.18em] text-sky-700">
            SHEIN category review
          </p>
          <p className="mt-1 text-sm leading-6 text-zinc-700">
            {model.isSuggestionApplied
              ? "The current category already matches the accepted suggestion."
              : "Current category mapping needs review before final submission."}
          </p>
        </div>

        <dl className="grid gap-3">
          <SuggestionRow label="Current category path" value={model.currentPath} />
          <SuggestionRow label="Current category ID" value={model.currentCategoryId} />
          <SuggestionRow label="Review reason" value={model.categoryReviewReason} />
        </dl>

        <SuggestedCategoryBlock
          suggestion={model.suggestedCategory}
          isApplied={model.isSuggestionApplied}
        />

        {model.suggestedCategory?.category_id &&
        !model.isSuggestionApplied &&
        onApplySuggestedCategory ? (
          <div className="flex justify-end">
            <Button
              tone="secondary"
              disabled={isApplying}
              onClick={onApplySuggestedCategory}
            >
              {isApplying ? "Applying..." : "Apply suggested category"}
            </Button>
          </div>
        ) : null}

        <div className="space-y-3 rounded-2xl border border-zinc-200/80 bg-white/70 p-4">
          <div className="space-y-1">
            <p className="text-xs font-semibold uppercase tracking-[0.18em] text-zinc-600">
              手工选类目
            </p>
            <p className="text-sm leading-6 text-zinc-700">
              不知道 category_id 也可以直接搜关键词，找到候选后应用到当前任务。
            </p>
          </div>

          <div className="flex flex-col gap-3 sm:flex-row">
            <input
              className="h-10 flex-1 rounded-xl border border-zinc-200 bg-white px-3 text-sm text-zinc-900 outline-none ring-0 placeholder:text-zinc-400 focus:border-zinc-400"
              placeholder="例如：sleep mask、eye mask、家居装饰"
              value={manualQuery}
              onChange={(event) => setManualQuery(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  void handleSearch();
                }
              }}
            />
            <Button
              tone="secondary"
              disabled={manualSearchLoading}
              onClick={() => void handleSearch()}
            >
              {manualSearchLoading ? "Searching..." : "搜索类目"}
            </Button>
          </div>

          {manualSearchError ? (
            <p className="text-sm text-rose-600">{manualSearchError}</p>
          ) : null}

          {manualResults.length ? (
            <ManualCategorySearchResults
              items={manualResults}
              applyingCategoryId={
                isApplying ? manualApplyingCategoryId ?? -1 : manualApplyingCategoryId
              }
              onApply={handleApplyManualCategory}
            />
          ) : null}
        </div>
      </div>
    </Card>
  );
}
