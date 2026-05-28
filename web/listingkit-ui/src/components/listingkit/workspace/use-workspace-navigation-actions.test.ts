import { describe, expect, it, vi } from "vitest";

import { runSheinFreshnessAction } from "@/components/listingkit/workspace/use-workspace-navigation-actions";

describe("runSheinFreshnessAction", () => {
  it("runs the matching freshness handler when available", () => {
    const handleRefreshCategory = vi.fn();

    const handled = runSheinFreshnessAction(
      "shein_category_template_freshness",
      {
        shein_category_template_freshness: handleRefreshCategory,
      },
    );

    expect(handled).toBe(true);
    expect(handleRefreshCategory).toHaveBeenCalledTimes(1);
  });

  it("falls back when the matching freshness handler is missing", () => {
    const handled = runSheinFreshnessAction(
      "shein_attribute_template_freshness",
      {},
    );

    expect(handled).toBe(false);
  });

  it("does not treat online auth as a local refresh action", () => {
    const handled = runSheinFreshnessAction("shein_online_auth", {
      shein_category_template_freshness: vi.fn(),
    });

    expect(handled).toBe(false);
  });
});
