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
import type { TargetPlatform } from "@/lib/api/generated";
import {
  formatWorkspaceDate,
  queryFromSearchParams,
  selectedPlatformFromReviewTarget,
  workspaceTaskStatusLabel,
} from "@/components/listingkit/workspace/workspace-screen-helpers";
import { useListingKitPreview } from "@/lib/query/use-preview";
import { useListingKitTaskResult } from "@/lib/query/use-task-result";
import type {
  PlatformCard,
  PreviewSlot,
  ReviewPreviewResponse,
  ReviewSession,
  ReviewSlot,
} from "@/lib/types/listingkit";

export function resolveWorkspaceTitle({
  selectedPlatform,
  amazonTitle,
  sheinFinalTitle,
  sheinSourceTitle,
  canonicalTitle,
}: {
  selectedPlatform?: string;
  amazonTitle?: string;
  sheinFinalTitle?: string;
  sheinSourceTitle?: string;
  canonicalTitle?: string;
}) {
  return (
    firstWorkspaceTitle(
      amazonTitle,
      selectedPlatform === "shein" ? sheinFinalTitle : undefined,
      selectedPlatform === "shein" ? sheinSourceTitle : undefined,
      canonicalTitle,
    ) ?? "未命名商品"
  );
}

function firstWorkspaceTitle(...candidates: Array<string | undefined>) {
  return candidates.map((candidate) => candidate?.trim()).find(Boolean);
}

export function mergeNavigationPlatformCards(
  previewCards?: PlatformCard[],
  sessionCards?: PlatformCard[],
) {
  const preview = previewCards ?? [];
  const session = sessionCards ?? [];
  const sessionCardsByPlatform = new Map(
    session.map((card) => [card.platform, card]),
  );
  const previewPlatforms = new Set(preview.map((card) => card.platform));

  return [
    ...preview.map(
      (card) => sessionCardsByPlatform.get(card.platform) ?? card,
    ),
    ...session.filter((card) => !previewPlatforms.has(card.platform)),
  ];
}

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
  const requestedPlatform = targetPlatformFromQuery(baseQuery.platform);
  const preview = useListingKitPreview(
    taskId,
    previewFreshnessKey,
    requestedPlatform,
  );
  const sessionData = useMemo<ReviewSession | undefined>(() => {
    const previewData = preview.data;
    if (!previewData) {
      return undefined;
    }
    const selectedPlatform = requestedPlatform ?? previewData.selected_platform;
    const slotNavigation = (previewData.asset_render_previews ?? []).map(
      reviewSlotFromPreview,
    );
    const selectedSlot = baseQuery.slot ?? slotNavigation[0]?.slot;
    const focusedTarget = selectedPlatform
      ? {
          platform: selectedPlatform,
          slot: selectedSlot,
          capability: baseQuery.preview_capability,
        }
      : undefined;
    return {
      selected_platform: selectedPlatform,
      selected_slot: selectedSlot,
      focused_target: focusedTarget,
      default_target: focusedTarget,
      focused_render_preview: findFocusedPreview(
        previewData.asset_render_previews,
        selectedSlot,
      ),
      review_summary: {
        approved_sections: previewData.asset_generation_overview?.approved_sections,
        deferred_sections: previewData.asset_generation_overview?.deferred_sections,
        pending_sections: previewData.asset_generation_overview?.review_pending_sections,
      },
      queue: previewData.asset_generation_queue,
      overview: previewData.asset_generation_overview,
      platform_cards: previewData.overview?.platform_cards,
      slot_navigation: slotNavigation,
      sections: [],
    };
  }, [baseQuery.preview_capability, baseQuery.slot, preview.data, requestedPlatform]);
  const focusedPreview = sessionData?.focused_render_preview;
  const reviewPreview: {
    data?: ReviewPreviewResponse;
    isLoading: boolean;
    isError: boolean;
    refetch: () => Promise<unknown>;
  } = {
    data: focusedPreview ? { preview: focusedPreview } : undefined,
    isLoading: preview.isLoading,
    isError: preview.isError,
    refetch: preview.refetch,
  };
  const session: {
    data?: import("@/lib/types/listingkit").ReviewSessionResponse;
    isLoading: boolean;
    isError: boolean;
    refetch: () => Promise<unknown>;
  } = {
    data: sessionData ? { session: sessionData } : undefined,
    isLoading: preview.isLoading,
    isError: preview.isError,
    refetch: preview.refetch,
  };
  const platformCards =
    sessionData?.platform_cards ?? preview.data?.overview?.platform_cards ?? [];
  const navigationPlatformCards = mergeNavigationPlatformCards(
    preview.data?.overview?.platform_cards,
    sessionData?.platform_cards,
  );
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
  const sheinAvailableImages = sheinWorkspaceProjection?.availableImages ?? [];
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

  const workspaceTitle = resolveWorkspaceTitle({
    selectedPlatform,
    amazonTitle:
      workspacePlatformProjection.kind === "amazon"
        ? workspacePlatformProjection.title
        : undefined,
    sheinFinalTitle: sheinPreviewPayload?.final_review?.title,
    sheinSourceTitle: sheinPreviewPayload?.source_product?.title,
    canonicalTitle: taskResult.data?.result?.canonical_product?.title,
  });
  const workspaceStatusLabel = workspaceTaskStatusLabel(taskResult.data?.status);
  const workspaceUpdatedAt = formatWorkspaceDate(
    taskResult.data?.result?.updated_at ??
      taskResult.data?.completed_at ??
      taskResult.data?.created_at,
  );
  const workspaceSubtitle =
    selectedPlatform === "shein"
      ? `SHEIN · ${isSheinFinalReviewMode ? "最终确认" : "商品审核"}`
      : workspacePlatformProjection.kind === "amazon"
        ? `Amazon · ${workspacePlatformProjection.subtitle ?? "商品审核"}`
        : "商品资料";

  return {
    baseQuery,
    preview,
    taskResult,
    session,
    reviewPreview,
    sessionData,
    platformCards,
    navigationPlatformCards,
    focusedPreview,
    selectedPlatform,
    focusedScenePreset,
    suppressResolvedActionSummary,
    resolvedActionSummary: effectiveResolvedActionSummary,
    previewSuggestion,
    sheinImages,
    sheinAvailableImages,
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

function findFocusedPreview(
  previews: PreviewSlot[] | undefined,
  selectedSlot?: string,
) {
  return previews?.find((preview) => preview.slot === selectedSlot) ?? previews?.[0];
}

function reviewSlotFromPreview(preview: PreviewSlot): ReviewSlot {
  return {
    slot: preview.slot,
    asset_id: preview.asset_id,
    state: preview.state_label,
    render_preview_available: Boolean(preview.preview_svg || preview.asset_url),
  };
}

function targetPlatformFromQuery(value?: string): TargetPlatform | undefined {
  switch (value) {
    case "amazon":
    case "shein":
    case "temu":
    case "walmart":
      return value;
    default:
      return undefined;
  }
}
