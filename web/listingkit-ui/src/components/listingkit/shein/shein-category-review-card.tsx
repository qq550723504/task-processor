"use client";

import { useState } from "react";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import {
  buildSheinCategoryReviewModel,
  formatCategoryLabel,
} from "@/components/listingkit/shein/shein-category-review-card-model";
import {
  ManualCategorySearchResults,
  SuggestedCategoryBlock,
  SuggestionRow,
} from "@/components/listingkit/shein/shein-category-review-card-sections";
import { searchSheinCategories } from "@/lib/api/shein-category-search";
import type {
  SheinEditorContext,
  SheinManualCategoryCandidate,
} from "@/lib/types/listingkit";

export function SheinCategoryReviewCard({
  taskId,
  editorContext,
  isApplying = false,
  onApplySuggestedCategory,
  onConfirmCurrentCategory,
  onApplyManualCategory,
}: {
  taskId: string;
  editorContext?: SheinEditorContext | null;
  isApplying?: boolean;
  onApplySuggestedCategory?: (() => void) | null;
  onConfirmCurrentCategory?: (() => void) | null;
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
    <Card
      className={
        model.isReviewNeeded
          ? "border-sky-200 bg-sky-50/60 p-5"
          : "border-emerald-200 bg-emerald-50/60 p-5"
      }
    >
      <div className="space-y-4">
        <div>
          <p
            className={
              model.isReviewNeeded
                ? "text-xs font-semibold uppercase tracking-[0.18em] text-sky-700"
                : "text-xs font-semibold uppercase tracking-[0.18em] text-emerald-700"
            }
          >
            {model.isReviewNeeded ? "SHEIN 类目审核" : "SHEIN 类目已确认"}
          </p>
          <p className="mt-1 text-sm leading-6 text-zinc-700">
            {model.isSuggestionApplied
              ? "当前类目已经与接受的建议类目一致。"
              : model.isReviewNeeded
                ? "当前类目映射需要在最终提交前再确认一次。"
                : "当前 SHEIN 类目已确认，后续只需要继续处理普通属性或提交检查。"}
          </p>
        </div>

        <dl className="grid gap-3">
          <SuggestionRow
            label="当前类目"
            value={formatCategoryLabel(
              model.currentCategory?.category_path,
              model.currentCategoryId,
            )}
          />
          <SuggestionRow label="复核原因" value={model.categoryReviewReason} />
        </dl>

        <SuggestedCategoryBlock
          suggestion={model.suggestedCategory}
          isApplied={model.isSuggestionApplied}
        />

        {model.isReviewNeeded &&
        model.suggestedCategory?.category_id &&
        !model.isSuggestionApplied &&
        onApplySuggestedCategory ? (
          <div className="flex flex-wrap justify-end gap-3">
            {model.currentCategory?.category_id && onConfirmCurrentCategory ? (
              <Button
                tone="ghost"
                disabled={isApplying}
                onClick={onConfirmCurrentCategory}
              >
                {isApplying ? "应用中..." : "确认当前类目"}
              </Button>
            ) : null}
            <Button
              tone="secondary"
              disabled={isApplying}
              onClick={onApplySuggestedCategory}
            >
              {isApplying ? "应用中..." : "应用建议类目"}
            </Button>
          </div>
        ) : null}

        {model.isReviewNeeded &&
        !model.suggestedCategory?.category_id &&
        model.currentCategory?.category_id &&
        onConfirmCurrentCategory ? (
          <div className="flex justify-end">
            <Button
              tone="secondary"
              disabled={isApplying}
              onClick={onConfirmCurrentCategory}
            >
              {isApplying ? "应用中..." : "确认当前类目"}
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
              {manualSearchLoading ? "搜索中..." : "搜索类目"}
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
