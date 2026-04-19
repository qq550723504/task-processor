"use client";

import { use } from "react";
import { LoaderCircle } from "lucide-react";

import { TaskStatusScreen } from "@/components/listingkit/task-status-screen";
import { EmptyState } from "@/components/shared/empty-state";
import { useListingKitTaskResult } from "@/lib/query/use-task-result";

export default function ListingKitTaskStatusPage({
  params,
}: {
  params: Promise<{ taskId: string }>;
}) {
  const { taskId } = use(params);
  const task = useListingKitTaskResult(taskId);

  if (task.isLoading) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <LoaderCircle className="h-8 w-8 animate-spin text-zinc-400" />
      </div>
    );
  }

  if (!task.data) {
    return (
      <EmptyState
        title="Task unavailable"
        description="The task did not return status data."
      />
    );
  }

  return <TaskStatusScreen taskId={taskId} task={task.data} />;
}
