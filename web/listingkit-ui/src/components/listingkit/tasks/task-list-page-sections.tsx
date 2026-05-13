import Link from "next/link";
import { ArrowRight, Boxes, Clock, LoaderCircle, Plus, RefreshCw } from "lucide-react";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
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
import type {
  ListingKitTaskListItem,
  ListingKitTaskListSummary,
  ListingKitTaskListTaxonomy,
} from "@/lib/types/listingkit";

type FilterKey =
  | "status"
  | "platform"
  | "shein_workflow_status"
  | "shein_work_queue"
  | "shein_action_queue"
  | "shein_blocker_key"
  | "shein_warning_key";

export function TaskListHero({ onRefresh }: { onRefresh: () => void }) {
  return (
    <section className="grid gap-5 rounded-[2rem] border border-white/70 bg-white/78 p-6 shadow-[0_24px_90px_rgba(39,39,42,0.10)] backdrop-blur lg:grid-cols-[1fr_auto] lg:items-end">
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
      <div className="flex flex-wrap gap-3">
        <Button tone="secondary" onClick={onRefresh}>
          <RefreshCw className="mr-2 h-4 w-4" />
          刷新
        </Button>
        <Link href="/listing-kits/shein" className={primaryLinkClass}>
          <Plus className="mr-2 h-4 w-4" />
          新建 SHEIN 批次
        </Link>
      </div>
    </section>
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
      <div className="flex flex-wrap gap-3">
        <select
          className="h-11 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-800 shadow-sm outline-none focus:border-zinc-400"
          value={status}
          onChange={(event) => updateFilter("status", event.target.value)}
        >
          {STATUS_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <select
          className="h-11 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-800 shadow-sm outline-none focus:border-zinc-400"
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
        </select>
        <select
          className="h-11 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-800 shadow-sm outline-none focus:border-zinc-400"
          value={sheinWorkQueue}
          onChange={(event) => updateFilter("shein_work_queue", event.target.value)}
        >
          {workQueueOptions.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <select
          className="h-11 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-800 shadow-sm outline-none focus:border-zinc-400"
          value={sheinActionQueue}
          onChange={(event) => updateFilter("shein_action_queue", event.target.value)}
        >
          {actionQueueOptions.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        <select
          className="h-11 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-800 shadow-sm outline-none focus:border-zinc-400"
          value={platform}
          onChange={(event) => updateFilter("platform", event.target.value)}
        >
          {PLATFORM_OPTIONS.map((option) => (
            <option key={option.value} value={option.value}>
              {option.label}
            </option>
          ))}
        </select>
        {blockerOptions.length > 1 ? (
          <select
            className="h-11 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-800 shadow-sm outline-none focus:border-zinc-400"
            value={sheinBlockerKey}
            onChange={(event) => updateFilter("shein_blocker_key", event.target.value)}
          >
            {blockerOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        ) : null}
        {warningOptions.length > 1 ? (
          <select
            className="h-11 rounded-2xl border border-zinc-200 bg-white px-4 text-sm text-zinc-800 shadow-sm outline-none focus:border-zinc-400"
            value={sheinWarningKey}
            onChange={(event) => updateFilter("shein_warning_key", event.target.value)}
          >
            {warningOptions.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        ) : null}
        <div className="ml-auto flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-zinc-500">
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
            <button
              key={filter.key}
              type="button"
              onClick={() => updateFilter(filter.key, "")}
              className="rounded-full border border-zinc-200 bg-zinc-50 px-3 py-1 text-xs font-medium text-zinc-700 transition hover:border-zinc-950 hover:text-zinc-950"
            >
              {filter.label}
            </button>
          ))}
          {activeFilters.length > 1 ? (
            <button
              type="button"
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
              className="text-xs font-medium text-zinc-500 transition hover:text-zinc-900"
            >
              清空全部
            </button>
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
                  <button
                    type="button"
                    onClick={() =>
                      updateFilter(section.filterKey, "")
                    }
                    className="text-[11px] font-medium text-zinc-500 transition hover:text-zinc-900"
                  >
                    清除
                  </button>
                ) : null}
              </div>
              <div className="flex flex-wrap gap-2">
                {section.entries.map((entry) => {
                  const active =
                    activeFacetValueByKey[section.filterKey] === entry.key;
                  return (
                    <button
                      key={entry.key}
                      type="button"
                      onClick={() =>
                        applySummaryFilter(section.filterKey, entry.key)
                      }
                      aria-pressed={active}
                      className={`rounded-full border px-3 py-1 text-xs font-semibold transition ${
                        active
                          ? "border-zinc-950 bg-zinc-950 text-white"
                          : queueTone(entry.severity)
                      }`}
                    >
                      {entry.label} · {entry.count}
                    </button>
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
          <Button tone="secondary" onClick={onRefresh}>
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
          <Link href="/listing-kits/shein" className={primaryLinkClass}>
            新建 SHEIN 批次
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
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div className="text-sm text-zinc-500">
              第 {page} / {totalPages} 页
              <span className="ml-2 text-zinc-400">
                {startItem}-{endItem} / {total}
              </span>
            </div>
            <div className="flex items-center gap-2">
              <Button
                tone="secondary"
                disabled={page <= 1}
                onClick={() => updatePage(page - 1)}
              >
                上一页
              </Button>
              <Button
                tone="secondary"
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

  return (
    <Card className="group border-white/70 bg-white/88 p-5 shadow-[0_16px_44px_rgba(39,39,42,0.07)] transition hover:-translate-y-0.5 hover:shadow-[0_22px_60px_rgba(39,39,42,0.11)]">
      <div className="grid gap-4 lg:grid-cols-[1fr_auto] lg:items-center">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span
              className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] ${statusTone(task.status)}`}
            >
              {taskStatusLabel(task.status)}
            </span>
            {task.sds_sync_status ? (
              <span className="rounded-full border border-teal-200 bg-teal-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-teal-700">
                SDS {task.sds_sync_status}
              </span>
            ) : null}
            {task.shein_workflow_status ? (
              <span className="rounded-full border border-orange-200 bg-orange-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-orange-700">
                {sheinWorkflowStatusLabel(task.shein_workflow_status)}
              </span>
            ) : null}
            {task.shein_work_queue ? (
              <span
                className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold ${queueTone(workQueueSeverity)}`}
              >
                {sheinWorkQueueLabel(task.shein_work_queue, taxonomy)}
              </span>
            ) : null}
            {task.shein_action_queue ? (
              <span
                className={`rounded-full border px-2.5 py-1 text-[11px] font-semibold ${queueTone(actionQueueSeverity)}`}
              >
                {sheinActionQueueLabel(task.shein_action_queue, taxonomy)}
              </span>
            ) : null}
            {task.shein_submission_remote_status ? (
              <span className="rounded-full border border-sky-200 bg-sky-50 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-sky-700">
                {sheinSubmissionRemoteStatusLabel(
                  task.shein_submission_remote_status,
                )}
              </span>
            ) : null}
            {(task.platforms ?? []).map((platform) => (
              <span
                key={platform}
                className="rounded-full bg-zinc-100 px-2.5 py-1 text-[11px] font-semibold uppercase tracking-[0.16em] text-zinc-600"
              >
                {platform}
              </span>
            ))}
          </div>
          <h2 className="mt-3 truncate text-xl font-semibold tracking-tight text-zinc-950">
            {taskTitle(task)}
          </h2>
          <p className="mt-1 text-xs font-medium uppercase tracking-[0.16em] text-zinc-400">
            任务 ID
          </p>
          <p className="mt-1 break-all text-sm text-zinc-500">{task.task_id}</p>
          {task.variant_label ? (
            <p className="mt-1 truncate text-sm text-zinc-500">
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

        <div className="flex flex-wrap items-center gap-3 lg:justify-end">
          <div className="rounded-2xl border border-zinc-200 bg-zinc-50 px-4 py-3 text-sm text-zinc-600">
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
            className={secondaryLinkClass}
          >
            状态
          </Link>
          <Link href={workspaceHref} className={primaryLinkClass}>
            工作台
            <ArrowRight className="ml-2 h-4 w-4 transition group-hover:translate-x-0.5" />
          </Link>
        </div>
      </div>
    </Card>
  );
}
