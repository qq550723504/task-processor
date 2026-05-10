"use client";

import { useEffect } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { LoaderCircle } from "lucide-react";

import { PlatformCardRail } from "@/components/listingkit/shared/platform-card-rail";
import { SheinFlowNav } from "@/components/listingkit/shein/shein-flow-nav";
import {
  canSelectSheinReadinessItem,
  isSheinWorkspaceActionKey,
  normalizeSheinWorkspaceActionKey,
  sheinWorkspaceTargetIdForKey,
} from "@/components/listingkit/shein/shein-workspace-actions";
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
import { useSheinWorkspaceActions } from "@/components/listingkit/workspace/use-shein-workspace-actions";
import { useWorkspaceData } from "@/components/listingkit/workspace/use-workspace-data";
import { deriveRecoveryNavigationTarget } from "@/components/listingkit/workspace/workspace-action-routing";
import { shouldSyncPlatformOnRecovery } from "@/components/listingkit/workspace/workspace-recovery-routing";
import { buildWorkspaceSearch } from "@/components/listingkit/workspace/workspace-routing";
import { EmptyState } from "@/components/shared/empty-state";
import {
  scrollSheinWorkspaceTarget,
  submitErrorMessage,
} from "@/components/listingkit/workspace/workspace-screen-helpers";
import { useExecuteAction } from "@/lib/query/use-action";
import { useDispatchNavigation } from "@/lib/query/use-dispatch";
import { useApplyRevision } from "@/lib/query/use-apply-revision";
import {
  useRefreshSubmissionStatus,
  useSubmitTask,
} from "@/lib/query/use-submit-task";
import { useUpdateSheinFinalDraft } from "@/lib/query/use-shein-final-draft";
import { useClearSheinResolutionCache } from "@/lib/query/use-shein-resolution-cache";
import type {
  ActionExecutionRequest,
  NavigationTarget,
  RecoveryDescriptor,
  ResolvedActionSummary,
  SheinReadinessItem,
  ToolbarAction,
} from "@/lib/types/listingkit";
import { toImageProxyUrl } from "@/lib/utils/image-proxy-url";
import { sanitizedNavigationSearchParams } from "@/lib/utils/navigation-query";
import { formatSheinSubmitError } from "@/lib/utils/shein-submit-error";

export function WorkspaceScreen({ taskId }: { taskId: string }) {
  const router = useRouter();
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
  const dispatch = useDispatchNavigation(taskId, baseQuery);
  const action = useExecuteAction(taskId, baseQuery);
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
  const sheinDisplayImages =
    sheinImages.length > 0 ? sheinImages : sheinMockupImages;
  const selectedSheinImage =
    sheinDisplayImages.find((image) => image.url === sheinActions.selectedSheinImageUrl) ??
    sheinDisplayImages[0];
  const sheinFallbackPreview =
    selectedPlatform === "shein" && !focusedPreview?.asset_url && !focusedPreview?.preview_svg
      ? {
          asset_url: toImageProxyUrl(selectedSheinImage?.url),
          template_label: selectedSheinImage?.label ?? "SHEIN product image",
          asset_id: selectedSheinImage?.id ?? "shein-product-image",
        }
      : undefined;

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
    const target = deriveRecoveryNavigationTarget(descriptor);
    if (target) {
      handleDispatch(target);
    }
  };

  const canSelectSheinBlockingItem = (item: SheinReadinessItem) =>
    canSelectSheinReadinessItem(item);

  const handleSelectSheinBlockingItem = (item: SheinReadinessItem) => {
    const normalizedKey = normalizeSheinWorkspaceActionKey(item.key);
    if (!normalizedKey) {
      return;
    }
    const targetId = sheinWorkspaceTargetIdForKey(normalizedKey);
    scrollSheinWorkspaceTarget(normalizedKey, targetId);
  };

  const handleRunSheinPrimaryAction = (key?: string | null) => {
    const normalizedKey = normalizeSheinWorkspaceActionKey(key);
    if (!normalizedKey) {
      return;
    }
    const targetId = sheinWorkspaceTargetIdForKey(normalizedKey);
    scrollSheinWorkspaceTarget(normalizedKey, targetId);
  };

  const handlePlatformSelect = (platform: string) => {
    const params = sanitizedNavigationSearchParams(searchParams);
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
        onSelectAction={(summary) => handleAction(summary)}
        onSelectRecovery={handleRecovery}
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
          imageGalleryProps={{
            images: sheinImages,
            mockupImages: sheinMockupImages,
            finalImages: preview.data?.shein?.final_review?.images,
            variantCount: sheinVariantCount,
            isSavingControls: updateSheinFinalDraft.isPending,
            saveErrorMessage: sheinActions.sheinFinalDraftError,
            saveMessage: sheinActions.sheinFinalDraftMessage,
            selectedUrl: selectedSheinImage?.url,
            onSelect: (image) => sheinActions.setSelectedSheinImageUrl(image.url),
            onSaveImageControls: (payload) =>
              sheinActions.handleSaveSheinFinalDraft(
                payload,
                "图片设置已保存，最终提交会使用当前排序和角色。",
              ),
            onRegenerate: sheinActions.handleRegenerateSheinImage,
            isRegenerating: sheinActions.regeneratingSheinImage,
            regenerationError: sheinActions.sheinImageRegenerationError,
          }}
          finalReviewProps={{
            shein: preview.data?.shein,
            isSaving: updateSheinFinalDraft.isPending,
            isSubmitting: submitTask.isPending,
            saveErrorMessage: sheinActions.sheinFinalDraftError,
            saveMessage: sheinActions.sheinFinalDraftMessage,
            submitAction: sheinActions.sheinSubmitAction,
            submitErrorMessage: formatSheinSubmitError(
              submitTask.error,
              preview.data?.shein,
            ),
            canSelectBlockingItem: canSelectSheinBlockingItem,
            onSaveFinalDraft: (payload) =>
              sheinActions.handleSaveSheinFinalDraft(
                payload,
                "最终草稿已确认。资料就绪后可以保存草稿或发布。",
              ),
            onSelectBlockingItem: handleSelectSheinBlockingItem,
            onSubmit: sheinActions.handleSubmitShein,
          }}
          readinessProps={{
            readiness: preview.data?.shein?.submit_readiness,
            checklist: preview.data?.shein?.submit_checklist,
            submission: preview.data?.shein?.submission,
            imageUpload: preview.data?.shein?.image_upload,
            resolutionCache: preview.data?.shein?.resolution_cache,
            workspaceOverview: preview.data?.shein?.workspace_overview,
            canSelectBlockingItem: canSelectSheinBlockingItem,
            onSelectBlockingItem: handleSelectSheinBlockingItem,
            canRunPrimaryAction: isSheinWorkspaceActionKey,
            onRunPrimaryAction: handleRunSheinPrimaryAction,
            canSubmit:
              preview.data?.shein?.submit_readiness?.ready === true &&
              preview.data?.shein?.final_review?.confirmed === true,
            isSubmitting: submitTask.isPending,
            submitAction: sheinActions.sheinSubmitAction,
            submitErrorMessage: formatSheinSubmitError(
              submitTask.error,
              preview.data?.shein,
            ),
            onSubmit: () => sheinActions.handleSubmitShein("publish"),
            onSaveDraft: sheinActions.handleSaveSheinDraft,
            clearingResolutionCacheKind: clearSheinResolutionCache.isPending
              ? clearSheinResolutionCache.variables
              : null,
            onClearResolutionCache: (kind) =>
              clearSheinResolutionCache.mutate(kind),
          }}
          timelineProps={{
            events: preview.data?.shein?.submission_events,
            canRefresh: Boolean(preview.data?.shein?.submission?.last_action),
            isRefreshing: refreshSubmissionStatus.isPending,
            onRefresh: () => refreshSubmissionStatus.mutate(),
          }}
        />
      ) : (
        <WorkspaceReviewView
          selectedPlatform={selectedPlatform}
          previewSuggestionProps={{
            suggestion: previewSuggestion,
            onSelect: (slot) =>
              handleDispatch(slot.review_target?.navigation_target),
          }}
          reviewSectionTabsProps={{
            sections: sessionData.sections,
            selectedKey: sessionData.focused_section_key,
            onSelect: (section) =>
              handleDispatch(section.review_target?.navigation_target),
          }}
          sheinSourceProductProps={{ shein: preview.data?.shein }}
          sheinImageGalleryProps={{
            images: sheinImages,
            mockupImages: sheinMockupImages,
            finalImages: preview.data?.shein?.final_review?.images,
            variantCount: sheinVariantCount,
            isSavingControls: updateSheinFinalDraft.isPending,
            saveErrorMessage: sheinActions.sheinFinalDraftError,
            saveMessage: sheinActions.sheinFinalDraftMessage,
            selectedUrl: selectedSheinImage?.url,
            onSelect: (image) => sheinActions.setSelectedSheinImageUrl(image.url),
            onSaveImageControls: (payload) =>
              sheinActions.handleSaveSheinFinalDraft(
                payload,
                "图片设置已保存，最终提交会使用当前排序和角色。",
              ),
            onRegenerate: sheinActions.handleRegenerateSheinImage,
            isRegenerating: sheinActions.regeneratingSheinImage,
            regenerationError: sheinActions.sheinImageRegenerationError,
          }}
          sheinFinalReviewProps={{
            shein: preview.data?.shein,
            isSaving: updateSheinFinalDraft.isPending,
            isSubmitting: submitTask.isPending,
            saveErrorMessage: sheinActions.sheinFinalDraftError,
            saveMessage: sheinActions.sheinFinalDraftMessage,
            submitAction: sheinActions.sheinSubmitAction,
            submitErrorMessage: formatSheinSubmitError(
              submitTask.error,
              preview.data?.shein,
            ),
            canSelectBlockingItem: canSelectSheinBlockingItem,
            onSaveFinalDraft: (payload) =>
              sheinActions.handleSaveSheinFinalDraft(
                payload,
                "最终草稿已确认。资料就绪后可以保存草稿或发布。",
              ),
            onSelectBlockingItem: handleSelectSheinBlockingItem,
            onSubmit: sheinActions.handleSubmitShein,
          }}
          previewCanvasProps={{
            preview: sheinFallbackPreview ?? focusedPreview,
            response: reviewPreview.data,
            emptyState: deriveTaskPreviewEmptyState(taskResult.data),
          }}
          slotNavigationProps={{
            slots: sessionData.slot_navigation,
            selectedSlot: sessionData.selected_slot,
            selectedAssetId: focusedPreview?.asset_id,
            onSelect: (slot) =>
              handleDispatch(slot.review_target?.navigation_target),
          }}
          reviewToolbarProps={{
            toolbar: reviewPreview.data?.toolbar ?? sessionData.focused_toolbar,
            onAction: handleToolbarAction,
          }}
          sheinReadinessProps={{
            readiness: preview.data?.shein?.submit_readiness,
            checklist: preview.data?.shein?.submit_checklist,
            submission: preview.data?.shein?.submission,
            imageUpload: preview.data?.shein?.image_upload,
            resolutionCache: preview.data?.shein?.resolution_cache,
            workspaceOverview: preview.data?.shein?.workspace_overview,
            canSelectBlockingItem: canSelectSheinBlockingItem,
            onSelectBlockingItem: handleSelectSheinBlockingItem,
            canRunPrimaryAction: isSheinWorkspaceActionKey,
            onRunPrimaryAction: handleRunSheinPrimaryAction,
            canSubmit:
              preview.data?.shein?.submit_readiness?.ready === true &&
              preview.data?.shein?.final_review?.confirmed === true,
            isSubmitting: submitTask.isPending,
            submitAction: sheinActions.sheinSubmitAction,
            submitErrorMessage: submitErrorMessage(submitTask.error),
            onSubmit: () => sheinActions.handleSubmitShein("publish"),
            onSaveDraft: sheinActions.handleSaveSheinDraft,
            clearingResolutionCacheKind: clearSheinResolutionCache.isPending
              ? clearSheinResolutionCache.variables
              : null,
            onClearResolutionCache: (kind) =>
              clearSheinResolutionCache.mutate(kind),
          }}
          sheinTimelineProps={{
            events: preview.data?.shein?.submission_events,
            canRefresh: Boolean(preview.data?.shein?.submission?.last_action),
            isRefreshing: refreshSubmissionStatus.isPending,
            onRefresh: () => refreshSubmissionStatus.mutate(),
          }}
          scenePresetPanelProps={{ summary: focusedScenePreset }}
          recoveryActionListProps={{
            descriptors:
              session.data?.recovery_summary?.recommended_descriptors ??
              sessionData.overview?.recovery_summary?.recommended_descriptors,
            onSelect: handleRecovery,
          }}
        />
      )}

      {!shouldOpenSheinAdvancedDetails ? sheinAdvancedReviewDetails : null}
    </div>
  );
}
