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
import type {
  ListingKitTaskListItem,
  ListingKitTaskListTaxonomy,
} from "@/lib/types/listingkit/tasks";

type ListingKitHomeTaskCardProps = {
  task: ListingKitTaskListItem;
  taxonomy?: ListingKitTaskListTaxonomy;
};

const RESUMABLE_SHEIN_WORKFLOW_STATUSES = new Set([
  "pending_confirmation",
  "ready_to_submit",
  "publish_failed",
  "draft_saved",
]);

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
  return (
    task.shein_status_overview?.primary_action ||
    task.shein_status_overview?.headline ||
    sheinSubmissionRemoteStatusLabel(task.shein_submission_remote_status) ||
    taskStatusLabel(task.status)
  );
}

function isResumableSheinTask(task: ListingKitTaskListItem) {
  return (
    (task.platforms ?? []).includes("shein") &&
    (RESUMABLE_SHEIN_WORKFLOW_STATUSES.has(task.shein_workflow_status ?? "") ||
      Boolean(task.shein_work_queue) ||
      Boolean(task.shein_action_queue))
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
    tone: "border-zinc-200 bg-zinc-100 text-zinc-700",
    value: generationSignalLabel(task.status),
  });

  if (task.shein_work_queue) {
    signals.push({
      tone: queueTone(workQueueSeverity),
      value: sheinWorkQueueLabel(task.shein_work_queue, taxonomy),
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
  const baseHref = `/listing-kits/${task.task_id}/workspace`;
  const platform = isResumableSheinTask(task) ? "shein" : task.platforms?.[0];

  if (!platform) {
    return baseHref;
  }

  return `${baseHref}?platform=${platform}`;
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
      className="grid gap-4 rounded-2xl border border-slate-200 bg-white/92 px-4 py-4 transition-colors hover:border-slate-300 hover:bg-white lg:grid-cols-[minmax(0,1fr)_auto] lg:items-center"
    >
      <div className="min-w-0 space-y-2.5">
        <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-500">
          <span>{platformLabel(task)}</span>
          {updatedAt ? <span className="tracking-[0.08em] text-slate-400">{updatedAt}</span> : null}
        </div>
        <div className="min-w-0">
          <h3 className="truncate text-sm font-semibold text-slate-950">{title}</h3>
          <p className="mt-1 truncate text-sm text-slate-500">
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
      <div className="flex min-w-[136px] items-center justify-between gap-3 rounded-xl bg-slate-50 px-3 py-3 lg:justify-end lg:bg-transparent lg:px-0 lg:py-0">
        <div className="min-w-0 lg:max-w-[160px]">
          <div className="text-[11px] font-semibold uppercase tracking-[0.14em] text-slate-400">
            下一步
          </div>
          <div className="truncate text-sm font-medium text-slate-700">{nextAction}</div>
        </div>
        <span className="inline-flex h-9 min-w-[88px] items-center justify-center rounded-lg border border-slate-200 bg-white px-3 text-sm font-semibold text-slate-900">
          进入
        </span>
      </div>
    </Link>
  );
}
