"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";

import {
  buildSheinWorkspaceHrefForAction,
  normalizeSheinFreshnessActionKey,
  isSheinWorkspaceActionKey,
  normalizeSheinWorkspaceActionKey,
  sheinWorkspaceSectionForAction,
  type SheinFreshnessActionKey,
  sheinWorkspaceTargetIdForKey,
} from "@/components/listingkit/shein/shein-workspace-actions";
import {
  deriveRecoveryNavigationTarget,
} from "@/components/listingkit/workspace/workspace-action-routing";
import { shouldSyncPlatformOnRecovery } from "@/components/listingkit/workspace/workspace-recovery-routing";
import {
  buildPlatformWorkspaceHref,
  buildProductWorkspaceHref,
  buildWorkspaceHistoryHref,
  buildWorkspaceSearch,
  shouldSyncFocusedTargetToRoute,
} from "@/components/listingkit/workspace/workspace-routing";
import { scrollSheinWorkspaceTarget } from "@/components/listingkit/workspace/workspace-screen-helpers";
import { useExecuteAction } from "@/lib/query/use-action";
import { useDispatchNavigation } from "@/lib/query/use-dispatch";
import type {
  ActionExecutionResult,
  ActionExecutionRequest,
  NavigationDispatchResponse,
  NavigationTarget,
  QueueQuery,
  RecoveryDescriptor,
  ResolvedActionSummary,
  ReviewTarget,
  SheinReadinessItem,
  ToolbarAction,
} from "@/lib/types/listingkit";
import type { ProductWorkspaceSectionKey } from "@/components/listingkit/workspace/product-workspace-model";

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

    const currentParams = new URLSearchParams(
      typeof window === "undefined" ? searchParams.toString() : window.location.search,
    );
    if (!shouldSyncFocusedTargetToRoute(currentParams.toString())) {
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

  const routeExplicitFocusedTarget = (target?: ReviewTarget) => {
    const currentSearch = searchParams.toString();
    if (!shouldRouteExplicitNavigation(currentSearch)) {
      return;
    }
    if (!target?.platform) {
      return;
    }
    const nextSearch = buildWorkspaceSearch(currentSearch, target);
    if (nextSearch === currentSearch) {
      return;
    }
    router.replace(
      `/listing-kits/${taskId}/workspace${nextSearch ? `?${nextSearch}` : ""}`,
    );
  };

  const dispatchTarget = (target?: NavigationTarget | null) => {
    if (!target) {
      return;
    }
    dispatch.mutate(target, {
      onSuccess: (result) => {
        routeExplicitFocusedTarget(
          focusedTargetFromDispatchResponse(result) ??
            reviewTargetFromNavigationTarget(target),
        );
      },
    });
  };

  const routeActionResult = (result: ActionExecutionResult) => {
    routeExplicitFocusedTarget(
      result.review_session?.focused_target ?? result.review_patch?.focused_target,
    );
  };

  const executeAction = (request: ActionExecutionRequest) => {
    action.mutate(request, {
      onSuccess: routeActionResult,
    });
  };

  const handleAction = (
    actionSummary?: ResolvedActionSummary | null,
    request?: ActionExecutionRequest,
  ) => {
    if (request) {
      executeAction(request);
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
      executeAction({
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
      executeAction({
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
    router.replace(
      buildPlatformWorkspaceHref(taskId, searchParams.toString(), platform),
    );
  };

  const handleProductSelect = (section: ProductWorkspaceSectionKey) => {
    router.replace(buildProductWorkspaceHref(taskId, searchParams.toString(), section));
  };

  const handleHistorySelect = () => {
    router.replace(buildWorkspaceHistoryHref(taskId, searchParams.toString()));
  };

  const handlePlatformRecovery = (
    descriptor: RecoveryDescriptor,
    platform: string,
  ) => {
    handleRecovery(descriptor);
    const currentParams = new URLSearchParams(searchParams.toString());
    if (
      shouldSyncPlatformOnRecovery(descriptor) ||
      currentParams.get("workspace_view") === "history" ||
      currentParams.has("product_section")
    ) {
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
    handleProductSelect,
    handleHistorySelect,
    handlePlatformRecovery,
    handleSelectSheinBlockingItem,
    handleRunSheinPrimaryAction,
  };
}

function shouldRouteExplicitNavigation(search: string) {
  const params = new URLSearchParams(search);
  return (
    params.has("product_section") ||
    params.get("section_key") === "general_review" ||
    params.get("section_key") === "final_review"
  );
}

function focusedTargetFromDispatchResponse(
  result: NavigationDispatchResponse,
): ReviewTarget | undefined {
  return (
    result.panel_update?.focused_target ??
    result.review_session?.session?.focused_target ??
    result.review_session?.patch?.focused_target ??
    result.review_preview?.review_target ??
    result.panel_update?.review_patch?.focused_target ??
    result.panel_update?.review_session?.session?.focused_target ??
    result.panel_update?.review_session?.patch?.focused_target ??
    result.panel_update?.review_preview?.review_target
  );
}

function reviewTargetFromNavigationTarget(
  target: NavigationTarget,
): ReviewTarget | undefined {
  const nestedTarget = target.action_target?.navigation_target;
  if (nestedTarget) {
    const nestedReviewTarget = reviewTargetFromNavigationTarget(nestedTarget);
    if (nestedReviewTarget) {
      return nestedReviewTarget;
    }
  }
  const query =
    target.preview_query ??
    target.session_query ??
    target.action_target?.queue_query ??
    target.queue_query;
  if (!query?.platform) {
    return undefined;
  }
  return {
    platform: query.platform,
    slot: query.slot,
    capability: query.preview_capability,
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
  const isOnTargetWorkspaceSurface =
    currentParams.get("platform") === "shein" &&
    currentParams.get("section_key") ===
      sheinWorkspaceSectionForAction(normalizedKey);
  if (!isOnTargetWorkspaceSurface) {
    router.replace(buildSheinWorkspaceHrefForAction(taskId, normalizedKey));
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
