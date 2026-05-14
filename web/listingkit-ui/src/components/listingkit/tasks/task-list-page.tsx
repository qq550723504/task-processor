"use client";

import { useRouter, useSearchParams } from "next/navigation";

import { SheinSettingsCard } from "@/components/listingkit/shein/shein-settings-card";
import { ListingKitPageShell } from "@/components/listingkit/shared/listingkit-page-shell";
import {
  TaskListContent,
  TaskListFilters,
  TaskListHero,
  type FilterKey,
} from "@/components/listingkit/tasks/task-list-page-sections";
import { useListingKitTasks } from "@/lib/query/use-task-list";
import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";

const PAGE_SIZE = 20;

export function TaskListPage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const status = searchParams.get("status") ?? "";
  const platform = searchParams.get("platform") ?? "";
  const sheinWorkflowStatus = searchParams.get("shein_workflow_status") ?? "";
  const sheinWorkQueue = searchParams.get("shein_work_queue") ?? "";
  const sheinActionQueue = searchParams.get("shein_action_queue") ?? "";
  const sheinBlockerKey = searchParams.get("shein_blocker_key") ?? "";
  const sheinWarningKey = searchParams.get("shein_warning_key") ?? "";
  const page = Number(searchParams.get("page") ?? "1") || 1;

  const tasks = useListingKitTasks({
    status: status || undefined,
    platform: platform || undefined,
    shein_workflow_status: sheinWorkflowStatus || undefined,
    shein_work_queue: sheinWorkQueue || undefined,
    shein_action_queue: sheinActionQueue || undefined,
    shein_blocker_key: sheinBlockerKey || undefined,
    shein_warning_key: sheinWarningKey || undefined,
    page,
    page_size: PAGE_SIZE,
  });
  const items = tasks.data?.items ?? [];

  const updateFilters = (updates: Partial<Record<FilterKey, string | null>>) => {
    const params = sanitizedNavigationSearchParams(searchParams);
    for (const [key, value] of Object.entries(updates)) {
      if (!value) {
        params.delete(key);
        continue;
      }
      params.set(key, value);
    }
    params.delete("page");
    router.push(`/listing-kits${params.toString() ? `?${params.toString()}` : ""}`);
  };

  const updateFilter = (key: FilterKey, value: string) => {
    updateFilters({ [key]: value });
  };

  const updatePage = (nextPage: number) => {
    const params = sanitizedNavigationSearchParams(searchParams);
    if (nextPage <= 1) {
      params.delete("page");
    } else {
      params.set("page", String(nextPage));
    }
    router.push(`/listing-kits${params.toString() ? `?${params.toString()}` : ""}`);
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
        sheinActionQueue={sheinActionQueue}
        sheinBlockerKey={sheinBlockerKey}
        sheinWorkflowStatus={sheinWorkflowStatus}
        sheinWarningKey={sheinWarningKey}
        sheinWorkQueue={sheinWorkQueue}
        status={status}
        summary={tasks.data?.summary}
        taxonomy={tasks.data?.taxonomy}
        total={tasks.data?.total ?? 0}
        updateFilters={updateFilters}
        updateFilter={updateFilter}
      />
      <TaskListContent
        isError={tasks.isError}
        isLoading={tasks.isLoading}
        items={items}
        onRefresh={() => tasks.refetch()}
        page={tasks.data?.page ?? page}
        pageSize={tasks.data?.page_size ?? PAGE_SIZE}
        total={tasks.data?.total ?? 0}
        taxonomy={tasks.data?.taxonomy}
        updatePage={updatePage}
      />
    </ListingKitPageShell>
  );
}
