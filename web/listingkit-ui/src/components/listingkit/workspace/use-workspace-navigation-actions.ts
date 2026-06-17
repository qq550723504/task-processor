"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

import {
  buildSheinGeneralReviewHref,
  normalizeSheinFreshnessActionKey,
  isSheinWorkspaceActionKey,
  isSheinAdvancedRepairKey,
  normalizeSheinWorkspaceActionKey,
  type SheinFreshnessActionKey,
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

type SheinFreshnessActionHandlers = Partial<
  Record<
    Exclude<SheinFreshnessActionKey, "shein_online_auth">,
    () => void
  >
>;

export function useWorkspaceNavigationActions({
  taskId,
  baseQuery,
  searchParams,
  focusedTarget,
  sheinStoreID,
  sheinFreshnessActions,
}: {
  taskId: string;
  baseQuery: QueueQuery;
  searchParams: SearchParamsLike;
  focusedTarget?: ReviewTarget;
  sheinStoreID?: number | null;
  sheinFreshnessActions?: SheinFreshnessActionHandlers;
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

    if (
      actionSummary?.action_key &&
      isSheinWorkspaceActionKey(actionSummary.action_key) &&
      !actionSummary.action_target
    ) {
      navigateOrScrollSheinActionTarget({
        taskId,
        router,
        searchParams: searchParams.toString(),
        key: actionSummary.action_key,
        storeID: sheinStoreID,
        sheinFreshnessActions,
      });
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
    navigateOrScrollSheinActionTarget({
      taskId,
      router,
      searchParams: searchParams.toString(),
      key: item.key,
      repairTarget: item.taxonomy?.repair_target,
      storeID: sheinStoreID,
      sheinFreshnessActions,
    });
  };

  const handleRunSheinPrimaryAction = (key?: string | null) => {
    navigateOrScrollSheinActionTarget({
      taskId,
      router,
      searchParams: searchParams.toString(),
      key,
      storeID: sheinStoreID,
      sheinFreshnessActions,
    });
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

function navigateOrScrollSheinActionTarget({
  taskId,
  router,
  searchParams,
  key,
  repairTarget,
  storeID,
  sheinFreshnessActions,
}: {
  taskId: string;
  router: ReturnType<typeof useRouter>;
  searchParams: string;
  key?: string | null;
  repairTarget?: string | null;
  storeID?: number | null;
  sheinFreshnessActions?: SheinFreshnessActionHandlers;
}) {
  if (runSheinFreshnessAction(key, sheinFreshnessActions)) {
    return;
  }
  const normalizedKey = normalizeSheinWorkspaceActionKey(key, repairTarget);
  if (!normalizedKey) {
    return;
  }
  if (normalizedKey === "store_login") {
    const target = storeID
      ? `/listing-kits/shein-login?store_id=${storeID}`
      : "/listing-kits/shein-login";
    router.push(target);
    return;
  }
  const targetId = sheinWorkspaceTargetIdForKey(normalizedKey);
  const currentParams = new URLSearchParams(searchParams);
  const sectionKey = currentParams.get("section_key");
  const needsGeneralReviewRoute =
    isSheinAdvancedRepairKey(normalizedKey) &&
    (sectionKey === "final_review" || !document.getElementById(targetId));
  if (needsGeneralReviewRoute) {
    router.replace(buildSheinGeneralReviewHref(taskId, targetId));
    return;
  }
  scrollSheinWorkspaceTarget(normalizedKey, targetId);
}

export function runSheinFreshnessAction(
  key?: string | null,
  sheinFreshnessActions?: SheinFreshnessActionHandlers,
) {
  const normalizedFreshnessKey = normalizeSheinFreshnessActionKey(key);
  if (!normalizedFreshnessKey || !sheinFreshnessActions) {
    return false;
  }
  switch (normalizedFreshnessKey) {
    case "shein_category_template_freshness":
      if (!sheinFreshnessActions.shein_category_template_freshness) {
        return false;
      }
      sheinFreshnessActions.shein_category_template_freshness();
      return true;
    case "shein_attribute_template_freshness":
      if (!sheinFreshnessActions.shein_attribute_template_freshness) {
        return false;
      }
      sheinFreshnessActions.shein_attribute_template_freshness();
      return true;
    case "shein_sale_attribute_template_freshness":
      if (!sheinFreshnessActions.shein_sale_attribute_template_freshness) {
        return false;
      }
      sheinFreshnessActions.shein_sale_attribute_template_freshness();
      return true;
    case "shein_sale_attribute_freshness":
      if (!sheinFreshnessActions.shein_sale_attribute_freshness) {
        return false;
      }
      sheinFreshnessActions.shein_sale_attribute_freshness();
      return true;
    default:
      return false;
  }
}
