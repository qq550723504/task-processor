import Link from "next/link";

import { ListingKitHomeTaskCard, taskWorkspaceHref } from "@/components/listingkit/home/listingkit-home-task-card";
import { EmptyState } from "@/components/shared/empty-state";
import {
  pickContinueTask,
  sortRecentTasksForHomepage,
} from "@/lib/listingkit/home-recent-tasks";
import type { ListingKitTaskListItem } from "@/lib/types/listingkit/tasks";

type ListingKitHomeRecentWorkProps = {
  tasks: ListingKitTaskListItem[];
  isLoading: boolean;
  isError: boolean;
};

function continueTaskTitle(task: ListingKitTaskListItem) {
  return task.product_name || task.title || task.task_id;
}

export function ListingKitHomeRecentWork({
  tasks,
  isLoading,
  isError,
}: ListingKitHomeRecentWorkProps) {
  if (isLoading) {
    return (
      <section
        role="status"
        aria-label="最近任务加载中"
        className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]"
      >
        <div className="h-44 animate-pulse rounded-[2rem] bg-white/60" />
        <div className="grid gap-4 md:grid-cols-3 lg:grid-cols-1">
          <div className="h-32 animate-pulse rounded-[1.5rem] bg-white/60" />
          <div className="h-32 animate-pulse rounded-[1.5rem] bg-white/60" />
          <div className="h-32 animate-pulse rounded-[1.5rem] bg-white/60" />
        </div>
      </section>
    );
  }

  if (isError && !tasks.length) {
    return (
      <section className="rounded-[2rem] border border-amber-200 bg-amber-50/80 p-5 text-sm text-amber-800">
        <p className="font-medium">最近任务暂时加载失败</p>
        <p className="mt-1">你仍然可以直接进入 SHEIN 工作台或新建任务。</p>
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

  return (
    <section className="grid gap-4 lg:grid-cols-[1.1fr_0.9fr]">
      {continueTask ? (
        <div className="rounded-[2rem] border border-white/70 bg-zinc-950 p-6 text-white shadow-[0_24px_80px_rgba(24,24,27,0.22)]">
          <p className="text-xs font-semibold uppercase tracking-[0.24em] text-teal-300">
            Continue
          </p>
          <h2 className="mt-3 text-2xl font-semibold">继续最近任务</h2>
          <p className="mt-2 text-sm leading-6 text-zinc-300">
            {continueTaskTitle(continueTask)}
          </p>
          <Link
            href={taskWorkspaceHref(continueTask)}
            className="mt-6 inline-flex h-11 items-center justify-center rounded-xl bg-white px-5 text-sm font-medium text-zinc-950 transition hover:bg-zinc-100"
          >
            继续最近任务
          </Link>
        </div>
      ) : null}

      <div className="grid gap-4 md:grid-cols-3 lg:grid-cols-1">
        {isError ? (
          <div className="md:col-span-3 lg:col-span-1 rounded-[1.5rem] border border-amber-200 bg-amber-50/80 p-4 text-sm text-amber-800">
            <p className="font-medium">最近任务暂时加载失败</p>
            <p className="mt-1">以下为上次成功加载的最近任务。</p>
          </div>
        ) : null}
        {recentTasks.map((task) => (
          <ListingKitHomeTaskCard key={task.task_id} task={task} />
        ))}
      </div>
    </section>
  );
}
