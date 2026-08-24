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
  originalImageUrl?: string;
  transparentBackgroundMode?: SheinStudioTransparencyMode;
  backgroundRemovalStatus?: SheinStudioBackgroundRemovalStatus;
  backgroundRemovalModel?: string;
  backgroundRemovalError?: string;
  variationIntensity?: SheinStudioVariationIntensity;
  role?: string;
  roleLabel?: string;
  reviewNote?: string;
  targetGroupKey?: string;
  targetGroupLabel?: string;
};

export type SheinStudioArtworkModel = string;
export type SheinStudioTransparencyMode = "none" | "native" | "removal";
 type SheinStudioBackgroundRemovalStatus =
  | "not_requested"
  | "pending"
  | "succeeded"
  | "failed";

export function resolveSheinStudioTransparencyMode({
  mode,
  transparentBackground,
}: {
  mode?: string;
  transparentBackground?: boolean;
}): SheinStudioTransparencyMode {
  if (mode === "native" || mode === "removal" || mode === "none") {
    return mode;
  }
  return transparentBackground ? "native" : "none";
}
export type SheinStudioPromptMode = "managed" | "raw";
export type SheinStudioVariationIntensity = "light" | "medium" | "strong";

 type SheinStudioGenerationJobStatus =
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
  transparentBackgroundMode?: SheinStudioTransparencyMode;
};

export type SheinStudioGenerateResponse = {
  prompt: string;
  printableWidth?: number;
  printableHeight?: number;
  imageModel?: SheinStudioArtworkModel | string;
  transparentBackground?: boolean;
  transparentBackgroundMode?: SheinStudioTransparencyMode;
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
