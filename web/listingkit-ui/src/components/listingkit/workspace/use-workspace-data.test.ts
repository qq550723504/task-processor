import { describe, expect, it } from "vitest";

import { buildSheinBlockingActionSummary } from "@/components/listingkit/workspace/use-workspace-data";

describe("buildSheinBlockingActionSummary", () => {
  it("prefers freshness-specific repair actions when templates drift", () => {
    const summary = buildSheinBlockingActionSummary({
      cookieBlocked: false,
      categoryBlocked: true,
      attributeBlocked: true,
      saleAttributeBlocked: true,
      categoryFreshnessBlocked: true,
    });

    expect(summary).toMatchObject({
      title: "刷新类目模板",
      action_key: "shein_category_template_freshness",
    });
  });

  it("keeps online auth as the highest-priority blocking action", () => {
    const summary = buildSheinBlockingActionSummary({
      cookieBlocked: false,
      categoryBlocked: true,
      attributeBlocked: true,
      saleAttributeBlocked: true,
      authFreshnessBlocked: true,
      categoryFreshnessBlocked: true,
      attributeFreshnessBlocked: true,
      saleAttributeFreshnessBlocked: true,
    });

    expect(summary).toMatchObject({
      title: "重新登录店铺",
      action_key: "shein_online_auth",
    });
  });

  it("falls back to legacy review actions when there is no freshness blocker", () => {
    const summary = buildSheinBlockingActionSummary({
      cookieBlocked: false,
      categoryBlocked: false,
      attributeBlocked: true,
      saleAttributeBlocked: true,
    });

    expect(summary).toMatchObject({
      title: "确认普通属性",
      action_key: "attributes",
    });
  });
});
