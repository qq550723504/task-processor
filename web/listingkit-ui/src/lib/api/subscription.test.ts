import { describe, expect, it } from "vitest";

import { ApiError } from "@/lib/api/client";
import {
  formatSubscriptionApiError,
  parseSubscriptionEntitlement,
  parseSubscriptionRequiredPayload,
  parseSubscriptionSummary,
} from "@/lib/api/subscription";

describe("subscription API schema", () => {
  it("parses a tenant subscription summary", () => {
    const parsed = parseSubscriptionSummary({
      tenant_id: "org-286",
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
    expect(parsed.entitlements[0]?.allowed).toBe(true);
    expect(parsed.entitlements[0]?.limits?.design_jobs).toBe(10);
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
