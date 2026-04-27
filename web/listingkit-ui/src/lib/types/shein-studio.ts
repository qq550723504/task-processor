import type { SDSProductVariantSelection } from "@/lib/types/sds";

export type SheinStudioGeneratedDesign = {
  id: string;
  dataUrl?: string;
  imageUrl?: string;
  productImageUrls?: string[];
  revisedPrompt?: string;
  role?: string;
  roleLabel?: string;
  reviewNote?: string;
};

export type SheinStudioImageStrategy =
  | "ai_generated"
  | "sds_official"
  | "hybrid";

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

export type SheinStudioCreatedTask = {
  id: string;
  title: string;
  designId: string;
};

export type SheinStudioGenerateRequest = {
  prompt: string;
  count: number;
  printableWidth?: number;
  printableHeight?: number;
  productReferenceImageUrls?: string[];
};

export type SheinStudioGenerateResponse = {
  prompt: string;
  printableWidth?: number;
  printableHeight?: number;
  images: SheinStudioGeneratedDesign[];
};

export type SheinStudioSavedBatch = {
  id: string;
  name: string;
  prompt: string;
  styleCount: string;
  productImageCount?: string;
  productImagePrompt?: string;
  productImagePrompts?: SheinStudioProductImagePrompt[];
  sheinStoreId: string;
  imageStrategy?: SheinStudioImageStrategy;
  selectionVariantId?: number;
  selection?: SDSProductVariantSelection;
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
  updatedAt: string;
};

export type SheinStudioDraft = Omit<
  SheinStudioSavedBatch,
  "id" | "name" | "updatedAt"
> & {
  updatedAt: string;
};

export type SheinStudioStorageData = {
  draft: SheinStudioDraft | null;
  batches: SheinStudioSavedBatch[];
};
