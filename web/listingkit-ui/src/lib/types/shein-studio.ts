import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type { SheinStudioCreatedTask } from "@/lib/types/shein-studio-task";

export type * from "@/lib/types/shein-studio-batch";
export type * from "@/lib/types/shein-studio-recent-batch";
export type * from "@/lib/types/shein-studio-task";

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

export type SheinStudioLegacyCompatibilitySnapshot = {
  designs?: SheinStudioGeneratedDesign[];
  selectedIds?: string[];
  createdTasks?: SheinStudioCreatedTask[];
  generationJobs?: SheinStudioGenerationJob[];
  generationError?: string;
  generationJobId?: string;
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
  legacyCompatibilitySnapshot?: SheinStudioLegacyCompatibilitySnapshot;
  updatedAt: string;
};

export type SheinStudioPersistedGroupedWorkspace = Omit<
  SheinStudioGroupedWorkspace,
  "designs" | "selectedIds" | "createdTasks"
> & {
  legacyCompatibilitySnapshot?: SheinStudioLegacyCompatibilitySnapshot;
};

export type SheinStudioPersistedBatchView = {
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
  groups?: SheinStudioPersistedGroupedWorkspace[];
  legacyCompatibilitySnapshot?: SheinStudioLegacyCompatibilitySnapshot;
  generationError?: string;
  generationJobId?: string;
  batchStatus?: string;
  draftUpdatedAt?: string;
  updatedAt: string;
};

export type SheinStudioPersistedDraft = SheinStudioPersistedBatchView;

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
  tenantId?: string;
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
  persistedDesignCount?: number;
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
  generationJobs?: SheinStudioGenerationJob[];
  legacyCompatibilitySnapshot?: SheinStudioLegacyCompatibilitySnapshot;
  generationError?: string;
  generationJobId?: string;
  batchStatus?: string;
  draftUpdatedAt?: string;
  updatedAt: string;
};

export type SheinStudioDraft = {
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
  legacyCompatibilitySnapshot?: SheinStudioLegacyCompatibilitySnapshot;
  generationError?: string;
  generationJobId?: string;
  batchStatus?: string;
  draftUpdatedAt?: string;
  updatedAt: string;
};

export type SheinStudioStorageData = {
  draft: SheinStudioDraft | null;
  batches: SheinStudioSavedBatch[];
};
