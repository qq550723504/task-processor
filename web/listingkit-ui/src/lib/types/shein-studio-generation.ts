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

export type SheinStudioArtworkModel = string;
export type SheinStudioPromptMode = "managed" | "raw";
export type SheinStudioVariationIntensity = "light" | "medium" | "strong";

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

export type SheinStudioGenerateRequest = {
  prompt: string;
  artworkGenerationMode?: import("@/lib/types/shein-studio-draft").SheinStudioArtworkGenerationMode;
  promptMode?: SheinStudioPromptMode;
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

export type SheinStudioReferenceAnalysisRequest = {
  referenceImageUrls: string[];
  productName?: string;
  categoryPath?: string[];
  basePrompt?: string;
  userInstruction?: string;
};

export type SheinStudioReferenceAnalysisResponse = {
  referenceStyleBrief: string;
  sanitizedPrompt: string;
  warnings: string[];
};
