import { describe, expect, it } from "vitest";

import { ApiError } from "@/lib/api/client";
import {
  formatSubscriptionApiError,
  parseSubscriptionEntitlement,
  parseSubscriptionPlanList,
  parseSubscriptionRequiredPayload,
  parseSubscriptionSummary,
  parseSubscriptionTenantOverviewList,
} from "@/lib/api/subscription";

describe("subscription API schema", () => {
  it("parses a tenant subscription summary", () => {
    const parsed = parseSubscriptionSummary({
      tenant_id: "org-286",
      subscription: {
        id: 1,
        tenant_id: "org-286",
        plan_code: "professional",
        status: "active",
      },
      current_plan: {
        plan: {
          code: "professional",
          name: "专业版",
          sort_order: 20,
          active: true,
        },
        modules: [
          {
            plan_code: "professional",
            module_code: "studio",
            limits: { design_jobs: 100 },
            sort_order: 50,
          },
        ],
      },
      modules: [
        {
          code: "studio",
          name: "Studio",
          sort_order: 50,
          active: true,
        },
      ],
      entitlements: [
        {
          module: {
            code: "studio",
            name: "Studio",
            sort_order: 50,
            active: true,
          },
          entitlement: {
            id: 1,
            tenant_id: "org-286",
            module_code: "studio",
            status: "active",
            limits: { design_jobs: 10 },
          },
          usage: [],
          allowed: true,
          limits: { design_jobs: 10 },
          used: { design_jobs: 2 },
        },
      ],
    });

    expect(parsed.tenant_id).toBe("org-286");
    expect(parsed.current_plan?.plan.name).toBe("专业版");
    expect(parsed.entitlements[0]?.allowed).toBe(true);
    expect(parsed.entitlements[0]?.limits?.design_jobs).toBe(10);
  });

  it("parses subscription plans", () => {
    const plans = parseSubscriptionPlanList({
      items: [
        {
          plan: {
            code: "professional",
            name: "专业版",
            sort_order: 20,
            active: true,
          },
          modules: [
            {
              plan_code: "professional",
              module_code: "studio",
              limits: { design_jobs: 100 },
              sort_order: 50,
            },
          ],
        },
      ],
    });

    expect(plans[0]?.plan.code).toBe("professional");
    expect(plans[0]?.modules[0]?.limits?.design_jobs).toBe(100);
  });

  it("parses tenant overviews with display names", () => {
    const items = parseSubscriptionTenantOverviewList({
      items: [
        {
          tenant_id: "org-286",
          tenant_display_name: "北美租户",
          entitlement_count: 3,
          active_count: 2,
        },
      ],
    });

    expect(items[0]?.tenant_id).toBe("org-286");
    expect(items[0]?.tenant_display_name).toBe("北美租户");
  });

  it("normalizes empty backend usage from null to an empty list", () => {
    const parsed = parseSubscriptionSummary({
      tenant_id: "org-286",
      modules: [
        {
          code: "oss_storage",
          name: "OSS 存储",
          sort_order: 60,
          active: true,
        },
      ],
      entitlements: [
        {
          module: {
            code: "oss_storage",
            name: "OSS 存储",
            sort_order: 60,
            active: true,
          },
          usage: null,
          allowed: false,
          reason: "not_configured",
          used: {},
        },
      ],
    });

    expect(parsed.entitlements[0]?.usage).toEqual([]);
  });

  it("rejects an invalid entitlement status", () => {
    expect(() =>
      parseSubscriptionEntitlement({
        id: 1,
        tenant_id: "org-286",
        module_code: "studio",
        status: "pending",
      }),
    ).toThrow(ApiError);
  });

  it("parses and formats subscription 402 payloads", () => {
    const payload = parseSubscriptionRequiredPayload({
      error: "quota_exceeded",
      module_code: "studio",
      metric: "design_jobs",
      limit: 1,
      used: 2,
    });

    expect(payload?.error).toBe("quota_exceeded");
    expect(
      formatSubscriptionApiError(
        new ApiError("failed", 402, {
          error: "subscription_required",
          module_code: "rules",
        }),
      ),
    ).toContain("rules");
  });
});
