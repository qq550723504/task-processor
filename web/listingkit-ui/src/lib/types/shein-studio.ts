import type { SDSProductVariantSelection } from "@/lib/types/sds";

export type SheinStudioGeneratedDesign = {
  id: string;
  dataUrl?: string;
  imageUrl?: string;
  revisedPrompt?: string;
  reviewNote?: string;
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
  sheinStoreId: string;
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
