import type { ListingKitTaskListItem } from "@/lib/types/listingkit/tasks";

type SheinFreshnessKey =
  | "shein_online_auth"
  | "shein_category_template_freshness"
  | "shein_attribute_template_freshness"
  | "shein_sale_attribute_template_freshness"
  | "shein_sale_attribute_freshness";

function normalizeSheinFreshnessKey(key?: string | null): SheinFreshnessKey | "" {
  switch ((key ?? "").toLowerCase()) {
    case "shein_online_auth":
    case "shein_category_template_freshness":
    case "shein_attribute_template_freshness":
    case "shein_sale_attribute_template_freshness":
    case "shein_sale_attribute_freshness":
      return (key ?? "").toLowerCase() as SheinFreshnessKey;
    default:
      return "";
  }
}

function firstFreshnessKey(task: ListingKitTaskListItem): SheinFreshnessKey | "" {
  for (const key of task.shein_blocking_keys ?? []) {
    const normalized = normalizeSheinFreshnessKey(key);
    if (normalized) {
      return normalized;
    }
  }
  for (const key of task.shein_warning_keys ?? []) {
    const normalized = normalizeSheinFreshnessKey(key);
    if (normalized) {
      return normalized;
    }
  }
  return "";
}

export function hasActionableSheinFreshness(task: ListingKitTaskListItem) {
  return Boolean(firstFreshnessKey(task));
}

export function sheinFreshnessBadgeLabel(task: ListingKitTaskListItem) {
  switch (firstFreshnessKey(task)) {
    case "shein_online_auth":
      return "SHEIN 店铺待登录";
    case "shein_category_template_freshness":
      return "SHEIN 类目模板漂移";
    case "shein_attribute_template_freshness":
      return "SHEIN 属性模板漂移";
    case "shein_sale_attribute_template_freshness":
    case "shein_sale_attribute_freshness":
      return "SHEIN 销售属性漂移";
    default:
      return "";
  }
}

export function sheinFreshnessTone(task: ListingKitTaskListItem) {
  switch (firstFreshnessKey(task)) {
    case "shein_online_auth":
      return "border-rose-200 bg-rose-50 text-rose-700";
    case "shein_category_template_freshness":
    case "shein_attribute_template_freshness":
    case "shein_sale_attribute_template_freshness":
    case "shein_sale_attribute_freshness":
      return "border-amber-200 bg-amber-50 text-amber-700";
    default:
      return "border-slate-200 bg-slate-50 text-slate-700";
  }
}

export function sheinFreshnessNextAction(task: ListingKitTaskListItem) {
  switch (firstFreshnessKey(task)) {
    case "shein_online_auth":
      return "重新登录 SHEIN 店铺";
    case "shein_category_template_freshness":
      return "刷新类目模板";
    case "shein_attribute_template_freshness":
      return "刷新属性模板";
    case "shein_sale_attribute_template_freshness":
    case "shein_sale_attribute_freshness":
      return "刷新销售属性";
    default:
      return "";
  }
}

export function sheinFreshnessSummaryText(task: ListingKitTaskListItem) {
  switch (firstFreshnessKey(task)) {
    case "shein_online_auth":
      return "SHEIN 提交店铺登录态已失效，刷新登录态后再继续正式发布。";
    case "shein_category_template_freshness":
      return "生成阶段使用的类目模板和当前在线模板已经不一致，需要先重新确认类目。";
    case "shein_attribute_template_freshness":
      return "生成阶段使用的普通属性模板和当前在线模板已经不一致，需要先重新刷新属性模板。";
    case "shein_sale_attribute_template_freshness":
    case "shein_sale_attribute_freshness":
      return "生成阶段使用的销售属性模板和当前在线模板已经不一致，需要先重新确认主副规格映射。";
    default:
      return "";
  }
}

export function sheinFreshnessFilter(task: ListingKitTaskListItem) {
  const key = firstFreshnessKey(task);
  if (!key) {
    return null;
  }
  const isBlocking = (task.shein_blocking_keys ?? []).includes(key);
  return {
    paramKey: isBlocking ? "shein_blocker_key" : "shein_warning_key",
    value: key,
  };
}
