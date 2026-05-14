"use client";

import { useRouter, useSearchParams } from "next/navigation";

import { SheinSettingsCard } from "@/components/listingkit/shein/shein-settings-card";
import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import {
  TaskListContent,
  TaskListFilters,
  TaskListHero,
} from "@/components/listingkit/tasks/task-list-page-sections";
import { useListingKitTasks } from "@/lib/query/use-task-list";
import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";

type FilterKey = "status" | "platform" | "shein_workflow_status";

export function TaskListPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const status = searchParams.get("status") ?? "";
  const platform = searchParams.get("platform") ?? "";
  const sheinWorkflowStatus = searchParams.get("shein_workflow_status") ?? "";
  const page = Number(searchParams.get("page") ?? "1") || 1;
  const tasks = useListingKitTasks({
    status: status || undefined,
    platform: platform || undefined,
    shein_workflow_status: sheinWorkflowStatus || undefined,
    page,
    page_size: 20,
  });
  const items = tasks.data?.items ?? [];

  const updateFilter = (key: FilterKey, value: string) => {
    const params = sanitizedNavigationSearchParams(searchParams);
    if (value) {
      params.set(key, value);
    } else {
      params.delete(key);
    }
    params.delete("page");
    router.push(
      `/listing-kits${params.toString() ? `?${params.toString()}` : ""}`,
    );
  };

  return (
    <ListingKitPageShell
      backgroundClassName="isolate overflow-hidden bg-[radial-gradient(circle_at_12%_10%,rgba(20,184,166,0.18),transparent_30%),radial-gradient(circle_at_86%_4%,rgba(251,146,60,0.16),transparent_26%),linear-gradient(180deg,#fbfaf6_0%,#efeee8_100%)]"
      overlayClassName="bg-[linear-gradient(rgba(24,24,27,0.035)_1px,transparent_1px),linear-gradient(90deg,rgba(24,24,27,0.035)_1px,transparent_1px)] bg-[size:34px_34px]"
    >
      <TaskListHero onRefresh={() => tasks.refetch()} />
      <SheinSettingsCard />
      <TaskListFilters
        platform={platform}
        sheinWorkflowStatus={sheinWorkflowStatus}
        status={status}
        total={tasks.data?.total ?? 0}
        updateFilter={updateFilter}
      />
      <TaskListContent
        isError={tasks.isError}
        isLoading={tasks.isLoading}
        items={items}
        onRefresh={() => tasks.refetch()}
      />
    </ListingKitPageShell>
  );
}
