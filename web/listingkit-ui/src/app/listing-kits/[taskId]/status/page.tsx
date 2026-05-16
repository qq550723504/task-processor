"use client";

import { use } from "react";
import { LoaderCircle } from "lucide-react";

import { TaskStatusScreen } from "@/components/listingkit/tasks/task-status-screen";
import { EmptyState } from "@/components/shared/empty-state";
import { useListingKitTaskResult } from "@/lib/query/use-task-result";
import { Button } from "@/components/ui/button";

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

  if (task.isError) {
    return (
      <EmptyState
        title="任务状态暂时无法加载"
        description="当前无法读取任务状态。你可以刷新重试，或先回到任务列表稍后重新进入。"
        action={
          <div className="flex flex-wrap gap-3">
            <Button variant="secondary" onClick={() => task.refetch()}>
              刷新当前页面
            </Button>
            <a
              className="inline-flex h-10 items-center justify-center rounded-xl bg-white px-4 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100"
              href="/listing-kits"
            >
              返回任务列表
            </a>
          </div>
        }
      />
    );
  }

  if (!task.data) {
    return (
      <EmptyState
        title="任务状态暂未准备完成"
        description="当前任务还没有返回完整状态数据。你可以稍后刷新，或先回到任务列表继续查看。"
        action={
          <div className="flex flex-wrap gap-3">
            <Button variant="secondary" onClick={() => task.refetch()}>
              重新加载
            </Button>
            <a
              className="inline-flex h-10 items-center justify-center rounded-xl bg-white px-4 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100"
              href="/listing-kits"
            >
              返回任务列表
            </a>
          </div>
        }
      />
    );
  }

  return <TaskStatusScreen taskId={taskId} task={task.data} />;
}
