"use client";

import { ListingKitHomeHero } from "@/components/listingkit/home/listingkit-home-hero";
import { ListingKitHomeQuickTools } from "@/components/listingkit/home/listingkit-home-quick-tools";
import { ListingKitHomeRecentWork } from "@/components/listingkit/home/listingkit-home-recent-work";
import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import { useListingKitTasks } from "@/lib/query/use-task-list";

export function ListingKitHomepage() {
  const tasks = useListingKitTasks({ page: 1, page_size: 6 });
  const items = tasks.data?.items ?? [];
  const summary = tasks.data?.summary;
  const taxonomy = tasks.data?.taxonomy;

  return (
    <ListingKitPageShell backgroundClassName="overflow-hidden rounded-lg bg-background" contentClassName="gap-6 px-4 py-4 sm:px-6 sm:py-6 lg:gap-8">
        <ListingKitHomeHero />
        <div className="space-y-4">
          <div className="space-y-1">
            <h2 className="text-xl font-semibold tracking-tight text-foreground">
              功能入口
            </h2>
            <p className="text-sm leading-6 text-muted-foreground">
              按来源、标准商品、平台资料和任务恢复进入对应工作区。
            </p>
          </div>
          <ListingKitHomeQuickTools />
        </div>
        <div className="space-y-4">
          <div className="space-y-1">
            <h2 className="text-xl font-semibold tracking-tight text-foreground">
              最近工作
            </h2>
            <p className="text-sm leading-6 text-muted-foreground">
              继续最近的生成、审核或上架任务，必要时直接回到对应工作区。
            </p>
          </div>
          <ListingKitHomeRecentWork
            tasks={items}
            isLoading={tasks.isLoading}
            isError={tasks.isError}
            summary={summary}
            taxonomy={taxonomy}
          />
        </div>
    </ListingKitPageShell>
  );
}
