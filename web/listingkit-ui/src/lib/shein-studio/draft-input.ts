import { buildSelectionSummary } from "@/lib/shein-studio/storage-shared";
import type { SDSProductVariantSelection } from "@/lib/types/sds";
import type { GroupedSDSSelectionEligibility } from "@/lib/types/sds-baseline";
import type {
  SheinStudioArtworkModel,
  SheinStudioCreatedTask,
  SheinStudioGeneratedDesign,
  SheinStudioImageStrategy,
  SheinStudioProductImagePrompt,
  SheinStudioSelectedSDSImage,
  SheinStudioVariationIntensity,
} from "@/lib/types/shein-studio";
import type { SheinStudioSaveInput } from "@/lib/utils/shein-studio-batches";

type BuildSheinStudioDraftInputArgs = {
  prompt: string;
  styleCount: string;
  variationIntensity: SheinStudioVariationIntensity;
  productImageCount: string;
  productImagePrompt: string;
  productImagePrompts: SheinStudioProductImagePrompt[];
  artworkModel: SheinStudioArtworkModel;
  transparentBackground: boolean;
  sheinStoreId: string;
  imageStrategy: SheinStudioImageStrategy;
  selectedSdsImages: SheinStudioSelectedSDSImage[];
  renderSizeImagesWithSds: boolean;
  selection?: SDSProductVariantSelection;
  groupedSelections: GroupedSDSSelectionEligibility[];
  designs: SheinStudioGeneratedDesign[];
  selectedIds: string[];
  createdTasks: SheinStudioCreatedTask[];
};

export function buildSheinStudioDraftInput(
  args: BuildSheinStudioDraftInputArgs,
): SheinStudioSaveInput {
  return {
    prompt: args.prompt,
    styleCount: args.styleCount,
    variationIntensity: args.variationIntensity,
    productImageCount: args.productImageCount,
    productImagePrompt: args.productImagePrompt,
    productImagePrompts: args.productImagePrompts,
    artworkModel: args.artworkModel,
    transparentBackground: args.transparentBackground,
    sheinStoreId: args.sheinStoreId,
    imageStrategy: args.imageStrategy,
    selectedSdsImages: args.selectedSdsImages,
    renderSizeImagesWithSds: args.renderSizeImagesWithSds,
    selection: buildSelectionSummary(args.selection),
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
    designs: args.designs,
    selectedIds: args.selectedIds,
    createdTasks: args.createdTasks,
  };
}
