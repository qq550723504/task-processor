import Link from "next/link";
import { LoaderCircle } from "lucide-react";

import { Button } from "@/components/ui/button";
import { Label } from "@/components/ui/label";
import { Select } from "@/components/ui/select";
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
        <div className="flex flex-col gap-3 sm:flex-row sm:flex-wrap">
          <Button asChild variant="outline">
            <Link href={`/listing-kits/${taskId}/workspace?platform=shein&section_key=general_review`}>
              打开工作台
            </Link>
          </Button>
          <Button asChild variant="outline">
            <Link href="/listing-kits">返回任务列表</Link>
          </Button>
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
        <Button asChild variant="outline">
          <Link href={`/listing-kits/${taskId}/workspace?platform=shein&section_key=general_review`}>
            打开工作台
          </Link>
        </Button>
      }
    />
  );
}

export function QueueTaskNavigation({ taskId }: { taskId: string }) {
  return (
    <div className="flex flex-col gap-3 rounded-2xl border border-zinc-200 bg-white px-4 py-3 shadow-sm sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
      <div>
        <p className="text-xs font-semibold uppercase tracking-[0.22em] text-zinc-500">
          任务导航
        </p>
        <p className="mt-1 text-sm text-zinc-600">
          队列用于处理生成/审核项；资料确认和提交请回到工作区。
        </p>
      </div>
      <div className="flex flex-col gap-2 sm:flex-row sm:flex-wrap">
        <Button asChild variant="outline" size="sm">
          <Link href={`/listing-kits/${taskId}/workspace?platform=shein&section_key=general_review`}>
            打开工作区
          </Link>
        </Button>
        <Button asChild variant="outline" size="sm">
          <Link href="/listing-kits/sds">返回 POD 工作室</Link>
        </Button>
      </div>
    </div>
  );
}

function canBulkRecoverBlockedTasks(task?: ListingKitTaskResult) {
  return task?.status === "blocked_retryable";
}

export function QueueScreenBody({
  filters,
  onAction,
  onApplyFilters,
  onBulkRecoverBlockedTasks,
  onChangePage,
  onExecuteAction,
  bulkRecovering,
  onSelectNavigation,
  onSelectRecovery,
  queueData,
  taskId,
  taskResult,
}: {
  filters: QueueFilterValue;
  onAction: (item: QueueItem) => void;
  onApplyFilters: (nextFilters: QueueFilterValue) => void;
  onBulkRecoverBlockedTasks?: () => void;
  onChangePage: (page: number, pageSize?: number) => void;
  bulkRecovering?: boolean;
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
      <div className="flex flex-col gap-3">
        <QueueTaskNavigation taskId={taskId} />
        {canBulkRecoverBlockedTasks(taskResult) ? (
          <div className="flex flex-col gap-3 rounded-2xl border border-amber-200 bg-amber-50/70 px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
            <div>
              <p className="text-sm font-medium text-amber-900">检测到可恢复阻塞任务</p>
              <p className="mt-1 text-sm text-amber-800">
                如果依赖已经恢复，可以批量恢复当前已到重试时间的阻塞任务。
              </p>
            </div>
            <Button
              className="w-full sm:w-auto"
              disabled={!onBulkRecoverBlockedTasks || bulkRecovering}
              onClick={onBulkRecoverBlockedTasks}
              type="button"
              variant="secondary"
            >
              {bulkRecovering ? "恢复中..." : "批量恢复到期任务"}
            </Button>
          </div>
        ) : null}
      </div>
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
      <ReviewReasonsCard task={taskResult} taskId={taskId} />
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
    <div className="flex flex-col gap-3 rounded-2xl border border-zinc-200 bg-white px-4 py-3 text-sm text-zinc-600 sm:flex-row sm:flex-wrap sm:items-center sm:justify-between">
      <div>
        第 {page} / {totalPages} 页 · 显示 {start}-{end} / {total} 条
      </div>
      <div className="grid gap-2 sm:flex sm:flex-wrap sm:items-center">
        <Label className="flex items-center gap-2">
          <span>每页</span>
          <Select
            className="h-9 w-full rounded-xl px-2 text-sm sm:w-auto"
            value={pageSize}
            onChange={(event) => onChange(1, Number(event.target.value))}
          >
            <option value={10}>10</option>
            <option value={20}>20</option>
            <option value={50}>50</option>
          </Select>
        </Label>
        <Button
          className="w-full sm:w-auto"
          variant="outline"
          size="sm"
          disabled={page <= 1}
          onClick={() => onChange(page - 1)}
          type="button"
        >
          上一页
        </Button>
        <Button
          className="w-full sm:w-auto"
          variant="outline"
          size="sm"
          disabled={page >= totalPages}
          onClick={() => onChange(page + 1)}
          type="button"
        >
          下一页
        </Button>
      </div>
    </div>
  );
}
