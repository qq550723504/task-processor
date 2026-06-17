import type { SheinReadinessItem } from "@/lib/types/listingkit";

export type SheinWorkspaceActionKey =
  | "store_login"
  | "category"
  | "category_review"
  | "attributes"
  | "attribute_review"
  | "sale_attributes"
  | "variants"
  | "images"
  | "pod_platform"
  | "pricing";

export type SheinFreshnessActionKey =
  | "shein_online_auth"
  | "shein_category_template_freshness"
  | "shein_attribute_template_freshness"
  | "shein_sale_attribute_template_freshness"
  | "shein_sale_attribute_freshness";

const SHEIN_REPAIR_TARGETS: Record<SheinWorkspaceActionKey, string> = {
  store_login: "shein-store-login",
  category: "shein-category-review-card",
  category_review: "shein-category-review-card",
  attributes: "shein-attribute-review-card",
  attribute_review: "shein-attribute-review-card",
  sale_attributes: "shein-sale-attribute-review-card",
  variants: "shein-sale-attribute-review-card",
  images: "shein-preview-images",
  pod_platform: "shein-preview-images",
  pricing: "shein-final-review-pricing",
};

export function normalizeSheinFreshnessActionKey(
  key?: string | null,
): SheinFreshnessActionKey | false {
  const normalized = (key ?? "").toLowerCase();
  switch (normalized) {
    case "shein_online_auth":
    case "shein_category_template_freshness":
    case "shein_attribute_template_freshness":
    case "shein_sale_attribute_template_freshness":
    case "shein_sale_attribute_freshness":
      return normalized;
    default:
      return false;
  }
}

export function normalizeSheinWorkspaceActionKey(
  key?: string | null,
  repairTarget?: string | null,
): SheinWorkspaceActionKey | false {
  const normalizedRepairTarget = (repairTarget ?? "").toLowerCase();
  const normalized =
    normalizedRepairTarget ||
    (key ?? "").toLowerCase();
  if (!normalized) {
    return false;
  }
  if (
    normalized === "store_login" ||
    normalized === "shein_online_auth" ||
    normalized.includes("cookie") ||
    normalized.includes("login")
  ) {
    return "store_login";
  }
  if (normalized === "shein_category_template_freshness") {
    return "category";
  }
  if (normalized === "shein_attribute_template_freshness") {
    return "attributes";
  }
  if (
    normalized === "shein_sale_attribute_template_freshness" ||
    normalized === "shein_sale_attribute_freshness"
  ) {
    return "sale_attributes";
  }
  if (normalized === "category" || normalized === "category_review") {
    return normalized;
  }
  if (normalized === "attributes" || normalized === "attribute_review") {
    return normalized;
  }
  if (normalized === "sale_attributes" || normalized === "variants") {
    return normalized;
  }
  if (
    normalized === "pod_platform" ||
    normalized.includes("pod")
  ) {
    return "pod_platform";
  }
  if (normalized.includes("sku")) {
    return "variants";
  }
  if (
    normalized === "images" ||
    normalized.includes("image") ||
    normalized.includes("preview_product")
  ) {
    return "images";
  }
  if (
    normalized.includes("sale_attribute") ||
    normalized.includes("variant")
  ) {
    return "sale_attributes";
  }
  if (normalized.includes("attribute")) {
    return "attributes";
  }
  if (normalized.includes("category")) {
    return "category";
  }
  if (
    normalized.includes("price") ||
    normalized.includes("stock") ||
    normalized.includes("inventory") ||
    normalized.includes("quantity")
  ) {
    return "pricing";
  }
  return false;
}

export function isSheinWorkspaceActionKey(
  key?: string | null,
): key is SheinWorkspaceActionKey {
  return normalizeSheinWorkspaceActionKey(key) !== false;
}

export function canSelectSheinReadinessItem(item: SheinReadinessItem) {
  return normalizeSheinWorkspaceActionKey(
    item.key,
    item.taxonomy?.repair_target,
  ) !== false;
}

export function sheinWorkspaceTargetIdForKey(key: SheinWorkspaceActionKey) {
  return SHEIN_REPAIR_TARGETS[key];
}

export function isSheinAdvancedRepairKey(key: SheinWorkspaceActionKey) {
  return (
    key === "category" ||
    key === "category_review" ||
    key === "attributes" ||
    key === "attribute_review" ||
    key === "sale_attributes" ||
    key === "variants"
  );
}

export function buildSheinGeneralReviewHref(
  taskId: string,
  targetId: string,
) {
  return `/listing-kits/${taskId}/workspace?platform=shein&section_key=general_review#${targetId}`;
}
