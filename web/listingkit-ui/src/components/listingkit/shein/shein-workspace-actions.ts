import type { SheinReadinessItem } from "@/lib/types/listingkit";

export type SheinWorkspaceActionKey =
  | "category"
  | "attributes"
  | "sale_attributes";

const SHEIN_REPAIR_TARGETS: Record<SheinWorkspaceActionKey, string> = {
  category: "shein-category-review-card",
  attributes: "shein-attribute-review-card",
  sale_attributes: "shein-sale-attribute-review-card",
};

export function isSheinWorkspaceActionKey(
  key?: string | null,
): key is SheinWorkspaceActionKey {
  return key === "category" || key === "attributes" || key === "sale_attributes";
}

export function canSelectSheinReadinessItem(item: SheinReadinessItem) {
  return isSheinWorkspaceActionKey(item.key);
}

export function sheinWorkspaceTargetIdForKey(key: SheinWorkspaceActionKey) {
  return SHEIN_REPAIR_TARGETS[key];
}
