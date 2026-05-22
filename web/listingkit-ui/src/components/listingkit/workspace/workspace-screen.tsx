"use client";

import { useSearchParams } from "next/navigation";

import { PlatformCardRail } from "@/components/listingkit/shared/platform-card-rail";
import { SheinFlowNav } from "@/components/listingkit/shein/shein-flow-nav";
import { ReviewReasonsCard } from "@/components/listingkit/review/review-reasons-card";
import { TaskRevisionHistoryPanel } from "@/components/listingkit/tasks/task-revision-history-panel";
import { TaskStatusPanel } from "@/components/listingkit/tasks/task-status-panel";
import { TaskProgressNotice } from "@/components/listingkit/tasks/task-progress-notice";
import { WorkspaceHeader } from "@/components/listingkit/workspace/workspace-header";
import { WorkspaceOverviewPanel } from "@/components/listingkit/workspace/workspace-overview-panel";
import {
  SheinFinalReviewWorkspaceView,
  WorkspaceReviewView,
} from "@/components/listingkit/workspace/workspace-screen-views";
import { SheinAdvancedReviewDetails } from "@/components/listingkit/workspace/shein-advanced-review-details";
import {
  buildSheinAdvancedReviewDetailsProps,
  buildSheinWorkspaceViewProps,
} from "@/components/listingkit/workspace/shein-workspace-view-props";
import { useSheinWorkspaceActions } from "@/components/listingkit/workspace/use-shein-workspace-actions";
import { useWorkspaceData } from "@/components/listingkit/workspace/use-workspace-data";
import { useWorkspaceNavigationActions } from "@/components/listingkit/workspace/use-workspace-navigation-actions";
import {
  WorkspaceLoadingState,
  WorkspaceLoadErrorState,
  WorkspacePendingDataState,
} from "@/components/listingkit/workspace/workspace-screen-states";
import { buildWorkspaceReviewViewProps } from "@/components/listingkit/workspace/workspace-review-view-props";
import { useApplyRevision } from "@/lib/query/use-apply-revision";
import {
  useRefreshSubmissionStatus,
  useSubmitTask,
} from "@/lib/query/use-submit-task";
import { useUpdateSheinFinalDraft } from "@/lib/query/use-shein-final-draft";
import { useClearSheinResolutionCache } from "@/lib/query/use-shein-resolution-cache";
import { useExecuteAction } from "@/lib/query/use-action";
import { useRetryChildTask } from "@/lib/query/use-child-task-retry";

export function WorkspaceScreen({ taskId }: { taskId: string }) {
  const searchParams = useSearchParams();
  const workspaceData = useWorkspaceData({ taskId, searchParams });
  const {
    baseQuery,
    preview,
    taskResult,
    session,
    reviewPreview,
    sessionData,
    platformCards,
    focusedPreview,
    selectedPlatform,
    focusedScenePreset,
    suppressResolvedActionSummary,
    resolvedActionSummary,
    previewSuggestion,
    sheinImages,
    sheinMockupImages,
    sheinVariantCount,
    sheinPreviewPayload,
    showSheinCategoryReview,
    showSheinAttributeReview,
    showSheinSaleAttributeReview,
    showSheinReviewDetails,
    shouldOpenSheinAdvancedDetails,
    isSheinFinalReviewMode,
    sheinFlowSteps,
    workspaceTitle,
    workspaceStatusLabel,
    workspaceUpdatedAt,
    workspaceSubtitle,
  } = workspaceData;
  const applyRevision = useApplyRevision(taskId);
  const submitTask = useSubmitTask(taskId);
  const refreshSubmissionStatus = useRefreshSubmissionStatus(taskId);
  const updateSheinFinalDraft = useUpdateSheinFinalDraft(taskId);
  const clearSheinResolutionCache = useClearSheinResolutionCache(taskId);
  const layerAction = useExecuteAction(taskId, baseQuery);
  const sheinActions = useSheinWorkspaceActions({
    taskId,
    sheinPreview: sheinPreviewPayload,
    preview,
    taskResult,
    applyRevision,
    submitTask,
    updateSheinFinalDraft,
  });
  const workspaceActions = useWorkspaceNavigationActions({
    taskId,
    baseQuery,
    searchParams,
    focusedTarget: session.data?.session?.focused_target,
  });
  const childTaskRetry = useRetryChildTask(taskId);
  const sheinViewProps = buildSheinWorkspaceViewProps({
    shein: preview.data?.shein,
    selectedPlatform,
    focusedPreview,
    sheinImages,
    sheinMockupImages,
    sheinVariantCount,
    sheinActions,
    isSavingFinalDraft: updateSheinFinalDraft.isPending,
    isSubmitting: submitTask.isPending,
    submitError: submitTask.error,
    clearingResolutionCacheKind: clearSheinResolutionCache.isPending
      ? clearSheinResolutionCache.variables
      : null,
    isRefreshingSubmissionStatus: refreshSubmissionStatus.isPending,
    onSelectBlockingItem: workspaceActions.handleSelectSheinBlockingItem,
    onRunPrimaryAction: workspaceActions.handleRunSheinPrimaryAction,
    onClearResolutionCache: (kind) => clearSheinResolutionCache.mutate(kind),
    onRefreshSubmissionStatus: () => refreshSubmissionStatus.mutate(),
  });
  const sheinAdvancedReviewDetailsProps = buildSheinAdvancedReviewDetailsProps({
    taskId,
    shein: preview.data?.shein,
    selectedPlatform,
    showReviewDetails: showSheinReviewDetails,
    showCategoryReview: showSheinCategoryReview,
    showAttributeReview: showSheinAttributeReview,
    showSaleAttributeReview: showSheinSaleAttributeReview,
    isFinalReviewMode: isSheinFinalReviewMode,
    open: shouldOpenSheinAdvancedDetails,
    isApplying: applyRevision.isPending,
    sheinActions,
  });
  const refetchWorkspace = () =>
    Promise.all([preview.refetch(), session.refetch(), taskResult.refetch()]);

  const handleRunStandardProductTemporal = () => {
    layerAction.mutate({
      action_key: "run_standard_product_temporal",
    });
  };

  const handleRunPlatformAdaptTemporal = () => {
    layerAction.mutate({
      action_key: "run_platform_adapt_temporal",
      target: {
        action_key: "run_platform_adapt_temporal",
        queue_query: {
          platform: "all",
        },
      },
    });
  };

  if (preview.isLoading || session.isLoading) {
    return <WorkspaceLoadingState />;
  }

  if (preview.isError || session.isError || taskResult.isError) {
    return <WorkspaceLoadErrorState onRetry={refetchWorkspace} />;
  }

  if (!preview.data || !sessionData) {
    return <WorkspacePendingDataState onRetry={refetchWorkspace} />;
  }

  const sheinAdvancedReviewDetails = sheinAdvancedReviewDetailsProps ? (
    <SheinAdvancedReviewDetails {...sheinAdvancedReviewDetailsProps} />
  ) : null;
  const shouldShowPlatformRail = platformCards.length > 1;
  const workspaceReviewViewProps = buildWorkspaceReviewViewProps({
    selectedPlatform,
    previewSuggestion,
    sessionData,
    reviewPreviewData: reviewPreview.data,
    taskResult: taskResult.data,
    focusedPreview,
    shein: preview.data?.shein,
    sheinViewProps,
    focusedScenePreset,
    recoveryDescriptors:
      session.data?.recovery_summary?.recommended_descriptors ??
      sessionData.overview?.recovery_summary?.recommended_descriptors,
    onDispatch: workspaceActions.dispatchTarget,
    onToolbarAction: workspaceActions.handleToolbarAction,
    onRecovery: workspaceActions.handleRecovery,
  });

  return (
    <div className="min-w-0 space-y-6 overflow-x-hidden">
      <WorkspaceHeader
        title={workspaceTitle}
        subtitle={workspaceSubtitle}
        statusLabel={workspaceStatusLabel}
        updatedAtLabel={workspaceUpdatedAt}
        summary={
          suppressResolvedActionSummary
            ? undefined
            : resolvedActionSummary
        }
        recoverySummary={
          sessionData?.overview?.recovery_summary ??
          session.data?.recovery_summary ??
          preview.data.asset_generation_overview?.recovery_summary
        }
        showSheinStudioLink={selectedPlatform === "shein"}
        showLayerActions
        layerActionsPending={layerAction.isPending}
        onRunStandardLayer={handleRunStandardProductTemporal}
        onRunPlatformLayer={handleRunPlatformAdaptTemporal}
        onSelectAction={(summary) => workspaceActions.handleAction(summary)}
        onSelectRecovery={workspaceActions.handleRecovery}
      />
      <TaskStatusPanel
        task={taskResult.data}
        onRetryChildTask={(kind) => childTaskRetry.mutate({ kind })}
        retryingChildTaskKind={childTaskRetry.isPending ? childTaskRetry.variables?.kind ?? null : null}
      />
      <ReviewReasonsCard task={taskResult.data} />
      <TaskProgressNotice task={taskResult.data} />

      {shouldShowPlatformRail ? (
        <PlatformCardRail
          cards={platformCards}
          selectedPlatform={sessionData.selected_platform}
          onSelect={(card) => {
            workspaceActions.dispatchTarget(
              card.primary_navigation_target ??
                card.resolved_action_summary?.navigation_target,
            );
            workspaceActions.handlePlatformSelect(card.platform);
          }}
          onSelectRecovery={(descriptor, card) =>
            workspaceActions.handlePlatformRecovery(descriptor, card.platform)
          }
        />
      ) : null}

      {selectedPlatform === "shein" ? (
        <SheinFlowNav
          eyebrow="SHEIN 审核流程"
          steps={sheinFlowSteps}
          title="先审核修复，再提交"
        />
      ) : null}
      {shouldOpenSheinAdvancedDetails ? sheinAdvancedReviewDetails : null}

      {isSheinFinalReviewMode ? (
        <SheinFinalReviewWorkspaceView
          taskId={taskId}
          imageGalleryProps={sheinViewProps.imageGalleryProps}
          finalReviewProps={sheinViewProps.finalReviewProps}
          readinessProps={sheinViewProps.finalModeReadinessProps}
          timelineProps={sheinViewProps.timelineProps}
        />
      ) : (
        <WorkspaceReviewView {...workspaceReviewViewProps} />
      )}

      {!shouldOpenSheinAdvancedDetails ? sheinAdvancedReviewDetails : null}
      <WorkspaceOverviewPanel
        overview={sessionData.overview}
        reviewSummary={sessionData.review_summary}
      />
      <TaskRevisionHistoryPanel taskId={taskId} defaultCollapsed />
    </div>
  );
}
