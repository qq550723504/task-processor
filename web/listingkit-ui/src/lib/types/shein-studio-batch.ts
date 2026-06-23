import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioGroupedImageMode,
  SheinStudioSelectedSDSImage,
} from "@/lib/types/shein-studio-draft";
import type {
  SheinStudioArtworkModel,
  SheinStudioPromptMode,
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

export type SheinStudioMaterializedDesignReviewStatus =
  | "unreviewed"
  | "approved"
  | "rejected";

export type SheinStudioBatchItemStatus =
  | "pending"
  | "generating"
  | "awaiting_materialization"
  | "review_ready"
  | "failed";

export type SheinStudioBatchStatusGroupKey =
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
  sheinStoreId: number;
  variationIntensity?: SheinStudioVariationIntensity;
  artworkModel?: SheinStudioArtworkModel;
  transparentBackground?: boolean;
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
