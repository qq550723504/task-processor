import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";

export type SheinStudioGeneratedDesign = {
  id: string;
  dataUrl?: string;
  imageUrl?: string;
  prompt?: string;
  productImageUrls?: string[];
  sourceWidth?: number;
  sourceHeight?: number;
  revisedPrompt?: string;
  imageModel?: SheinStudioArtworkModel | string;
  transparentBackground?: boolean;
  variationIntensity?: SheinStudioVariationIntensity;
  role?: string;
  roleLabel?: string;
  reviewNote?: string;
  targetGroupKey?: string;
  targetGroupLabel?: string;
};

export type SheinStudioImageStrategy =
  | "ai_generated"
  | "sds_official"
  | "hybrid";

export type SheinStudioArtworkModel = string;
export type SheinStudioVariationIntensity = "light" | "medium" | "strong";
export type SheinStudioGroupedImageMode = "shared_by_size" | "per_product";
export type SheinStudioBatchQueueMode = "generate" | "create_tasks";

export type SheinStudioProductImagePrompt = {
  role: string;
  label: string;
  prompt: string;
};

export type SheinStudioVariantProductImageSet = {
  variantSku?: string;
  color?: string;
  imageUrls: string[];
};

export type SheinStudioSelectedSDSImage = {
  imageUrl: string;
  variantSku?: string;
  color?: string;
};

export type SheinStudioCreatedTask = {
  id: string;
  title: string;
  designId: string;
};

export type SheinStudioGenerationJobStatus =
  | "running"
  | "succeeded"
  | "failed";

export type SheinStudioGenerationJob = {
  jobId: string;
  targetGroupKey?: string;
  targetGroupLabel?: string;
  status: SheinStudioGenerationJobStatus;
};

export type SDSGroupedPromptHistoryEntry = {
  prompt: string;
  groupedImageMode: SheinStudioGroupedImageMode;
  createdAt: string;
};

export type SheinStudioGroupedWorkspace = {
  id: string;
  name: string;
  primarySelection: SDSProductVariantSelection;
  groupedSelections: GroupedSDSSelectionEligibility[];
  styleCount?: string;
  sheinStoreId: string;
  imageStrategy?: SheinStudioImageStrategy;
  groupedImageMode?: SheinStudioGroupedImageMode;
  selectedSdsImages?: SheinStudioSelectedSDSImage[];
  renderSizeImagesWithSds?: boolean;
  currentPrompt: string;
  promptHistory: SDSGroupedPromptHistoryEntry[];
  productImageCount?: string;
  productImagePrompt?: string;
  productImagePrompts?: SheinStudioProductImagePrompt[];
  artworkModel?: SheinStudioArtworkModel;
  transparentBackground?: boolean;
  variationIntensity?: SheinStudioVariationIntensity;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
  updatedAt: string;
};

export type SheinStudioGenerateRequest = {
  prompt: string;
  count: number;
  variationIntensity?: SheinStudioVariationIntensity;
  printableWidth?: number;
  printableHeight?: number;
  productReferenceImageUrls?: string[];
  imageModel?: string;
  transparentBackground?: boolean;
};

export type SheinStudioGenerateResponse = {
  prompt: string;
  printableWidth?: number;
  printableHeight?: number;
  imageModel?: SheinStudioArtworkModel | string;
  transparentBackground?: boolean;
  images: SheinStudioGeneratedDesign[];
  warnings?: string[];
};

export type SheinStudioSavedBatch = {
  id: string;
  name: string;
  prompt: string;
  styleCount: string;
  variationIntensity?: SheinStudioVariationIntensity;
  productImageCount?: string;
  productImagePrompt?: string;
  productImagePrompts?: SheinStudioProductImagePrompt[];
  artworkModel?: SheinStudioArtworkModel;
  transparentBackground?: boolean;
  sheinStoreId: string;
  imageStrategy?: SheinStudioImageStrategy;
  groupedImageMode?: SheinStudioGroupedImageMode;
  selectedSdsImages?: SheinStudioSelectedSDSImage[];
  renderSizeImagesWithSds?: boolean;
  selectionVariantId?: number;
  selection?: SDSProductVariantSelection;
  groupedSelections?: GroupedSDSSelectionEligibility[];
  groups?: SheinStudioGroupedWorkspace[];
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
  generationJobs?: SheinStudioGenerationJob[];
  generationError?: string;
  generationJobId?: string;
  sessionStatus?: string;
  updatedAt: string;
};

export type SheinStudioBatchStatus =
  | "pending"
  | "generating"
  | "partially_materialized"
  | "review_ready"
  | "partially_failed"
  | "failed"
  | "tasks_created";

export type SheinStudioBatchItemStatus =
  | "pending"
  | "generating"
  | "awaiting_materialization"
  | "review_ready"
  | "failed";

export type SheinStudioBatchRecord = {
  id: string;
  status: SheinStudioBatchStatus;
  prompt: string;
  styleCount: string;
  sheinStoreId: string;
  createdAt: string;
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
  approved: boolean;
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
};

export type SheinStudioDraft = Omit<
  SheinStudioSavedBatch,
  "id" | "name" | "updatedAt"
> & {
  generationError?: string;
  generationJobId?: string;
  generationJobs?: SheinStudioGenerationJob[];
  sessionStatus?: string;
  updatedAt: string;
};

export type SheinStudioStorageData = {
  draft: SheinStudioDraft | null;
  batches: SheinStudioSavedBatch[];
};

export type SheinStudioRecentBatchSummary = {
  id: string;
  source: "batch" | "local_draft";
  isRecoverableDraft: boolean;
  title: string;
  primaryProductName: string;
  productCount: number;
  promptPreview: string;
  storeSummary: string;
  designCount: number;
  createdTaskCount: number;
  updatedAt: string;
  alerts?: SheinStudioRecentBatchAlert[];
  recentResults?: SheinStudioRecentBatchResult[];
};

export type SheinStudioRecentBatchAlert = {
  tone: "warning" | "danger";
  label: string;
  reasonCode?: string;
  detail?: string;
};

export type SheinStudioRecentBatchResult = {
  tone: "success" | "warning" | "danger";
  label: string;
  detail?: string;
};
