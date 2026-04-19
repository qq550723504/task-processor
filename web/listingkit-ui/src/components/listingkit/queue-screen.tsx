"use client";

import { useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { LoaderCircle } from "lucide-react";

import {
  QueueFiltersBar,
  type QueueFilterValue,
} from "@/components/listingkit/queue-filters-bar";
import { deriveQueueItemAction } from "@/components/listingkit/queue-actions";
import { deriveWorkspaceTargetFromNavigationTarget } from "@/components/listingkit/queue-routing";
import {
  deriveTaskQueueEmptyState,
  shouldSuppressResolvedActionSummary,
} from "@/components/listingkit/task-status-display";
import { QueueSummaryStrip } from "@/components/listingkit/queue-summary-strip";
import { QueueTable } from "@/components/listingkit/queue-table";
import { TaskProgressNotice } from "@/components/listingkit/task-progress-notice";
import { WorkspaceHeader } from "@/components/listingkit/workspace-header";
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
import { TaskStatusPanel } from "@/components/listingkit/task-status-panel";
import { buildWorkspaceSearch } from "@/components/listingkit/workspace-routing";

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

  const queueQuery = useMemo<QueueQuery>(
    () => ({
      ...filters,
      page: 1,
      page_size: 50,
      sort_by: "render_preview_available",
      sort_order: "desc",
    }),
    [filters],
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
      <WorkspaceHeader
        title={`Queue ${taskId}`}
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
      <TaskProgressNotice task={taskResult.data} />
      <QueueSummaryStrip summary={queue.data.summary} />
      <QueueFiltersBar value={filters} onApply={handleApply} />
      {queue.data.total === 0 && queueEmptyState ? (
        <EmptyState
          title={queueEmptyState.title}
          description={queueEmptyState.description}
        />
      ) : (
        <QueueTable items={queue.data.items} onAction={(item) => handleReview(item)} />
      )}
    </div>
  );
}
