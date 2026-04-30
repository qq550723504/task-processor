"use client";

import { ListingKitHomeHero } from "@/components/listingkit/home/listingkit-home-hero";
import { ListingKitHomeQuickTools } from "@/components/listingkit/home/listingkit-home-quick-tools";
import { ListingKitHomeRecentWork } from "@/components/listingkit/home/listingkit-home-recent-work";
import { useListingKitTasks } from "@/lib/query/use-task-list";

export function ListingKitHomepage() {
  const tasks = useListingKitTasks({ page: 1, page_size: 6 });
  const items = tasks.data?.items ?? [];

  return (
    <div className="relative isolate flex flex-1 overflow-hidden rounded-[2.5rem] bg-[radial-gradient(circle_at_top_left,rgba(45,212,191,0.18),transparent_26%),radial-gradient(circle_at_top_right,rgba(251,191,36,0.18),transparent_24%),linear-gradient(180deg,#fbfaf6_0%,#efeee8_100%)] px-4 py-4 sm:px-6 sm:py-6">
      <div className="pointer-events-none absolute inset-0 bg-[linear-gradient(rgba(24,24,27,0.03)_1px,transparent_1px),linear-gradient(90deg,rgba(24,24,27,0.03)_1px,transparent_1px)] bg-[size:34px_34px]" />
      <div className="relative mx-auto flex w-full max-w-7xl flex-col gap-6 lg:gap-8">
        <ListingKitHomeHero />
        <div className="space-y-4">
          <div className="space-y-1">
            <h2 className="text-xl font-semibold tracking-tight text-zinc-950">
              快速入口
            </h2>
            <p className="text-sm leading-6 text-zinc-600">
              保留常用工具入口，但把 SHEIN 放在最前面。
            </p>
          </div>
          <ListingKitHomeQuickTools />
        </div>
        <div className="space-y-4">
          <div className="space-y-1">
            <h2 className="text-xl font-semibold tracking-tight text-zinc-950">
              最近工作
            </h2>
            <p className="text-sm leading-6 text-zinc-600">
              继续最近处理过的任务，或在异常时看到不阻塞的提示。
            </p>
          </div>
          <ListingKitHomeRecentWork
            tasks={items}
            isLoading={tasks.isLoading}
            isError={tasks.isError}
          />
        </div>
      </div>
    </div>
  );
}
