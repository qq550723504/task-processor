"use client";

import Link from "next/link";
import { useMemo } from "react";
import { useQueries } from "@tanstack/react-query";
import { LoaderCircle } from "lucide-react";

import { Button } from "@/components/shared/button";
import { getListingKitTaskResult } from "@/lib/api/task-result";
import { listingKitKeys } from "@/lib/query/keys";
import { shouldPollTaskResult } from "@/components/listingkit/tasks/task-status-query";
import type { ListingKitTaskResult } from "@/lib/types/listingkit";
import type { SheinStudioCreatedTask } from "@/lib/types/shein-studio";

function summarizeStatuses(items: ListingKitTaskResult[]) {
  return items.reduce<Record<string, number>>((acc, item) => {
    const key = item.status || "unknown";
    acc[key] = (acc[key] ?? 0) + 1;
    return acc;
  }, {});
}

export function SheinBatchTaskTracker({
  tasks,
}: {
  tasks: SheinStudioCreatedTask[];
}) {
  const taskQueries = useQueries({
    queries: tasks.map((task) => ({
      queryKey: listingKitKeys.taskResult(task.id),
      queryFn: () => getListingKitTaskResult(task.id),
      refetchInterval: (query: { state: { data?: ListingKitTaskResult } }) =>
        shouldPollTaskResult(query.state.data?.status) ? 5000 : false,
      refetchOnWindowFocus: true,
    })),
  });

  const resolvedTasks = useMemo(
    () =>
      tasks.map((task, index) => ({
        task,
        query: taskQueries[index],
        result: taskQueries[index]?.data,
      })),
    [taskQueries, tasks],
  );

  const loadedResults = resolvedTasks
    .map((item) => item.result)
    .filter((item): item is ListingKitTaskResult => Boolean(item));
  const statusSummary = summarizeStatuses(loadedResults);
  const runningCount = Object.entries(statusSummary)
    .filter(([key]) => shouldPollTaskResult(key))
    .reduce((sum, [, count]) => sum + count, 0);

  if (tasks.length === 0) {
    return null;
  }

  return (
    <section className="space-y-4 rounded-[1.75rem] border border-zinc-200/80 bg-white px-5 py-5 shadow-sm">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <p className="text-[11px] font-semibold uppercase tracking-[0.28em] text-zinc-500">
            Task tracker
          </p>
          <h2 className="mt-1 font-serif text-2xl tracking-[-0.03em] text-zinc-950">
            Monitor created SHEIN review tasks in one place.
          </h2>
        </div>
        <div className="flex flex-wrap gap-2">
          {Object.entries(statusSummary).map(([status, count]) => (
            <div
              className="rounded-xl border border-zinc-200 bg-zinc-50 px-3 py-2 text-xs font-semibold text-zinc-700"
              key={status}
            >
              {status}: {count}
            </div>
          ))}
          {runningCount > 0 ? (
            <div className="rounded-xl border border-amber-200 bg-amber-50 px-3 py-2 text-xs font-semibold text-amber-700">
              polling {runningCount}
            </div>
          ) : null}
        </div>
      </div>

      <div className="grid gap-3">
        {resolvedTasks.map(({ query, result, task }) => {
          const status = result?.status ?? (query.isLoading ? "loading" : "unknown");
          const sdsStatus = result?.result?.sds_sync?.status;
          return (
            <article
              className="flex flex-wrap items-center justify-between gap-4 rounded-[1.25rem] border border-zinc-200 bg-zinc-50 px-4 py-4"
              key={task.id}
            >
              <div className="space-y-1">
                <div className="flex items-center gap-2">
                  <div className="text-sm font-semibold text-zinc-950">
                    {task.title}
                  </div>
                  {query.isFetching ? (
                    <LoaderCircle className="h-4 w-4 animate-spin text-zinc-400" />
                  ) : null}
                </div>
                <div className="text-xs text-zinc-500">{task.id}</div>
                <div className="flex flex-wrap gap-2 text-xs">
                  <span className="rounded-lg bg-white px-2 py-1 font-medium text-zinc-700">
                    task: {status}
                  </span>
                  {sdsStatus ? (
                    <span className="rounded-lg bg-white px-2 py-1 font-medium text-zinc-700">
                      sds: {sdsStatus}
                    </span>
                  ) : null}
                </div>
              </div>

              <div className="flex flex-wrap gap-2">
                <Link href={`/listing-kits/${task.id}/status`}>
                  <Button tone="secondary">Open status</Button>
                </Link>
                <Link href={`/listing-kits/${task.id}/workspace`}>
                  <Button tone="ghost">Open workspace</Button>
                </Link>
              </div>
            </article>
          );
        })}
      </div>
    </section>
  );
}
