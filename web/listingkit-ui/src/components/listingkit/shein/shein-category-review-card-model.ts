import type { SheinEditorContext } from "@/lib/types/listingkit";

export function joinPath(path?: string[] | null): string {
  return path?.filter(Boolean).join(" > ") ?? "";
}

export function formatCategoryLabel(
  path?: string[] | null,
  categoryId?: number | null,
): string {
  const pathLabel = joinPath(path);
  if (pathLabel && categoryId) {
    return `${pathLabel} (${categoryId})`;
  }
  if (pathLabel) {
    return pathLabel;
  }
  if (categoryId) {
    return String(categoryId);
  }
  return "";
}

export function buildSheinCategoryReviewModel(
  editorContext?: SheinEditorContext | null,
) {
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
    currentCategory?.review_notes?.[0] ??
    currentCategory?.status ??
    "";
  const suggestedCategory = currentCategory?.suggested_category;
  const isSuggestionApplied =
    Boolean(suggestedCategory?.category_id) &&
    suggestedCategory?.category_id === currentCategory?.category_id;

  if (
    !currentCategory?.category_id &&
    !currentCategory?.category_path?.length &&
    !currentCategory?.status &&
    !(currentCategory?.review_notes?.length ?? 0) &&
    !recommendCategoryReview &&
    !categoryReviewReason &&
    !suggestedCategory?.category_id
  ) {
    return null;
  }

  return {
    recommendCategoryReview,
    categoryReviewReason,
    currentPath: joinPath(currentCategory?.category_path),
    currentCategoryId: currentCategory?.category_id,
    currentCategory: currentCategory ?? null,
    suggestedCategory,
    isSuggestionApplied,
    isReviewNeeded: Boolean(
      recommendCategoryReview ||
        categoryReviewReason ||
        (suggestedCategory?.category_id && !isSuggestionApplied),
    ),
  };
}
