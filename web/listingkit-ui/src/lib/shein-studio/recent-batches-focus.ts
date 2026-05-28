export const SHEIN_STUDIO_RECENT_BATCHES_FOCUS_EVENT =
  "listingkit:shein-studio:focus-recent-batches";
export const SHEIN_STUDIO_RECENT_BATCHES_RECOMMENDATION_EVENT =
  "listingkit:shein-studio:recent-batches-recommendation";

export type SheinStudioRecentBatchesFocusDetail = {
  preferRisk?: boolean;
};

export type SheinStudioRecentBatchesRecommendationDetail = {
  hasRecoverableBatches?: boolean;
  recommendedRiskLabel?: string;
  recommendedRiskReasonCode?: string;
};

export function dispatchSheinStudioRecentBatchesFocus(
  detail: SheinStudioRecentBatchesFocusDetail,
) {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(
    new CustomEvent<SheinStudioRecentBatchesFocusDetail>(
      SHEIN_STUDIO_RECENT_BATCHES_FOCUS_EVENT,
      { detail },
    ),
  );
}

export function dispatchSheinStudioRecentBatchesRecommendation(
  detail: SheinStudioRecentBatchesRecommendationDetail,
) {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(
    new CustomEvent<SheinStudioRecentBatchesRecommendationDetail>(
      SHEIN_STUDIO_RECENT_BATCHES_RECOMMENDATION_EVENT,
      { detail },
    ),
  );
}
