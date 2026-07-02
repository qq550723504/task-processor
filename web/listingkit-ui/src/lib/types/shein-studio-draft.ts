import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioArtworkModel,
  SheinStudioGeneratedDesign,
  SheinStudioGenerationJob,
  SheinStudioPromptMode,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio-generation";
import type { SheinStudioCreatedTask } from "@/lib/types/shein-studio-task";

export type SheinStudioImageStrategy =
  "ai_generated" | "sds_official" | "hybrid";

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
  promptMode?: SheinStudioPromptMode;
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
  hotStyleReferenceImageUrls?: string[];
  hotStyleReferenceBrief?: string;
  hotStyleReferencePrompt?: string;
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
  promptMode?: SheinStudioPromptMode;
  styleCount: string;
  hotStyleReferenceImageUrls?: string[];
  hotStyleReferenceBrief?: string;
  hotStyleReferencePrompt?: string;
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

export type SheinStudioSavedBatch = {
  id: string;
  tenantId?: string;
  name: string;
  prompt: string;
  promptMode?: SheinStudioPromptMode;
  styleCount: string;
  hotStyleReferenceImageUrls?: string[];
  hotStyleReferenceBrief?: string;
  hotStyleReferencePrompt?: string;
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
  promptMode?: SheinStudioPromptMode;
  styleCount: string;
  hotStyleReferenceImageUrls?: string[];
  hotStyleReferenceBrief?: string;
  hotStyleReferencePrompt?: string;
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
