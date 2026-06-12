import { AlertTriangle, CheckCircle2, LoaderCircle } from "lucide-react";

import { Card } from "@/components/ui/card";
import { Button } from "@/components/ui/button";
import { presentTaskStatus } from "@/components/listingkit/shared/status-presentation";
import { extractTaskReviewReasons } from "@/components/listingkit/tasks/task-review-reasons";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

function hasFailedSDSChildTask(task?: ListingKitTaskResult | null) {
  return (
    task?.result?.child_tasks?.some(
      (child) => child.kind === "sds_design_sync" && child.status === "failed",
    ) ?? false
  );
}

function primaryTaskError(task: ListingKitTaskResult) {
  const blockingIssue = task.result?.workflow_issues?.find(
    (issue) => issue.severity === "blocking" && (issue.detail || issue.message),
  );
  if (blockingIssue?.detail) return blockingIssue.detail;
  if (blockingIssue?.message) return blockingIssue.message;
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

function blockedRetryableReason(task?: ListingKitTaskResult | null) {
  return task?.retryable_block?.reason_message || primaryTaskError(task as ListingKitTaskResult);
}

function isBlockedRetryable(task?: ListingKitTaskResult | null) {
  return task?.status === "blocked_retryable";
}

export function TaskStatusPanel({
  task,
  onRecoverNow,
  onRetryChildTask,
  recoveringNow,
  retryingChildTaskKind,
}: {
  task?: ListingKitTaskResult | null;
  onRecoverNow?: () => void;
  onRetryChildTask?: (kind: string) => void;
  recoveringNow?: boolean;
  retryingChildTaskKind?: string | null;
}) {
  const keepsActionableFailureVisible =
    task?.status === "completed" && hasFailedSDSChildTask(task);
  if (!task?.status || (task.status === "completed" && !keepsActionableFailureVisible)) {
    return null;
  }

  const presentation = isBlockedRetryable(task)
    ? {
        label: "等待依赖恢复",
        title: "等待依赖恢复",
        tone: "warning" as const,
      }
    : presentTaskStatus(keepsActionableFailureVisible ? "needs_review" : task.status);
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
  const error = isBlockedRetryable(task)
    ? blockedRetryableReason(task)
    : primaryTaskError(task);
  const reviewReasons = extractTaskReviewReasons(task);
  const failedStages =
    task.result?.workflow_stages?.filter((stage) => stage.status === "failed") ?? [];
  const failedChildren =
    failedStages.length === 0
      ? (task.result?.child_tasks?.filter((child) => child.status === "failed") ?? [])
      : [];
  const createdAt = formatTaskDate(task.created_at);
  const updatedAt = formatTaskDate(task.result?.updated_at ?? task.completed_at);
  const taskIdentifier = task.task_id ?? task.result?.task_id;
  const storeResolution = task.result?.shein_store_resolution;
  const retryableBlock = task.retryable_block;
  const nextRetryAt = formatTaskDate(retryableBlock?.next_retry_at);

  return (
    <Card className="border-border bg-card/95 p-5">
      <div className="space-y-4">
        <div className="flex flex-col gap-4 sm:flex-row sm:items-start sm:justify-between">
          <div className="flex items-start gap-3">
            <Icon className={`mt-0.5 h-5 w-5 ${tone.iconClassName}`} />
            <div className="space-y-1">
              <div className="text-sm font-semibold text-foreground">
                {presentation.title}
              </div>
              <p className="text-sm leading-6 text-muted-foreground">
                当前状态：
                <span className="ml-1 font-medium text-foreground">
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
          <div className="grid gap-3 rounded-2xl border border-border bg-muted p-4 sm:grid-cols-2 xl:grid-cols-3">
            {taskIdentifier ? (
              <div className="space-y-1">
                <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                  任务标识
                </div>
                <p className="break-all text-sm text-foreground">{taskIdentifier}</p>
              </div>
            ) : null}
            {updatedAt ? (
              <div className="space-y-1">
                <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                  最近更新
                </div>
                <p className="text-sm text-foreground">{updatedAt}</p>
              </div>
            ) : null}
            {createdAt ? (
              <div className="space-y-1">
                <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                  已创建
                </div>
                <p className="text-sm text-foreground">{createdAt}</p>
              </div>
            ) : null}
          </div>
        ) : null}

        {isBlockedRetryable(task) && retryableBlock ? (
          <div className="grid gap-3 rounded-2xl border border-amber-200 bg-amber-50/70 p-4 sm:grid-cols-2 xl:grid-cols-4">
            <div className="space-y-1">
              <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-amber-700">
                恢复原因
              </div>
              <p className="text-sm text-foreground">
                {retryableBlock.reason_message ?? retryableBlock.reason_code ?? "等待上游依赖恢复"}
              </p>
            </div>
            {nextRetryAt ? (
              <div className="space-y-1">
                <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-amber-700">
                  下次重试
                </div>
                <p className="text-sm text-foreground">{nextRetryAt}</p>
              </div>
            ) : null}
            <div className="space-y-1">
              <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-amber-700">
                自动恢复
              </div>
              <p className="text-sm text-foreground">
                {retryableBlock.auto_retry_paused
                  ? "已暂停"
                  : retryableBlock.auto_resume_enabled
                    ? "已开启"
                    : "未开启"}
              </p>
            </div>
            {onRecoverNow ? (
              <div className="space-y-1">
                <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-amber-700">
                  恢复操作
                </div>
                <Button
                  disabled={recoveringNow}
                  onClick={onRecoverNow}
                  type="button"
                  variant="secondary"
                >
                  {recoveringNow ? "恢复中..." : "立即恢复"}
                </Button>
              </div>
            ) : null}
          </div>
        ) : null}

        {storeResolution?.store_id ? (
          <div className="rounded-2xl border border-border bg-muted p-4">
            <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
              <div className="space-y-1">
                <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-muted-foreground">
                  店铺解析
                </div>
                <p className="text-sm font-medium text-foreground">
                  SHEIN 店铺 {storeResolution.store_id}
                  {storeResolution.site ? ` · ${storeResolution.site}` : ""}
                </p>
              {storeResolution.reason ? (
                  <p className="text-sm leading-6 text-muted-foreground">{storeResolution.reason}</p>
                ) : null}
              </div>
            </div>
            {storeResolution.matched_profile_id || storeResolution.resolved_at ? (
              <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-muted-foreground">
                {storeResolution.matched_profile_id ? (
                  <span>Profile #{storeResolution.matched_profile_id}</span>
                ) : null}
                {storeResolution.resolved_at ? (
                  <span>固化时间：{formatTaskDate(storeResolution.resolved_at)}</span>
                ) : null}
              </div>
            ) : null}
          </div>
        ) : null}

        {task.status === "needs_review" && reviewReasons.length > 0 ? null : error ? (
          <div className="rounded-2xl border border-border bg-muted p-4 text-sm leading-6 text-foreground whitespace-pre-wrap">
            {error}
          </div>
        ) : null}

        {failedStages.length > 0 ? (
          <div className="space-y-2">
            <div className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
              失败的流程阶段
            </div>
            <div className="space-y-2">
              {failedStages.map((stage) => (
                <div
                  key={`${stage.kind}-${stage.task_id}-${stage.started_at}`}
                  className="rounded-2xl border border-border bg-background px-4 py-3 text-sm text-foreground"
                >
                  <div className="font-medium text-foreground">
                    {stage.kind ?? "workflow_stage"}
                  </div>
                  {stage.task_id ? (
                    <div className="mt-1 text-muted-foreground">{stage.task_id}</div>
                  ) : null}
                  {stage.error ? (
                    <div className="mt-1 text-muted-foreground">{stage.error}</div>
                  ) : null}
                </div>
              ))}
            </div>
          </div>
        ) : failedChildren.length > 0 ? (
          <div className="space-y-2">
            <div className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
              失败的子任务
            </div>
            <div className="space-y-2">
              {failedChildren.map((child) => (
                <div
                  key={`${child.kind}-${child.task_id}`}
                  className="rounded-2xl border border-border bg-background px-4 py-3 text-sm text-foreground"
                >
                  <div className="font-medium text-foreground">
                    {child.kind ?? "child_task"}
                  </div>
                  <div className="mt-1 text-muted-foreground">{child.task_id}</div>
                  {child.kind === "sds_design_sync" && onRetryChildTask ? (
                    <div className="mt-3">
                      <Button
                        disabled={retryingChildTaskKind === child.kind}
                        onClick={() => onRetryChildTask(child.kind ?? "sds_design_sync")}
                        type="button"
                        variant="secondary"
                      >
                        {retryingChildTaskKind === child.kind ? "重试中..." : "重试子任务"}
                      </Button>
                    </div>
                  ) : null}
                </div>
              ))}
            </div>
          </div>
        ) : null}
      </div>
    </Card>
  );
}
