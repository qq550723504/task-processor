import { describe, expect, it } from "vitest";

import { buildSheinCategoryReviewModel } from "@/components/listingkit/shein/shein-category-review-card-model";

describe("buildSheinCategoryReviewModel", () => {
  it("builds a review model when category resolution is partial without a category id", () => {
    const model = buildSheinCategoryReviewModel({
      category: {
        current: {
          status: "partial",
          source: "fallback",
          review_notes: ["SHEIN 类目在线解析失败"],
        },
      },
    });

    expect(model).not.toBeNull();
    expect(model?.isReviewNeeded).toBe(true);
    expect(model?.categoryReviewReason).toBe("SHEIN 类目在线解析失败");
  });
});
