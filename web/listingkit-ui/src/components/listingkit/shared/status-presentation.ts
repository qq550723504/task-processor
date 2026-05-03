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

export function presentTaskStatus(status?: string): TaskStatusPresentation {
  switch (status) {
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
      return { label: "Ready for review", tone: "warning" };
    case "retry_needed":
      return { label: "Retry needed", tone: "danger" };
    case "failed":
      return { label: "Failed", tone: "danger" };
    case "processing":
    case "pending":
      return { label: "In progress", tone: "neutral" };
    case "completed":
      return { label: "Completed", tone: "success" };
    default:
      return {
        label: card.needs_review ? "Needs review" : titleCaseWords(card.status ?? "unknown"),
        tone: card.needs_review ? "warning" : "neutral",
      };
  }
}

export function presentQueueState(
  item: Pick<QueueItem, "state">,
): StatusPresentation {
  switch (item.state) {
    case "ready":
      return { label: "Ready", tone: "success" };
    case "fallback":
      return { label: "Fallback", tone: "warning" };
    case "failed":
      return { label: "Failed", tone: "danger" };
    case "pending":
      return { label: "Pending", tone: "neutral" };
    case "processing":
    case "running":
      return { label: "In progress", tone: "warning" };
    case "missing":
      return { label: "Missing", tone: "danger" };
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
      return { label: "Pending review", tone: "warning" };
    case "approved":
      return { label: "Approved", tone: "success" };
    case "deferred":
      return { label: "Deferred", tone: "neutral" };
    case "rejected":
      return { label: "Rejected", tone: "danger" };
    default:
      return {
        label: item.review_status ? titleCaseWords(item.review_status) : "Unreviewed",
        tone: "neutral",
      };
  }
}
