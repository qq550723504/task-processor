import type { ComponentProps } from "react";

import { deriveTaskPreviewEmptyState } from "@/components/listingkit/tasks/task-status-display";
import { WorkspaceReviewView } from "@/components/listingkit/workspace/workspace-screen-views";
import type {
  ListingKitTaskResult,
  PreviewSlot,
  ReviewPreviewResponse,
  ReviewSession,
  ReviewTarget,
  ToolbarAction,
} from "@/lib/types/listingkit";

type WorkspaceReviewViewProps = ComponentProps<typeof WorkspaceReviewView>;

export function buildWorkspaceReviewViewProps({
  selectedPlatform,
  previewSuggestion,
  sessionData,
  reviewPreviewData,
  taskResult,
  focusedPreview,
  shein,
  sheinViewProps,
  focusedScenePreset,
  recoveryDescriptors,
  onDispatch,
  onToolbarAction,
  onRecovery,
}: {
  selectedPlatform?: string;
  previewSuggestion: WorkspaceReviewViewProps["previewSuggestionProps"]["suggestion"];
  sessionData: ReviewSession;
  reviewPreviewData?: ReviewPreviewResponse;
  taskResult?: ListingKitTaskResult;
  focusedPreview?: PreviewSlot;
  shein: WorkspaceReviewViewProps["sheinSourceProductProps"]["shein"];
  sheinViewProps: {
    sheinFallbackPreview?: PreviewSlot;
    imageGalleryProps: WorkspaceReviewViewProps["sheinImageGalleryProps"];
    finalReviewProps: WorkspaceReviewViewProps["sheinFinalReviewProps"];
    reviewModeReadinessProps: WorkspaceReviewViewProps["sheinReadinessProps"];
    timelineProps: WorkspaceReviewViewProps["sheinTimelineProps"];
  };
  focusedScenePreset: WorkspaceReviewViewProps["scenePresetPanelProps"]["summary"];
  recoveryDescriptors: WorkspaceReviewViewProps["recoveryActionListProps"]["descriptors"];
  onDispatch: (target?: ReviewTarget["navigation_target"] | null) => void;
  onToolbarAction: (toolbarAction: ToolbarAction) => void;
  onRecovery: WorkspaceReviewViewProps["recoveryActionListProps"]["onSelect"];
}): WorkspaceReviewViewProps {
  return {
    selectedPlatform,
    previewSuggestionProps: {
      suggestion: previewSuggestion,
      onSelect: (slot) => onDispatch(slot.review_target?.navigation_target),
    },
    reviewSectionTabsProps: {
      sections: sessionData.sections,
      selectedKey: sessionData.focused_section_key,
      onSelect: (section) => onDispatch(section.review_target?.navigation_target),
    },
    sheinSourceProductProps: { shein },
    sheinImageGalleryProps: sheinViewProps.imageGalleryProps,
    sheinFinalReviewProps: sheinViewProps.finalReviewProps,
    previewCanvasProps: {
      preview: sheinViewProps.sheinFallbackPreview ?? focusedPreview,
      response: reviewPreviewData,
      emptyState: deriveTaskPreviewEmptyState(taskResult),
    },
    slotNavigationProps: {
      slots: sessionData.slot_navigation,
      selectedSlot: sessionData.selected_slot,
      selectedAssetId: focusedPreview?.asset_id,
      onSelect: (slot) => onDispatch(slot.review_target?.navigation_target),
    },
    reviewToolbarProps: {
      toolbar: reviewPreviewData?.toolbar ?? sessionData.focused_toolbar,
      onAction: onToolbarAction,
    },
    sheinReadinessProps: sheinViewProps.reviewModeReadinessProps,
    sheinTimelineProps: sheinViewProps.timelineProps,
    scenePresetPanelProps: { summary: focusedScenePreset },
    recoveryActionListProps: {
      descriptors: recoveryDescriptors,
      onSelect: onRecovery,
    },
  };
}
