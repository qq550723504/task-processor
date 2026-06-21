import { AlertTriangle, CheckCircle2, LoaderCircle } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Card } from "@/components/ui/card";
import {
  podExecutionBadgeLabel,
  podExecutionSummaryText,
} from "@/lib/listingkit/pod-execution";
import { getTaskSDSDesignResult } from "@/lib/listingkit/semantic-fields";
import type { ListingKitTaskResult, PodExecutionSummary } from "@/lib/types/listingkit";

function legacyStatusPresentation(status?: string) {
  switch (status) {
    case "completed":
      return {
        title: "POD 平台处理已完成",
        icon: CheckCircle2,
        iconClassName: "text-emerald-600",
        badgeClassName: "border border-emerald-200 bg-emerald-50 text-emerald-800",
      };
    case "failed":
      return {
        title: "POD 平台处理失败",
        icon: AlertTriangle,
        iconClassName: "text-amber-600",
        badgeClassName: "border border-amber-200 bg-amber-50 text-amber-800",
      };
    default:
      return {
        title: "POD 平台处理中",
        icon: LoaderCircle,
        iconClassName: "animate-spin text-sky-600",
        badgeClassName: "border border-sky-200 bg-sky-50 text-sky-800",
      };
  }
}

function firstWarning(task?: ListingKitTaskResult | null) {
  const workflowIssue = task?.result?.workflow_issues?.find(
    (issue) =>
      issue.stage === "sds_design_sync" &&
      (issue.severity === "warning" || issue.severity === "review" || issue.severity === "blocking") &&
      issue.message,
  );
  if (workflowIssue?.message) {
    return workflowIssue.message;
  }
  return task?.result?.summary?.warnings?.find((warning) =>
    warning.toLowerCase().includes("sds"),
  );
}

function sdsAuthIssue(task?: ListingKitTaskResult | null) {
  return task?.result?.workflow_issues?.find(
    (issue) =>
      issue.stage === "sds_design_sync" &&
      issue.code === "sds_auth_required" &&
      issue.message,
  );
}

function sdsIssueDetail(task?: ListingKitTaskResult | null, summary?: string) {
  const normalizedSummary = summary?.trim();
  const issue = task?.result?.workflow_issues?.find(
    (item) =>
      item.stage === "sds_design_sync" &&
      item.detail &&
      item.detail.trim() !== "" &&
      item.detail.trim() !== normalizedSummary,
  );
  return issue?.detail?.trim();
}

function latestSDSWorkflowStage(task?: ListingKitTaskResult | null) {
  const stages =
    task?.result?.workflow_stages?.filter((stage) => stage.kind === "sds_design_sync") ?? [];
  return stages[stages.length - 1];
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

function podAuditTitle(
  event: NonNullable<PodExecutionSummary["history"]>[number],
) {
  if (event.kind === "policy_decision") {
    return `策略判定为 ${podModeLabel(event.dependency_mode)}`;
  }
  if (event.kind === "status_transition") {
    return `状态从 ${podStatusLabel(event.from_status)} 变为 ${podStatusLabel(event.to_status)}`;
  }
  return event.message || "POD 处理轨迹";
}

function podAuditDetail(
  event: NonNullable<PodExecutionSummary["history"]>[number],
) {
  if (event.detail) {
    return event.detail;
  }
  if (event.kind === "policy_decision") {
    const provider = event.provider?.trim().toUpperCase();
    if (provider) {
      return `平台 ${provider}${event.decision_source ? ` · ${event.decision_source}` : ""}`;
    }
    return event.decision_source || "";
  }
  return event.message || "";
}

export function TaskPodExecutionCard({
  task,
}: {
  task?: ListingKitTaskResult | null;
}) {
  const sync = getTaskSDSDesignResult(task?.result);
  const pod = task?.result?.pod_execution;
  if (!sync?.variant_id && !pod?.status) {
    return null;
  }

  const presentation = legacyStatusPresentation(sync?.status);
  const Icon = presentation.icon;
  const warning = firstWarning(task);
  const authIssue = sdsAuthIssue(task);
  const detailedReason = sdsIssueDetail(task, sync?.error ?? warning);
  const workflowStage = latestSDSWorkflowStage(task);
  const podBadge = podExecutionBadgeLabel(pod);
  const podSummary = podExecutionSummaryText(pod);
  const podHistory = [...(pod?.history ?? [])].reverse();

  return (
    <Card className="border-border bg-card/95 p-5">
      <div className="space-y-4">
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-start gap-3">
            <Icon className={`mt-0.5 h-5 w-5 ${presentation.iconClassName}`} />
            <div className="space-y-1">
              <div className="text-sm font-semibold text-foreground">
                {presentation.title}
              </div>
              <p className="text-sm leading-6 text-muted-foreground">
                {podSummary ||
                  (sync?.variant_id ? (
                    <>
                      Variant <span className="font-mono text-foreground">{sync.variant_id}</span>
                      {sync.product_id ? (
                        <>
                          {" "}
                          synced to product{" "}
                          <span className="font-mono text-foreground">{sync.product_id}</span>
                        </>
                      ) : null}
                      .
                    </>
                  ) : (
                    "当前任务包含 POD 平台执行结果，可在这里查看处理状态和失败原因。"
                  ))}
              </p>
            </div>
          </div>
          <span
            className={`inline-flex rounded-full px-2.5 py-1 text-xs font-medium ${presentation.badgeClassName}`}
          >
            {podBadge || sync?.status || "pending"}
          </span>
        </div>

        {sync?.variant_id ? (
          <div className="grid gap-3 md:grid-cols-2">
            <div className="rounded-2xl border border-border bg-muted px-4 py-3">
              <div className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
                Variant
              </div>
              <div className="mt-2 font-mono text-sm font-medium text-foreground">
                {sync.variant_id}
              </div>
            </div>
            <div className="rounded-2xl border border-border bg-muted px-4 py-3">
              <div className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
                Product
              </div>
              <div className="mt-2 font-mono text-sm font-medium text-foreground">
                {sync.product_id ?? "Pending"}
              </div>
            </div>
            <div className="rounded-2xl border border-border bg-muted px-4 py-3">
              <div className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
                Prototype group
              </div>
              <div className="mt-2 text-sm font-medium text-foreground">
                {sync.prototype_group_id ?? "Auto"}
              </div>
            </div>
            <div className="rounded-2xl border border-border bg-muted px-4 py-3">
              <div className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
                Layer
              </div>
              <div className="mt-2 break-all font-mono text-sm text-foreground">
                {sync.layer_id ?? "Auto"}
              </div>
            </div>
            <div className="rounded-2xl border border-border bg-muted px-4 py-3">
              <div className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
                Material ID
              </div>
              <div className="mt-2 text-sm font-medium text-foreground">
                {sync.material_id ?? "Pending"}
              </div>
            </div>
            <div className="rounded-2xl border border-border bg-muted px-4 py-3">
              <div className="text-xs font-semibold uppercase tracking-[0.2em] text-muted-foreground">
                Workflow stage
              </div>
              <div className="mt-2 text-sm font-medium text-foreground">
                {workflowStage?.status ??
                  task?.result?.child_tasks?.find((child) => child.kind === "sds_design_sync")
                    ?.status ??
                  "pending"}
              </div>
            </div>
          </div>
        ) : null}

        {podHistory.length ? (
          <details className="rounded-2xl border border-border bg-muted/80 p-4">
            <summary className="cursor-pointer list-none text-sm font-semibold text-foreground">
              查看处理轨迹（{podHistory.length}）
            </summary>
            <div className="mt-3 space-y-3">
              {podHistory.map((event, index) => (
                <article
                  className="rounded-2xl border border-border/80 bg-background px-4 py-3"
                  key={`${event.kind ?? "event"}-${event.occurred_at ?? "time"}-${index}`}
                >
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="text-sm font-medium text-foreground">
                      {podAuditTitle(event)}
                    </div>
                    {event.occurred_at ? (
                      <div className="text-xs text-muted-foreground">
                        {formatAuditTime(event.occurred_at)}
                      </div>
                    ) : null}
                  </div>
                  {podAuditDetail(event) ? (
                    <p className="mt-1 text-xs leading-5 text-muted-foreground">
                      {podAuditDetail(event)}
                    </p>
                  ) : null}
                </article>
              ))}
            </div>
          </details>
        ) : null}

        {pod?.failure_reason && !sync?.error && !authIssue?.message ? (
          <Alert variant={pod.status === "failed_degraded" ? "warning" : "destructive"}>
            <AlertDescription>{pod.failure_reason}</AlertDescription>
          </Alert>
        ) : null}

        {authIssue?.message ? (
          <Alert variant="destructive">
            <AlertTitle>SDS 登录状态需要处理</AlertTitle>
            <AlertDescription>{authIssue.message}</AlertDescription>
            {authIssue.detail ? (
              <div className="mt-1 break-all font-mono text-xs text-red-800">{authIssue.detail}</div>
            ) : null}
          </Alert>
        ) : sync?.error ? (
          <Alert variant="warning">
            <AlertDescription>
              <div>{sync.error}</div>
              {detailedReason ? (
                <div className="mt-2 rounded-md border border-warning/30 bg-background/60 px-3 py-2 text-xs leading-5 text-foreground">
                  <div className="font-semibold">详细原因</div>
                  <div className="mt-1 break-all font-mono">{detailedReason}</div>
                </div>
              ) : null}
            </AlertDescription>
          </Alert>
        ) : warning ? (
          <div className="rounded-2xl border border-border bg-muted px-4 py-3 text-sm leading-6 text-foreground">
            {warning}
            {detailedReason ? (
              <div className="mt-2 rounded-md border border-border bg-background/70 px-3 py-2 text-xs leading-5">
                <div className="font-semibold">详细原因</div>
                <div className="mt-1 break-all font-mono">{detailedReason}</div>
              </div>
            ) : null}
          </div>
        ) : null}
      </div>
    </Card>
  );
}
