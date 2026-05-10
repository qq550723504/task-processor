"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

import {
  normalizeSheinWorkspaceActionKey,
  sheinWorkspaceTargetIdForKey,
} from "@/components/listingkit/shein/shein-workspace-actions";
import {
  deriveRecoveryNavigationTarget,
} from "@/components/listingkit/workspace/workspace-action-routing";
import { shouldSyncPlatformOnRecovery } from "@/components/listingkit/workspace/workspace-recovery-routing";
import { buildWorkspaceSearch } from "@/components/listingkit/workspace/workspace-routing";
import { scrollSheinWorkspaceTarget } from "@/components/listingkit/workspace/workspace-screen-helpers";
import { useExecuteAction } from "@/lib/query/use-action";
import { useDispatchNavigation } from "@/lib/query/use-dispatch";
import type {
  ActionExecutionRequest,
  NavigationTarget,
  QueueQuery,
  RecoveryDescriptor,
  ResolvedActionSummary,
  ReviewTarget,
  SheinReadinessItem,
  ToolbarAction,
} from "@/lib/types/listingkit";
import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";

type SearchParamsLike = {
  toString(): string;
};

export function useWorkspaceNavigationActions({
  taskId,
  baseQuery,
  searchParams,
  focusedTarget,
}: {
  taskId: string;
  baseQuery: QueueQuery;
  searchParams: SearchParamsLike;
  focusedTarget?: ReviewTarget;
}) {
  const router = useRouter();
  const dispatch = useDispatchNavigation(taskId, baseQuery);
  const action = useExecuteAction(taskId, baseQuery);

  useEffect(() => {
    if (!focusedTarget) {
      return;
    }

    const nextSearch = buildWorkspaceSearch(searchParams.toString(), focusedTarget);
    const currentSearch = searchParams.toString();
    if (nextSearch === currentSearch) {
      return;
    }

    router.replace(
      `/listing-kits/${taskId}/workspace${nextSearch ? `?${nextSearch}` : ""}`,
    );
  }, [focusedTarget, router, searchParams, taskId]);

  const dispatchTarget = (target?: NavigationTarget | null) => {
    if (!target) {
      return;
    }
    dispatch.mutate(target);
  };

  const handleAction = (
    actionSummary?: ResolvedActionSummary | null,
    request?: ActionExecutionRequest,
  ) => {
    if (request) {
      action.mutate(request);
      return;
    }

    if (actionSummary?.action_target || actionSummary?.action_key) {
      action.mutate({
        action_key: actionSummary.action_key,
        response_mode: "patch_only",
        target: actionSummary.action_target,
      });
      return;
    }

    dispatchTarget(actionSummary?.navigation_target);
  };

  const handleToolbarAction = (toolbarAction: ToolbarAction) => {
    if (toolbarAction.action_target || toolbarAction.kind === "workflow") {
      action.mutate({
        action_key: toolbarAction.action_target?.action_key,
        response_mode: "patch_only",
        target: toolbarAction.action_target,
      });
      return;
    }

    dispatchTarget(
      toolbarAction.navigation_target ?? toolbarAction.target?.navigation_target,
    );
  };

  const handleRecovery = (descriptor: RecoveryDescriptor) => {
    const target = deriveRecoveryNavigationTarget(descriptor);
    if (target) {
      dispatchTarget(target);
    }
  };

  const handlePlatformSelect = (platform: string) => {
    const params = sanitizedNavigationSearchParams(searchParams);
    params.set("platform", platform);
    router.replace(`/listing-kits/${taskId}/workspace?${params.toString()}`);
  };

  const handlePlatformRecovery = (
    descriptor: RecoveryDescriptor,
    platform: string,
  ) => {
    handleRecovery(descriptor);
    if (shouldSyncPlatformOnRecovery(descriptor)) {
      handlePlatformSelect(platform);
    }
  };

  const handleSelectSheinBlockingItem = (item: SheinReadinessItem) => {
    scrollSheinActionTarget(item.key);
  };

  const handleRunSheinPrimaryAction = (key?: string | null) => {
    scrollSheinActionTarget(key);
  };

  return {
    dispatchTarget,
    handleAction,
    handleToolbarAction,
    handleRecovery,
    handlePlatformSelect,
    handlePlatformRecovery,
    handleSelectSheinBlockingItem,
    handleRunSheinPrimaryAction,
  };
}

function scrollSheinActionTarget(key?: string | null) {
  const normalizedKey = normalizeSheinWorkspaceActionKey(key);
  if (!normalizedKey) {
    return;
  }
  const targetId = sheinWorkspaceTargetIdForKey(normalizedKey);
  scrollSheinWorkspaceTarget(normalizedKey, targetId);
}
