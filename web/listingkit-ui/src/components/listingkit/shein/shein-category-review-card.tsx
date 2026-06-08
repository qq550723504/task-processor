"use client";

import { useState } from "react";

import { Alert, AlertDescription } from "@/components/ui/alert";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Input } from "@/components/ui/input";
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
  applyErrorMessage,
  statusMessage,
  statusTone = "default",
  taskId,
  editorContext,
  isApplying = false,
  onApplySuggestedCategory,
  onConfirmCurrentCategory,
  onApplyManualCategory,
  onRefreshCategory,
}: {
  applyErrorMessage?: string | null;
  statusMessage?: string | null;
  statusTone?: "default" | "success";
  taskId: string;
  editorContext?: SheinEditorContext | null;
  isApplying?: boolean;
  onApplySuggestedCategory?: (() => void) | null;
  onConfirmCurrentCategory?: (() => void) | null;
  onApplyManualCategory?: ((candidate: SheinManualCategoryCandidate) => Promise<void> | void) | null;
  onRefreshCategory?: (() => void) | null;
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
        {statusMessage ? (
          <Alert variant={statusTone === "success" ? "success" : "default"}>
            <AlertDescription>{statusMessage}</AlertDescription>
          </Alert>
        ) : null}
        {applyErrorMessage ? (
          <div className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-3 text-sm leading-6 text-rose-700">
            保存类目修改失败：{applyErrorMessage}
          </div>
        ) : null}
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

        {/* AI重选分类按钮 */}
        {onRefreshCategory && model.isReviewNeeded ? (
          <div className="mt-3">
            <Button
              className="h-9 w-full sm:w-auto"
              disabled={isApplying}
              onClick={() => onRefreshCategory()}
              variant="secondary"
            >
              {isApplying ? "AI重选中..." : "AI重选分类"}
            </Button>
          </div>
        ) : null}

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
                variant="ghost"
                disabled={isApplying}
                onClick={onConfirmCurrentCategory}
              >
                {isApplying ? "应用中..." : "确认当前类目"}
              </Button>
            ) : null}
            <Button
              variant="secondary"
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
              variant="secondary"
              disabled={isApplying}
              onClick={onConfirmCurrentCategory}
            >
              {isApplying ? "应用中..." : "确认当前类目"}
            </Button>
          </div>
        ) : null}

        {/* 类目解析失败时的提示 */}
        {model.isReviewNeeded &&
        !model.suggestedCategory?.category_id &&
        !model.currentCategory?.category_id &&
        (model.currentCategory?.review_notes?.length ?? 0) > 0 ? (
          <Alert variant="destructive">
            <AlertDescription>
              <p className="font-semibold">类目解析失败</p>
              <div className="mt-2 space-y-1 text-xs">
                {model.currentCategory!.review_notes!.map((note, index) => (
                  <p key={index} className="text-rose-600">
                    • {note}
                  </p>
                ))}
              </div>
              <p className="mt-3 text-xs font-medium text-zinc-700">
                👉 请使用下方的“手工选类目”功能手动搜索并选择正确的类目。
              </p>
            </AlertDescription>
          </Alert>
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
            <Input
              className="h-10 flex-1 rounded-xl"
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
              variant="secondary"
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
