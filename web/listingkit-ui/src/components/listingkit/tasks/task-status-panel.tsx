import { AlertTriangle, CheckCircle2, LoaderCircle } from "lucide-react";

import { Card } from "@/components/ui/card";
import { presentTaskStatus } from "@/components/listingkit/shared/status-presentation";
import { extractTaskReviewReasons } from "@/components/listingkit/tasks/task-review-reasons";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

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

function routeStrategyLabel(strategy?: string) {
  switch (strategy) {
    case "priority":
      return "按优先级";
    case "country":
      return "按国家匹配";
    case "manual":
    default:
      return "手工优先";
  }
}

function ruleLabel(kind?: string) {
  switch (kind) {
    case "country":
      return "国家规则";
    case "category":
      return "类目规则";
    default:
      return kind;
  }
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

        {storeResolution?.store_id ? (
          <div className="rounded-2xl border border-zinc-200 bg-zinc-50 p-4">
            <div className="flex flex-wrap items-start justify-between gap-3">
              <div className="space-y-1">
                <div className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  店铺解析
                </div>
                <p className="text-sm font-medium text-zinc-900">
                  SHEIN 店铺 {storeResolution.store_id}
                  {storeResolution.site ? ` · ${storeResolution.site}` : ""}
                </p>
                {storeResolution.reason ? (
                  <p className="text-sm leading-6 text-zinc-600">{storeResolution.reason}</p>
                ) : null}
              </div>
              {storeResolution.strategy ? (
                <span className="inline-flex rounded-full border border-zinc-200 bg-white px-2.5 py-1 text-xs font-medium text-zinc-700">
                  {routeStrategyLabel(storeResolution.strategy)}
                </span>
              ) : null}
            </div>
            {storeResolution.matched_rule_kinds?.length ? (
              <p className="mt-2 text-xs text-zinc-500">
                命中规则：
                {storeResolution.matched_rule_kinds
                  .map(ruleLabel)
                  .filter(Boolean)
                  .join(" / ")}
              </p>
            ) : null}
            {storeResolution.matched_profile_id || storeResolution.resolved_at ? (
              <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-xs text-zinc-500">
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
          <div className="rounded-2xl border border-zinc-200 bg-zinc-50 p-4 text-sm leading-6 text-zinc-700 whitespace-pre-wrap">
            {error}
          </div>
        ) : null}

        {failedStages.length > 0 ? (
          <div className="space-y-2">
            <div className="text-xs font-semibold uppercase tracking-[0.2em] text-zinc-500">
              失败的流程阶段
            </div>
            <div className="space-y-2">
              {failedStages.map((stage) => (
                <div
                  key={`${stage.kind}-${stage.task_id}-${stage.started_at}`}
                  className="rounded-2xl border border-zinc-200 px-4 py-3 text-sm text-zinc-700"
                >
                  <div className="font-medium text-zinc-900">
                    {stage.kind ?? "workflow_stage"}
                  </div>
                  {stage.task_id ? (
                    <div className="mt-1 text-zinc-600">{stage.task_id}</div>
                  ) : null}
                  {stage.error ? (
                    <div className="mt-1 text-zinc-600">{stage.error}</div>
                  ) : null}
                </div>
              ))}
            </div>
          </div>
        ) : failedChildren.length > 0 ? (
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
