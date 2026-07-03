import { buildSelectionSummary } from "@/lib/shein-studio/storage-shared";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioGroupedWorkspace,
  SheinStudioArtworkModel,
  SheinStudioArtworkGenerationMode,
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
  artworkGenerationMode?: SheinStudioArtworkGenerationMode;
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

  const artworkInputs = resolveExclusiveArtworkInputs({
    artworkGenerationMode: group.artworkGenerationMode,
    prompt: group.currentPrompt,
    hotStyleReferenceImageUrls: group.hotStyleReferenceImageUrls,
    hotStyleReferenceBrief: group.hotStyleReferenceBrief,
    hotStyleReferencePrompt: group.hotStyleReferencePrompt,
  });

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
    artworkGenerationMode: artworkInputs.artworkGenerationMode,
    currentPrompt: artworkInputs.prompt,
    promptHistory: group.promptHistory,
    productImageCount: group.productImageCount,
    productImagePrompt: group.productImagePrompt,
    productImagePrompts: group.productImagePrompts,
    hotStyleReferenceImageUrls: artworkInputs.hotStyleReferenceImageUrls,
    hotStyleReferenceBrief: artworkInputs.hotStyleReferenceBrief,
    hotStyleReferencePrompt: artworkInputs.hotStyleReferencePrompt,
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

function hasHotStyleReferenceInput(input: {
  hotStyleReferenceImageUrls?: string[];
  hotStyleReferenceBrief?: string;
  hotStyleReferencePrompt?: string;
}) {
  return (
    (input.hotStyleReferenceImageUrls ?? []).some((value) => value.trim()) ||
    Boolean(input.hotStyleReferenceBrief?.trim()) ||
    Boolean(input.hotStyleReferencePrompt?.trim())
  );
}

function resolveArtworkGenerationMode(input: {
  artworkGenerationMode?: SheinStudioArtworkGenerationMode;
  hotStyleReferenceImageUrls?: string[];
  hotStyleReferenceBrief?: string;
  hotStyleReferencePrompt?: string;
}): SheinStudioArtworkGenerationMode {
  if (input.artworkGenerationMode) {
    return input.artworkGenerationMode;
  }
  return hasHotStyleReferenceInput(input) ? "hot_reference" : "theme_prompt";
}

function resolveExclusiveArtworkInputs(input: {
  artworkGenerationMode?: SheinStudioArtworkGenerationMode;
  prompt: string;
  hotStyleReferenceImageUrls?: string[];
  hotStyleReferenceBrief?: string;
  hotStyleReferencePrompt?: string;
}) {
  const artworkGenerationMode = resolveArtworkGenerationMode(input);
  if (artworkGenerationMode === "hot_reference") {
    return {
      artworkGenerationMode,
      prompt: "",
      hotStyleReferenceImageUrls: input.hotStyleReferenceImageUrls ?? [],
      hotStyleReferenceBrief: input.hotStyleReferenceBrief ?? "",
      hotStyleReferencePrompt: input.hotStyleReferencePrompt ?? "",
    };
  }
  return {
    artworkGenerationMode,
    prompt: input.prompt,
    hotStyleReferenceImageUrls: [],
    hotStyleReferenceBrief: "",
    hotStyleReferencePrompt: "",
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
  const artworkInputs = resolveExclusiveArtworkInputs(args);
  const groupedPrimarySelection = args.groups
    ?.map((group) => buildSelectionSummary(group.primarySelection))
    .find(Boolean);
  const primarySelection =
    buildSelectionSummary(args.selection) ?? groupedPrimarySelection;

  return {
    updatedAt: args.updatedAt,
    artworkGenerationMode: artworkInputs.artworkGenerationMode,
    prompt: artworkInputs.prompt,
    promptMode: args.promptMode,
    styleCount: args.styleCount,
    variationIntensity: args.variationIntensity,
    productImageCount: args.productImageCount,
    productImagePrompt: args.productImagePrompt,
    productImagePrompts: args.productImagePrompts,
    hotStyleReferenceImageUrls: artworkInputs.hotStyleReferenceImageUrls,
    hotStyleReferenceBrief: artworkInputs.hotStyleReferenceBrief,
    hotStyleReferencePrompt: artworkInputs.hotStyleReferencePrompt,
    artworkModel: args.artworkModel,
    transparentBackground: args.transparentBackground,
    sheinStoreId: args.sheinStoreId,
    imageStrategy: args.imageStrategy,
    groupedImageMode: args.groupedImageMode,
    selectedSdsImages: args.selectedSdsImages,
    renderSizeImagesWithSds: args.renderSizeImagesWithSds,
    selection: primarySelection,
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
