"use client";

import { useEffect, useMemo } from "react";
import Link from "next/link";
import { useRouter, useSearchParams } from "next/navigation";
import { LoaderCircle } from "lucide-react";

import { PlatformCardRail } from "@/components/listingkit/shared/platform-card-rail";
import { SheinCategoryReviewCard } from "@/components/listingkit/shein/shein-category-review-card";
import { SheinAttributeReviewCard } from "@/components/listingkit/shein/shein-attribute-review-card";
import {
  SheinFlowNav,
  type SheinFlowStep,
} from "@/components/listingkit/shein/shein-flow-nav";
import { SheinSaleAttributeReviewCard } from "@/components/listingkit/shein/shein-sale-attribute-review-card";
import { collectSheinPreviewImageGroups } from "@/components/listingkit/shein/shein-preview-image";
import {
  canSelectSheinReadinessItem,
  isSheinWorkspaceActionKey,
  normalizeSheinWorkspaceActionKey,
  sheinWorkspaceTargetIdForKey,
} from "@/components/listingkit/shein/shein-workspace-actions";
import { ReviewReasonsCard } from "@/components/listingkit/review/review-reasons-card";
import { TaskStatusPanel } from "@/components/listingkit/tasks/task-status-panel";
import { TaskProgressNotice } from "@/components/listingkit/tasks/task-progress-notice";
import { deriveWorkspacePreviewSuggestion } from "@/components/listingkit/workspace/workspace-preview-routing";
import { resolveWorkspaceScenePreset } from "@/components/listingkit/workspace/workspace-scene-preset";
import {
  deriveTaskPreviewEmptyState,
  shouldSuppressResolvedActionSummary,
} from "@/components/listingkit/tasks/task-status-display";
import { WorkspaceHeader } from "@/components/listingkit/workspace/workspace-header";
import { WorkspaceOverviewPanel } from "@/components/listingkit/workspace/workspace-overview-panel";
import {
  SheinFinalReviewWorkspaceView,
  WorkspaceReviewView,
} from "@/components/listingkit/workspace/workspace-screen-views";
import { useSheinWorkspaceActions } from "@/components/listingkit/workspace/use-shein-workspace-actions";
import {
  deriveRecoveryNavigationTarget,
  pickWorkspaceResolvedActionSummary,
} from "@/components/listingkit/workspace/workspace-action-routing";
import { shouldSyncPlatformOnRecovery } from "@/components/listingkit/workspace/workspace-recovery-routing";
import { buildWorkspaceSearch } from "@/components/listingkit/workspace/workspace-routing";
import { EmptyState } from "@/components/shared/empty-state";
import {
  formatWorkspaceDate,
  hasSheinAttributeReviewSignal,
  hasSheinCategoryReviewSignal,
  hasSheinSaleAttributeReviewSignal,
  queryFromSearchParams,
  scrollSheinWorkspaceTarget,
  selectedPlatformFromReviewTarget,
  submitErrorMessage,
  workspaceTaskStatusLabel,
} from "@/components/listingkit/workspace/workspace-screen-helpers";
import { useExecuteAction } from "@/lib/query/use-action";
import { useDispatchNavigation } from "@/lib/query/use-dispatch";
import { useListingKitPreview } from "@/lib/query/use-preview";
import { useReviewPreview } from "@/lib/query/use-review-preview";
import { useReviewSession } from "@/lib/query/use-review-session";
import { useListingKitTaskResult } from "@/lib/query/use-task-result";
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
  const applyRevision = useApplyRevision(taskId);
  const submitTask = useSubmitTask(taskId);
  const refreshSubmissionStatus = useRefreshSubmissionStatus(taskId);
  const updateSheinFinalDraft = useUpdateSheinFinalDraft(taskId);
  const clearSheinResolutionCache = useClearSheinResolutionCache(taskId);
  const sheinPreviewPayload = preview.data?.shein;
  const sheinActions = useSheinWorkspaceActions({
    taskId,
    sheinPreview: sheinPreviewPayload,
    preview,
    taskResult,
    applyRevision,
    submitTask,
    updateSheinFinalDraft,
  });

  const sessionData = session.data?.session;
  const platformCards =
    sessionData?.platform_cards ?? preview.data?.overview?.platform_cards ?? [];
  const focusedPreview =
    reviewPreview.data?.preview ?? sessionData?.focused_render_preview;
  const selectedPlatform =
    sessionData?.selected_platform ??
    selectedPlatformFromReviewTarget(sessionData?.focused_target) ??
    selectedPlatformFromReviewTarget(sessionData?.default_target) ??
    (platformCards.length === 1 ? platformCards[0]?.platform : undefined) ??
    (preview.data?.platforms?.length === 1 ? preview.data.platforms[0] : undefined) ??
    preview.data?.selected_platform;
  const focusedScenePreset = resolveWorkspaceScenePreset({
    reviewPreviewPreset: reviewPreview.data?.scene_preset,
    focusedScenePreset: sessionData?.focused_scene_preset,
    previewScenePresets: {
      amazon: preview.data?.amazon?.scene_presets,
      shein: preview.data?.shein?.scene_presets,
      temu: preview.data?.temu?.scene_presets,
      walmart: preview.data?.walmart?.scene_presets,
    },
    queueItems: sessionData?.queue?.items,
    selectedPlatform,
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
  const resolvedActionSummary = pickWorkspaceResolvedActionSummary(
    sessionData?.overview?.resolved_action_summary ??
      session.data?.resolved_action_summary,
    preview.data?.asset_generation_overview?.resolved_action_summary,
  );
  const previewSuggestion = deriveWorkspacePreviewSuggestion({
    slots: sessionData?.slot_navigation,
    selectedSlot: sessionData?.selected_slot,
    focusedPreview,
  });
  const sheinImageGroups = useMemo(
    () =>
      collectSheinPreviewImageGroups(
        preview.data?.shein,
        taskResult.data?.result?.sds_sync,
      ),
    [preview.data?.shein, taskResult.data?.result?.sds_sync],
  );
  const sheinImages = sheinImageGroups.productImages;
  const sheinMockupImages = sheinImageGroups.mockupImages;
  const sheinVariantCount =
    preview.data?.shein?.final_review?.skus?.length ??
    preview.data?.overview?.variant_count;
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
  const sheinEditorContext = sheinPreviewPayload?.editor_context;
  const showSheinCategoryReview =
    hasSheinCategoryReviewSignal(sheinEditorContext);
  const showSheinAttributeReview =
    hasSheinAttributeReviewSignal(sheinEditorContext);
  const showSheinSaleAttributeReview =
    hasSheinSaleAttributeReviewSignal(sheinEditorContext);
  const showSheinReviewDetails =
    showSheinCategoryReview ||
    showSheinAttributeReview ||
    showSheinSaleAttributeReview;
  const sheinBlockingKeys = new Set(
    sheinPreviewPayload?.submit_readiness?.blocking_items?.map((item) => item.key) ?? [],
  );
  const sheinCategoryBlocked =
    sheinBlockingKeys.has("category") || sheinBlockingKeys.has("category_review");
  const sheinAttributeBlocked =
    sheinBlockingKeys.has("attributes") || sheinBlockingKeys.has("attribute_review");
  const sheinSaleAttributeBlocked =
    sheinBlockingKeys.has("sale_attributes") || sheinBlockingKeys.has("variants");
  const sheinPreviewBlocked =
    sheinBlockingKeys.has("images") || sheinBlockingKeys.has("preview_product");
  const sheinReadyStatus = sheinPreviewPayload?.submit_readiness?.status;
  const isSheinFinalReviewMode =
    selectedPlatform === "shein" &&
    searchParams.get("section_key") === "final_review";
  const shouldOpenSheinAdvancedDetails =
    selectedPlatform === "shein" &&
    !isSheinFinalReviewMode &&
    (sheinCategoryBlocked || sheinAttributeBlocked || sheinSaleAttributeBlocked);
  const sheinFlowSteps: SheinFlowStep[] = [
    {
      key: "preview",
      label: "检查图片",
      description: sheinImages.length
        ? `已准备 ${sheinImages.length} 张 SHEIN 成品图，SDS mockup 会单独作为渲染参考展示。`
        : "检查 SHEIN 成品图是否已经生成；SDS mockup 仅作为渲染参考。",
      href: "#shein-preview-images",
      state: sheinPreviewBlocked || !sheinImages.length ? "blocked" : "done",
      actionLabel: "查看图片",
    },
    {
      key: "category",
      label: "确认类目",
      description: sheinCategoryBlocked
        ? "确认 SHEIN 类目和 category path，不使用静态兜底。"
        : "SHEIN 类目已确认，可查看当前类目摘要。",
      href: "#shein-category-review-card",
      state: sheinCategoryBlocked ? "blocked" : "done",
      actionLabel: sheinCategoryBlocked ? "确认类目" : "查看类目",
    },
    {
      key: "attributes",
      label: "确认普通属性",
      description: "补齐普通属性候选值，人工确认后才缓存。",
      href: "#shein-attribute-review-card",
      state: sheinAttributeBlocked ? "blocked" : "done",
      actionLabel: "确认属性",
    },
    {
      key: "sale-attributes",
      label: "确认销售属性",
      description: "检查颜色、尺寸等销售属性映射。",
      href: "#shein-sale-attribute-review-card",
      state: sheinSaleAttributeBlocked ? "blocked" : "done",
      actionLabel: "确认销售属性",
    },
    {
      key: "submit",
      label: "提交",
      description: "先上传 SHEIN 图片，再保存草稿或发布。",
      href: `/listing-kits/${taskId}/workspace?platform=shein&section_key=final_review`,
      state:
        sheinReadyStatus === "ready"
          ? "active"
          : sheinReadyStatus === "blocked"
            ? "blocked"
            : "pending",
      actionLabel: "打开最终确认",
    },
  ];

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

  const workspaceTitle =
    preview.data?.shein?.final_review?.title ||
    preview.data?.shein?.source_product?.title ||
    `任务 ${taskId.slice(0, 8)}`;
  const workspaceStatusLabel = workspaceTaskStatusLabel(taskResult.data?.status);
  const workspaceUpdatedAt = formatWorkspaceDate(
    taskResult.data?.result?.updated_at ??
      taskResult.data?.completed_at ??
      taskResult.data?.created_at,
  );
  const workspaceSubtitle =
    selectedPlatform === "shein"
      ? `SHEIN · ${isSheinFinalReviewMode ? "最终确认" : "审核工作台"} · ${taskId}`
      : `任务标识 · ${taskId}`;
  const sheinAdvancedReviewDetails =
    selectedPlatform === "shein" && showSheinReviewDetails && !isSheinFinalReviewMode ? (
      <details
        className="group rounded-[1.75rem] border border-zinc-200 bg-white p-5 shadow-sm"
        id="shein-advanced-review-details"
        open={shouldOpenSheinAdvancedDetails}
      >
        <summary className="flex cursor-pointer list-none flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.24em] text-zinc-500">
              高级详情
            </p>
            <h2 className="mt-1 text-xl font-semibold tracking-tight text-zinc-950">
              类目和属性映射诊断
            </h2>
            <p className="mt-1 text-sm leading-6 text-zinc-600">
              {shouldOpenSheinAdvancedDetails
                ? "当前存在 SHEIN 阻断项，已经为你展开需要优先处理的类目和属性诊断。"
                : "这里是内部排查信息，默认收起。需要处理类目、普通属性或销售属性时再展开。"}
            </p>
          </div>
          <span className="rounded-full border border-zinc-200 bg-zinc-50 px-3 py-1 text-xs font-semibold text-zinc-600">
            {shouldOpenSheinAdvancedDetails ? "已自动展开" : "点击展开"}
          </span>
        </summary>
        <div className="mt-5 grid min-w-0 items-start gap-4 xl:grid-cols-2">
          {showSheinCategoryReview ? (
            <div id="shein-category-review-card" className="min-w-0">
              <SheinCategoryReviewCard
                taskId={taskId}
                editorContext={preview.data?.shein?.editor_context}
                isApplying={applyRevision.isPending}
                onApplySuggestedCategory={sheinActions.handleApplySuggestedSheinCategory}
                onConfirmCurrentCategory={sheinActions.handleConfirmCurrentSheinCategory}
                onApplyManualCategory={sheinActions.handleApplyManualSheinCategory}
              />
            </div>
          ) : null}
          {showSheinAttributeReview ? (
            <div id="shein-attribute-review-card" className="min-w-0">
              <SheinAttributeReviewCard
                editorContext={preview.data?.shein?.editor_context}
                isApplying={applyRevision.isPending}
                onConfirmAttributes={sheinActions.handleConfirmSheinAttributes}
                onConfirmFallbackAttributes={sheinActions.handleConfirmSheinFallbackAttributes}
              />
            </div>
          ) : null}
          {showSheinSaleAttributeReview ? (
            <div id="shein-sale-attribute-review-card" className="min-w-0">
              <SheinSaleAttributeReviewCard
                editorContext={preview.data?.shein?.editor_context}
                isApplying={applyRevision.isPending}
                onConfirmCurrentSaleAttributes={sheinActions.handleConfirmCurrentSheinSaleAttributes}
              />
            </div>
          ) : null}
        </div>
      </details>
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
