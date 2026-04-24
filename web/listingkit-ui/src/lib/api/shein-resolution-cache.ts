import { apiRequest } from "@/lib/api/client";

export type SheinResolutionCacheKind =
  | "category"
  | "attribute"
  | "sale_attribute"
  | "all";

export type ClearSheinResolutionCacheResponse = {
  task_id: string;
  kind: SheinResolutionCacheKind;
  deleted_kinds?: SheinResolutionCacheKind[];
};

export function clearSheinResolutionCache(
  taskId: string,
  kind: SheinResolutionCacheKind,
) {
  return apiRequest<ClearSheinResolutionCacheResponse>(
    `/tasks/${taskId}/shein-resolution-cache`,
    {
      method: "DELETE",
      query: { kind },
    },
  );
}
