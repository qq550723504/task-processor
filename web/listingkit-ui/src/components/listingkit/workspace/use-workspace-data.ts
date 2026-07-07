import { useMemo } from "react";

import { collectSheinPreviewImageGroups } from "@/components/listingkit/shein/shein-preview-image";
import type { SheinFlowStep } from "@/components/listingkit/shein/shein-flow-nav";
import {
  projectSheinReadinessActions,
} from "@/components/listingkit/shein/shein-workspace-actions";
import {
  deriveWorkspacePreviewSuggestion,
} from "@/components/listingkit/workspace/workspace-preview-routing";
import { resolveWorkspaceScenePreset } from "@/components/listingkit/workspace/workspace-scene-preset";
import {
  shouldSuppressResolvedActionSummary,
} from "@/components/listingkit/tasks/task-status-display";
import {
  pickWorkspaceResolvedActionSummary,
} from "@/components/listingkit/workspace/workspace-action-routing";
import { getTaskSDSDesignResult } from "@/lib/listingkit/semantic-fields";
import {
  formatWorkspaceDate,
  hasSheinAttributeReviewSignal,
  hasSheinCategoryReviewSignal,
  hasSheinSaleAttributeReviewSignal,
  queryFromSearchParams,
  selectedPlatformFromReviewTarget,
  workspaceTaskStatusLabel,
} from "@/components/listingkit/workspace/workspace-screen-helpers";
import { useListingKitPreview } from "@/lib/query/use-preview";
import { useReviewPreview } from "@/lib/query/use-review-preview";
import { useReviewSession } from "@/lib/query/use-review-session";
import { useListingKitTaskResult } from "@/lib/query/use-task-result";

export function useWorkspaceData({
  taskId,
  searchParams,
}: {
  taskId: string;
  searchParams: URLSearchParams;
}) {
  const baseQuery = useMemo(
    () => queryFromSearchParams(searchParams),
    [searchParams],
  );

  const taskResult = useListingKitTaskResult(taskId);
  const previewFreshnessKey =
    taskResult.data?.result?.updated_at ??
    taskResult.data?.completed_at ??
    taskResult.data?.status;
  const preview = useListingKitPreview(taskId, previewFreshnessKey);
  const session = useReviewSession(taskId, baseQuery);
  const focusedPreviewQuery =
    session.data?.session?.focused_target?.navigation_target?.preview_query;
  const reviewPreview = useReviewPreview(
    taskId,
    focusedPreviewQuery ?? baseQuery,
    Boolean(focusedPreviewQuery ?? baseQuery.slot ?? baseQuery.platform),
  );

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
  const previewSuggestionCandidate = deriveWorkspacePreviewSuggestion({
    slots: sessionData?.slot_navigation,
    selectedSlot: sessionData?.selected_slot,
    focusedPreview,
  });
  const sheinImageGroups = useMemo(
    () =>
      collectSheinPreviewImageGroups(
        preview.data?.shein,
        getTaskSDSDesignResult(taskResult.data?.result),
      ),
    [preview.data?.shein, taskResult.data?.result],
  );
  const sheinImages = sheinImageGroups.productImages;
  const sheinMockupImages = sheinImageGroups.mockupImages;
  const sheinVariantCount =
    preview.data?.shein?.final_review?.skus?.length ??
    preview.data?.overview?.variant_count;
  const sheinPreviewPayload = preview.data?.shein;
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
  const sheinReadinessProjection = projectSheinReadinessActions(
    sheinPreviewPayload?.submit_readiness?.blocking_items,
  );
  const sheinCookieBlocked = sheinReadinessProjection.cookieBlocked;
  const sheinCategoryBlocked = sheinReadinessProjection.categoryBlocked;
  const sheinAttributeBlocked = sheinReadinessProjection.attributeBlocked;
  const sheinSaleAttributeBlocked = sheinReadinessProjection.saleAttributeBlocked;
  const sheinPreviewBlocked = sheinReadinessProjection.previewBlocked;
  const sheinReadyStatus = sheinPreviewPayload?.submit_readiness?.status;
  const sheinPreFinalReviewBlocked =
    sheinCookieBlocked ||
    sheinCategoryBlocked ||
    sheinAttributeBlocked ||
    sheinSaleAttributeBlocked ||
    sheinPreviewBlocked;
  const isSheinFinalReviewMode =
    selectedPlatform === "shein" &&
    searchParams.get("section_key") === "final_review";
  const shouldOpenSheinAdvancedDetails =
    selectedPlatform === "shein" &&
    !isSheinFinalReviewMode &&
    (sheinCategoryBlocked || sheinAttributeBlocked || sheinSaleAttributeBlocked);
  const sheinBlockingActionSummary =
    sheinReadinessProjection.blockingActionSummary;
  const effectiveResolvedActionSummary =
    selectedPlatform === "shein" && sheinBlockingActionSummary
      ? sheinBlockingActionSummary
      : resolvedActionSummary;
  const previewSuggestion =
    selectedPlatform === "shein" && sheinBlockingActionSummary
      ? null
      : previewSuggestionCandidate;
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
        isSheinFinalReviewMode || sheinReadyStatus === "ready" || !sheinPreFinalReviewBlocked
          ? "active"
          : sheinPreFinalReviewBlocked
            ? "blocked"
            : "pending",
      actionLabel: "打开最终确认",
    },
  ];

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

  return {
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
    resolvedActionSummary: effectiveResolvedActionSummary,
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
  };
}
