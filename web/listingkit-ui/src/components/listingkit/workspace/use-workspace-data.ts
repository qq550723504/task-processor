import { useMemo } from "react";

import { collectSheinPreviewImageGroups } from "@/components/listingkit/shein/shein-preview-image";
import type { SheinFlowStep } from "@/components/listingkit/shein/shein-flow-nav";
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
import type { ResolvedActionSummary } from "@/lib/types/listingkit";

export function useWorkspaceData({
  taskId,
  searchParams,
}: {
  taskId: string;
  searchParams: URLSearchParams;
}) {
  const buildSheinBlockingActionSummary = ({
    cookieBlocked,
    categoryBlocked,
    attributeBlocked,
    saleAttributeBlocked,
  }: {
    cookieBlocked: boolean;
    categoryBlocked: boolean;
    attributeBlocked: boolean;
    saleAttributeBlocked: boolean;
  }): ResolvedActionSummary | undefined => {
    if (cookieBlocked) {
      return {
        title: "重新登录店铺",
        summary: "先重新登录当前 SHEIN 店铺，恢复在线类目、属性和销售属性能力后再继续提交。",
        cta_kind: "review",
        action_key: "store_login",
      };
    }
    if (attributeBlocked) {
      return {
        title: "确认普通属性",
        summary: "先补齐 SHEIN 模板要求的普通属性，再继续最终确认和提交。",
        cta_kind: "review",
        action_key: "attributes",
      };
    }
    if (saleAttributeBlocked) {
      return {
        title: "确认销售属性",
        summary: "先确认颜色、尺寸等销售属性映射，再继续最终确认和提交。",
        cta_kind: "review",
        action_key: "sale_attributes",
      };
    }
    if (categoryBlocked) {
      return {
        title: "确认类目",
        summary: "先确认 SHEIN 类目和类目模板，再继续最终确认和提交。",
        cta_kind: "review",
        action_key: "category",
      };
    }
    return undefined;
  };

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
        taskResult.data?.result?.sds_sync,
      ),
    [preview.data?.shein, taskResult.data?.result?.sds_sync],
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
  const sheinBlockingKeys = new Set(
    sheinPreviewPayload?.submit_readiness?.blocking_items?.map((item) => item.key) ?? [],
  );
  const sheinCookieBlocked = sheinBlockingKeys.has("shein_cookie_unavailable");
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
  const sheinBlockingActionSummary = buildSheinBlockingActionSummary({
    cookieBlocked: sheinCookieBlocked,
    categoryBlocked: sheinCategoryBlocked,
    attributeBlocked: sheinAttributeBlocked,
    saleAttributeBlocked: sheinSaleAttributeBlocked,
  });
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
        sheinReadyStatus === "ready"
          ? "active"
          : sheinReadyStatus === "blocked"
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
