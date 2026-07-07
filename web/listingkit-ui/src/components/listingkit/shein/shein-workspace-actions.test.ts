import { describe, expect, it } from "vitest";

import {
  buildSheinGeneralReviewHref,
  buildSheinBlockingActionSummary,
  canSelectSheinReadinessItem,
  isSheinAdvancedRepairKey,
  normalizeSheinWorkspaceActionKey,
  projectSheinReadinessActions,
  sheinWorkspaceTargetIdForKey,
} from "@/components/listingkit/shein/shein-workspace-actions";

describe("shein workspace actions", () => {
  it("normalizes backend blocker aliases to actionable workspace keys", () => {
    expect(normalizeSheinWorkspaceActionKey("preview_product")).toBe("images");
    expect(normalizeSheinWorkspaceActionKey("final_images")).toBe("images");
    expect(normalizeSheinWorkspaceActionKey("pod_platform")).toBe("pod_platform");
    expect(normalizeSheinWorkspaceActionKey("shein_online_auth")).toBe("store_login");
    expect(normalizeSheinWorkspaceActionKey("shein_category_template_freshness")).toBe("category");
    expect(normalizeSheinWorkspaceActionKey("shein_attribute_template_freshness")).toBe("attributes");
    expect(normalizeSheinWorkspaceActionKey("shein_sale_attribute_freshness")).toBe("sale_attributes");
    expect(normalizeSheinWorkspaceActionKey("variant_mapping")).toBe("sale_attributes");
    expect(normalizeSheinWorkspaceActionKey("required_attribute")).toBe("attributes");
    expect(normalizeSheinWorkspaceActionKey("price")).toBe("pricing");
    expect(normalizeSheinWorkspaceActionKey("inventory")).toBe("pricing");
  });

  it("only exposes repair actions for supported readiness items", () => {
    expect(canSelectSheinReadinessItem({ key: "preview_product" })).toBe(true);
    expect(canSelectSheinReadinessItem({ key: "pod_platform" })).toBe(true);
    expect(canSelectSheinReadinessItem({ key: "shein_online_auth" })).toBe(true);
    expect(canSelectSheinReadinessItem({ key: "shein_category_template_freshness" })).toBe(true);
    expect(canSelectSheinReadinessItem({ key: "price" })).toBe(true);
    expect(canSelectSheinReadinessItem({ key: "manual_notes" })).toBe(false);
  });

  it("uses readiness taxonomy repair targets before legacy blocker aliases", () => {
    expect(
      canSelectSheinReadinessItem({
        key: "remote_gate",
        taxonomy: {
          blocker_key: "image_upload_failed",
          domain: "image",
          repair_target: "image_review",
          repair_route: "workspace.images",
          recoverable: true,
        },
      }),
    ).toBe(true);
    expect(
      normalizeSheinWorkspaceActionKey("remote_gate", "image_review"),
    ).toBe("images");
    expect(
      normalizeSheinWorkspaceActionKey("remote_gate", "sku_review"),
    ).toBe("variants");
  });

  it("projects backend taxonomy into workspace readiness state", () => {
    const projection = projectSheinReadinessActions([
      {
        key: "remote_gate",
        taxonomy: {
          blocker_key: "remote_category_gate",
          domain: "remote",
          repair_target: "category_review",
          repair_route: "workspace.category",
          recoverable: true,
        },
      },
      {
        key: "image_policy_gate",
        taxonomy: {
          blocker_key: "image_policy",
          domain: "image",
          repair_target: "image_review",
          repair_route: "workspace.images",
          recoverable: true,
        },
      },
    ]);

    expect(projection.categoryBlocked).toBe(true);
    expect(projection.previewBlocked).toBe(true);
    expect(projection.blockingActionSummary).toMatchObject({
      title: "确认类目",
      action_key: "category",
    });
  });

  it("projects suggested actions without adding workspace hook rules", () => {
    const projection = projectSheinReadinessActions([
      {
        key: "remote_auth_gate",
        suggested_action: "重新登录 SHEIN 店铺",
      },
      {
        key: "remote_sale_gate",
        suggested_action: "刷新销售属性模板",
      },
    ]);

    expect(projection.cookieBlocked).toBe(true);
    expect(projection.saleAttributeBlocked).toBe(true);
    expect(projection.blockingActionSummary).toMatchObject({
      title: "重新登录店铺",
      action_key: "shein_online_auth",
    });
  });

  it("projects legacy blocker keys as compatibility fallback", () => {
    const projection = projectSheinReadinessActions([
      { key: "preview_product" },
      { key: "attribute_review" },
      { key: "shein_sale_attribute_template_freshness" },
    ]);

    expect(projection.previewBlocked).toBe(true);
    expect(projection.attributeBlocked).toBe(true);
    expect(projection.saleAttributeBlocked).toBe(true);
    expect(projection.blockingActionSummary).toMatchObject({
      title: "刷新销售属性",
      action_key: "shein_sale_attribute_freshness",
    });
  });

  it("does not treat final payload variant blockers as sale attribute review blockers", () => {
    const projection = projectSheinReadinessActions([
      {
        key: "variants",
        label: "发布载荷结构",
        message: "SHEIN publish blocked: missing required size chart attributes: 胸围 (cm)",
      },
    ]);

    expect(projection.saleAttributeBlocked).toBe(false);
    expect(projection.blockingActionSummary).toBeUndefined();
  });

  it("maps normalized keys to concrete workspace section ids", () => {
    expect(sheinWorkspaceTargetIdForKey("images")).toBe("shein-preview-images");
    expect(sheinWorkspaceTargetIdForKey("pod_platform")).toBe("shein-preview-images");
    expect(sheinWorkspaceTargetIdForKey("pricing")).toBe("shein-final-review-pricing");
    expect(sheinWorkspaceTargetIdForKey("variants")).toBe("shein-final-review-size-chart");
  });

  it("flags advanced review keys that require the editable workspace", () => {
    expect(isSheinAdvancedRepairKey("attributes")).toBe(true);
    expect(isSheinAdvancedRepairKey("sale_attributes")).toBe(true);
    expect(isSheinAdvancedRepairKey("variants")).toBe(false);
    expect(isSheinAdvancedRepairKey("images")).toBe(false);
    expect(isSheinAdvancedRepairKey("pricing")).toBe(false);
  });

  it("builds a general review deep link for advanced repair targets", () => {
    expect(
      buildSheinGeneralReviewHref(
        "task-123",
        "shein-attribute-review-card",
      ),
    ).toBe(
      "/listing-kits/task-123/workspace?platform=shein&section_key=general_review#shein-attribute-review-card",
    );
  });
});

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
