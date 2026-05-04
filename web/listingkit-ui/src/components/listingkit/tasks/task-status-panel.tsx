import { AlertTriangle, CheckCircle2, LoaderCircle } from "lucide-react";

import { Card } from "@/components/shared/card";
import { presentTaskStatus } from "@/components/listingkit/shared/status-presentation";
import { extractTaskReviewReasons } from "@/components/listingkit/tasks/task-review-reasons";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

function primaryTaskError(task: ListingKitTaskResult) {
  if (task.error) return task.error;
  const failedChild = task.result?.child_tasks?.find((child) => child.error);
  return failedChild?.error;
}

function formatTaskDate(value?: string) {
  if (!value) {
    return null;
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  }).format(date);
}

export function TaskStatusPanel({ task }: { task?: ListingKitTaskResult | null }) {
  if (!task?.status || task.status === "completed") {
    return null;
  }

  const presentation = presentTaskStatus(task.status);
  const tone =
    presentation.tone === "danger"
      ? {
          icon: AlertTriangle,
          iconClassName: "text-amber-600",
          badgeClassName: "border border-amber-200 bg-amber-50 text-amber-800",
        }
      : presentation.tone === "warning"
        ? {
            icon: LoaderCircle,
            iconClassName: "animate-spin text-sky-600",
            badgeClassName: "border border-sky-200 bg-sky-50 text-sky-800",
          }
        : {
            icon: CheckCircle2,
            iconClassName: "text-emerald-600",
            badgeClassName:
              "border border-emerald-200 bg-emerald-50 text-emerald-800",
          };
  const Icon = tone.icon;
  const error = primaryTaskError(task);
  const reviewReasons = extractTaskReviewReasons(task);
  const failedChildren =
    task.result?.child_tasks?.filter((child) => child.status === "failed") ?? [];
  const createdAt = formatTaskDate(task.created_at);
  const updatedAt = formatTaskDate(task.result?.updated_at ?? task.completed_at);
  const taskIdentifier = task.task_id ?? task.result?.task_id;

  return (
    <Card className="border-zinc-200 bg-white/90 p-5">
      <div className="space-y-4">
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-start gap-3">
            <Icon className={`mt-0.5 h-5 w-5 ${tone.iconClassName}`} />
            <div className="space-y-1">
              <div className="text-sm font-semibold text-zinc-950">
                {presentation.title}
              </div>
              <p className="text-sm leading-6 text-zinc-600">
                当前状态：
                <span className="ml-1 font-medium text-zinc-900">
                  {presentation.label}
                </span>
              </p>
            </div>
          </div>
          <span
            className={`inline-flex rounded-full px-2.5 py-1 text-xs font-medium ${tone.badgeClassName}`}
          >
            {presentation.label}
          </span>
        </div>

        {taskIdentifier || createdAt || updatedAt ? (
          <div className="grid gap-3 rounded-2xl border border-zinc-200 bg-zinc-50 p-4 md:grid-cols-3">
            {taskIdentifier ? (
              <div className="space-y-1">
                <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  任务标识
                </div>
                <p className="break-all text-sm text-zinc-700">{taskIdentifier}</p>
              </div>
            ) : null}
            {updatedAt ? (
              <div className="space-y-1">
                <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  最近更新
                </div>
                <p className="text-sm text-zinc-700">{updatedAt}</p>
              </div>
            ) : null}
            {createdAt ? (
              <div className="space-y-1">
                <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  已创建
                </div>
                <p className="text-sm text-zinc-700">{createdAt}</p>
              </div>
            ) : null}
          </div>
        ) : null}

        {task.status === "needs_review" && reviewReasons.length > 0 ? (
          <div className="space-y-2">
            <div className="text-xs font-semibold uppercase tracking-[0.2em] text-zinc-500">
              需要人工确认的原因
            </div>
            <ul className="space-y-2">
              {reviewReasons.map((reason) => (
                <li
                  className="rounded-2xl border border-amber-200 bg-amber-50/70 px-4 py-3 text-sm leading-6 text-zinc-700"
                  key={reason}
                >
                  {reason}
                </li>
              ))}
            </ul>
          </div>
        ) : error ? (
          <div className="rounded-2xl border border-zinc-200 bg-zinc-50 p-4 text-sm leading-6 text-zinc-700 whitespace-pre-wrap">
            {error}
          </div>
        ) : null}

        {failedChildren.length > 0 ? (
          <div className="space-y-2">
            <div className="text-xs font-semibold uppercase tracking-[0.2em] text-zinc-500">
              失败的子任务
            </div>
            <div className="space-y-2">
              {failedChildren.map((child) => (
                <div
                  key={`${child.kind}-${child.task_id}`}
                  className="rounded-2xl border border-zinc-200 px-4 py-3 text-sm text-zinc-700"
                >
                  <div className="font-medium text-zinc-900">
                    {child.kind ?? "child_task"}
                  </div>
                  <div className="mt-1 text-zinc-600">{child.task_id}</div>
                </div>
              ))}
            </div>
          </div>
        ) : null}
      </div>
    </Card>
  );
}
