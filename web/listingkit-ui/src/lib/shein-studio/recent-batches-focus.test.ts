import { afterEach, describe, expect, it, vi } from "vitest";

import {
  dispatchSheinStudioRecentBatchesFocus,
  dispatchSheinStudioRecentBatchesRecommendation,
  SHEIN_STUDIO_RECENT_BATCHES_FOCUS_EVENT,
  SHEIN_STUDIO_RECENT_BATCHES_RECOMMENDATION_EVENT,
} from "@/lib/shein-studio/recent-batches-focus";

describe("recent batches focus helpers", () => {
  afterEach(() => {
    vi.unstubAllGlobals();
    vi.restoreAllMocks();
  });

  it("dispatches the recent batches focus event", () => {
    const dispatchSpy = vi.spyOn(window, "dispatchEvent");

    dispatchSheinStudioRecentBatchesFocus({ preferRisk: true });

    expect(dispatchSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        type: SHEIN_STUDIO_RECENT_BATCHES_FOCUS_EVENT,
        detail: { preferRisk: true },
      }),
    );
  });

  it("dispatches the recent batches recommendation event", () => {
    const dispatchSpy = vi.spyOn(window, "dispatchEvent");

    dispatchSheinStudioRecentBatchesRecommendation({
      hasRecoverableBatches: true,
      recommendedRiskLabel: "Baseline 未就绪",
    });

    expect(dispatchSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        type: SHEIN_STUDIO_RECENT_BATCHES_RECOMMENDATION_EVENT,
        detail: {
          hasRecoverableBatches: true,
          recommendedRiskLabel: "Baseline 未就绪",
        },
      }),
    );
  });

  it("quietly skips recent batches events when window is unavailable", () => {
    const originalWindow = window;
    vi.stubGlobal("window", undefined);

    expect(() =>
      dispatchSheinStudioRecentBatchesFocus({ preferRisk: true }),
    ).not.toThrow();
    expect(() =>
      dispatchSheinStudioRecentBatchesRecommendation({
        hasRecoverableBatches: false,
      }),
    ).not.toThrow();

    vi.stubGlobal("window", originalWindow);
  });
});
