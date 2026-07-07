import type { SheinReadinessItem } from "@/lib/types/listingkit";
import type { ResolvedActionSummary } from "@/lib/types/listingkit";

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

export type SheinReadinessProjection = {
  cookieBlocked: boolean;
  categoryBlocked: boolean;
  attributeBlocked: boolean;
  saleAttributeBlocked: boolean;
  previewBlocked: boolean;
  blockingActionSummary?: ResolvedActionSummary;
};

const SHEIN_REPAIR_TARGETS: Record<SheinWorkspaceActionKey, string> = {
  store_login: "shein-store-login",
  category: "shein-category-review-card",
  category_review: "shein-category-review-card",
  attributes: "shein-attribute-review-card",
  attribute_review: "shein-attribute-review-card",
  sale_attributes: "shein-sale-attribute-review-card",
  variants: "shein-final-review-size-chart",
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
      if (
        normalized.includes("online_auth") ||
        normalized.includes("重新登录")
      ) {
        return "shein_online_auth";
      }
      if (
        normalized.includes("category_template_freshness") ||
        normalized.includes("刷新类目")
      ) {
        return "shein_category_template_freshness";
      }
      if (
        normalized.includes("sale_attribute") ||
        normalized.includes("刷新销售属性")
      ) {
        return "shein_sale_attribute_freshness";
      }
      if (
        normalized.includes("attribute_template_freshness") ||
        normalized.includes("刷新普通属性")
      ) {
        return "shein_attribute_template_freshness";
      }
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
    normalized === "store" ||
    normalized === "shein_online_auth" ||
    normalized.includes("cookie") ||
    normalized.includes("login") ||
    normalized.includes("登录")
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
  if (
    normalized === "category" ||
    normalized === "category_review"
  ) {
    return normalized;
  }
  if (normalized.includes("类目")) {
    return "category";
  }
  if (
    normalized === "attributes" ||
    normalized === "attribute" ||
    normalized === "attribute_review"
  ) {
    return normalized === "attribute" ? "attributes" : normalized;
  }
  if (normalized.includes("普通属性")) {
    return "attributes";
  }
  if (
    normalized === "sale_attributes" ||
    normalized === "sale_attribute" ||
    normalized === "variants"
  ) {
    return normalized === "sale_attribute" ? "sale_attributes" : normalized;
  }
  if (normalized.includes("销售属性")) {
    return "sale_attributes";
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
    normalized === "image" ||
    normalized.includes("image") ||
    normalized.includes("图片") ||
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
    normalized.includes("价格") ||
    normalized.includes("stock") ||
    normalized.includes("inventory") ||
    normalized.includes("quantity")
  ) {
    return "pricing";
  }
  return false;
}

function normalizeSheinReadinessItemFreshnessActionKey(
  item: SheinReadinessItem,
): SheinFreshnessActionKey | false {
  return (
    normalizeSheinFreshnessActionKey(item.taxonomy?.repair_target) ||
    normalizeSheinFreshnessActionKey(item.suggested_action) ||
    normalizeSheinFreshnessActionKey(item.taxonomy?.blocker_key) ||
    normalizeSheinFreshnessActionKey(item.key)
  );
}

function normalizeSheinReadinessItemWorkspaceActionKey(
  item: SheinReadinessItem,
): SheinWorkspaceActionKey | false {
  return (
    normalizeSheinWorkspaceActionKey(
      item.key,
      item.taxonomy?.repair_target,
    ) ||
    normalizeSheinWorkspaceActionKey(item.suggested_action) ||
    normalizeSheinWorkspaceActionKey(item.taxonomy?.blocker_key) ||
    normalizeSheinWorkspaceActionKey(item.taxonomy?.domain)
  );
}

export function buildSheinBlockingActionSummary({
  cookieBlocked,
  categoryBlocked,
  attributeBlocked,
  saleAttributeBlocked,
  authFreshnessBlocked = false,
  categoryFreshnessBlocked = false,
  attributeFreshnessBlocked = false,
  saleAttributeFreshnessBlocked = false,
}: {
  cookieBlocked: boolean;
  categoryBlocked: boolean;
  attributeBlocked: boolean;
  saleAttributeBlocked: boolean;
  authFreshnessBlocked?: boolean;
  categoryFreshnessBlocked?: boolean;
  attributeFreshnessBlocked?: boolean;
  saleAttributeFreshnessBlocked?: boolean;
}): ResolvedActionSummary | undefined {
  if (cookieBlocked || authFreshnessBlocked) {
    return {
      title: "重新登录店铺",
      summary: "先重新登录当前 SHEIN 店铺，恢复在线类目、属性和销售属性能力后再继续提交。",
      cta_kind: "review",
      action_key: "shein_online_auth",
    };
  }
  if (categoryFreshnessBlocked) {
    return {
      title: "刷新类目模板",
      summary: "当前 SHEIN 类目模板已经变化，先重新拉取类目结果并同步后续属性映射，再继续提交。",
      cta_kind: "review",
      action_key: "shein_category_template_freshness",
    };
  }
  if (attributeFreshnessBlocked) {
    return {
      title: "刷新普通属性",
      summary: "当前 SHEIN 普通属性模板已经变化，先重新生成并确认普通属性，再继续最终确认和提交。",
      cta_kind: "review",
      action_key: "shein_attribute_template_freshness",
    };
  }
  if (saleAttributeFreshnessBlocked) {
    return {
      title: "刷新销售属性",
      summary: "当前 SHEIN 销售属性模板已经变化，先重新生成并确认颜色、尺寸等销售属性映射，再继续提交。",
      cta_kind: "review",
      action_key: "shein_sale_attribute_freshness",
    };
  }
  if (attributeBlocked) {
    return {
      title: "确认普通属性",
      summary: "先补齐 SHEIN 模板要求的普通属性，再继续最终确认和提交。",
      cta_kind: "review",
      action_key: "attributes",
    };
  }
  if (saleAttributeBlocked) {
    return {
      title: "确认销售属性",
      summary: "先确认颜色、尺寸等销售属性映射，再继续最终确认和提交。",
      cta_kind: "review",
      action_key: "sale_attributes",
    };
  }
  if (categoryBlocked) {
    return {
      title: "确认类目",
      summary: "先确认 SHEIN 类目和类目模板，再继续最终确认和提交。",
      cta_kind: "review",
      action_key: "category",
    };
  }
  return undefined;
}

export function projectSheinReadinessActions(
  items: SheinReadinessItem[] = [],
): SheinReadinessProjection {
  const workspaceActionKeys = new Set<SheinWorkspaceActionKey>();
  const freshnessActionKeys = new Set<SheinFreshnessActionKey>();

  for (const item of items) {
    const workspaceActionKey =
      normalizeSheinReadinessItemWorkspaceActionKey(item);
    if (workspaceActionKey) {
      workspaceActionKeys.add(workspaceActionKey);
    }
    const freshnessActionKey =
      normalizeSheinReadinessItemFreshnessActionKey(item);
    if (freshnessActionKey) {
      freshnessActionKeys.add(freshnessActionKey);
    }
  }

  const authFreshnessBlocked = freshnessActionKeys.has("shein_online_auth");
  const categoryFreshnessBlocked = freshnessActionKeys.has(
    "shein_category_template_freshness",
  );
  const attributeFreshnessBlocked = freshnessActionKeys.has(
    "shein_attribute_template_freshness",
  );
  const saleAttributeFreshnessBlocked =
    freshnessActionKeys.has("shein_sale_attribute_freshness") ||
    freshnessActionKeys.has("shein_sale_attribute_template_freshness");
  const cookieBlocked =
    authFreshnessBlocked || workspaceActionKeys.has("store_login");
  const categoryBlocked =
    categoryFreshnessBlocked ||
    workspaceActionKeys.has("category") ||
    workspaceActionKeys.has("category_review");
  const attributeBlocked =
    attributeFreshnessBlocked ||
    workspaceActionKeys.has("attributes") ||
    workspaceActionKeys.has("attribute_review");
  const saleAttributeBlocked =
    saleAttributeFreshnessBlocked ||
    workspaceActionKeys.has("sale_attributes");
  const previewBlocked = workspaceActionKeys.has("images");

  return {
    cookieBlocked,
    categoryBlocked,
    attributeBlocked,
    saleAttributeBlocked,
    previewBlocked,
    blockingActionSummary: buildSheinBlockingActionSummary({
      cookieBlocked,
      categoryBlocked,
      attributeBlocked,
      saleAttributeBlocked,
      authFreshnessBlocked,
      categoryFreshnessBlocked,
      attributeFreshnessBlocked,
      saleAttributeFreshnessBlocked,
    }),
  };
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
    key === "sale_attributes"
  );
}

export function buildSheinGeneralReviewHref(
  taskId: string,
  targetId: string,
) {
  return `/listing-kits/${taskId}/workspace?platform=shein&section_key=general_review#${targetId}`;
}
