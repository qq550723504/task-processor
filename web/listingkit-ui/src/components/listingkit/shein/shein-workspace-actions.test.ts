import { describe, expect, it } from "vitest";

import {
  buildSheinGeneralReviewHref,
  canSelectSheinReadinessItem,
  isSheinAdvancedRepairKey,
  normalizeSheinWorkspaceActionKey,
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

  it("maps normalized keys to concrete workspace section ids", () => {
    expect(sheinWorkspaceTargetIdForKey("images")).toBe("shein-preview-images");
    expect(sheinWorkspaceTargetIdForKey("pod_platform")).toBe("shein-preview-images");
    expect(sheinWorkspaceTargetIdForKey("pricing")).toBe("shein-final-review-pricing");
  });

  it("flags advanced review keys that require the editable workspace", () => {
    expect(isSheinAdvancedRepairKey("attributes")).toBe(true);
    expect(isSheinAdvancedRepairKey("sale_attributes")).toBe(true);
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
