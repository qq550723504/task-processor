import type { PodExecutionSummary } from "@/lib/types/listingkit/shein";

function providerLabel(provider?: string) {
  const value = provider?.trim();
  if (!value) {
    return "POD";
  }
  return value.toUpperCase();
}

function podModeLabel(mode?: string) {
  switch (mode) {
    case "required":
      return "Required";
    case "optional":
      return "Optional";
    case "disabled":
      return "Disabled";
    default:
      return mode || "Unknown";
  }
}

function podStatusLabel(status?: string) {
  switch (status) {
    case "not_applicable":
      return "Not applicable";
    case "pending":
      return "Pending";
    case "processing":
      return "Processing";
    case "succeeded":
      return "Succeeded";
    case "failed_blocking":
      return "Failed blocking";
    case "failed_degraded":
      return "Failed degraded";
    case "bypassed":
      return "Bypassed";
    default:
      return status || "Unknown";
  }
}

function formatAuditTime(value?: string) {
  if (!value) return "";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function podExecutionBadgeLabel(pod?: PodExecutionSummary) {
  if (!pod?.status || pod.status === "not_applicable") {
    return "";
  }

  const provider = providerLabel(pod.provider);
  switch (pod.status) {
    case "pending":
      return `POD ${provider} 待处理`;
    case "processing":
      return `POD ${provider} 处理中`;
    case "succeeded":
      return `POD ${provider} 已就绪`;
    case "failed_blocking":
      return `POD ${provider} 阻断中`;
    case "failed_degraded":
      return isPodSizeImageFallback(pod)
        ? `POD ${provider} 尺寸图已降级`
        : `POD ${provider} 已降级`;
    case "bypassed":
      return `POD ${provider} 已跳过`;
    default:
      return `POD ${provider} ${pod.status}`;
  }
}

export function podExecutionTone(pod?: PodExecutionSummary) {
  switch (pod?.status) {
    case "failed_blocking":
      return "border-sky-200 bg-sky-50 text-sky-700";
    case "failed_degraded":
    case "bypassed":
      return "border-amber-200 bg-amber-50 text-amber-700";
    case "processing":
    case "pending":
      return "border-cyan-200 bg-cyan-50 text-cyan-700";
    case "succeeded":
      return "border-emerald-200 bg-emerald-50 text-emerald-700";
    default:
      return "border-slate-200 bg-slate-50 text-slate-700";
  }
}

export function hasActionablePodExecution(pod?: PodExecutionSummary) {
  switch (pod?.status) {
    case "pending":
    case "processing":
    case "failed_blocking":
    case "failed_degraded":
      return true;
    default:
      return false;
  }
}

export function podExecutionNextAction(pod?: PodExecutionSummary) {
  if (!hasActionablePodExecution(pod)) {
    return "";
  }
  switch (pod?.status) {
    case "processing":
      return "等待 POD 平台处理";
    case "pending":
      return "启动 POD 平台处理";
    case "failed_degraded":
      return isPodSizeImageFallback(pod)
        ? "确认尺寸图降级结果"
        : "确认 POD 降级结果";
    default:
      return "处理 POD 平台结果";
  }
}

export function podExecutionSummaryText(pod?: PodExecutionSummary) {
  if (!hasActionablePodExecution(pod)) {
    return "";
  }
  switch (pod?.status) {
    case "processing":
      return "POD 平台结果仍在处理中，完成后再继续正式发布。";
    case "pending":
      return "POD 平台结果还未开始处理，完成后才能继续正式发布。";
    case "failed_degraded":
      return isPodSizeImageFallback(pod)
        ? "POD 平台尺寸图生成失败，当前会保留主图和场景图，按非 SDS 尺寸图路径继续发布。"
        : "POD 平台处理失败，当前任务将使用降级素材继续发布。";
    default:
      return "POD 平台结果还未就绪，处理完成后才能继续正式发布。";
  }
}

export function isPodSizeImageFallback(pod?: PodExecutionSummary) {
  if (pod?.status !== "failed_degraded") {
    return false;
  }
  const reason = `${pod.failure_reason ?? ""} ${pod.fallback_type ?? ""}`.toLowerCase();
  return (
    reason.includes("size image") ||
    reason.includes("size map") ||
    reason.includes("尺寸图")
  );
}

export function podExecutionHistorySummary(pod?: PodExecutionSummary) {
  if (!pod?.history?.length) {
    return podExecutionSummaryText(pod);
  }
  const lines = pod.history
    .slice()
    .reverse()
    .slice(0, 3)
    .map((event) => {
      const prefix = event.occurred_at ? `[${formatAuditTime(event.occurred_at)}] ` : "";
      if (event.kind === "policy_decision") {
        const detail = event.provider?.trim()
          ? `平台 ${event.provider.toUpperCase()}`
          : "POD 策略";
        return `${prefix}策略判定为 ${podModeLabel(event.dependency_mode)} · ${detail}`;
      }
      if (event.kind === "status_transition") {
        const detail = event.detail?.trim();
        return `${prefix}状态 ${podStatusLabel(event.from_status)} -> ${podStatusLabel(event.to_status)}${detail ? ` · ${detail}` : ""}`;
      }
      return `${prefix}${event.message ?? "POD 处理轨迹"}`;
    });
  return lines.join("\n");
}
