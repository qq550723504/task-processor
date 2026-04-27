import type { SheinReadinessItem } from "@/lib/types/listingkit";

export type SheinWorkspaceActionKey =
  | "category"
  | "category_review"
  | "attributes"
  | "attribute_review"
  | "sale_attributes"
  | "variants"
  | "images";

const SHEIN_REPAIR_TARGETS: Record<SheinWorkspaceActionKey, string> = {
  category: "shein-category-review-card",
  category_review: "shein-category-review-card",
  attributes: "shein-attribute-review-card",
  attribute_review: "shein-attribute-review-card",
  sale_attributes: "shein-sale-attribute-review-card",
  variants: "shein-sale-attribute-review-card",
  images: "shein-preview-images",
};

export function isSheinWorkspaceActionKey(
  key?: string | null,
): key is SheinWorkspaceActionKey {
  return (
    key === "category" ||
    key === "category_review" ||
    key === "attributes" ||
    key === "attribute_review" ||
    key === "sale_attributes" ||
    key === "variants" ||
    key === "images"
  );
}

export function canSelectSheinReadinessItem(item: SheinReadinessItem) {
  return isSheinWorkspaceActionKey(item.key);
}

export function sheinWorkspaceTargetIdForKey(key: SheinWorkspaceActionKey) {
  return SHEIN_REPAIR_TARGETS[key];
}
