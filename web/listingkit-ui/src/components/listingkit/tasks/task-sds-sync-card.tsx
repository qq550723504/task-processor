import { AlertTriangle, CheckCircle2, LoaderCircle } from "lucide-react";

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert";
import { Card } from "@/components/ui/card";
import { getTaskSDSDesignResult } from "@/lib/listingkit/semantic-fields";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";

function statusPresentation(status?: string) {
  switch (status) {
    case "completed":
      return {
        title: "SDS sync completed",
        icon: CheckCircle2,
        iconClassName: "text-emerald-600",
        badgeClassName: "border border-emerald-200 bg-emerald-50 text-emerald-800",
      };
    case "failed":
      return {
        title: "SDS sync failed",
        icon: AlertTriangle,
        iconClassName: "text-amber-600",
        badgeClassName: "border border-amber-200 bg-amber-50 text-amber-800",
      };
    default:
      return {
        title: "SDS sync pending",
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

function latestSDSWorkflowStage(task?: ListingKitTaskResult | null) {
  const stages =
    task?.result?.workflow_stages?.filter((stage) => stage.kind === "sds_design_sync") ?? [];
  return stages[stages.length - 1];
}

export function TaskSDSSyncCard({
  task,
}: {
  task?: ListingKitTaskResult | null;
}) {
  const sync = getTaskSDSDesignResult(task?.result);
  if (!sync?.variant_id) {
    return null;
  }

  const presentation = statusPresentation(sync.status);
  const Icon = presentation.icon;
  const warning = firstWarning(task);
  const authIssue = sdsAuthIssue(task);
  const workflowStage = latestSDSWorkflowStage(task);

  return (
    <Card className="border-zinc-200 bg-white/90 p-5">
      <div className="space-y-4">
        <div className="flex items-start justify-between gap-4">
          <div className="flex items-start gap-3">
            <Icon className={`mt-0.5 h-5 w-5 ${presentation.iconClassName}`} />
            <div className="space-y-1">
              <div className="text-sm font-semibold text-zinc-950">
                {presentation.title}
              </div>
              <p className="text-sm leading-6 text-zinc-600">
                Variant <span className="font-mono text-zinc-900">{sync.variant_id}</span>
                {sync.product_id ? (
                  <>
                    {" "}
                    synced to product{" "}
                    <span className="font-mono text-zinc-900">{sync.product_id}</span>
                  </>
                ) : null}
                .
              </p>
            </div>
          </div>
          <span
            className={`inline-flex rounded-full px-2.5 py-1 text-xs font-medium ${presentation.badgeClassName}`}
          >
            {sync.status ?? "pending"}
          </span>
        </div>

        <div className="grid gap-3 md:grid-cols-2">
          <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3">
            <div className="text-xs font-semibold uppercase tracking-[0.2em] text-zinc-500">
              Prototype group
            </div>
            <div className="mt-2 text-sm font-medium text-zinc-900">
              {sync.prototype_group_id ?? "Auto"}
            </div>
          </div>
          <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3">
            <div className="text-xs font-semibold uppercase tracking-[0.2em] text-zinc-500">
              Layer
            </div>
            <div className="mt-2 break-all font-mono text-sm text-zinc-900">
              {sync.layer_id ?? "Auto"}
            </div>
          </div>
          <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3">
            <div className="text-xs font-semibold uppercase tracking-[0.2em] text-zinc-500">
              Material ID
            </div>
            <div className="mt-2 text-sm font-medium text-zinc-900">
              {sync.material_id ?? "Pending"}
            </div>
          </div>
          <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3">
            <div className="text-xs font-semibold uppercase tracking-[0.2em] text-zinc-500">
              Workflow stage
            </div>
            <div className="mt-2 text-sm font-medium text-zinc-900">
              {workflowStage?.status ??
                task?.result?.child_tasks?.find((child) => child.kind === "sds_design_sync")
                  ?.status ??
                "pending"}
            </div>
          </div>
        </div>

        {authIssue?.message ? (
          <Alert variant="destructive">
            <AlertTitle>SDS 登录状态需要处理</AlertTitle>
            <AlertDescription>{authIssue.message}</AlertDescription>
            {authIssue.detail ? (
              <div className="mt-1 break-all font-mono text-xs text-red-800">{authIssue.detail}</div>
            ) : null}
          </Alert>
        ) : sync.error ? (
          <Alert variant="warning">
            <AlertDescription>{sync.error}</AlertDescription>
          </Alert>
        ) : warning ? (
          <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm leading-6 text-zinc-700">
            {warning}
          </div>
        ) : null}
      </div>
    </Card>
  );
}
