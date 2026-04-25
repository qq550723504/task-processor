import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import type {
  SheinCategorySuggestion,
  SheinEditorContext,
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

  if (!recommendCategoryReview && !suggestedCategory?.category_id) {
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

export function SheinCategoryReviewCard({
  editorContext,
  isApplying = false,
  onApplySuggestedCategory,
}: {
  editorContext?: SheinEditorContext | null;
  isApplying?: boolean;
  onApplySuggestedCategory?: (() => void) | null;
}) {
  const model = buildSheinCategoryReviewModel(editorContext);
  if (!model) {
    return null;
  }

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
          <SuggestionRow
            label="Review reason"
            value={model.categoryReviewReason}
          />
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
      </div>
    </Card>
  );
}
