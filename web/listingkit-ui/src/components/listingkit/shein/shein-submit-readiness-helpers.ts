import type {
  SheinChecklistGroupItem,
  SheinResolutionCacheSummary,
  SheinSubmissionReport,
} from "@/lib/types/listingkit";

export function statusLabel(status?: string) {
  switch (status) {
    case "blocked":
      return "有阻断";
    case "ready_with_warnings":
      return "可提交但有提醒";
    case "ready":
      return "可提交";
    default:
      return "未知";
  }
}

export function statusTone(status?: string) {
  switch (status) {
    case "blocked":
      return "border-rose-200 bg-rose-50 text-rose-700";
    case "ready_with_warnings":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "ready":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    default:
      return "border-zinc-200 bg-zinc-50 text-zinc-700";
  }
}

export function readinessItemTone(key?: string) {
  switch ((key ?? "").toLowerCase()) {
    case "shein_online_auth":
      return "border-rose-200 bg-rose-50 text-rose-700";
    case "shein_category_template_freshness":
    case "shein_attribute_template_freshness":
    case "shein_sale_attribute_template_freshness":
    case "shein_sale_attribute_freshness":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "pod_platform":
      return "border-sky-200 bg-sky-50 text-sky-700";
    default:
      return "border-zinc-200 bg-zinc-50 text-zinc-700";
  }
}

export function readinessItemBadgeLabel(key?: string, suggestedAction?: string | null) {
  switch ((key ?? "").toLowerCase()) {
    case "shein_online_auth":
      return "店铺登录";
    case "shein_category_template_freshness":
      return "类目模板";
    case "shein_attribute_template_freshness":
      return "属性模板";
    case "shein_sale_attribute_template_freshness":
    case "shein_sale_attribute_freshness":
      return "销售属性模板";
    case "pod_platform":
      return "POD 平台";
    default:
      return suggestedAction ?? "";
  }
}

export function readinessItemDisplayLabel(
  key?: string,
  message?: string | null,
  fallbackLabel?: string | null,
) {
  const normalizedKey = (key ?? "").toLowerCase();
  const normalizedMessage = (message ?? "").toLowerCase();
  switch (normalizedKey) {
    case "shein_online_auth":
      return "SHEIN 在线登录态";
    case "shein_category_template_freshness":
      return "类目模板新鲜度";
    case "shein_attribute_template_freshness":
      return "普通属性模板新鲜度";
    case "shein_sale_attribute_template_freshness":
    case "shein_sale_attribute_freshness":
      return "销售属性模板新鲜度";
    default:
      break;
  }
  if (
    normalizedKey === "pod_platform" &&
    (normalizedMessage.includes("size image") ||
      normalizedMessage.includes("size map") ||
      normalizedMessage.includes("尺寸图"))
  ) {
    return "POD 尺寸图降级";
  }
  return fallbackLabel ?? key ?? "未命名问题";
}

export function readinessItemActionLabel(
  key?: string,
  message?: string | null,
  defaultLabel = "POD 平台",
) {
  const normalizedKey = (key ?? "").toLowerCase();
  const normalizedMessage = (message ?? "").toLowerCase();
  switch (normalizedKey) {
    case "shein_online_auth":
      return "店铺登录";
    case "shein_category_template_freshness":
      return "类目模板";
    case "shein_attribute_template_freshness":
      return "属性模板";
    case "shein_sale_attribute_template_freshness":
    case "shein_sale_attribute_freshness":
      return "销售属性模板";
    default:
      break;
  }
  if (
    normalizedKey === "pod_platform" &&
    (normalizedMessage.includes("size image") ||
      normalizedMessage.includes("size map") ||
      normalizedMessage.includes("尺寸图"))
  ) {
    return "POD 尺寸图";
  }
  return defaultLabel;
}

export function readinessItemButtonLabel(
  key?: string,
  defaultLabel = "去处理",
) {
  switch ((key ?? "").toLowerCase()) {
    case "shein_online_auth":
    case "store_login":
      return "去登录店铺";
    case "shein_category_template_freshness":
    case "category":
    case "category_review":
      return "去确认类目";
    case "shein_attribute_template_freshness":
    case "attributes":
    case "attribute_review":
      return "去确认属性";
    case "shein_sale_attribute_template_freshness":
    case "shein_sale_attribute_freshness":
    case "sale_attributes":
    case "variants":
      return "去确认销售属性";
    case "pod_platform":
      return "去检查 POD 结果";
    case "images":
      return "去检查图片";
    case "pricing":
      return "去检查价格和 SKU";
    default:
      return defaultLabel;
  }
}

export function checklistLabel(items?: SheinChecklistGroupItem[] | null) {
  if (!items?.length) {
    return null;
  }
  return items;
}

export function fieldPathsLabel(paths?: string[] | null) {
  if (!paths?.length) {
    return null;
  }
  return paths.join(" · ");
}

export function compactSubmissionMessage(message?: string | null) {
  if (!message) {
    return null;
  }

  const status = message.match(/STATUS:\s*([0-9]+)/i)?.[1];
  const eventId = message.match(/EVENT ID:\s*([0-9]+)/i)?.[1];
  const url = message.match(/\(URL:\s*([^)]+)\)/i)?.[1];
  if (status) {
    return [
      `SHEIN endpoint returned ${status}.`,
      eventId ? `Event ID: ${eventId}.` : null,
      url ? `URL: ${url}` : null,
    ]
      .filter(Boolean)
      .join(" ");
  }

  const text = message
    .replace(/<style[\s\S]*?<\/style>/gi, " ")
    .replace(/<script[\s\S]*?<\/script>/gi, " ")
    .replace(/<[^>]*>/g, " ")
    .replace(/\s+/g, " ")
    .trim();

  if (text.length <= 320) {
    return text;
  }
  return `${text.slice(0, 320)}...`;
}

export function normalizedSubmissionStatus(
  submissionState?: SheinSubmissionReport | null,
) {
  const status = submissionState?.last_status;
  const result = submissionState?.last_result;
  if (
    status === "unknown" &&
    (result?.success === false || result?.validation_notes?.length)
  ) {
    return "failed";
  }
  return status;
}

export function cacheSourceLabel(source?: string) {
  switch (source) {
    case "manual_cache":
      return "人工缓存";
    case "history_cache":
      return "历史缓存";
    case "memory_cache":
      return "内存缓存";
    case "live_resolver":
      return "实时解析";
    case "static_fallback":
      return "静态兜底";
    case "llm":
      return "LLM";
    default:
      return source ?? "未知";
  }
}

export function cacheHitSourceLabel(hitSource?: string, status?: string) {
  switch (hitSource) {
    case "memory_cache":
      return "内存缓存命中";
    case "persistent_manual_cache":
      return "数据库人工缓存命中";
    case "persistent_history_cache":
      return "数据库历史缓存命中";
    case "publish_remembered":
      return status === "stored" ? "发布后写入缓存" : "发布缓存复用";
    default:
      return null;
  }
}

export function cacheStatusLabel(status?: string) {
  switch (status) {
    case "hit":
      return "已命中";
    case "stored":
      return "已写入";
    default:
      return status ?? "未知";
  }
}

export function cacheUpdatedLabel(value?: string) {
  if (!value) {
    return "暂无时间";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value.replace("T", " ").replace(/\.\d+Z?$/, "").replace(/Z$/, "");
  }
  return new Intl.DateTimeFormat("zh-CN", {
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
    timeZone: "Asia/Shanghai",
  }).format(date);
}

export function hasResolutionCache(
  cache?: SheinResolutionCacheSummary | null,
) {
  return Boolean(
    cache?.category || cache?.attributes || cache?.sale_attributes || cache?.pricing,
  );
}
