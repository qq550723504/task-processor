import { ApiError } from "@/lib/api/client";
import type { SheinPreviewPayload, SheinReadinessItem } from "@/lib/types/listingkit";

function summarizeBlockingItems(items: SheinReadinessItem[]) {
  const useful = items
    .map((item) => item.message?.trim() || item.label?.trim() || item.key?.trim())
    .filter((value): value is string => Boolean(value));
  if (useful.length === 0) {
    return null;
  }
  return useful.slice(0, 2).join("；");
}

export function formatSheinSubmitError(
  error: unknown,
  shein?: SheinPreviewPayload | null,
) {
  if (!error) {
    return null;
  }

  const readinessBlockers = shein?.submit_readiness?.blocking_items ?? [];
  const finalReviewBlockers = shein?.final_review?.blocking_items ?? [];
  const blockerSummary = summarizeBlockingItems([
    ...readinessBlockers,
    ...finalReviewBlockers,
  ]);

  if (error instanceof ApiError) {
    const payload = error.payload;
    const payloadMessage =
      payload && typeof payload === "object" && "message" in payload
        ? (payload as { message?: unknown }).message
        : undefined;

    if (
      typeof payloadMessage === "string" &&
      payloadMessage.includes("submit blocked by readiness")
    ) {
      return blockerSummary
        ? `提交前检查未通过：${blockerSummary}`
        : "提交前检查未通过，请先处理阻断项后再提交。";
    }

    if (typeof payloadMessage === "string" && payloadMessage.trim()) {
      return payloadMessage;
    }

    return error.message;
  }

  if (error instanceof Error) {
    return error.message;
  }

  return String(error);
}
