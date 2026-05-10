"use client";

import Link from "next/link";
import { useSearchParams } from "next/navigation";
import { LoaderCircle } from "lucide-react";

import { PlatformCardRail } from "@/components/listingkit/shared/platform-card-rail";
import { SheinFlowNav } from "@/components/listingkit/shein/shein-flow-nav";
import { ReviewReasonsCard } from "@/components/listingkit/review/review-reasons-card";
import { TaskStatusPanel } from "@/components/listingkit/tasks/task-status-panel";
import { TaskProgressNotice } from "@/components/listingkit/tasks/task-progress-notice";
import { deriveTaskPreviewEmptyState } from "@/components/listingkit/tasks/task-status-display";
import { WorkspaceHeader } from "@/components/listingkit/workspace/workspace-header";
import { WorkspaceOverviewPanel } from "@/components/listingkit/workspace/workspace-overview-panel";
import {
  SheinFinalReviewWorkspaceView,
  WorkspaceReviewView,
} from "@/components/listingkit/workspace/workspace-screen-views";
import { SheinAdvancedReviewDetails } from "@/components/listingkit/workspace/shein-advanced-review-details";
import { buildSheinWorkspaceViewProps } from "@/components/listingkit/workspace/shein-workspace-view-props";
import { useSheinWorkspaceActions } from "@/components/listingkit/workspace/use-shein-workspace-actions";
import { useWorkspaceData } from "@/components/listingkit/workspace/use-workspace-data";
import { useWorkspaceNavigationActions } from "@/components/listingkit/workspace/use-workspace-navigation-actions";
import { EmptyState } from "@/components/shared/empty-state";
import { useApplyRevision } from "@/lib/query/use-apply-revision";
import {
  useRefreshSubmissionStatus,
  useSubmitTask,
} from "@/lib/query/use-submit-task";
import { useUpdateSheinFinalDraft } from "@/lib/query/use-shein-final-draft";
import { useClearSheinResolutionCache } from "@/lib/query/use-shein-resolution-cache";

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

  if (preview.isLoading || session.isLoading) {
    return (
      <div className="flex min-h-[60vh] items-center justify-center">
        <LoaderCircle className="h-8 w-8 animate-spin text-zinc-400" />
      </div>
    );
  }

  if (preview.isError || session.isError || taskResult.isError) {
    return (
      <EmptyState
        title="工作台暂时无法加载"
        description="当前无法完整读取任务状态、预览或审核会话。你可以刷新重试，或先回到任务列表重新进入。"
        action={
          <div className="flex flex-wrap gap-3">
            <button
              className="inline-flex h-10 items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800"
              onClick={() =>
                Promise.all([
                  preview.refetch(),
                  session.refetch(),
                  taskResult.refetch(),
                ])
              }
              type="button"
            >
              刷新当前页面
            </button>
            <Link
              href="/listing-kits"
              className="inline-flex h-10 items-center justify-center rounded-xl bg-white px-4 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100"
            >
              返回任务列表
            </Link>
          </div>
        }
      />
    );
  }

  if (!preview.data || !sessionData) {
    return (
      <EmptyState
        title="工作台数据暂未准备完成"
        description="当前任务还没有返回完整的预览和审核会话数据。可以稍后刷新，或先回到任务列表查看任务状态。"
        action={
          <div className="flex flex-wrap gap-3">
            <button
              className="inline-flex h-10 items-center justify-center rounded-xl bg-zinc-950 px-4 text-sm font-medium text-white transition hover:bg-zinc-800"
              onClick={() =>
                Promise.all([
                  preview.refetch(),
                  session.refetch(),
                  taskResult.refetch(),
                ])
              }
              type="button"
            >
              重新加载
            </button>
            <Link
              href="/listing-kits"
              className="inline-flex h-10 items-center justify-center rounded-xl bg-white px-4 text-sm font-medium text-zinc-900 ring-1 ring-zinc-200 transition hover:bg-zinc-100"
            >
              返回任务列表
            </Link>
          </div>
        }
      />
    );
  }

  const sheinAdvancedReviewDetails =
    selectedPlatform === "shein" && showSheinReviewDetails && !isSheinFinalReviewMode ? (
      <SheinAdvancedReviewDetails
        open={shouldOpenSheinAdvancedDetails}
        showCategoryReview={showSheinCategoryReview}
        showAttributeReview={showSheinAttributeReview}
        showSaleAttributeReview={showSheinSaleAttributeReview}
        categoryReviewProps={{
          taskId,
          editorContext: preview.data?.shein?.editor_context,
          isApplying: applyRevision.isPending,
          onApplySuggestedCategory: sheinActions.handleApplySuggestedSheinCategory,
          onConfirmCurrentCategory: sheinActions.handleConfirmCurrentSheinCategory,
          onApplyManualCategory: sheinActions.handleApplyManualSheinCategory,
        }}
        attributeReviewProps={{
          editorContext: preview.data?.shein?.editor_context,
          isApplying: applyRevision.isPending,
          onConfirmAttributes: sheinActions.handleConfirmSheinAttributes,
          onConfirmFallbackAttributes: sheinActions.handleConfirmSheinFallbackAttributes,
        }}
        saleAttributeReviewProps={{
          editorContext: preview.data?.shein?.editor_context,
          isApplying: applyRevision.isPending,
          onConfirmCurrentSaleAttributes: sheinActions.handleConfirmCurrentSheinSaleAttributes,
        }}
      />
    ) : null;

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
        onSelectAction={(summary) => workspaceActions.handleAction(summary)}
        onSelectRecovery={workspaceActions.handleRecovery}
      />
      <TaskStatusPanel task={taskResult.data} />
      <ReviewReasonsCard task={taskResult.data} />
      <TaskProgressNotice task={taskResult.data} />
      <WorkspaceOverviewPanel
        overview={sessionData.overview}
        reviewSummary={sessionData.review_summary}
      />

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
        <WorkspaceReviewView
          selectedPlatform={selectedPlatform}
          previewSuggestionProps={{
            suggestion: previewSuggestion,
            onSelect: (slot) =>
              workspaceActions.dispatchTarget(slot.review_target?.navigation_target),
          }}
          reviewSectionTabsProps={{
            sections: sessionData.sections,
            selectedKey: sessionData.focused_section_key,
            onSelect: (section) =>
              workspaceActions.dispatchTarget(section.review_target?.navigation_target),
          }}
          sheinSourceProductProps={{ shein: preview.data?.shein }}
          sheinImageGalleryProps={sheinViewProps.imageGalleryProps}
          sheinFinalReviewProps={sheinViewProps.finalReviewProps}
          previewCanvasProps={{
            preview: sheinViewProps.sheinFallbackPreview ?? focusedPreview,
            response: reviewPreview.data,
            emptyState: deriveTaskPreviewEmptyState(taskResult.data),
          }}
          slotNavigationProps={{
            slots: sessionData.slot_navigation,
            selectedSlot: sessionData.selected_slot,
            selectedAssetId: focusedPreview?.asset_id,
            onSelect: (slot) =>
              workspaceActions.dispatchTarget(slot.review_target?.navigation_target),
          }}
          reviewToolbarProps={{
            toolbar: reviewPreview.data?.toolbar ?? sessionData.focused_toolbar,
            onAction: workspaceActions.handleToolbarAction,
          }}
          sheinReadinessProps={sheinViewProps.reviewModeReadinessProps}
          sheinTimelineProps={sheinViewProps.timelineProps}
          scenePresetPanelProps={{ summary: focusedScenePreset }}
          recoveryActionListProps={{
            descriptors:
              session.data?.recovery_summary?.recommended_descriptors ??
              sessionData.overview?.recovery_summary?.recommended_descriptors,
            onSelect: workspaceActions.handleRecovery,
          }}
        />
      )}

      {!shouldOpenSheinAdvancedDetails ? sheinAdvancedReviewDetails : null}
    </div>
  );
}
