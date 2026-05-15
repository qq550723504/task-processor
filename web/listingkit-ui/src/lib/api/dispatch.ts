import { apiRequest } from "@/lib/api/client";
import { parseDispatchResponse } from "@/lib/api/listingkit-response-schema";
import type { NavigationDispatchResponse, NavigationTarget } from "@/lib/types/listingkit";

export async function dispatchNavigation(
  taskId: string,
  target: NavigationTarget,
  planMode: "primary_only" | "execute_plan" = "primary_only",
) {
  return parseDispatchResponse(
    await apiRequest<NavigationDispatchResponse>(
      `/tasks/${taskId}/generation-navigation/dispatch`,
      {
        method: "POST",
        body: {
          target,
          plan_mode: planMode,
        },
      },
    ),
  );
}
