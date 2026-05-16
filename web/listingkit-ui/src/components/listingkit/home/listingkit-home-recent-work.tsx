import Link from "next/link";

import {
  ListingKitHomeTaskCard,
  taskWorkspaceHref,
} from "@/components/listingkit/home/listingkit-home-task-card";
import {
  buildHomeActionQueueSummaryEntries,
  buildHomeWorkQueueSummaryEntries,
  queueTone,
  sheinActionQueueLabel,
  sheinWorkQueueLabel,
  taxonomySeverity,
} from "@/components/listingkit/tasks/task-list-page-model";
import { EmptyState } from "@/components/shared/empty-state";
import { Skeleton } from "@/components/ui/skeleton";
import {
  pickContinueTask,
  sortRecentTasksForHomepage,
} from "@/lib/listingkit/home-recent-tasks";
import type {
  ListingKitTaskListItem,
  ListingKitTaskListSummary,
  ListingKitTaskListTaxonomy,
} from "@/lib/types/listingkit/tasks";

type ListingKitHomeRecentWorkProps = {
  tasks: ListingKitTaskListItem[];
  isLoading: boolean;
  isError: boolean;
  summary?: ListingKitTaskListSummary;
  taxonomy?: ListingKitTaskListTaxonomy;
};

function continueTaskTitle(task: ListingKitTaskListItem) {
  const source = task.product_name || task.title || task.task_id;
  const compact = source.replace(/\s+/g, " ").trim();

  if (compact.length <= 120) {
    return compact;
  }

  const firstSentence = compact.split(/(?<=[.!?。；;])\s+/)[0]?.trim();
  if (firstSentence && firstSentence.length >= 24) {
    return firstSentence.slice(0, 120).trim();
  }

  return compact.slice(0, 120).trim();
}

function continueTaskSummary(task: ListingKitTaskListItem) {
  return (
    task.shein_status_overview?.subheadline ||
    task.variant_label ||
    task.task_id
  );
}

function sheinListHref(paramKey: string, value: string) {
  return `/listing-kits?platform=shein&${paramKey}=${value}`;
}

function continueTaskListHref(task: ListingKitTaskListItem) {
  if (task.shein_action_queue) {
    return sheinListHref("shein_action_queue", task.shein_action_queue);
  }
  if (task.shein_work_queue) {
    return sheinListHref("shein_work_queue", task.shein_work_queue);
  }
  if (task.shein_workflow_status) {
    return sheinListHref("shein_workflow_status", task.shein_workflow_status);
  }
  return "/listing-kits";
}

function summaryTileTone(severity?: string) {
  switch (severity) {
    case "positive":
      return "border-emerald-200 bg-emerald-50 text-emerald-900";
    case "warning":
      return "border-amber-200 bg-amber-50 text-amber-900";
    case "negative":
      return "border-rose-200 bg-rose-50 text-rose-900";
    default:
      return "border-slate-200 bg-white text-slate-900";
  }
}

function QueueSummaryBlock({
  title,
  entries,
  filterKey,
}: {
  title: string;
  entries: Array<{ key: string; label: string; count: number; severity?: string }>;
  filterKey: "shein_work_queue" | "shein_action_queue";
}) {
  if (!entries.length) {
    return null;
  }

  return (
    <div className="rounded-2xl border border-slate-200 bg-white/88 p-4 shadow-[0_14px_36px_rgba(39,39,42,0.05)]">
      <p className="text-[11px] font-bold uppercase tracking-[0.2em] text-slate-500">
        {title}
      </p>
      <div className="mt-3 grid gap-2 sm:grid-cols-2">
        {entries.map((entry) => (
          <Link
            key={entry.key}
            href={sheinListHref(filterKey, entry.key)}
            className={`grid min-h-[74px] content-between rounded-xl border px-3.5 py-3 transition-colors hover:border-slate-300 ${summaryTileTone(entry.severity)}`}
          >
            <span className="text-[11px] font-semibold uppercase tracking-[0.14em] opacity-70">
              {entry.label}
            </span>
            <span className="text-2xl font-semibold leading-none">{entry.count}</span>
          </Link>
        ))}
      </div>
    </div>
  );
}

export function ListingKitHomeRecentWork({
  tasks,
  isLoading,
  isError,
  summary,
  taxonomy,
}: ListingKitHomeRecentWorkProps) {
  if (isLoading) {
    return (
      <section
        role="status"
        aria-label="最近任务加载中"
        className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]"
      >
        <Skeleton className="h-44 rounded-2xl border border-slate-200 bg-white/80" />
        <div className="grid gap-4">
          <Skeleton className="h-32 rounded-2xl border border-slate-200 bg-white/80" />
          <Skeleton className="h-32 rounded-2xl border border-slate-200 bg-white/80" />
        </div>
      </section>
    );
  }

  if (isError && !tasks.length) {
    return (
      <section className="rounded-2xl border border-amber-200 bg-amber-50/80 p-6 text-sm text-amber-800">
        <p className="font-semibold">最近任务暂时加载失败</p>
        <p className="mt-2">你仍然可以直接进入 SHEIN 工作台或新建任务。</p>
      </section>
    );
  }

  if (!tasks.length) {
    return (
      <EmptyState
        title="还没有最近任务"
        description="从 SHEIN 工作台或通用任务入口开始，新的联调和正式任务会显示在这里。"
      />
    );
  }

  const continueTask = pickContinueTask(tasks);
  const recentTasks = sortRecentTasksForHomepage(tasks);
  const summaryEntries = buildHomeWorkQueueSummaryEntries(
    summary?.shein_work_queue_counts,
    taxonomy?.shein_work_queues,
  ).slice(0, 4);
  const shouldShowActionSummary =
    Boolean(summary?.shein_work_queue_counts?.repair_queue) ||
    Boolean(summary?.shein_work_queue_counts?.review_queue);
  const actionSummaryEntries = shouldShowActionSummary
    ? buildHomeActionQueueSummaryEntries(
        summary?.shein_action_queue_counts,
        taxonomy?.shein_action_queues,
      ).slice(0, 4)
    : [];
  const continueWorkQueueSeverity = taxonomySeverity(
    continueTask?.shein_work_queue,
    taxonomy?.shein_work_queues,
  );
  const continueActionQueueSeverity = taxonomySeverity(
    continueTask?.shein_action_queue,
    taxonomy?.shein_action_queues,
  );

  return (
    <section className="grid gap-5">
      {summaryEntries.length || actionSummaryEntries.length ? (
        <div className="grid gap-4 lg:grid-cols-2">
          <QueueSummaryBlock
            title="工作队列"
            entries={summaryEntries}
            filterKey="shein_work_queue"
          />
          <QueueSummaryBlock
            title="处理动作"
            entries={actionSummaryEntries}
            filterKey="shein_action_queue"
          />
        </div>
      ) : null}

      <div className="grid gap-4 lg:grid-cols-[1.08fr_0.92fr]">
        {continueTask ? (
          <div className="rounded-2xl border border-slate-900/10 bg-gradient-to-br from-slate-900 to-slate-950 p-7 text-white shadow-[0_20px_60px_rgba(15,23,42,0.35)]">
            <p className="text-xs font-bold uppercase tracking-[0.3em] text-blue-300">
              Continue
            </p>
            <h2 className="mt-4 text-2xl font-bold">继续最近任务</h2>
            <p className="mt-3 line-clamp-2 max-w-3xl text-base font-medium leading-8 text-slate-100">
              {continueTaskTitle(continueTask)}
            </p>
            <p className="mt-2 line-clamp-2 max-w-xl text-sm leading-6 text-slate-400">
              {continueTaskSummary(continueTask)}
            </p>
            <div className="mt-5 flex flex-wrap gap-2">
              {continueTask.shein_work_queue ? (
                <span
                  className={`rounded-full border px-3 py-1.5 text-[11px] font-semibold ${queueTone(continueWorkQueueSeverity)}`}
                >
                  {sheinWorkQueueLabel(continueTask.shein_work_queue, taxonomy)}
                </span>
              ) : null}
              {continueTask.shein_action_queue ? (
                <span
                  className={`rounded-full border px-3 py-1.5 text-[11px] font-semibold ${queueTone(continueActionQueueSeverity)}`}
                >
                  {sheinActionQueueLabel(continueTask.shein_action_queue, taxonomy)}
                </span>
              ) : null}
            </div>
            {continueTask.shein_status_overview ? (
              <div className="mt-5 rounded-xl border border-white/10 bg-white/5 px-4 py-4">
                <div className="grid gap-3 sm:grid-cols-[auto_1fr] sm:gap-x-6">
                  {typeof continueTask.shein_status_overview.blocking_count === "number" ? (
                    <>
                      <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-400">
                        阻断
                      </div>
                      <div className="text-sm font-semibold text-white">
                        {continueTask.shein_status_overview.blocking_count}
                      </div>
                    </>
                  ) : null}
                  {typeof continueTask.shein_status_overview.warning_count === "number" ? (
                    <>
                      <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-400">
                        待确认
                      </div>
                      <div className="text-sm font-semibold text-white">
                        {continueTask.shein_status_overview.warning_count}
                      </div>
                    </>
                  ) : null}
                  {continueTask.shein_status_overview.primary_action ? (
                    <>
                      <div className="text-[11px] font-semibold uppercase tracking-[0.16em] text-slate-400">
                        下一步
                      </div>
                      <div className="text-sm font-semibold text-white">
                        {continueTask.shein_status_overview.primary_action}
                      </div>
                    </>
                  ) : null}
                </div>
              </div>
            ) : null}
            <div className="mt-7 flex flex-wrap gap-3">
              <Link
                href={taskWorkspaceHref(continueTask)}
                className="inline-flex h-12 items-center justify-center rounded-xl bg-white px-7 text-sm font-semibold text-slate-900 transition-all hover:scale-105 hover:bg-slate-100 shadow-lg"
              >
                继续最近任务
              </Link>
              <Link
                href={continueTaskListHref(continueTask)}
                className="inline-flex h-12 items-center justify-center rounded-xl border border-white/20 px-7 text-sm font-semibold text-white transition-all hover:scale-105 hover:bg-white/10"
              >
                查看同队列
              </Link>
            </div>
          </div>
        ) : null}

        <div className="rounded-2xl border border-slate-200 bg-white/88 p-3 shadow-[0_14px_36px_rgba(39,39,42,0.05)]">
          <div className="flex items-center justify-between px-2 py-2">
            <p className="text-[11px] font-bold uppercase tracking-[0.2em] text-slate-500">
              最近任务
            </p>
            <Link
              href="/listing-kits"
              className="text-xs font-semibold text-slate-500 transition-colors hover:text-slate-900"
            >
              查看全部
            </Link>
          </div>
          <div className="grid gap-3">
            {isError ? (
              <div className="rounded-xl border border-amber-200 bg-amber-50/80 p-4 text-sm text-amber-800">
                <p className="font-semibold">最近任务暂时加载失败</p>
                <p className="mt-2">以下为上次成功加载的最近任务。</p>
              </div>
            ) : null}
            {recentTasks.map((task) => (
              <ListingKitHomeTaskCard
                key={task.task_id}
                task={task}
                taxonomy={taxonomy}
              />
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
