"use client";

import { useMemo, useState } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { LoaderCircle } from "lucide-react";

import {
  QueueFiltersBar,
  type QueueFilterValue,
} from "@/components/listingkit/queue/queue-filters-bar";
import { deriveQueueItemAction } from "@/components/listingkit/queue/queue-actions";
import { deriveWorkspaceTargetFromNavigationTarget } from "@/components/listingkit/queue/queue-routing";
import {
  deriveTaskQueueEmptyState,
  shouldSuppressResolvedActionSummary,
} from "@/components/listingkit/tasks/task-status-display";
import { QueueSummaryStrip } from "@/components/listingkit/queue/queue-summary-strip";
import { QueueTable } from "@/components/listingkit/queue/queue-table";
import { ReviewReasonsCard } from "@/components/listingkit/review/review-reasons-card";
import { TaskProgressNotice } from "@/components/listingkit/tasks/task-progress-notice";
import { WorkspaceHeader } from "@/components/listingkit/workspace/workspace-header";
import { EmptyState } from "@/components/shared/empty-state";
import { useExecuteAction } from "@/lib/query/use-action";
import { useDispatchNavigation } from "@/lib/query/use-dispatch";
import { useGenerationQueue } from "@/lib/query/use-queue";
import { useListingKitTaskResult } from "@/lib/query/use-task-result";
import type {
  NavigationTarget,
  QueueItem,
  QueueQuery,
  RecoveryDescriptor,
  ReviewTarget,
} from "@/lib/types/listingkit";
import { TaskStatusPanel } from "@/components/listingkit/tasks/task-status-panel";
import { buildWorkspaceSearch } from "@/components/listingkit/workspace/workspace-routing";

const defaultQueuePageSize = 20;

function parsePositiveInt(value: string | null, fallback: number) {
  const parsed = Number(value);
  return Number.isFinite(parsed) && parsed > 0 ? Math.floor(parsed) : fallback;
}

function initialFilters(searchParams: URLSearchParams): QueueFilterValue {
  return {
    platform: searchParams.get("platform") ?? "",
    slot: searchParams.get("slot") ?? "",
    quality_grade: searchParams.get("quality_grade") ?? "",
    preview_capability: searchParams.get("preview_capability") ?? "",
    review_status: searchParams.get("review_status") ?? "",
    render_preview_available: searchParams.get("render_preview_available") === "true",
  };
}

export function QueueScreen({ taskId }: { taskId: string }) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [filters, setFilters] = useState(() => initialFilters(searchParams));
  const page = parsePositiveInt(searchParams.get("page"), 1);
  const pageSize = parsePositiveInt(
    searchParams.get("page_size"),
    defaultQueuePageSize,
  );

  const queueQuery = useMemo<QueueQuery>(
    () => ({
      ...filters,
      page,
      page_size: pageSize,
      sort_by: "render_preview_available",
      sort_order: "desc",
    }),
    [filters, page, pageSize],
  );

  const queue = useGenerationQueue(taskId, queueQuery);
  const taskResult = useListingKitTaskResult(taskId);
  const dispatch = useDispatchNavigation(taskId, queueQuery);
  const action = useExecuteAction(taskId, queueQuery);
  const queueEmptyState = deriveTaskQueueEmptyState(taskResult.data);
  const suppressResolvedActionSummary = shouldSuppressResolvedActionSummary(
    taskResult.data,
    {
      hasPreviewSvg: false,
      queueTotal: queue.data?.total ?? 0,
    },
  );

  const handleApply = (nextFilters: QueueFilterValue) => {
    setFilters(nextFilters);
    const params = new URLSearchParams();
    Object.entries(nextFilters).forEach(([key, value]) => {
      if (value === "" || value === false) return;
      params.set(key, String(value));
    });
    params.set("page", "1");
    params.set("page_size", String(pageSize));
    router.replace(`/listing-kits/${taskId}/queue?${params.toString()}`);
  };

  const updateQueuePage = (nextPage: number, nextPageSize = pageSize) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set("page", String(Math.max(1, nextPage)));
    params.set("page_size", String(nextPageSize));
    router.replace(`/listing-kits/${taskId}/queue?${params.toString()}`);
  };

  const handleNavigationTarget = (target?: NavigationTarget | null) => {
    if (!target) return;
    const workspaceTarget = deriveWorkspaceTargetFromNavigationTarget(target);
    if (workspaceTarget) {
      const search = buildWorkspaceSearch("", workspaceTarget as ReviewTarget);
      router.push(
        `/listing-kits/${taskId}/workspace${search ? `?${search}` : ""}`,
      );
      return;
    }
    dispatch.mutate(target);
  };

  const handleRecovery = (descriptor: RecoveryDescriptor) => {
    handleNavigationTarget(descriptor.recovery_target);
  };

  const handleReview = (item: QueueItem) => {
    const primaryAction = deriveQueueItemAction(item);

    if (primaryAction.request) {
      action.mutate(primaryAction.request);
      return;
    }

    const params = new URLSearchParams();
    Object.entries(primaryAction.workspaceQuery ?? {}).forEach(([key, value]) => {
      if (value === undefined || value === null || value === "") return;
      params.set(key, String(value));
    });
    if (params.size > 0) {
      router.push(`/listing-kits/${taskId}/workspace?${params.toString()}`);
      return;
    }

    if (item.platform) params.set("platform", item.platform);
    if (item.slot) params.set("slot", item.slot);
    if (item.preview_capabilities?.[0]) {
      params.set("preview_capability", item.preview_capabilities[0]);
    }
    router.push(`/listing-kits/${taskId}/workspace?${params.toString()}`);
  };

  if (queue.isLoading) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <LoaderCircle className="h-8 w-8 animate-spin text-zinc-400" />
      </div>
    );
  }

  if (!queue.data) {
    return (
      <EmptyState
        title="Queue unavailable"
        description="The task did not return generation queue data."
      />
    );
  }

  return (
    <div className="space-y-6">
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
      <WorkspaceHeader
        title={`任务队列 ${taskId}`}
        summary={
          suppressResolvedActionSummary ? undefined : queue.data.resolved_action_summary
        }
        recoverySummary={queue.data.recovery_summary}
        onSelectAction={(summary) => {
          if (summary.action_target || summary.action_key) {
            action.mutate({
              action_key: summary.action_key,
              response_mode: "patch_only",
              target: summary.action_target,
            });
            return;
          }
          handleNavigationTarget(summary.navigation_target);
        }}
        onSelectRecovery={handleRecovery}
      />
      <TaskStatusPanel task={taskResult.data} />
      <ReviewReasonsCard task={taskResult.data} />
      <TaskProgressNotice task={taskResult.data} />
      <QueueSummaryStrip summary={queue.data.summary} />
      <QueueFiltersBar value={filters} onApply={handleApply} />
      {queue.data.total === 0 && queueEmptyState ? (
        <EmptyState
          title={queueEmptyState.title}
          description={queueEmptyState.description}
        />
      ) : (
        <>
          <QueuePagination
            page={queue.data.page ?? page}
            pageSize={queue.data.page_size ?? pageSize}
            total={queue.data.total ?? 0}
            onChange={updateQueuePage}
          />
          <QueueTable items={queue.data.items} onAction={(item) => handleReview(item)} />
          <QueuePagination
            page={queue.data.page ?? page}
            pageSize={queue.data.page_size ?? pageSize}
            total={queue.data.total ?? 0}
            onChange={updateQueuePage}
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
