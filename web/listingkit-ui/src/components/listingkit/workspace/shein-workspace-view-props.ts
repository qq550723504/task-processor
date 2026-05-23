import type { ComponentProps } from "react";

import { SheinDataImageGallery } from "@/components/listingkit/shein/shein-data-image-gallery";
import type { SheinPreviewImage } from "@/components/listingkit/shein/shein-preview-image";
import { SheinFinalReviewPanel } from "@/components/listingkit/shein/shein-final-review-panel";
import { SheinSubmitReadinessPanel } from "@/components/listingkit/shein/shein-submit-readiness-panel";
import { SheinSubmissionTimeline } from "@/components/listingkit/shein/shein-submission-timeline";
import {
  canSelectSheinReadinessItem,
  isSheinWorkspaceActionKey,
} from "@/components/listingkit/shein/shein-workspace-actions";
import { SheinAdvancedReviewDetails } from "@/components/listingkit/workspace/shein-advanced-review-details";
import { submitErrorMessage } from "@/components/listingkit/workspace/workspace-screen-helpers";
import type { useSheinWorkspaceActions } from "@/components/listingkit/workspace/use-shein-workspace-actions";
import type {
  PreviewSlot,
  SheinPreviewPayload,
  SheinReadinessItem,
} from "@/lib/types/listingkit";
import { toImageProxyUrl } from "@/lib/utils/image-proxy-url";
import { formatSheinSubmitError } from "@/lib/utils/shein-submit-error";

type SheinImageGalleryProps = ComponentProps<typeof SheinDataImageGallery>;
type SheinFinalReviewPanelProps = ComponentProps<typeof SheinFinalReviewPanel>;
type SheinSubmitReadinessPanelProps = ComponentProps<
  typeof SheinSubmitReadinessPanel
>;
type SheinSubmissionTimelineProps = ComponentProps<
  typeof SheinSubmissionTimeline
>;
type SheinAdvancedReviewDetailsProps = ComponentProps<
  typeof SheinAdvancedReviewDetails
>;
type SheinWorkspaceActions = ReturnType<typeof useSheinWorkspaceActions>;

export function buildSheinWorkspaceViewProps({
  shein,
  selectedPlatform,
  focusedPreview,
  sheinImages,
  sheinMockupImages,
  sheinVariantCount,
  sheinActions,
  isSavingFinalDraft,
  isSubmitting,
  submitError,
  clearingResolutionCacheKind,
  isRefreshingSubmissionStatus,
  onSelectBlockingItem,
  onRunPrimaryAction,
  onClearResolutionCache,
  onRefreshSubmissionStatus,
}: {
  shein?: SheinPreviewPayload | null;
  selectedPlatform?: string;
  focusedPreview?: PreviewSlot;
  sheinImages: SheinPreviewImage[];
  sheinMockupImages: SheinPreviewImage[];
  sheinVariantCount?: number;
  sheinActions: SheinWorkspaceActions;
  isSavingFinalDraft: boolean;
  isSubmitting: boolean;
  submitError?: unknown;
  clearingResolutionCacheKind?: string | null;
  isRefreshingSubmissionStatus: boolean;
  onSelectBlockingItem: (item: SheinReadinessItem) => void;
  onRunPrimaryAction: (key?: string | null) => void;
  onClearResolutionCache: (kind: "category" | "attribute" | "sale_attribute" | "pricing") => void;
  onRefreshSubmissionStatus: () => void;
}) {
  const sheinDisplayImages =
    sheinImages.length > 0 ? sheinImages : sheinMockupImages;
  const selectedSheinImage =
    sheinDisplayImages.find(
      (image) => image.url === sheinActions.selectedSheinImageUrl,
    ) ?? sheinDisplayImages[0];
  const sheinFallbackPreview: PreviewSlot | undefined =
    selectedPlatform === "shein" &&
    !focusedPreview?.asset_url &&
    !focusedPreview?.preview_svg
      ? {
          asset_url: toImageProxyUrl(selectedSheinImage?.url),
          template_label: selectedSheinImage?.label ?? "SHEIN product image",
          asset_id: selectedSheinImage?.id ?? "shein-product-image",
        }
      : undefined;
  const canSubmit = shein?.submit_readiness?.ready === true;
  const imageGalleryProps: SheinImageGalleryProps = {
    images: sheinImages,
    mockupImages: sheinMockupImages,
    finalImages: shein?.final_review?.images,
    variantCount: sheinVariantCount,
    isSavingControls: isSavingFinalDraft,
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
  };
  const finalReviewProps: SheinFinalReviewPanelProps = {
    shein,
    isSaving: isSavingFinalDraft,
    isSubmitting,
    saveErrorMessage: sheinActions.sheinFinalDraftError,
    saveMessage: sheinActions.sheinFinalDraftMessage,
    submitAction: sheinActions.sheinSubmitAction,
    submitErrorMessage: formatSheinSubmitError(submitError, shein),
    canSelectBlockingItem: canSelectSheinReadinessItem,
    onSelectBlockingItem,
    onSubmit: sheinActions.handleSubmitShein,
  };
  const sharedReadinessProps = {
    readiness: shein?.submit_readiness,
    checklist: shein?.submit_checklist,
    submission: shein?.submission,
    imageUpload: shein?.image_upload,
    resolutionCache: shein?.resolution_cache,
    workspaceOverview: shein?.workspace_overview,
    canSelectBlockingItem: canSelectSheinReadinessItem,
    onSelectBlockingItem,
    canRunPrimaryAction: isSheinWorkspaceActionKey,
    onRunPrimaryAction,
    canSubmit,
    isSubmitting,
    submitAction: sheinActions.sheinSubmitAction,
    onSubmit: () => sheinActions.handleSubmitShein("publish"),
    onSaveDraft: sheinActions.handleSaveSheinDraft,
    clearingResolutionCacheKind,
    onClearResolutionCache,
  };
  const finalModeReadinessProps: SheinSubmitReadinessPanelProps = {
    ...sharedReadinessProps,
    submitErrorMessage: formatSheinSubmitError(submitError, shein),
    showSubmitActions: false,
  };
  const reviewModeReadinessProps: SheinSubmitReadinessPanelProps = {
    ...sharedReadinessProps,
    submitErrorMessage: submitErrorMessage(submitError),
  };
  const timelineProps: SheinSubmissionTimelineProps = {
    events: shein?.submission_events,
    canRefresh: Boolean(shein?.submission?.last_action),
    isRefreshing: isRefreshingSubmissionStatus,
    onRefresh: onRefreshSubmissionStatus,
  };

  return {
    selectedSheinImage,
    sheinFallbackPreview,
    imageGalleryProps,
    finalReviewProps,
    finalModeReadinessProps,
    reviewModeReadinessProps,
    timelineProps,
  };
}

export function buildSheinAdvancedReviewDetailsProps({
  taskId,
  shein,
  selectedPlatform,
  showReviewDetails,
  showCategoryReview,
  showAttributeReview,
  showSaleAttributeReview,
  isFinalReviewMode,
  open,
  isApplying,
  sheinActions,
}: {
  taskId: string;
  shein?: SheinPreviewPayload | null;
  selectedPlatform?: string;
  showReviewDetails: boolean;
  showCategoryReview: boolean;
  showAttributeReview: boolean;
  showSaleAttributeReview: boolean;
  isFinalReviewMode: boolean;
  open: boolean;
  isApplying: boolean;
  sheinActions: SheinWorkspaceActions;
}): SheinAdvancedReviewDetailsProps | null {
  if (selectedPlatform !== "shein" || !showReviewDetails || isFinalReviewMode) {
    return null;
  }

  return {
    open,
    showCategoryReview,
    showAttributeReview,
    showSaleAttributeReview,
    categoryReviewProps: {
      taskId,
      editorContext: shein?.editor_context,
      isApplying,
      onApplySuggestedCategory: sheinActions.handleApplySuggestedSheinCategory,
      onConfirmCurrentCategory: sheinActions.handleConfirmCurrentSheinCategory,
      onApplyManualCategory: sheinActions.handleApplyManualSheinCategory,
    },
    attributeReviewProps: {
      editorContext: shein?.editor_context,
      isApplying,
      onConfirmAttributes: sheinActions.handleConfirmSheinAttributes,
      onConfirmFallbackAttributes: sheinActions.handleConfirmSheinFallbackAttributes,
      onRegenerateAttributes: sheinActions.handleRegenerateSheinAttributes,
    },
    saleAttributeReviewProps: {
      editorContext: shein?.editor_context,
      isApplying,
      onConfirmCurrentSaleAttributes:
        sheinActions.handleConfirmCurrentSheinSaleAttributes,
      onRegenerateSaleAttributes:
        sheinActions.handleRegenerateSheinSaleAttributes,
      onApplyManualSaleAttributes:
        sheinActions.handleApplyManualSheinSaleAttributes,
    },
  };
}
