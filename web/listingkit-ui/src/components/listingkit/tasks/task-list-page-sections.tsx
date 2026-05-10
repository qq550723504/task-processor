import Link from "next/link";
import { ArrowRight, Boxes, Clock, LoaderCircle, Plus, RefreshCw } from "lucide-react";

import { Button } from "@/components/shared/button";
import { Card } from "@/components/shared/card";
import { EmptyState } from "@/components/shared/empty-state";
import {
  formatDate,
  PLATFORM_OPTIONS,
  primaryLinkClass,
  secondaryLinkClass,
  SHEIN_WORKFLOW_OPTIONS,
  STATUS_OPTIONS,
  statusTone,
  taskStatusLabel,
  taskTitle,
} from "@/components/listingkit/tasks/task-list-page-model";
import {
  sheinSubmissionRemoteStatusLabel,
  sheinSubmissionStatusLabel,
  sheinWorkflowStatusLabel,
} from "@/lib/shein-studio/shein-submission-display";
import type { ListingKitTaskListItem } from "@/lib/types/listingkit";

type FilterKey = "status" | "platform" | "shein_workflow_status";

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
  sheinWorkflowStatus,
  status,
  total,
  updateFilter,
}: {
  platform: string;
  sheinWorkflowStatus: string;
  status: string;
  total: number;
  updateFilter: (key: FilterKey, value: string) => void;
}) {
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
          {SHEIN_WORKFLOW_OPTIONS.map((option) => (
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
        <div className="ml-auto flex items-center gap-2 text-xs font-medium uppercase tracking-[0.18em] text-zinc-500">
          <Boxes className="h-4 w-4" />
          {total} 个任务
        </div>
      </div>
    </Card>
  );
}

export function TaskListContent({
  isError,
  isLoading,
  items,
  onRefresh,
}: {
  isError: boolean;
  isLoading: boolean;
  items: ListingKitTaskListItem[];
  onRefresh: () => void;
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
  return (
    <div className="grid gap-4">
      {items.map((task) => (
        <TaskRow key={task.task_id} task={task} />
      ))}
    </div>
  );
}

function TaskRow({ task }: { task: ListingKitTaskListItem }) {
  const workspaceHref = `/listing-kits/${task.task_id}/workspace?platform=${task.platforms?.[0] ?? "shein"}`;
  const remoteCheckedAt = task.shein_submission_remote_checked_at
    ? formatDate(task.shein_submission_remote_checked_at)
    : null;

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
