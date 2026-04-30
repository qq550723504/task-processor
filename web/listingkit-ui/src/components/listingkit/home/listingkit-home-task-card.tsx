import Link from "next/link";

import { Card } from "@/components/shared/card";
import { sheinWorkflowStatusLabel } from "@/lib/shein-studio/shein-submission-display";
import type { ListingKitTaskListItem } from "@/lib/types/listingkit/tasks";

type ListingKitHomeTaskCardProps = {
  task: ListingKitTaskListItem;
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

function isResumableSheinTask(task: ListingKitTaskListItem) {
  return (
    (task.platforms ?? []).includes("shein") &&
    RESUMABLE_SHEIN_WORKFLOW_STATUSES.has(task.shein_workflow_status ?? "")
  );
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
}: ListingKitHomeTaskCardProps) {
  const title = taskTitle(task);

  return (
    <Card className="border-white/70 bg-white/88 p-4 shadow-[0_14px_36px_rgba(39,39,42,0.07)]">
      <div className="space-y-3">
        <div className="flex flex-wrap items-center gap-2">
          <span className="rounded-full bg-zinc-100 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-zinc-600">
            {platformLabel(task)}
          </span>
          <span className="rounded-full border border-zinc-200 bg-zinc-50 px-2.5 py-1 text-[11px] font-semibold text-zinc-700">
            {taskStatusLabel(task.status)}
          </span>
          {task.shein_workflow_status ? (
            <span className="rounded-full border border-orange-200 bg-orange-50 px-2.5 py-1 text-[11px] font-semibold text-orange-700">
              {sheinWorkflowStatusLabel(task.shein_workflow_status)}
            </span>
          ) : null}
        </div>
        <div className="space-y-1">
          <h3 className="text-base font-semibold text-zinc-950">
            {title}
          </h3>
          <p className="text-sm text-zinc-500">
            {task.variant_label || task.task_id}
          </p>
        </div>
        <Link
          href={taskWorkspaceHref(task)}
          aria-label={`继续处理 ${title}`}
          className="inline-flex h-10 items-center justify-center rounded-xl bg-white px-4 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100"
        >
          继续处理
        </Link>
      </div>
    </Card>
  );
}
