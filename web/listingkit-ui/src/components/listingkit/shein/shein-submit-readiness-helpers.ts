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
  submission?: SheinSubmissionReport | null,
) {
  const status = submission?.last_status;
  const result = submission?.last_result;
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
      return "Manual";
    case "history_cache":
      return "DB";
    case "memory_cache":
      return "Memory";
    case "live_resolver":
      return "Live";
    case "static_fallback":
      return "Static";
    case "llm":
      return "LLM";
    default:
      return source ?? "未知";
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
  return Boolean(cache?.category || cache?.attributes || cache?.sale_attributes);
}
