import type {
  SheinStudioGroupedWorkspace,
  SheinStudioSavedBatch,
} from "@/lib/types/shein-studio";
import type { SheinStudioSaveInput } from "@/lib/utils/shein-studio-batches";

function resetGroupedWorkspaceProgress(
  groups: SheinStudioSavedBatch["groups"],
): SheinStudioGroupedWorkspace[] | undefined {
  return groups?.map((group) => ({
    ...group,
    designs: [],
    selectedIds: [],
    createdTasks: [],
    legacyCompatibilitySnapshot: undefined,
  }));
}

export function buildDuplicatedSheinStudioBatchInput(
  batch: SheinStudioSavedBatch,
): SheinStudioSaveInput {
  return {
    id: undefined,
    name: `${batch.name} 副本`,
    prompt: batch.prompt,
    promptMode: batch.promptMode,
    styleCount: batch.styleCount,
    variationIntensity: batch.variationIntensity,
    productImageCount: batch.productImageCount,
    productImagePrompt: batch.productImagePrompt,
    productImagePrompts: batch.productImagePrompts,
    artworkModel: batch.artworkModel,
    transparentBackground: batch.transparentBackground,
    sheinStoreId: batch.sheinStoreId,
    imageStrategy: batch.imageStrategy,
    groupedImageMode: batch.groupedImageMode,
    selectedSdsImages: batch.selectedSdsImages,
    renderSizeImagesWithSds: batch.renderSizeImagesWithSds,
    selection: batch.selection,
    groupedSelections: batch.groupedSelections,
    groups: resetGroupedWorkspaceProgress(batch.groups),
    designs: [],
    selectedIds: [],
    createdTasks: [],
    generationJobs: [],
    generationError: "",
    generationJobId: "",
    batchStatus: "draft",
    legacyCompatibilitySnapshot: undefined,
  };
}
