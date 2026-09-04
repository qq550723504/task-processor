import { describe, expect, it } from "vitest";

import { subscriptionMetricDisplayName } from "@/components/listingkit/subscription/subscription-display";

describe("subscriptionMetricDisplayName", () => {
  it("labels the ListingKit generation metric used by backend reservations", () => {
    expect(subscriptionMetricDisplayName("listingkit_generations_succeeded")).toBe(
      "ListingKit 生成额度",
    );
  });
});
