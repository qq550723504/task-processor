import { describe, expect, it } from "vitest";

import {
  canSelectSheinReadinessItem,
  normalizeSheinWorkspaceActionKey,
  sheinWorkspaceTargetIdForKey,
} from "@/components/listingkit/shein/shein-workspace-actions";

describe("shein workspace actions", () => {
  it("normalizes backend blocker aliases to actionable workspace keys", () => {
    expect(normalizeSheinWorkspaceActionKey("preview_product")).toBe("images");
    expect(normalizeSheinWorkspaceActionKey("final_images")).toBe("images");
    expect(normalizeSheinWorkspaceActionKey("variant_mapping")).toBe("sale_attributes");
    expect(normalizeSheinWorkspaceActionKey("required_attribute")).toBe("attributes");
    expect(normalizeSheinWorkspaceActionKey("price")).toBe("pricing");
    expect(normalizeSheinWorkspaceActionKey("inventory")).toBe("pricing");
  });

  it("only exposes repair actions for supported readiness items", () => {
    expect(canSelectSheinReadinessItem({ key: "preview_product" })).toBe(true);
    expect(canSelectSheinReadinessItem({ key: "price" })).toBe(true);
    expect(canSelectSheinReadinessItem({ key: "manual_notes" })).toBe(false);
  });

  it("maps normalized keys to concrete workspace section ids", () => {
    expect(sheinWorkspaceTargetIdForKey("images")).toBe("shein-preview-images");
    expect(sheinWorkspaceTargetIdForKey("pricing")).toBe("shein-final-review-pricing");
  });
});
