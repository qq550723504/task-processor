import { apiRequest } from "@/lib/api/client";
import type { NavigationDispatchResponse, NavigationTarget } from "@/lib/types/listingkit";

export function dispatchNavigation(
  taskId: string,
  target: NavigationTarget,
  planMode: "primary_only" | "execute_plan" = "primary_only",
) {
  return apiRequest<NavigationDispatchResponse>(
    `/tasks/${taskId}/generation-navigation/dispatch`,
    {
      method: "POST",
      body: {
        target,
        plan_mode: planMode,
      },
    },
  );
}
