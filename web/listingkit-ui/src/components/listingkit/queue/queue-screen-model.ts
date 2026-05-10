import type { QueueFilterValue } from "@/components/listingkit/queue/queue-filters-bar";

export const defaultQueuePageSize = 20;

export function parsePositiveInt(value: string | null, fallback: number) {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : fallback;
}

export function initialQueueFilters(
  searchParams: URLSearchParams,
): QueueFilterValue {
  return {
    platform: searchParams.get("platform") ?? "",
    slot: searchParams.get("slot") ?? "",
    quality_grade: searchParams.get("quality_grade") ?? "",
    preview_capability: searchParams.get("preview_capability") ?? "",
    review_status: searchParams.get("review_status") ?? "",
    render_preview_available:
      searchParams.get("render_preview_available") === "true",
  };
}
