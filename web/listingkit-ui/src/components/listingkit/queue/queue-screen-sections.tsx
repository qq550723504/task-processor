import Link from "next/link";
import { LoaderCircle } from "lucide-react";

import { QueueFiltersBar, type QueueFilterValue } from "@/components/listingkit/queue/queue-filters-bar";
import { QueueSummaryStrip } from "@/components/listingkit/queue/queue-summary-strip";
import { QueueTable } from "@/components/listingkit/queue/queue-table";
import { ReviewReasonsCard } from "@/components/listingkit/review/review-reasons-card";
import { TaskProgressNotice } from "@/components/listingkit/tasks/task-progress-notice";
import {
  deriveTaskQueueEmptyState,
  shouldSuppressResolvedActionSummary,
} from "@/components/listingkit/tasks/task-status-display";
import { TaskStatusPanel } from "@/components/listingkit/tasks/task-status-panel";
import { WorkspaceHeader } from "@/components/listingkit/workspace/workspace-header";
import { EmptyState } from "@/components/shared/empty-state";
import type {
  ActionExecutionRequest,
  ListingKitTaskResult,
  QueueItem,
  QueuePage,
  RecoveryDescriptor,
  NavigationTarget,
  ResolvedActionSummary,
} from "@/lib/types/listingkit";

export function QueueLoadingState() {
  return (
    <div className="flex min-h-[60vh] items-center justify-center">
      <LoaderCircle className="h-8 w-8 animate-spin text-zinc-400" />
    </div>
  );
}

export function QueueErrorState({ taskId }: { taskId: string }) {
  return (
    <EmptyState
      title="队列暂时无法加载"
      description="当前无法读取生成队列或任务状态。你可以返回工作台继续查看，或回到任务列表稍后重试。"
      action={
        <div className="flex flex-wrap gap-3">
          <Link
            className="inline-flex h-10 items-center justify-center rounded-xl border border-zinc-200 bg-white px-4 text-sm font-medium text-zinc-800 hover:bg-zinc-50"
            href={`/listing-kits/${taskId}/workspace?platform=shein&section_key=general_review`}
          >
            打开工作台
          </Link>
          <Link
            className="inline-flex h-10 items-center justify-center rounded-xl border border-zinc-200 bg-white px-4 text-sm font-medium text-zinc-800 hover:bg-zinc-50"
            href="/listing-kits"
          >
            返回任务列表
          </Link>
        </div>
      }
    />
  );
}

export function QueuePendingDataState({ taskId }: { taskId: string }) {
  return (
    <EmptyState
      title="队列数据暂未准备完成"
      description="当前任务还没有返回完整的生成队列。你可以先打开工作台查看处理进度，或稍后回到这里继续。"
      action={
        <Link
          className="inline-flex h-10 items-center justify-center rounded-xl border border-zinc-200 bg-white px-4 text-sm font-medium text-zinc-800 hover:bg-zinc-50"
          href={`/listing-kits/${taskId}/workspace?platform=shein&section_key=general_review`}
        >
          打开工作台
        </Link>
      }
    />
  );
}

export function QueueTaskNavigation({ taskId }: { taskId: string }) {
  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-zinc-200 bg-white px-4 py-3 shadow-sm">
      <div>
        <p className="text-xs font-semibold uppercase tracking-[0.22em] text-zinc-500">
          任务导航
        </p>
        <p className="mt-1 text-sm text-zinc-600">
          队列用于处理生成/审核项；资料确认和提交请回到工作区。
        </p>
      </div>
      <div className="flex flex-wrap gap-2">
        <Link
          className="inline-flex h-9 items-center justify-center rounded-xl border border-zinc-200 bg-white px-3 text-sm font-medium text-zinc-800 hover:bg-zinc-50"
          href={`/listing-kits/${taskId}/workspace?platform=shein&section_key=general_review`}
        >
          打开工作区
        </Link>
        <Link
          className="inline-flex h-9 items-center justify-center rounded-xl border border-zinc-200 bg-white px-3 text-sm font-medium text-zinc-800 hover:bg-zinc-50"
          href="/listing-kits/shein"
        >
          返回 SHEIN 工作室
        </Link>
      </div>
    </div>
  );
}

export function QueueScreenBody({
  filters,
  onAction,
  onApplyFilters,
  onChangePage,
  onExecuteAction,
  onSelectNavigation,
  onSelectRecovery,
  queueData,
  taskId,
  taskResult,
}: {
  filters: QueueFilterValue;
  onAction: (item: QueueItem) => void;
  onApplyFilters: (nextFilters: QueueFilterValue) => void;
  onChangePage: (page: number, pageSize?: number) => void;
  onExecuteAction: (request: ActionExecutionRequest) => void;
  onSelectRecovery: (descriptor: RecoveryDescriptor) => void;
  onSelectNavigation: (target?: NavigationTarget | null) => void;
  queueData: QueuePage;
  taskId: string;
  taskResult?: ListingKitTaskResult;
}) {
  const queueEmptyState = deriveTaskQueueEmptyState(taskResult);
  const suppressResolvedActionSummary = shouldSuppressResolvedActionSummary(
    taskResult,
    {
      hasPreviewSvg: false,
      queueTotal: queueData.total ?? 0,
    },
  );

  return (
    <div className="space-y-6">
      <QueueTaskNavigation taskId={taskId} />
      <WorkspaceHeader
        title={`任务队列 ${taskId}`}
        summary={
          suppressResolvedActionSummary
            ? undefined
            : queueData.resolved_action_summary
        }
        recoverySummary={queueData.recovery_summary}
        onSelectAction={(summary: ResolvedActionSummary) => {
          if (summary.action_target || summary.action_key) {
            onExecuteAction({
              action_key: summary.action_key,
              response_mode: "patch_only",
              target: summary.action_target,
            });
            return;
          }
          onSelectNavigation(summary.navigation_target);
        }}
        onSelectRecovery={onSelectRecovery}
      />
      <TaskStatusPanel task={taskResult} />
      <ReviewReasonsCard task={taskResult} />
      <TaskProgressNotice task={taskResult} />
      <QueueSummaryStrip summary={queueData.summary} />
      <QueueFiltersBar value={filters} onApply={onApplyFilters} />
      {queueData.total === 0 && queueEmptyState ? (
        <EmptyState
          title={queueEmptyState.title}
          description={queueEmptyState.description}
        />
      ) : (
        <>
          <QueuePagination
            page={queueData.page}
            pageSize={queueData.page_size}
            total={queueData.total ?? 0}
            onChange={onChangePage}
          />
          <QueueTable items={queueData.items} onAction={onAction} />
          <QueuePagination
            page={queueData.page}
            pageSize={queueData.page_size}
            total={queueData.total ?? 0}
            onChange={onChangePage}
          />
        </>
      )}
    </div>
  );
}

function QueuePagination({
  page,
  pageSize,
  total,
  onChange,
}: {
  page: number;
  pageSize: number;
  total: number;
  onChange: (page: number, pageSize?: number) => void;
}) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  const start = total === 0 ? 0 : (page - 1) * pageSize + 1;
  const end = Math.min(total, page * pageSize);

  return (
    <div className="flex flex-wrap items-center justify-between gap-3 rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-600">
      <div>
        第 {page} / {totalPages} 页 · 显示 {start}-{end} / {total} 条
      </div>
      <div className="flex flex-wrap items-center gap-2">
        <label className="flex items-center gap-2">
          <span>每页</span>
          <select
            className="h-9 rounded-xl border border-zinc-200 bg-white px-2 text-sm"
            value={pageSize}
            onChange={(event) => onChange(1, Number(event.target.value))}
          >
            <option value={10}>10</option>
            <option value={20}>20</option>
            <option value={50}>50</option>
          </select>
        </label>
        <button
          className="h-9 rounded-xl border border-zinc-200 px-3 font-medium text-zinc-800 disabled:text-zinc-300"
          disabled={page <= 1}
          onClick={() => onChange(page - 1)}
          type="button"
        >
          上一页
        </button>
        <button
          className="h-9 rounded-xl border border-zinc-200 px-3 font-medium text-zinc-800 disabled:text-zinc-300"
          disabled={page >= totalPages}
          onClick={() => onChange(page + 1)}
          type="button"
        >
          下一页
        </button>
      </div>
    </div>
  );
}
