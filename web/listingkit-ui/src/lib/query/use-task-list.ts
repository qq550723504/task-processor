"use client";

import { useQuery } from "@tanstack/react-query";

import { getListingKitTasks } from "@/lib/api/task-list";
import { listingKitKeys } from "@/lib/query/keys";
import type { ListingKitTaskListQuery } from "@/lib/types/listingkit";

export function useListingKitTasks(query: ListingKitTaskListQuery) {
  return useQuery({
    queryKey: listingKitKeys.tasks(query),
    queryFn: () => getListingKitTasks(query),
    refetchInterval: 10000,
    refetchOnWindowFocus: true,
  });
}
