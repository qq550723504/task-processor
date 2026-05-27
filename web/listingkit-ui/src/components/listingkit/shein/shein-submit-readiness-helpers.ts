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
  return value.replace("T", " ").replace(/\.\d+Z?$/, "").replace(/Z$/, "");
}

export function hasResolutionCache(
  cache?: SheinResolutionCacheSummary | null,
) {
  return Boolean(
    cache?.category || cache?.attributes || cache?.sale_attributes || cache?.pricing,
  );
}
