import Link from "next/link";
import { ArrowRight, Boxes, Clock, LoaderCircle, Plus, RefreshCw } from "lucide-react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Select } from "@/components/ui/select";
import { EmptyState } from "@/components/shared/empty-state";
import {
  buildFacetSummarySections,
  descriptorOptions,
  facetDescriptorLabel,
  formatDate,
  PLATFORM_OPTIONS,
  primaryLinkClass,
  queueTone,
  secondaryLinkClass,
  sheinActionQueueLabel,
  sheinWorkQueueLabel,
  SHEIN_WORKFLOW_OPTIONS,
  SHEIN_ACTION_QUEUE_OPTIONS,
  SHEIN_WORK_QUEUE_OPTIONS,
  STATUS_OPTIONS,
  statusTone,
  taskStatusLabel,
  taskTitle,
  taxonomySeverity,
} from "@/components/listingkit/tasks/task-list-page-model";
import {
  sheinSubmissionRemoteStatusLabel,
  sheinSubmissionStatusLabel,
  sheinWorkflowStatusLabel,
} from "@/lib/shein-studio/shein-submission-display";
import {
  hasActionablePodExecution,
  podExecutionBadgeLabel,
  podExecutionHistorySummary,
  podExecutionTone,
} from "@/lib/listingkit/pod-execution";
import {
  hasActionableSheinFreshness,
  sheinFreshnessBadgeLabel,
  sheinFreshnessSummaryText,
  sheinFreshnessTone,
} from "@/lib/listingkit/shein-freshness";
import type {
  ListingKitTaskListItem,
  ListingKitTaskListSummary,
  ListingKitTaskListTaxonomy,
} from "@/lib/types/listingkit";

export type FilterKey =
  | "status"
  | "platform"
  | "shein_workflow_status"
  | "shein_work_queue"
  | "shein_action_queue"
  | "shein_blocker_key"
  | "shein_warning_key";

export function TaskListHero({ onRefresh }: { onRefresh: () => void }) {
  return (
    <section className="grid gap-5 rounded-[2rem] border border-white/70 bg-white/78 p-5 shadow-[0_24px_90px_rgba(39,39,42,0.10)] backdrop-blur sm:p-6 xl:grid-cols-[1fr_auto] xl:items-end">
      <div>
        <p className="text-[11px] font-semibold uppercase tracking-[0.34em] text-teal-700">
          任务总览
        </p>
        <h1 className="mt-3 text-4xl font-semibold tracking-[-0.04em] text-zinc-950">
          任务列表
        </h1>
        <p className="mt-2 max-w-2xl text-sm leading-6 text-zinc-600">
          查看最近生成、审核和提交到 SHEIN 的任务。这里直接读取后端任务仓储，不再靠你手动记 task id。
        </p>
      </div>
      <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap">
        <Button className="w-full sm:w-auto" variant="secondary" onClick={onRefresh}>
          <RefreshCw className="mr-2 h-4 w-4" />
          刷新
        </Button>
        <Link href="/listing-kits/sds" className={`${primaryLinkClass} w-full sm:w-auto`}>
          <Plus className="mr-2 h-4 w-4" />
          新建 POD 批次
        </Link>
      </div>
    </section>
  );
}

function storeResolutionStrategyLabel(strategy?: string) {
  switch (strategy) {
    case "priority":
      return "按优先级";
    case "country":
      return "按国家匹配";
    case "manual":
      return "手工优先";
    default:
      return strategy || "";
  }
}

function storeResolutionRuleLabel(kind?: string) {
  switch (kind) {
    case "country":
      return "国家规则";
    case "category":
      return "类目规则";
    default:
      return kind || "";
  }
}

function storeResolutionAuditTitle(task: ListingKitTaskListItem) {
  const lines: string[] = [];
  if (task.shein_store_id) {
    lines.push(
      `SHEIN 店铺 ${task.shein_store_id}${task.shein_store_site ? ` · ${task.shein_store_site}` : ""}`,
    );
  }
  if (task.shein_store_profile_id) {
    lines.push(`Profile #${task.shein_store_profile_id}`);
  }
  if (task.shein_store_strategy) {
    lines.push(`路由策略：${storeResolutionStrategyLabel(task.shein_store_strategy)}`);
  }
  if (task.shein_store_matched_rule_kinds?.length) {
    lines.push(
      `命中规则：${task.shein_store_matched_rule_kinds
        .map(storeResolutionRuleLabel)
        .filter(Boolean)
        .join(" / ")}`,
    );
  }
  if (task.shein_store_manual_override) {
    lines.push("手工指定：是");
  }
  if (task.shein_store_fallback) {
    lines.push("Fallback：是");
  }
  if (task.shein_store_reason) {
    lines.push(`原因：${task.shein_store_reason}`);
  }
  if (task.shein_store_resolved_at) {
    lines.push(`固化时间：${formatDate(task.shein_store_resolved_at)}`);
  }
  return lines.join("\n");
}

function taskLifecycleBadgeLabel(status?: string) {
  return `生成 ${taskStatusLabel(status)}`;
}

function sheinWorkflowBadgeLabel(status?: string) {
  const label = sheinWorkflowStatusLabel(status);
  return label ? `SHEIN ${label}` : "";
}

function sheinRemoteBadgeLabel(status?: string) {
  return sheinSubmissionRemoteStatusLabel(status);
}

function hasPodPlatformIssue(task: ListingKitTaskListItem) {
  return (
    task.shein_blocking_keys?.includes("pod_platform") ||
    task.shein_warning_keys?.includes("pod_platform")
  );
}

export function TaskListFilters({
  platform,
  sheinActionQueue,
  sheinBlockerKey,
  sheinWorkflowStatus,
  sheinWarningKey,
  sheinWorkQueue,
  status,
  summary,
  taxonomy,
  total,
  updateFilters,
  updateFilter,
}: {
  platform: string;
  sheinActionQueue: string;
  sheinBlockerKey: string;
  sheinWorkflowStatus: string;
  sheinWarningKey: string;
  sheinWorkQueue: string;
  status: string;
  summary?: ListingKitTaskListSummary;
  taxonomy?: ListingKitTaskListTaxonomy;
  total: number;
  updateFilters: (updates: Partial<Record<FilterKey, string | null>>) => void;
  updateFilter: (key: FilterKey, value: string) => void;
}) {
  const workflowOptions = descriptorOptions(
    taxonomy?.shein_workflow_statuses,
    SHEIN_WORKFLOW_OPTIONS,
    "全部 SHEIN 状态",
  );
  const workQueueOptions = descriptorOptions(
    taxonomy?.shein_work_queues,
    SHEIN_WORK_QUEUE_OPTIONS,
    "全部工作队列",
  );
  const actionQueueOptions = descriptorOptions(
    taxonomy?.shein_action_queues,
    SHEIN_ACTION_QUEUE_OPTIONS,
    "全部处理动作",
  );
  const blockerOptions = descriptorOptions(
    taxonomy?.shein_blockers,
    [],
    "全部阻断项",
  );
  const warningOptions = descriptorOptions(
    taxonomy?.shein_warnings,
    [],
    "全部待确认项",
  );
  const summarySections = buildFacetSummarySections(summary, taxonomy);
  const activeFacetValueByKey: Record<FilterKey, string> = {
    platform,
    shein_action_queue: sheinActionQueue,
    shein_blocker_key: sheinBlockerKey,
    shein_warning_key: sheinWarningKey,
    shein_work_queue: sheinWorkQueue,
    shein_workflow_status: sheinWorkflowStatus,
    status,
  };
  const activeFilters = [
    status
      ? {
          key: "status" as const,
          label: facetDescriptorLabel(status, undefined, STATUS_OPTIONS),
        }
      : null,
    platform
      ? {
          key: "platform" as const,
          label: facetDescriptorLabel(platform, undefined, PLATFORM_OPTIONS),
        }
      : null,
    sheinWorkflowStatus
      ? {
          key: "shein_workflow_status" as const,
          label: facetDescriptorLabel(
            sheinWorkflowStatus,
            taxonomy?.shein_workflow_statuses,
            SHEIN_WORKFLOW_OPTIONS,
          ),
        }
      : null,
    sheinWorkQueue
      ? {
          key: "shein_work_queue" as const,
          label: facetDescriptorLabel(
            sheinWorkQueue,
            taxonomy?.shein_work_queues,
            SHEIN_WORK_QUEUE_OPTIONS,
          ),
        }
      : null,
    sheinActionQueue
      ? {
          key: "shein_action_queue" as const,
          label: facetDescriptorLabel(
            sheinActionQueue,
            taxonomy?.shein_action_queues,
            SHEIN_ACTION_QUEUE_OPTIONS,
          ),
        }
      : null,
    sheinBlockerKey
      ? {
          key: "shein_blocker_key" as const,
          label: facetDescriptorLabel(sheinBlockerKey, taxonomy?.shein_blockers, []),
        }
      : null,
    sheinWarningKey
      ? {
          key: "shein_warning_key" as const,
          label: facetDescriptorLabel(sheinWarningKey, taxonomy?.shein_warnings, []),
        }
      : null,
  ].filter((item): item is { key: FilterKey; label: string } => Boolean(item));

  const applySummaryFilter = (key: FilterKey, value: string) => {
    if (key === "shein_work_queue") {
      updateFilters({
        shein_work_queue: sheinWorkQueue === value ? null : value,
        shein_action_queue: null,
        shein_blocker_key: null,
        shein_warning_key: null,
      });
      return;
    }
    if (key === "shein_action_queue") {
      updateFilters({
        shein_action_queue: sheinActionQueue === value ? null : value,
        shein_blocker_key: null,
        shein_warning_key: null,
      });
      return;
    }
    if (key === "shein_blocker_key") {
      updateFilters({
        shein_blocker_key: sheinBlockerKey === value ? null : value,
        shein_warning_key: null,
      });
      return;
    }
    if (key === "shein_warning_key") {
      updateFilters({
        shein_warning_key: sheinWarningKey === value ? null : value,
        shein_blocker_key: null,
      });
    }
  };

  return (
    <Card className="border-white/70 bg-white/82 p-4">
      <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4 2xl:grid-cols-6">
        <Select
          className="h-11 w-full rounded-2xl px-4 text-sm"
          value={status}
          onChange={(event) => updateFilter("status", event.target.value)}
        >
          {STATUS_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </Select>
        <Select
          className="h-11 w-full rounded-2xl px-4 text-sm"
          value={sheinWorkflowStatus}
          onChange={(event) =>
            updateFilter("shein_workflow_status", event.target.value)
          }
        >
          {workflowOptions.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
            ))}
        </Select>
        <Select
          className="h-11 w-full rounded-2xl px-4 text-sm"
          value={sheinWorkQueue}
          onChange={(event) => updateFilter("shein_work_queue", event.target.value)}
        >
          {workQueueOptions.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </Select>
        <Select
          className="h-11 w-full rounded-2xl px-4 text-sm"
          value={sheinActionQueue}
          onChange={(event) => updateFilter("shein_action_queue", event.target.value)}
        >
          {actionQueueOptions.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </Select>
        <Select
          className="h-11 w-full rounded-2xl px-4 text-sm"
          value={platform}
          onChange={(event) => updateFilter("platform", event.target.value)}
        >
          {PLATFORM_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </Select>
        {blockerOptions.length > 1 ? (
          <Select
            className="h-11 w-full rounded-2xl px-4 text-sm"
            value={sheinBlockerKey}
            onChange={(event) => updateFilter("shein_blocker_key", event.target.value)}
          >
            {blockerOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </Select>
        ) : null}
        {warningOptions.length > 1 ? (
          <Select
            className="h-11 w-full rounded-2xl px-4 text-sm"
            value={sheinWarningKey}
            onChange={(event) => updateFilter("shein_warning_key", event.target.value)}
          >
            {warningOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </Select>
        ) : null}
        <div className="flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-zinc-500 md:col-span-2 xl:col-span-4 2xl:col-span-6 xl:justify-end">
          <Boxes className="h-4 w-4" />
          {total} 个任务
        </div>
      </div>
      {activeFilters.length ? (
        <div className="mt-4 flex flex-wrap items-center gap-2">
          <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
            当前筛选
          </p>
          {activeFilters.map((filter) => (
            <Button
              key={filter.key}
              type="button"
              variant="outline"
              onClick={() => updateFilter(filter.key, "")}
              className="h-auto rounded-full bg-zinc-50 px-3 py-1 text-xs text-zinc-700"
            >
              {filter.label}
            </Button>
          ))}
          {activeFilters.length > 1 ? (
            <Button
              type="button"
              variant="ghost"
              onClick={() =>
                updateFilters({
                  platform: null,
                  shein_action_queue: null,
                  shein_blocker_key: null,
                  shein_warning_key: null,
                  shein_work_queue: null,
                  shein_workflow_status: null,
                  status: null,
                })
              }
              className="h-auto px-2 py-1 text-xs text-zinc-500"
            >
              清空全部
            </Button>
          ) : null}
        </div>
      ) : null}
      {summarySections.length ? (
        <div className="mt-4 grid gap-3">
          {summarySections.map((section) => (
            <div key={section.filterKey} className="grid gap-2">
              <div className="flex items-center justify-between gap-3">
                <p className="text-[11px] font-semibold uppercase tracking-[0.18em] text-zinc-500">
                  {section.title}
                </p>
                {activeFacetValueByKey[section.filterKey] ? (
                  <Button
                    type="button"
                    variant="ghost"
                    onClick={() =>
                      updateFilter(section.filterKey, "")
                    }
                    className="h-auto px-2 py-1 text-[11px] text-zinc-500"
                  >
                    清除
                  </Button>
                ) : null}
              </div>
              <div className="flex flex-wrap gap-2">
                {section.entries.map((entry) => {
                  const active =
                    activeFacetValueByKey[section.filterKey] === entry.key;
                  return (
                    <Button
                      key={entry.key}
                      type="button"
                      variant={active ? "default" : "outline"}
                      onClick={() =>
                        applySummaryFilter(section.filterKey, entry.key)
                      }
                      aria-pressed={active}
                      className={`h-auto rounded-full px-3 py-1 text-xs ${active ? "text-white" : queueTone(entry.severity)}`}
                    >
                      {entry.label} · {entry.count}
                    </Button>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      ) : null}
    </Card>
  );
}

export function TaskListContent({
  isError,
  isLoading,
  items,
  onRefresh,
  page,
  pageSize,
  total,
  taxonomy,
  updatePage,
}: {
  isError: boolean;
  isLoading: boolean;
  items: ListingKitTaskListItem[];
  onRefresh: () => void;
  page: number;
  pageSize: number;
  total: number;
  taxonomy?: ListingKitTaskListTaxonomy;
  updatePage: (page: number) => void;
}) {
  if (isLoading) {
    return (
      <Card className="flex min-h-72 items-center justify-center border-white/70 bg-white/80">
        <LoaderCircle className="h-6 w-6 animate-spin text-zinc-500" />
      </Card>
    );
  }
  if (isError) {
    return (
      <EmptyState
        title="任务列表加载失败"
        description="后端列表接口暂时不可用，可以刷新重试。"
        action={
          <Button variant="secondary" onClick={onRefresh}>
            <RefreshCw className="mr-2 h-4 w-4" />
            刷新
          </Button>
        }
      />
    );
  }
  if (items.length === 0) {
    return (
      <EmptyState
        title="暂无任务"
        description="先从 SHEIN Studio 创建一个批次，生成后会出现在这里。"
        action={
          <Link href="/listing-kits/sds" className={primaryLinkClass}>
            新建 POD 批次
          </Link>
        }
      />
    );
  }
  const totalPages = Math.max(1, Math.ceil(total / Math.max(pageSize, 1)));
  const startItem = total > 0 ? (page - 1) * pageSize + 1 : 0;
  const endItem = total > 0 ? Math.min(page * pageSize, total) : 0;

  return (
    <div className="grid gap-4">
      {items.map((task) => (
        <TaskRow key={task.task_id} task={task} taxonomy={taxonomy} />
      ))}
      {totalPages > 1 ? (
        <Card className="border-white/70 bg-white/82 p-4">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
            <div className="text-sm text-zinc-500">
              第 {page} / {totalPages} 页
              <span className="ml-2 text-zinc-400">
                {startItem}-{endItem} / {total}
              </span>
            </div>
            <div className="grid grid-cols-2 gap-2 sm:flex sm:items-center">
              <Button
                className="w-full sm:w-auto"
                variant="secondary"
                disabled={page <= 1}
                onClick={() => updatePage(page - 1)}
              >
                上一页
              </Button>
              <Button
                className="w-full sm:w-auto"
                variant="secondary"
                disabled={page >= totalPages}
                onClick={() => updatePage(page + 1)}
              >
                下一页
              </Button>
            </div>
          </div>
        </Card>
      ) : null}
    </div>
  );
}

function TaskRow({
  task,
  taxonomy,
}: {
  task: ListingKitTaskListItem;
  taxonomy?: ListingKitTaskListTaxonomy;
}) {
  const workspaceHref = `/listing-kits/${task.task_id}/workspace?platform=${task.platforms?.[0] ?? "shein"}`;
  const remoteCheckedAt = task.shein_submission_remote_checked_at
    ? formatDate(task.shein_submission_remote_checked_at)
    : null;
  const sheinOverview = task.shein_status_overview;
  const workQueueSeverity = taxonomySeverity(
    task.shein_work_queue,
    taxonomy?.shein_work_queues,
  );
  const actionQueueSeverity = taxonomySeverity(
    task.shein_action_queue,
    taxonomy?.shein_action_queues,
  );
  const podAuditTitle = podExecutionHistorySummary(task.pod_execution);

  return (
    <Card className="group border-white/70 bg-white/88 p-5 shadow-[0_16px_44px_rgba(39,39,42,0.07)] transition hover:-translate-y-0.5 hover:shadow-[0_22px_60px_rgba(39,39,42,0.11)]">
      <div className="grid gap-4 xl:grid-cols-[1fr_auto] xl:items-center">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <Badge
              className={`rounded-full px-2.5 py-1 text-[11px] uppercase tracking-[0.16em] ${statusTone(task.status)}`}
              variant="outline"
            >
              {taskLifecycleBadgeLabel(task.status)}
            </Badge>
            {hasActionablePodExecution(task.pod_execution) ? (
              <Badge
                className={`rounded-full px-2.5 py-1 text-[11px] uppercase tracking-[0.16em] ${podExecutionTone(task.pod_execution)}`}
                variant="outline"
                title={podAuditTitle || undefined}
              >
                {podExecutionBadgeLabel(task.pod_execution)}
              </Badge>
            ) : task.sds_sync_status ? (
              <Badge
                className="rounded-full border-teal-200 bg-teal-50 px-2.5 py-1 text-[11px] uppercase tracking-[0.16em] text-teal-700"
                variant="outline"
              >
                SDS {task.sds_sync_status}
              </Badge>
            ) : null}
            {hasPodPlatformIssue(task) ? (
              <Badge
                className="rounded-full border-sky-200 bg-sky-50 px-2.5 py-1 text-[11px] uppercase tracking-[0.16em] text-sky-700"
                variant="outline"
              >
                POD 平台待处理
              </Badge>
            ) : null}
            {hasActionableSheinFreshness(task) ? (
              <Badge
                className={`rounded-full px-2.5 py-1 text-[11px] uppercase tracking-[0.16em] ${sheinFreshnessTone(task)}`}
                variant="outline"
                title={sheinFreshnessSummaryText(task) || undefined}
              >
                {sheinFreshnessBadgeLabel(task)}
              </Badge>
            ) : null}
            {task.shein_workflow_status ? (
              <Badge
                className="rounded-full border-orange-200 bg-orange-50 px-2.5 py-1 text-[11px] uppercase tracking-[0.16em] text-orange-700"
                variant="outline"
              >
                {sheinWorkflowBadgeLabel(task.shein_workflow_status)}
              </Badge>
            ) : null}
            {task.shein_work_queue ? (
              <Badge
                className={`rounded-full px-2.5 py-1 text-[11px] ${queueTone(workQueueSeverity)}`}
                variant="outline"
              >
                {sheinWorkQueueLabel(task.shein_work_queue, taxonomy)}
              </Badge>
            ) : null}
            {task.shein_action_queue ? (
              <Badge
                className={`rounded-full px-2.5 py-1 text-[11px] ${queueTone(actionQueueSeverity)}`}
                variant="outline"
              >
                {sheinActionQueueLabel(task.shein_action_queue, taxonomy)}
              </Badge>
            ) : null}
            {task.shein_submission_remote_status ? (
              <Badge
                className="rounded-full border-sky-200 bg-sky-50 px-2.5 py-1 text-[11px] uppercase tracking-[0.16em] text-sky-700"
                variant="outline"
              >
                {sheinRemoteBadgeLabel(task.shein_submission_remote_status)}
              </Badge>
            ) : null}
            {(task.platforms ?? []).map((platform) => (
              <Badge
                key={platform}
                className="rounded-full px-2.5 py-1 text-[11px] uppercase tracking-[0.16em]"
                variant="neutral"
              >
                {platform}
              </Badge>
            ))}
          </div>
          <h2 className="mt-3 line-clamp-2 break-words text-xl font-semibold tracking-tight text-zinc-950">
            {taskTitle(task)}
          </h2>
          <p className="mt-1 text-xs font-medium uppercase tracking-[0.16em] text-zinc-400">
            任务 ID
          </p>
          <p className="mt-1 break-all text-sm text-zinc-500">{task.task_id}</p>
          {task.shein_store_id ? (
            <p className="mt-1 text-sm text-zinc-500" title={storeResolutionAuditTitle(task)}>
              SHEIN 店铺 {task.shein_store_id}
              {task.shein_store_site ? ` · ${task.shein_store_site}` : ""}
            </p>
          ) : null}
          {task.shein_store_profile_id || task.shein_store_resolved_at ? (
            <p className="mt-1 text-xs text-zinc-400" title={storeResolutionAuditTitle(task)}>
              {task.shein_store_strategy
                ? `路由 ${storeResolutionStrategyLabel(task.shein_store_strategy)}`
                : ""}
              {task.shein_store_strategy &&
              (task.shein_store_profile_id || task.shein_store_resolved_at)
                ? " · "
                : ""}
              {task.shein_store_profile_id
                ? `Profile #${task.shein_store_profile_id}`
                : ""}
              {(task.shein_store_strategy || task.shein_store_profile_id) &&
              task.shein_store_resolved_at
                ? " · "
                : ""}
              {task.shein_store_resolved_at
                ? `固化 ${formatDate(task.shein_store_resolved_at)}`
                : ""}
            </p>
          ) : null}
          {task.variant_label ? (
            <p className="mt-1 line-clamp-2 break-all text-sm text-zinc-500">
              {task.variant_label}
            </p>
          ) : null}
          {sheinOverview?.headline ? (
            <p className="mt-2 text-sm font-medium text-zinc-700">
              {sheinOverview.headline}
            </p>
          ) : null}
          {sheinOverview?.subheadline ? (
            <p className="mt-1 text-sm text-zinc-500">{sheinOverview.subheadline}</p>
          ) : null}
          {sheinOverview ? (
            <div className="mt-2 flex flex-wrap gap-2 text-xs text-zinc-500">
              {typeof sheinOverview.blocking_count === "number" ? (
                <span>阻断 {sheinOverview.blocking_count}</span>
              ) : null}
              {typeof sheinOverview.warning_count === "number" ? (
                <span>待确认 {sheinOverview.warning_count}</span>
              ) : null}
              {sheinOverview.primary_action ? (
                <span>下一步 {sheinOverview.primary_action}</span>
              ) : null}
            </div>
          ) : null}
          {task.error ? (
            <p className="mt-2 line-clamp-2 text-sm text-rose-600">
              {task.error}
            </p>
          ) : null}
          {task.shein_latest_submission_error ? (
            <p className="mt-2 line-clamp-2 text-sm text-rose-600">
              最近提交：发布失败。原始错误：
              {task.shein_latest_submission_error}
            </p>
          ) : task.shein_latest_submission_status ? (
            <p className="mt-2 text-sm text-zinc-500">
              最近提交：
              {sheinSubmissionStatusLabel(task.shein_latest_submission_status)}
            </p>
          ) : null}
          {task.shein_submission_remote_status ? (
            <p className="mt-1 text-sm text-zinc-500">
              SHEIN 远端：
              {sheinSubmissionRemoteStatusLabel(
                task.shein_submission_remote_status,
              )}
              {task.shein_submission_remote_record_id
                ? ` · ${task.shein_submission_remote_record_id}`
                : ""}
              {remoteCheckedAt ? ` · ${remoteCheckedAt}` : ""}
            </p>
          ) : null}
        </div>

        <div className="flex flex-col items-stretch gap-3 sm:flex-row sm:flex-wrap sm:items-center xl:justify-end">
          <div className="w-full rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-600 sm:w-auto">
            <div className="flex items-center gap-2">
              <Clock className="h-4 w-4" />
              创建于 {formatDate(task.created_at)}
            </div>
            <div className="mt-1 text-xs text-zinc-500">
              最近更新 {formatDate(task.updated_at ?? task.created_at)}
            </div>
            {task.completed_at ? (
              <div className="mt-1 text-xs text-zinc-500">
                完成时间 {formatDate(task.completed_at)}
              </div>
            ) : null}
            <div className="mt-1 text-xs text-zinc-500">
              {task.image_count ?? 0} 张图片
            </div>
          </div>
          <Link
            href={`/listing-kits/${task.task_id}/status`}
            className={`${secondaryLinkClass} w-full sm:w-auto`}
          >
            状态
          </Link>
          <Link href={workspaceHref} className={`${primaryLinkClass} w-full sm:w-auto`}>
            工作台
            <ArrowRight className="ml-2 h-4 w-4 transition group-hover:translate-x-0.5" />
          </Link>
        </div>
      </div>
    </Card>
  );
}
