"use client";

import { ListingKitHomeHero } from "@/components/listingkit/home/listingkit-home-hero";
import { ListingKitHomeQuickTools } from "@/components/listingkit/home/listingkit-home-quick-tools";
import { ListingKitHomeRecentWork } from "@/components/listingkit/home/listingkit-home-recent-work";
import { useListingKitTasks } from "@/lib/query/use-task-list";

export function ListingKitHomepage() {
  const tasks = useListingKitTasks({ page: 1, page_size: 6 });
  const items = tasks.data?.items ?? [];

  return (
    <div className="flex flex-1 overflow-hidden rounded-lg bg-zinc-50 px-4 py-4 sm:px-6 sm:py-6">
      <div className="mx-auto flex w-full max-w-7xl flex-col gap-6 lg:gap-8">
        <ListingKitHomeHero />
        <div className="space-y-4">
          <div className="space-y-1">
            <h2 className="text-xl font-semibold tracking-tight text-zinc-950">
              功能入口
            </h2>
            <p className="text-sm leading-6 text-zinc-600">
              按来源、标准商品、平台资料和任务恢复进入对应工作区。
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
              继续最近的生成、审核或上架任务，必要时直接回到对应工作区。
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
