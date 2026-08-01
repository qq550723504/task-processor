import { useMemo } from "react";

import { createAmazonWorkspaceAdapter } from "@/components/listingkit/workspace/amazon-workspace-adapter";
import {
  createSheinWorkspaceAdapter,
  type SheinWorkspaceProjection,
} from "@/components/listingkit/workspace/shein-workspace-adapter";
import {
  createGenericWorkspaceProjection,
  resolveWorkspacePlatformAdapter,
} from "@/components/listingkit/workspace/workspace-platform-adapter";
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
  const sheinPreviewPayload = preview.data?.shein;
  const isSheinFinalReviewMode =
    selectedPlatform === "shein" &&
    searchParams.get("section_key") === "final_review";
  const workspacePlatformProjection = resolveWorkspacePlatformAdapter(
    selectedPlatform,
    [
      createSheinWorkspaceAdapter({
        taskId,
        isFinalReviewMode: isSheinFinalReviewMode,
        shein: sheinPreviewPayload,
        sdsDesignResult: getTaskSDSDesignResult(taskResult.data?.result),
      }),
      createAmazonWorkspaceAdapter<SheinWorkspaceProjection>({
        amazon: preview.data?.amazon,
      }),
    ],
    () => createGenericWorkspaceProjection<SheinWorkspaceProjection>(selectedPlatform),
  );
  const sheinWorkspaceProjection =
    workspacePlatformProjection.kind === "shein"
      ? workspacePlatformProjection.projection
      : undefined;
  const sheinImages = sheinWorkspaceProjection?.images ?? [];
  const sheinMockupImages = sheinWorkspaceProjection?.mockupImages ?? [];
  const sheinVariantCount =
    sheinWorkspaceProjection?.variantCount ?? preview.data?.overview?.variant_count;
  const showSheinCategoryReview =
    sheinWorkspaceProjection?.showCategoryReview ?? false;
  const showSheinAttributeReview =
    sheinWorkspaceProjection?.showAttributeReview ?? false;
  const showSheinSaleAttributeReview =
    sheinWorkspaceProjection?.showSaleAttributeReview ?? false;
  const showSheinReviewDetails =
    sheinWorkspaceProjection?.showReviewDetails ?? false;
  const shouldOpenSheinAdvancedDetails =
    sheinWorkspaceProjection?.shouldOpenAdvancedDetails ?? false;
  const sheinBlockingActionSummary =
    sheinWorkspaceProjection?.readiness.blockingActionSummary;
  const effectiveResolvedActionSummary =
    selectedPlatform === "shein" && sheinBlockingActionSummary
      ? sheinBlockingActionSummary
      : resolvedActionSummary;
  const previewSuggestion =
    selectedPlatform === "shein" && sheinBlockingActionSummary
      ? null
      : previewSuggestionCandidate;
  const sheinFlowSteps = sheinWorkspaceProjection?.flowSteps ?? [];

  const workspaceTitle =
    (workspacePlatformProjection.kind === "amazon"
      ? workspacePlatformProjection.title
      : undefined) ||
    sheinPreviewPayload?.final_review?.title ||
    sheinPreviewPayload?.source_product?.title ||
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
      : workspacePlatformProjection.kind === "amazon"
        ? `Amazon · ${workspacePlatformProjection.subtitle ?? "审核工作台"} · ${taskId}`
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
