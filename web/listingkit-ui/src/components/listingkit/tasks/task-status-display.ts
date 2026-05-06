import type { ListingKitTaskResult } from "@/lib/types/listingkit";

type EmptyStateCopy = {
  title: string;
  description: string;
};

function primaryTaskError(task?: ListingKitTaskResult | null) {
  if (!task) return undefined;
  const blockingIssue = task.result?.workflow_issues?.find(
    (issue) => issue.severity === "blocking" && (issue.detail || issue.message),
  );
  if (blockingIssue) return blockingIssue.detail || blockingIssue.message;
  if (task.error) return task.error;
  return task.result?.child_tasks?.find((child) => child.error)?.error;
}

function isPendingTaskStatus(status?: string) {
  return (
    status === "pending" ||
    status === "processing" ||
    status === "queued" ||
    status === "running"
  );
}

function isCompletedTaskStatus(status?: string) {
  return status === "completed" || status === "succeeded";
}

function isFailedTaskStatus(status?: string) {
  return status === "failed" || status === "error";
}

export function shouldSuppressResolvedActionSummary(
  task: ListingKitTaskResult | null | undefined,
  options: { hasPreviewSvg: boolean; queueTotal: number },
) {
  return (
    task?.status === "failed" &&
    !options.hasPreviewSvg &&
    options.queueTotal === 0
  );
}

export function deriveTaskPreviewEmptyState(
  task?: ListingKitTaskResult | null,
): EmptyStateCopy | undefined {
  if (!task?.status || isCompletedTaskStatus(task.status)) {
    return undefined;
  }

  if (isFailedTaskStatus(task.status)) {
    return {
      title: "预览暂不可用",
      description:
        primaryTaskError(task) ??
        "当前任务在生成可预览内容前就中断了，请先查看失败原因或返回重新创建任务。",
    };
  }

  if (isPendingTaskStatus(task.status)) {
    return {
      title: "预览还在生成中",
      description:
        "任务仍在处理中，生成完成后这里会自动显示预览内容。你也可以稍后从任务列表继续。",
    };
  }

  return undefined;
}

export function deriveTaskQueueEmptyState(
  task?: ListingKitTaskResult | null,
): EmptyStateCopy | undefined {
  if (!task?.status || isCompletedTaskStatus(task.status)) {
    return undefined;
  }

  if (isFailedTaskStatus(task.status)) {
    return {
      title: "暂时没有可处理的队列项",
      description:
        primaryTaskError(task) ??
        "当前任务在生成队列项前就中断了，请先查看失败原因或返回重新创建任务。",
    };
  }

  if (isPendingTaskStatus(task.status)) {
    return {
      title: "队列项还在准备中",
      description:
        "任务仍在处理中，生成规划完成后这里会自动出现队列项。你也可以稍后回到任务列表继续。",
    };
  }

  return undefined;
}
