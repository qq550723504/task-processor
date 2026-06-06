import { describe, expect, it } from "vitest";

import { hasSheinCategoryReviewSignal } from "@/components/listingkit/workspace/workspace-screen-helpers";

describe("hasSheinCategoryReviewSignal", () => {
  it("returns true when category review only has partial status and review notes", () => {
    expect(
      hasSheinCategoryReviewSignal({
        category: {
          current: {
            status: "partial",
            source: "fallback",
            review_notes: ["missing real category id"],
          },
        },
      }),
    ).toBe(true);
  });
});
