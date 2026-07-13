"use client";

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { getTaskSDSRepair, repairAndRetryTaskSDS } from "@/lib/api/sds-repair";
import type { ApplyTaskSDSRepairRequest } from "@/lib/types/listingkit/tasks";

const invalidateTask = (client: ReturnType<typeof useQueryClient>, taskId: string) =>
  client.invalidateQueries({ predicate: (query) => Array.isArray(query.queryKey) && query.queryKey[0] === "listingkit" && query.queryKey[1] === taskId });

export function useTaskSDSRepair(taskId: string, enabled: boolean) {
  return useQuery({ queryKey: ["listingkit", taskId, "sds-repair"], queryFn: () => getTaskSDSRepair(taskId), enabled });
}

export function useRepairAndRetryTaskSDS(taskId: string) {
  const client = useQueryClient();
  return useMutation({ mutationFn: (request: ApplyTaskSDSRepairRequest) => repairAndRetryTaskSDS(taskId, request), onSettled: () => invalidateTask(client, taskId) });
}
