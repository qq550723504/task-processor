import { useCallback } from "react";

import { buildSheinStudioDraftInput } from "@/lib/shein-studio/draft-input";
import { saveLocalSheinStudioDraftSnapshot } from "@/lib/shein-studio/local-draft-cache";
import type {
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioGenerationJob,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioGroupedWorkspace,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";

type DraftInput = ReturnType<typeof buildSheinStudioDraftInput>;

type ResultBackedDraftOverrides = Partial<{
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
  generationJobs: SheinStudioGenerationJob[];
  generationError: string;
  generationJobId: string;
}>;

type BuildDraftInput = (overrides?: ResultBackedDraftOverrides) => DraftInput;

type DedicatedDraftPersistenceParams = {
  buildDraftInput: BuildDraftInput;
  createdTasks: SheinStudioCreatedTask[];
  currentGenerationJobId: string;
  designs: SheinStudioGeneratedDesign[];
  generationError: string;
  generationJobs: SheinStudioGenerationJob[];
  initialBatchId?: string;
  saveLocalSnapshot?: typeof saveLocalSheinStudioDraftSnapshot;
  selectedIds: string[];
};

type DraftPersistenceProjectionParams = {
  activeSelection?: SDSProductVariantSelection;
  artworkModel: SheinStudioArtworkModel;
  createdTasks: SheinStudioCreatedTask[];
  currentGenerationJobId?: string;
  designs: SheinStudioGeneratedDesign[];
  generationError: string;
  generationJobs: SheinStudioGenerationJob[];
  groups: SheinStudioGroupedWorkspace[];
  groupedImageMode: SheinStudioGroupedImageMode;
  groupedSelections: GroupedSDSSelectionEligibility[];
  imageStrategy: SheinStudioImageStrategy;
  isCreatingTasks: boolean;
  isGenerating: boolean;
  isLoadingWorkspace: boolean;
  persistedUpdatedAt: string;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  prompt: string;
  promptMode: "managed" | "raw";
  regeneratingId: string;
  renderSizeImagesWithSds: boolean;
  selectedIds: string[];
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  setDraftWarning: (value: string | ((current: string) => string)) => void;
  setPersistedUpdatedAt: (value: string) => void;
  sheinStoreId: string;
  styleCount: string;
  transparentBackground: boolean;
  variationIntensity: SheinStudioVariationIntensity;
};

const dedicatedBatchPromptOverrides = new Map<string, string>();

export function buildSheinStudioDraftPersistenceState({
  activeSelection,
  artworkModel,
  createdTasks,
  currentGenerationJobId,
  designs,
  generationError,
  generationJobs,
  groups,
  groupedImageMode,
  groupedSelections,
  imageStrategy,
  isCreatingTasks,
  isGenerating,
  isLoadingWorkspace,
  persistedUpdatedAt,
  productImageCount,
  productImagePrompt,
  productImagePrompts,
  prompt,
  promptMode,
  regeneratingId,
  renderSizeImagesWithSds,
  selectedIds,
  selectedSdsImages,
  setDraftWarning,
  setPersistedUpdatedAt,
  sheinStoreId,
  styleCount,
  transparentBackground,
  variationIntensity,
}: DraftPersistenceProjectionParams) {
  return {
    activeSelection,
    artworkModel,
    createdTasks,
    designs,
    generationError,
    generationJobId: currentGenerationJobId,
    generationJobs,
    groups,
    groupedImageMode,
    groupedSelections,
    imageStrategy,
    isCreatingTasks,
    isGenerating,
    isLoadingWorkspace,
    persistedUpdatedAt,
    productImageCount,
    productImagePrompt,
    productImagePrompts,
    prompt,
    promptMode,
    regeneratingId,
    renderSizeImagesWithSds,
    selectedIds,
    selectedSdsImages,
    setDraftWarning,
    setPersistedUpdatedAt,
    sheinStoreId,
    styleCount,
    transparentBackground,
    variationIntensity,
  };
}

export function resetDedicatedBatchPromptOverrides() {
  dedicatedBatchPromptOverrides.clear();
}

export function getDedicatedBatchPromptOverride(batchId?: string) {
  const resolvedBatchId = batchId?.trim();
  return resolvedBatchId
    ? dedicatedBatchPromptOverrides.get(resolvedBatchId)
    : undefined;
}

export function useSheinStudioDedicatedDraftPersistence({
  buildDraftInput,
  createdTasks,
  currentGenerationJobId,
  designs,
  generationError,
  generationJobs,
  initialBatchId,
  saveLocalSnapshot = saveLocalSheinStudioDraftSnapshot,
  selectedIds,
}: DedicatedDraftPersistenceParams) {
  const buildResultBackedDraftInput = useCallback(
    () =>
      buildDraftInput({
        designs,
        selectedIds,
        createdTasks,
        generationJobs,
        generationError,
        generationJobId: currentGenerationJobId,
      }),
    [
      buildDraftInput,
      createdTasks,
      currentGenerationJobId,
      designs,
      generationError,
      generationJobs,
      selectedIds,
    ],
  );

  const saveDedicatedBatchDraftSnapshot = useCallback(
    (overrides?: Partial<DraftInput>) => {
      if (!initialBatchId) {
        return;
      }
      if (typeof overrides?.prompt === "string") {
        dedicatedBatchPromptOverrides.set(initialBatchId, overrides.prompt);
      }
      saveLocalSnapshot(
        {
          ...buildDraftInput(),
          ...overrides,
        },
        {
          batchId: initialBatchId,
        },
      );
    },
    [buildDraftInput, initialBatchId, saveLocalSnapshot],
  );

  return {
    buildResultBackedDraftInput,
    promptOverride: getDedicatedBatchPromptOverride(initialBatchId),
    saveDedicatedBatchDraftSnapshot,
  };
}
