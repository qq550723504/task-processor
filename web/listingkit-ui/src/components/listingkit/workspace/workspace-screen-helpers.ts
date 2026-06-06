import { ApiError } from "@/lib/api/client";
import type { QueueQuery, SheinEditorContext } from "@/lib/types/listingkit";

export function queryFromSearchParams(searchParams: URLSearchParams): QueueQuery {
  return {
    platform: searchParams.get("platform") ?? undefined,
    slot: searchParams.get("slot") ?? undefined,
    preview_capability: searchParams.get("preview_capability") ?? undefined,
    response_mode: searchParams.get("response_mode") ?? undefined,
  };
}

export function submitErrorMessage(error: unknown) {
  if (!error) {
    return null;
  }
  if (error instanceof ApiError) {
    const payload = error.payload;
    if (payload && typeof payload === "object" && "message" in payload) {
      const message = (payload as { message?: unknown }).message;
      if (typeof message === "string" && message.trim()) {
        return message;
      }
    }
    return error.message;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return String(error);
}

export function formatWorkspaceDate(value?: string) {
  if (!value) {
    return undefined;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function workspaceTaskStatusLabel(status?: string) {
  switch (status) {
    case "pending":
      return "待处理";
    case "processing":
      return "处理中";
    case "completed":
      return "已完成";
    case "needs_review":
      return "待审核";
    case "failed":
      return "失败";
    default:
      return status ?? "未知";
  }
}

export function hasSheinCategoryReviewSignal(
  editorContext?: SheinEditorContext | null,
) {
  const currentCategory = editorContext?.category?.current;
  const currentSale = editorContext?.sale_attributes?.current;
  const revisionSale =
    editorContext?.revision_skeleton?.shein?.sale_attribute_resolution;

  return Boolean(
    currentCategory?.category_id ||
      currentCategory?.category_path?.length ||
      currentCategory?.status ||
      currentCategory?.review_notes?.length ||
      currentCategory?.suggested_category?.category_id ||
      currentSale?.recommend_category_review ||
      revisionSale?.recommend_category_review,
  );
}

export function hasSheinAttributeReviewSignal(
  editorContext?: SheinEditorContext | null,
) {
  const current = editorContext?.attributes?.current;
  return Boolean(
    current?.status ||
      current?.review_notes?.length ||
      current?.resolved_attributes?.length ||
      current?.pending_attribute_candidates?.length ||
      current?.recommended_attribute_candidates?.length,
  );
}

export function hasSheinSaleAttributeReviewSignal(
  editorContext?: SheinEditorContext | null,
) {
  const current = editorContext?.sale_attributes?.current;
  return Boolean(
    current?.status ||
      current?.review_notes?.length ||
      current?.skc_attributes?.length ||
      current?.sku_attributes?.length ||
      current?.candidates?.length,
  );
}

export function selectedPlatformFromReviewTarget(
  target?: { platform?: string; panel_state?: { selected_platform?: string } } | null,
) {
  return target?.platform ?? target?.panel_state?.selected_platform;
}

export function openSheinAdvancedDetailsForTarget(targetId: string) {
  if (
    targetId !== "shein-category-review-card" &&
    targetId !== "shein-attribute-review-card" &&
    targetId !== "shein-sale-attribute-review-card" &&
    targetId !== "shein-final-review-pricing"
  ) {
    return;
  }
  const detailsId =
    targetId === "shein-final-review-pricing"
      ? "shein-general-final-review-details"
      : "shein-advanced-review-details";
  const details = document.getElementById(detailsId);
  if (details instanceof HTMLDetailsElement) {
    details.open = true;
  }
}

export function scrollSheinWorkspaceTarget(
  normalizedKey: string,
  targetId: string,
) {
  openSheinAdvancedDetailsForTarget(targetId);
  const card =
    normalizedKey === "attributes" || normalizedKey === "attribute_review"
      ? document.getElementById("shein-attribute-required-group") ??
        document.getElementById(targetId)
      : normalizedKey === "sale_attributes" || normalizedKey === "variants"
        ? document.getElementById("shein-sale-attribute-unresolved-group") ??
          document.getElementById(targetId)
        : document.getElementById(targetId);
  card?.scrollIntoView({ behavior: "smooth", block: "start" });
}
