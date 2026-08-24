import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioGroupedImageMode,
  SheinStudioSelectedSDSImage,
} from "@/lib/types/shein-studio-draft";
import type {
  SheinStudioArtworkModel,
  SheinStudioPromptMode,
  SheinStudioTransparencyMode,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio-generation";
import type {
  SheinStudioCreatedTask,
  SheinStudioFailedTask,
  SheinStudioRejectedTask,
} from "@/lib/types/shein-studio-task";

export type SheinStudioBatchStatus =
  | "draft"
  | "generating"
  | "partially_materialized"
  | "review_ready"
  | "partially_failed"
  | "failed"
  | "tasks_creating"
  | "tasks_created";

 type SheinStudioMaterializedDesignReviewStatus =
  | "unreviewed"
  | "approved"
  | "rejected";

export type SheinStudioBatchItemStatus =
  | "pending"
  | "generating"
  | "awaiting_materialization"
  | "review_ready"
  | "failed";

 type SheinStudioBatchStatusGroupKey =
  | "submittable"
  | "needs_fix"
  | "processing"
  | "generation_failed"
  | "submission_failed"
  | "draft_saved"
  | "published";

export type SheinStudioBatchStatusGroup = {
  key: SheinStudioBatchStatusGroupKey | string;
  label: string;
  count: number;
  ids?: string[];
};

export type SheinStudioBatchStatusGroups = {
  items: SheinStudioBatchStatusGroup[];
  byKey: Record<string, SheinStudioBatchStatusGroup>;
};

export type SheinStudioBatchRecord = {
  id: string;
  tenantId?: string;
  status: SheinStudioBatchStatus;
  prompt: string;
  promptMode?: SheinStudioPromptMode;
  styleCount: string;
  hotStyleReferenceImageUrls?: string[];
  hotStyleReferenceBrief?: string;
  hotStyleReferencePrompt?: string;
  sheinStoreId: number;
  variationIntensity?: SheinStudioVariationIntensity;
  artworkModel?: SheinStudioArtworkModel;
  transparentBackground?: boolean;
  transparentBackgroundMode?: SheinStudioTransparencyMode;
  groupedImageMode?: SheinStudioGroupedImageMode;
  selectedSdsImages?: SheinStudioSelectedSDSImage[];
  selectionVariantId?: number;
  selection?: SDSProductVariantSelection;
  groupedSelections?: GroupedSDSSelectionEligibility[];
  createdAt: string;
  draftUpdatedAt?: string;
  updatedAt: string;
};

export type SheinStudioMaterializedDesign = {
  id: string;
  batchId: string;
  itemId: string;
  sourceAttemptId: string;
  targetGroupKey: string;
  targetGroupLabel?: string;
  imageUrl: string;
  originalImageUrl?: string;
  transparentBackgroundMode?: SheinStudioTransparencyMode;
  backgroundRemovalStatus?:
    | "not_requested"
    | "pending"
    | "succeeded"
    | "failed";
  backgroundRemovalModel?: string;
  backgroundRemovalError?: string;
  reviewStatus: SheinStudioMaterializedDesignReviewStatus;
  reviewNote?: string;
  role?: string;
  roleLabel?: string;
  productImageUrls?: string[];
  createdAt: string;
  updatedAt: string;
};

export type SheinStudioBatchItem = {
  id: string;
  batchId: string;
  targetGroupKey: string;
  targetGroupLabel?: string;
  status: SheinStudioBatchItemStatus;
  selectionCount: number;
  lastError?: string;
  createdAt: string;
  updatedAt: string;
};

export type SheinStudioItemizedBatchItem = {
  item: SheinStudioBatchItem;
  designs: SheinStudioMaterializedDesign[];
};

export type SheinStudioBatchDetail = {
  batch: SheinStudioBatchRecord;
  items: SheinStudioItemizedBatchItem[];
  createdTasks?: SheinStudioCreatedTask[];
  reusedTasks?: SheinStudioCreatedTask[];
  rejectedTasks?: SheinStudioRejectedTask[];
  failedTasks?: SheinStudioFailedTask[];
  statusGroups?: SheinStudioBatchStatusGroups;
};
