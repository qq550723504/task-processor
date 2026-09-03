"use client";

import { useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";

import type { QueueFilterValue } from "@/components/listingkit/queue/queue-filters-bar";
import { useQueueScreenActions } from "@/components/listingkit/queue/queue-screen-actions";
import {
  defaultQueuePageSize,
  initialQueueFilters,
  parsePositiveInt,
} from "@/components/listingkit/queue/queue-screen-model";
import {
  QueueErrorState,
  QueueLoadingState,
  QueuePendingDataState,
  QueueScreenBody,
} from "@/components/listingkit/queue/queue-screen-sections";
import { useExecuteAction } from "@/lib/query/use-action";
import { useDispatchNavigation } from "@/lib/query/use-dispatch";
import { useGenerationQueue } from "@/lib/query/use-queue";
import { useBulkRecoverTasks } from "@/lib/query/use-task-recovery";
import { useListingKitTaskResult } from "@/lib/query/use-task-result";
import type { QueueQuery } from "@/lib/types/listingkit";
import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";

export function QueueScreen({ taskId }: { taskId: string }) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [filters, setFilters] = useState(() =>
    initialQueueFilters(searchParams),
  );
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
  const bulkRecovery = useBulkRecoverTasks();
  const { handleNavigationTarget, handleRecovery, handleReview } =
    useQueueScreenActions({
      action,
      dispatch,
      router,
      taskId,
    });

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
    const params = sanitizedNavigationSearchParams(searchParams);
    params.set("page", String(Math.max(1, nextPage)));
    params.set("page_size", String(nextPageSize));
    router.replace(`/listing-kits/${taskId}/queue?${params.toString()}`);
  };

  if (queue.isLoading) {
    return <QueueLoadingState />;
  }

  if (queue.isError || taskResult.isError) {
    return <QueueErrorState taskId={taskId} />;
  }

  if (!queue.data) {
    return <QueuePendingDataState taskId={taskId} />;
  }

  return (
    <QueueScreenBody
      filters={filters}
      onAction={handleReview}
      onApplyFilters={handleApply}
      onBulkRecoverBlockedTasks={() =>
        bulkRecovery.mutate({
          due_before: new Date().toISOString(),
          limit: 20,
        })
      }
      onChangePage={updateQueuePage}
      bulkRecovering={bulkRecovery.isPending}
      onExecuteAction={(request) => action.mutate(request)}
      onSelectNavigation={handleNavigationTarget}
      onSelectRecovery={handleRecovery}
      queueData={queue.data}
      taskId={taskId}
      taskResult={taskResult.data}
    />
  );
}
