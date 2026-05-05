import type { PlatformCard, QueueItem } from "@/lib/types/listingkit";

type BadgeTone = "neutral" | "success" | "warning" | "danger";

type StatusPresentation = {
  label: string;
  tone: BadgeTone;
};

type TaskStatusPresentation = StatusPresentation & {
  title: string;
};

function titleCaseWords(value: string) {
  return value
    .split(/[_\s-]+/)
    .filter(Boolean)
    .map((part) => part.charAt(0).toUpperCase() + part.slice(1))
    .join(" ");
}

function normalizeTaskStatus(status?: string) {
  switch (status) {
    case "queued":
      return "pending";
    case "running":
      return "processing";
    case "succeeded":
      return "completed";
    case "review_ready":
      return "needs_review";
    case "error":
      return "failed";
    default:
      return status;
  }
}

export function presentTaskStatus(status?: string): TaskStatusPresentation {
  switch (normalizeTaskStatus(status)) {
    case "failed":
      return { label: "失败", title: "任务处理失败", tone: "danger" };
    case "needs_review":
      return {
        label: "待确认",
        title: "任务需要人工确认",
        tone: "warning",
      };
    case "processing":
      return { label: "处理中", title: "任务处理中", tone: "warning" };
    case "pending":
      return { label: "待开始", title: "任务已创建", tone: "neutral" };
    case "completed":
      return { label: "已完成", title: "任务已处理完成", tone: "success" };
    default:
      return {
        label: titleCaseWords(status ?? "unknown"),
        title: "任务状态",
        tone: "neutral",
      };
  }
}

export function presentPlatformStatus(
  card: Pick<PlatformCard, "status" | "needs_review">,
): StatusPresentation {
  switch (card.status) {
    case "review_ready":
      return { label: "待检查", tone: "warning" };
    case "retry_needed":
      return { label: "需要重试", tone: "danger" };
    case "failed":
      return { label: "失败", tone: "danger" };
    case "processing":
    case "pending":
      return { label: "处理中", tone: "neutral" };
    case "completed":
      return { label: "已完成", tone: "success" };
    default:
      return {
        label: card.needs_review ? "需处理" : titleCaseWords(card.status ?? "unknown"),
        tone: card.needs_review ? "warning" : "neutral",
      };
  }
}

export function presentQueueState(
  item: Pick<QueueItem, "state">,
): StatusPresentation {
  switch (item.state) {
    case "ready":
      return { label: "已就绪", tone: "success" };
    case "fallback":
      return { label: "使用兜底结果", tone: "warning" };
    case "failed":
      return { label: "失败", tone: "danger" };
    case "pending":
      return { label: "等待中", tone: "neutral" };
    case "processing":
    case "running":
      return { label: "处理中", tone: "warning" };
    case "missing":
      return { label: "缺失", tone: "danger" };
    default:
      return {
        label: titleCaseWords(item.state ?? "unknown"),
        tone: "neutral",
      };
  }
}

export function presentQueueReviewStatus(
  item: Pick<QueueItem, "review_status">,
): StatusPresentation {
  switch (item.review_status) {
    case "pending":
      return { label: "待复核", tone: "warning" };
    case "approved":
      return { label: "已通过", tone: "success" };
    case "deferred":
      return { label: "已延后", tone: "neutral" };
    case "rejected":
      return { label: "已驳回", tone: "danger" };
    default:
      return {
        label: item.review_status ? titleCaseWords(item.review_status) : "未复核",
        tone: "neutral",
      };
  }
}
