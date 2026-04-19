import type {
  ActionExecutionResult,
  AssetGenerationOverview,
  CreateListingKitTaskResponse,
  ListingKitPreview,
  ListingKitTaskResult,
  NavigationDispatchResponse,
  PlatformCard,
  PreviewSlot,
  QueueItem,
  QueuePage,
  QueueSummary,
  RecoveryDescriptor,
  RecoverySummary,
  ResolvedActionSummary,
  ReviewPreviewResponse,
  ReviewSession,
  ReviewSessionResponse,
  ReviewSlot,
  ReviewSummary,
  ReviewTarget,
  ReviewToolbar,
} from "@/lib/types/listingkit";

export type ListingKitMockBundle = {
  createTask: CreateListingKitTaskResponse;
  taskResult: ListingKitTaskResult;
  preview: ListingKitPreview;
  queue: QueuePage;
  reviewSession: ReviewSessionResponse;
  reviewPreview: ReviewPreviewResponse;
  dispatch: NavigationDispatchResponse;
  action: ActionExecutionResult;
};

export type ListingKitMockShared = {
  taskId: string;
  previewSvg: string;
  focusedTarget: ReviewTarget;
  focusedToolbar: ReviewToolbar;
  focusedPreview: PreviewSlot;
  queueSummary: QueueSummary;
  overview: AssetGenerationOverview;
  reviewSummary: ReviewSummary;
  recoveryDescriptors: RecoveryDescriptor[];
  recoverySummary: RecoverySummary;
  resolvedActionSummary: ResolvedActionSummary;
  platformCards: PlatformCard[];
  slotNavigation: ReviewSlot[];
  reviewSession: ReviewSession;
  queueItems: QueueItem[];
};
