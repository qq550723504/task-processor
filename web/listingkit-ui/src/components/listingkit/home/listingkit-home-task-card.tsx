import Link from "next/link";

import {
  queueTone,
  sheinActionQueueLabel,
  sheinWorkQueueLabel,
  taxonomySeverity,
} from "@/components/listingkit/tasks/task-list-page-model";
import {
  sheinSubmissionRemoteStatusLabel,
  sheinWorkflowStatusLabel,
} from "@/lib/shein-studio/shein-submission-display";
import {
  hasActionablePodExecution,
  podExecutionBadgeLabel,
  podExecutionNextAction,
  podExecutionTone,
} from "@/lib/listingkit/pod-execution";
import {
  hasActionableSheinFreshness,
  sheinFreshnessBadgeLabel,
  sheinFreshnessNextAction,
  sheinFreshnessTone,
} from "@/lib/listingkit/shein-freshness";
import { buildTaskWorkspaceHref } from "@/lib/listingkit/task-workspace-href";
import type {
  ListingKitTaskListItem,
  ListingKitTaskListTaxonomy,
} from "@/lib/types/listingkit/tasks";

type ListingKitHomeTaskCardProps = {
  task: ListingKitTaskListItem;
  taxonomy?: ListingKitTaskListTaxonomy;
};

function taskStatusLabel(status?: string) {
  switch (status) {
    case "pending":
      return "待处理";
    case "processing":
      return "处理中";
    case "completed":
      return "已完成";
    case "needs_review":
      return "待审核";
    case "failed":
      return "失败";
    default:
      return status ?? "未知";
  }
}

function generationSignalLabel(status?: string) {
  return `生成 ${taskStatusLabel(status)}`;
}

function workflowSignalLabel(status?: string | null) {
  const label = sheinWorkflowStatusLabel(status);
  return label ? `SHEIN ${label}` : "";
}

function platformLabel(task: ListingKitTaskListItem) {
  const platforms = task.platforms ?? [];
  if (!platforms.length) {
    return "LISTINGKIT";
  }
  return platforms.map((platform) => platform.toUpperCase()).join(" / ");
}

function taskTitle(task: ListingKitTaskListItem) {
  return task.product_name || task.title || task.task_id.slice(0, 8);
}

function formatTaskTime(value?: string) {
  if (!value) {
    return "";
  }

  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }

  return new Intl.DateTimeFormat("zh-CN", {
    month: "numeric",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

function taskNextAction(task: ListingKitTaskListItem) {
  const podAction = podExecutionNextAction(task.pod_execution);
  if (podAction) {
    return podAction;
  }
  const freshnessAction = sheinFreshnessNextAction(task);
  if (freshnessAction) {
    return freshnessAction;
  }
  if (
    task.shein_blocking_keys?.includes("pod_platform") ||
    task.shein_warning_keys?.includes("pod_platform")
  ) {
    return "处理 POD 平台结果";
  }
  return (
    task.shein_status_overview?.primary_action ||
    task.shein_status_overview?.headline ||
    sheinSubmissionRemoteStatusLabel(task.shein_submission_remote_status) ||
    taskStatusLabel(task.status)
  );
}

function hasPodPlatformIssue(task: ListingKitTaskListItem) {
  return hasActionablePodExecution(task.pod_execution) || (
    task.shein_blocking_keys?.includes("pod_platform") ||
    task.shein_warning_keys?.includes("pod_platform")
  );
}

function compactSignals(
  task: ListingKitTaskListItem,
  taxonomy?: ListingKitTaskListTaxonomy,
) {
  const signals: Array<{ tone: string; value: string }> = [];
  const workQueueSeverity = taxonomySeverity(
    task.shein_work_queue,
    taxonomy?.shein_work_queues,
  );
  const actionQueueSeverity = taxonomySeverity(
    task.shein_action_queue,
    taxonomy?.shein_action_queues,
  );

  signals.push({
    tone: "border-border bg-muted text-muted-foreground",
    value: generationSignalLabel(task.status),
  });

  if (task.shein_work_queue) {
    signals.push({
      tone: queueTone(workQueueSeverity),
      value: sheinWorkQueueLabel(task.shein_work_queue, taxonomy),
    });
  }

  if (hasPodPlatformIssue(task)) {
    signals.push({
      tone: podExecutionTone(task.pod_execution),
      value: podExecutionBadgeLabel(task.pod_execution) || "POD 平台待处理",
    });
  }

  if (hasActionableSheinFreshness(task)) {
    signals.push({
      tone: sheinFreshnessTone(task),
      value: sheinFreshnessBadgeLabel(task),
    });
  }

  if (task.shein_action_queue) {
    signals.push({
      tone: queueTone(actionQueueSeverity),
      value: sheinActionQueueLabel(task.shein_action_queue, taxonomy),
    });
  }

  if (task.shein_workflow_status) {
    signals.push({
      tone: "border-orange-200 bg-orange-50 text-orange-700",
      value: workflowSignalLabel(task.shein_workflow_status),
    });
  }

  if (task.shein_submission_remote_status) {
    signals.push({
      tone: "border-sky-200 bg-sky-50 text-sky-700",
      value: sheinSubmissionRemoteStatusLabel(task.shein_submission_remote_status),
    });
  }

  return signals.slice(0, 3);
}

export function taskWorkspaceHref(task: ListingKitTaskListItem) {
  return buildTaskWorkspaceHref(task);
}

export function ListingKitHomeTaskCard({
  task,
  taxonomy,
}: ListingKitHomeTaskCardProps) {
  const title = taskTitle(task);
  const updatedAt = formatTaskTime(task.updated_at);
  const signals = compactSignals(task, taxonomy);
  const nextAction = taskNextAction(task);

  return (
    <Link
      href={taskWorkspaceHref(task)}
      aria-label={`继续处理 ${title}`}
      className="grid gap-4 rounded-2xl border border-border bg-card/95 px-4 py-4 transition-colors hover:border-foreground/25 hover:bg-card xl:grid-cols-[minmax(0,1fr)_auto] xl:items-center"
    >
      <div className="min-w-0 space-y-2.5">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-muted-foreground">
          <span>{platformLabel(task)}</span>
          {updatedAt ? <span className="tracking-[0.08em] text-muted-foreground/80">{updatedAt}</span> : null}
        </div>
        <div className="min-w-0">
          <h3 className="line-clamp-2 break-words text-sm font-semibold text-foreground">
            {title}
          </h3>
          <p className="mt-1 line-clamp-2 break-all text-sm text-muted-foreground">
            {task.variant_label || task.task_id}
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {signals.map((signal) => (
            <span
              key={signal.value}
              className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold ${signal.tone}`}
            >
              {signal.value}
            </span>
          ))}
        </div>
      </div>
      <div className="flex w-full min-w-0 flex-col items-start gap-3 rounded-xl bg-muted px-3 py-3 sm:flex-row sm:items-center sm:justify-between xl:w-auto xl:justify-end xl:bg-transparent xl:px-0 xl:py-0">
        <div className="min-w-0 xl:max-w-[160px]">
          <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-muted-foreground">
            下一步
          </div>
          <div className="line-clamp-2 text-sm font-medium text-foreground">{nextAction}</div>
        </div>
        <span className="inline-flex h-9 w-full items-center justify-center rounded-lg border border-border bg-background px-3 text-sm font-semibold text-foreground sm:min-w-[88px] sm:w-auto">
          进入
        </span>
      </div>
    </Link>
  );
}
