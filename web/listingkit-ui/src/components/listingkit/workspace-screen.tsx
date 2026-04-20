"use client";

import { useEffect, useMemo } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { LoaderCircle } from "lucide-react";

import { PlatformCardRail } from "@/components/listingkit/platform-card-rail";
import { PreviewCanvas } from "@/components/listingkit/preview-canvas";
import { RecoveryActionList } from "@/components/listingkit/recovery-action-list";
import { ReviewSectionTabs } from "@/components/listingkit/review-section-tabs";
import { ReviewToolbar } from "@/components/listingkit/review-toolbar";
import { ScenePresetPanel } from "@/components/listingkit/scene-preset-panel";
import { SlotNavigationList } from "@/components/listingkit/slot-navigation-list";
import { TaskStatusPanel } from "@/components/listingkit/task-status-panel";
import { TaskProgressNotice } from "@/components/listingkit/task-progress-notice";
import { resolveWorkspaceScenePreset } from "@/components/listingkit/workspace-scene-preset";
import {
  deriveTaskPreviewEmptyState,
  shouldSuppressResolvedActionSummary,
} from "@/components/listingkit/task-status-display";
import { WorkspaceHeader } from "@/components/listingkit/workspace-header";
import { WorkspaceOverviewPanel } from "@/components/listingkit/workspace-overview-panel";
import { shouldSyncPlatformOnRecovery } from "@/components/listingkit/workspace-recovery-routing";
import { buildWorkspaceSearch } from "@/components/listingkit/workspace-routing";
import { EmptyState } from "@/components/shared/empty-state";
import { useExecuteAction } from "@/lib/query/use-action";
import { useDispatchNavigation } from "@/lib/query/use-dispatch";
import { useListingKitPreview } from "@/lib/query/use-preview";
import { useReviewPreview } from "@/lib/query/use-review-preview";
import { useReviewSession } from "@/lib/query/use-review-session";
import { useListingKitTaskResult } from "@/lib/query/use-task-result";
import type {
  ActionExecutionRequest,
  NavigationTarget,
  QueueQuery,
  RecoveryDescriptor,
  ResolvedActionSummary,
  ReviewSection,
  ReviewSlot,
  ToolbarAction,
} from "@/lib/types/listingkit";

function queryFromSearchParams(searchParams: URLSearchParams): QueueQuery {
  return {
    platform: searchParams.get("platform") ?? undefined,
    slot: searchParams.get("slot") ?? undefined,
    preview_capability: searchParams.get("preview_capability") ?? undefined,
    response_mode: searchParams.get("response_mode") ?? undefined,
  };
}

export function WorkspaceScreen({ taskId }: { taskId: string }) {
  const router = useRouter();
  const searchParams = useSearchParams();
  const baseQuery = useMemo(
    () => queryFromSearchParams(searchParams),
    [searchParams],
  );

  const preview = useListingKitPreview(taskId);
  const taskResult = useListingKitTaskResult(taskId);
  const session = useReviewSession(taskId, baseQuery);
  const focusedPreviewQuery =
    session.data?.session?.focused_target?.navigation_target?.preview_query;
  const reviewPreview = useReviewPreview(
    taskId,
    focusedPreviewQuery ?? baseQuery,
    Boolean(focusedPreviewQuery ?? baseQuery.slot ?? baseQuery.platform),
  );
  const dispatch = useDispatchNavigation(taskId, baseQuery);
  const action = useExecuteAction(taskId, baseQuery);

  const sessionData = session.data?.session;
  const platformCards =
    sessionData?.platform_cards ?? preview.data?.overview?.platform_cards ?? [];
  const focusedPreview =
    reviewPreview.data?.preview ?? sessionData?.focused_render_preview;
  const focusedScenePreset = resolveWorkspaceScenePreset({
    reviewPreviewPreset: reviewPreview.data?.scene_preset,
    focusedScenePreset: sessionData?.focused_scene_preset,
    queueItems: sessionData?.queue?.items,
    selectedSlot: sessionData?.selected_slot,
    focusedAssetId: focusedPreview?.asset_id,
  });
  const suppressResolvedActionSummary = shouldSuppressResolvedActionSummary(
    taskResult.data,
    {
      hasPreviewSvg: Boolean(focusedPreview?.preview_svg),
      queueTotal: sessionData?.queue?.summary?.total_items ?? 0,
    },
  );

  useEffect(() => {
    const focusedTarget = session.data?.session?.focused_target;
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
  }, [router, searchParams, session.data?.session?.focused_target, taskId]);

  const handleDispatch = (target?: NavigationTarget | null) => {
    if (!target) return;
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

    handleDispatch(actionSummary?.navigation_target);
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

    handleDispatch(
      toolbarAction.navigation_target ?? toolbarAction.target?.navigation_target,
    );
  };

  const handleRecovery = (descriptor: RecoveryDescriptor) => {
    if (descriptor.recovery_target) {
      handleDispatch(descriptor.recovery_target);
    }
  };

  const handlePlatformSelect = (platform: string) => {
    const params = new URLSearchParams(searchParams.toString());
    params.set("platform", platform);
    router.replace(`/listing-kits/${taskId}/workspace?${params.toString()}`);
  };

  if (preview.isLoading || session.isLoading) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <LoaderCircle className="h-8 w-8 animate-spin text-zinc-400" />
      </div>
    );
  }

  if (!preview.data || !sessionData) {
    return (
      <EmptyState
        title="Workspace unavailable"
        description="The task did not return preview and review session data."
      />
    );
  }

  return (
    <div className="space-y-6">
      <WorkspaceHeader
        title={`Task ${taskId}`}
        summary={
          suppressResolvedActionSummary
            ? undefined
            : (session.data?.resolved_action_summary ??
              preview.data.asset_generation_overview?.resolved_action_summary)
        }
        recoverySummary={
          session.data?.recovery_summary ??
          preview.data.asset_generation_overview?.recovery_summary
        }
        onSelectAction={(summary) => handleAction(summary)}
        onSelectRecovery={handleRecovery}
      />
      <TaskStatusPanel task={taskResult.data} />
      <TaskProgressNotice task={taskResult.data} />
      <WorkspaceOverviewPanel
        overview={sessionData.overview}
        reviewSummary={sessionData.review_summary}
      />

      <div className="grid gap-6 xl:grid-cols-[20rem_minmax(0,1fr)_24rem]">
        <aside className="space-y-4">
          <PlatformCardRail
            cards={platformCards}
            selectedPlatform={sessionData.selected_platform}
            onSelect={(card) => {
              handleDispatch(
                card.primary_navigation_target ??
                  card.resolved_action_summary?.navigation_target,
              );
              handlePlatformSelect(card.platform);
            }}
            onSelectRecovery={(descriptor, card) => {
              handleRecovery(descriptor);
              if (shouldSyncPlatformOnRecovery(descriptor)) {
                handlePlatformSelect(card.platform);
              }
            }}
          />
          <SlotNavigationList
            slots={sessionData.slot_navigation}
            selectedSlot={sessionData.selected_slot}
            onSelect={(slot: ReviewSlot) =>
              handleDispatch(slot.review_target?.navigation_target)
            }
          />
        </aside>

        <main className="space-y-4">
          <ReviewSectionTabs
            sections={sessionData.sections}
            selectedKey={sessionData.focused_section_key}
            onSelect={(section: ReviewSection) =>
              handleDispatch(section.review_target?.navigation_target)
            }
          />
          <PreviewCanvas
            preview={focusedPreview}
            response={reviewPreview.data}
            emptyState={deriveTaskPreviewEmptyState(taskResult.data)}
          />
        </main>

        <aside className="space-y-4">
          <ReviewToolbar
            toolbar={reviewPreview.data?.toolbar ?? sessionData.focused_toolbar}
            onAction={handleToolbarAction}
          />
          <ScenePresetPanel summary={focusedScenePreset} />
          <RecoveryActionList
            descriptors={
              session.data?.recovery_summary?.recommended_descriptors ??
              sessionData.overview?.recovery_summary?.recommended_descriptors
            }
            onSelect={handleRecovery}
          />
        </aside>
      </div>
    </div>
  );
}
