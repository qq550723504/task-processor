import type { AppRouterInstance } from "next/dist/shared/lib/app-router-context.shared-runtime";

import { deriveQueueItemAction } from "@/components/listingkit/queue/queue-actions";
import { deriveWorkspaceTargetFromNavigationTarget } from "@/components/listingkit/queue/queue-routing";
import { buildWorkspaceSearch } from "@/components/listingkit/workspace/workspace-routing";
import type {
  ActionExecutionRequest,
  NavigationTarget,
  QueueItem,
  RecoveryDescriptor,
  ReviewTarget,
} from "@/lib/types/listingkit";

type DispatchMutation = {
  mutate: (target: NavigationTarget) => void;
};

type ActionMutation = {
  mutate: (request: ActionExecutionRequest) => void;
};

export function useQueueScreenActions({
  action,
  dispatch,
  router,
  taskId,
}: {
  action: ActionMutation;
  dispatch: DispatchMutation;
  router: AppRouterInstance;
  taskId: string;
}) {
  const handleNavigationTarget = (target?: NavigationTarget | null) => {
    if (!target) return;
    const workspaceTarget = deriveWorkspaceTargetFromNavigationTarget(target);
    if (workspaceTarget) {
      const search = buildWorkspaceSearch("", workspaceTarget as ReviewTarget);
      router.push(
        `/listing-kits/${taskId}/workspace${search ? `?${search}` : ""}`,
      );
      return;
    }
    dispatch.mutate(target);
  };

  const handleRecovery = (descriptor: RecoveryDescriptor) => {
    handleNavigationTarget(descriptor.recovery_target);
  };

  const handleReview = (item: QueueItem) => {
    const primaryAction = deriveQueueItemAction(item);

    if (primaryAction.request) {
      action.mutate(primaryAction.request);
      return;
    }

    const params = new URLSearchParams();
    Object.entries(primaryAction.workspaceQuery ?? {}).forEach(
      ([key, value]) => {
        if (value === undefined || value === null || value === "") return;
        params.set(key, String(value));
      },
    );
    if (params.size > 0) {
      router.push(`/listing-kits/${taskId}/workspace?${params.toString()}`);
      return;
    }

    if (item.platform) params.set("platform", item.platform);
    if (item.slot) params.set("slot", item.slot);
    if (item.preview_capabilities?.[0]) {
      params.set("preview_capability", item.preview_capabilities[0]);
    }
    router.push(`/listing-kits/${taskId}/workspace?${params.toString()}`);
  };

  return {
    handleNavigationTarget,
    handleRecovery,
    handleReview,
  };
}
