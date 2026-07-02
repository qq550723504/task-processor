import { buildSelectionSummary } from "@/lib/shein-studio/storage-shared";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioGroupedWorkspace,
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioGenerationJob,
  SheinStudioGeneratedDesign,
  SheinStudioGroupedImageMode,
  SheinStudioImageStrategy,
  SheinStudioLegacyCompatibilitySnapshot,
  SheinStudioPersistedGroupedWorkspace,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";
import type { SheinStudioSaveInput } from "@/lib/utils/shein-studio-batches";

type BuildSheinStudioDraftInputArgs = {
  updatedAt?: string;
  prompt: string;
  promptMode?: "managed" | "raw";
  styleCount: string;
  variationIntensity: SheinStudioVariationIntensity;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  hotStyleReferenceImageUrls?: string[];
  hotStyleReferenceBrief?: string;
  hotStyleReferencePrompt?: string;
  artworkModel: SheinStudioArtworkModel;
  transparentBackground: boolean;
  sheinStoreId: string;
  imageStrategy: SheinStudioImageStrategy;
  groupedImageMode: SheinStudioGroupedImageMode;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  renderSizeImagesWithSds: boolean;
  selection?: SDSProductVariantSelection;
  groupedSelections: GroupedSDSSelectionEligibility[];
  groups?: SheinStudioGroupedWorkspace[];
  designs?: SheinStudioGeneratedDesign[];
  selectedIds?: string[];
  createdTasks?: SheinStudioCreatedTask[];
  generationJobs?: SheinStudioGenerationJob[];
  generationError?: string;
  generationJobId?: string;
  legacyCompatibilitySnapshot?: SheinStudioLegacyCompatibilitySnapshot;
};

function toPersistedGroupedWorkspace(
  group: SheinStudioGroupedWorkspace,
): SheinStudioPersistedGroupedWorkspace | null {
  const primarySelection = buildSelectionSummary(group.primarySelection);
  if (!primarySelection) {
    return null;
  }

  return {
    id: group.id,
    name: group.name,
    primarySelection,
    groupedSelections: group.groupedSelections
      .map((item) => {
        const selection = buildSelectionSummary(item.selection);
        if (!selection) {
          return null;
        }
        return {
          ...item,
          selection,
        };
      })
      .filter((item): item is NonNullable<typeof item> => Boolean(item)),
    styleCount: group.styleCount,
    promptMode: group.promptMode,
    sheinStoreId: group.sheinStoreId,
    imageStrategy: group.imageStrategy,
    groupedImageMode: group.groupedImageMode,
    selectedSdsImages: group.selectedSdsImages,
    renderSizeImagesWithSds: group.renderSizeImagesWithSds,
    currentPrompt: group.currentPrompt,
    promptHistory: group.promptHistory,
    productImageCount: group.productImageCount,
    productImagePrompt: group.productImagePrompt,
    productImagePrompts: group.productImagePrompts,
    hotStyleReferenceImageUrls: group.hotStyleReferenceImageUrls,
    hotStyleReferenceBrief: group.hotStyleReferenceBrief,
    hotStyleReferencePrompt: group.hotStyleReferencePrompt,
    artworkModel: group.artworkModel,
    transparentBackground: group.transparentBackground,
    variationIntensity: group.variationIntensity,
    legacyCompatibilitySnapshot: buildLegacyCompatibilitySnapshot({
      designs: group.designs,
      selectedIds: group.selectedIds,
      createdTasks: group.createdTasks,
      generationJobs: group.legacyCompatibilitySnapshot?.generationJobs,
      generationError: group.legacyCompatibilitySnapshot?.generationError,
      generationJobId: group.legacyCompatibilitySnapshot?.generationJobId,
      existing: group.legacyCompatibilitySnapshot,
    }),
    updatedAt: group.updatedAt,
  };
}

function buildLegacyCompatibilitySnapshot({
  designs,
  selectedIds,
  createdTasks,
  generationJobs,
  generationError,
  generationJobId,
  existing,
}: {
  designs?: SheinStudioGeneratedDesign[];
  selectedIds?: string[];
  createdTasks?: SheinStudioCreatedTask[];
  generationJobs?: SheinStudioGenerationJob[];
  generationError?: string;
  generationJobId?: string;
  existing?: SheinStudioLegacyCompatibilitySnapshot;
}): SheinStudioLegacyCompatibilitySnapshot | undefined {
  const nextDesigns = designs ?? existing?.designs ?? [];
  const nextSelectedIds = selectedIds ?? existing?.selectedIds ?? [];
  const nextCreatedTasks = createdTasks ?? existing?.createdTasks ?? [];
  const nextGenerationJobs = generationJobs ?? existing?.generationJobs ?? [];
  const nextGenerationError = generationError ?? existing?.generationError;
  const nextGenerationJobId = generationJobId ?? existing?.generationJobId;
  const hasDesigns = nextDesigns.length > 0;
  const hasSelectedIds = nextSelectedIds.length > 0;
  const hasCreatedTasks = nextCreatedTasks.length > 0;
  const hasGenerationJobs = nextGenerationJobs.length > 0;
  if (
    !hasDesigns &&
    !hasSelectedIds &&
    !hasCreatedTasks &&
    !hasGenerationJobs &&
    !nextGenerationError &&
    !nextGenerationJobId
  ) {
    return undefined;
  }
  return {
    designs: nextDesigns,
    selectedIds: nextSelectedIds,
    createdTasks: nextCreatedTasks,
    generationJobs: nextGenerationJobs,
    generationError: nextGenerationError,
    generationJobId: nextGenerationJobId,
  };
}

export function buildSheinStudioDraftInput(
  args: BuildSheinStudioDraftInputArgs,
): SheinStudioSaveInput {
  return {
    updatedAt: args.updatedAt,
    prompt: args.prompt,
    promptMode: args.promptMode,
    styleCount: args.styleCount,
    variationIntensity: args.variationIntensity,
    productImageCount: args.productImageCount,
    productImagePrompt: args.productImagePrompt,
    productImagePrompts: args.productImagePrompts,
    hotStyleReferenceImageUrls: args.hotStyleReferenceImageUrls ?? [],
    hotStyleReferenceBrief: args.hotStyleReferenceBrief ?? "",
    hotStyleReferencePrompt: args.hotStyleReferencePrompt ?? "",
    artworkModel: args.artworkModel,
    transparentBackground: args.transparentBackground,
    sheinStoreId: args.sheinStoreId,
    imageStrategy: args.imageStrategy,
    groupedImageMode: args.groupedImageMode,
    selectedSdsImages: args.selectedSdsImages,
    renderSizeImagesWithSds: args.renderSizeImagesWithSds,
    selection: buildSelectionSummary(args.selection),
    legacyCompatibilitySnapshot: buildLegacyCompatibilitySnapshot({
      designs: args.designs,
      selectedIds: args.selectedIds,
      createdTasks: args.createdTasks,
      generationJobs: args.generationJobs,
      generationError: args.generationError,
      generationJobId: args.generationJobId,
      existing: args.legacyCompatibilitySnapshot,
    }),
    groups: args.groups
      ?.map(toPersistedGroupedWorkspace)
      .filter((group): group is NonNullable<typeof group> => Boolean(group)),
    groupedSelections: args.groupedSelections
      .map((item) => {
        const selection = buildSelectionSummary(item.selection);
        if (!selection) {
          return null;
        }
        return {
          ...item,
          selection,
        };
      })
      .filter((item): item is NonNullable<typeof item> => Boolean(item)),
  };
}
